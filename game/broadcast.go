package game

import (
	"battleship-backend/models"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// SendToPlayer serializes a message and tries to shove it into their send channel
func SendToPlayer(p *models.Player, msgType string, payload interface{}) {
	if p == nil || p.Send == nil {
		return
	}

	// recover from panics if we try to send on a closed channel
	// this happens during simultaneous stress test disconnects
	defer func() {
		if r := recover(); r != nil {
			log.Printf("caught panic sending to player %s (channel likely closed)", p.ID)
		}
	}()

	out := models.OutboundMessage{
		Type:    msgType,
		Payload: payload,
	}

	b, err := json.Marshal(out)
	if err != nil {
		log.Printf("failed to marshal message %s: %v", msgType, err)
		return
	}

	// non-blocking write to prevent slow clients from blocking the game loop
	select {
	case p.Send <- b:
	default:
		log.Printf("player %s send channel is full, dropping message", p.ID)
	}
}

// BroadcastToRoom sends a message to both players in a game state.
func BroadcastToRoom(state *models.GameState, msgType string, payload interface{}) {
	for _, p := range state.Players {
		SendToPlayer(p, msgType, payload)
	}
}

func StartWritePump(p *models.Player) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		p.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-p.Send:
			p.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// channel got closed, which means they disconnected
				// gracefully close the socket
				p.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := p.Conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Printf("write pump error for player %s: %v", p.ID, err)
				return
			}
		case <-ticker.C:
			p.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := p.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
