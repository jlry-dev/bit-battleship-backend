package ws

import (
	"battleship-backend/game"
	"battleship-backend/matchmaking"
	"battleship-backend/metrics"
	"battleship-backend/models"
	"battleship-backend/store"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// allow all origins for WebSocket connections
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Handler struct {
	Store      *store.Store
	Matchmaker *matchmaking.Matchmaker
	Metrics    *metrics.Metrics
}

func (h *Handler) HandleWS(w http.ResponseWriter, r *http.Request) {
	// 1. upgrade the connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade to websocket: %v", err)
		return
	}

	// extract name from query params
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "Anonymous"
	}

	// 2. make a random ID for the player
	b := make([]byte, 8)
	rand.Read(b)
	playerID := hex.EncodeToString(b)

	// 3. create the player struct
	player := &models.Player{
		ID:           playerID,
		Name:         name,
		Conn:         conn,
		Send:         make(chan []byte, 256), // decent buffer so we don't block
		Disconnected: make(chan struct{}),
	}

	// 4. stats!
	h.Store.IncrementUsers()

	// 5. start the background writer
	go game.StartWritePump(player)

	log.Printf("player %s connected but NOT in queue yet", playerID)

	// Set up ping/pong and read deadline to prevent hanging connections
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 6. read pump (this blocks until they disconnect)
	for {
		var rawMsg []byte
		var msg models.Message

		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			// connection was likely closed by the client
			break
		}

		// metric bump
		h.Metrics.IncrementMessages()

		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			log.Printf("player %s sent garbage json: %v", playerID, err)
			continue
		}

		if msg.Type == "FIND_MATCH" {
			log.Printf("player %s joining queue", playerID)
			h.Matchmaker.Enqueue(player)
			continue
		}

		// figure out what room they're in
		roomID, ok := h.Store.GetRoomByPlayer(playerID)
		if !ok {
			// maybe they're still in queue or sending msgs too early
			continue
		}

		room, ok := h.Store.GetRoom(roomID)
		if !ok {
			// room no longer exists
			continue
		}

		// throw it in the room's inbox
		inbound := game.InboundMessage{
			PlayerID: playerID,
			Message:  msg,
		}

		// non-blocking write to inbox
		select {
		case room.Inbox <- inbound:
		default:
			log.Printf("room %s inbox is full", roomID)
		}
	}

	// 8. cleanup when they leave
	log.Printf("player %s disconnected", playerID)
	h.Store.DecrementUsers()
	close(player.Send) // this kills the write pump
	close(player.Disconnected) // signals the matchmaker that this player is dead

	// if they were in a room, we gotta tell the other guy
	roomID, ok := h.Store.GetRoomByPlayer(playerID)
	if ok {
		h.Store.UnregisterPlayer(playerID)
		room, ok := h.Store.GetRoom(roomID)
		if ok {
			// Tell the room this player disconnected so it can safely clean up
			inbound := game.InboundMessage{
				PlayerID: playerID,
				Message:  models.Message{Type: "PLAYER_DISCONNECTED"},
			}
			
			select {
			case room.Inbox <- inbound:
			default:
				log.Printf("room %s inbox full, could not send disconnect message", roomID)
				// If the inbox is somehow full and blocking, we forcefully close the room to prevent leak
				room.Close()
			}
	}
}
}