package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type GameStartPayload struct {
	RoomID       string `json:"room_id"`
	YourTurn     bool   `json:"your_turn"`
	OpponentName string `json:"opponent_name"`
}

type AttackResultPayload struct {
	YourTurn bool `json:"your_turn"`
}

// Hardcoded valid ship placement
var dummyShips = []map[string]any{
	{"name": "Carrier", "size": 5, "cells": []map[string]int{{"x": 0, "y": 0}, {"x": 1, "y": 0}, {"x": 2, "y": 0}, {"x": 3, "y": 0}, {"x": 4, "y": 0}}},
	{"name": "Battleship", "size": 4, "cells": []map[string]int{{"x": 0, "y": 1}, {"x": 1, "y": 1}, {"x": 2, "y": 1}, {"x": 3, "y": 1}}},
	{"name": "Cruiser", "size": 3, "cells": []map[string]int{{"x": 0, "y": 2}, {"x": 1, "y": 2}, {"x": 2, "y": 2}}},
	{"name": "Submarine", "size": 3, "cells": []map[string]int{{"x": 0, "y": 3}, {"x": 1, "y": 3}, {"x": 2, "y": 3}}},
	{"name": "Destroyer", "size": 2, "cells": []map[string]int{{"x": 0, "y": 4}, {"x": 1, "y": 4}}},
}

func main() {
	count := flag.Int("count", 100, "Number of concurrent connections to spawn")
	url := flag.String("url", "ws://localhost:8080/ws", "Websocket URL to connect to")
	delay := flag.Int("delay", 10, "Delay in milliseconds between spawning connections")
	flag.Parse()

	log.Printf("Starting stress test with %d concurrent connections...", *count)
	log.Printf("Target URL: %s", *url)

	var wg sync.WaitGroup
	connections := make([]*websocket.Conn, *count)
	var connMu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		<-interrupt
		log.Println("Interrupt received, closing connections...")
		cancel()

		connMu.Lock()
		for _, c := range connections {
			if c != nil {
				c.Close()
			}
		}
		connMu.Unlock()
	}()

	for i := 0; i < *count; i++ {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				botUrl := fmt.Sprintf("%s?name=StressBot_%d", *url, id)

				c, _, err := websocket.DefaultDialer.Dial(botUrl, nil)
				if err != nil {
					log.Printf("Bot %d failed to connect: %v", id, err)
					select {
					case <-ctx.Done():
						return
					case <-time.After(1 * time.Second):
					}
					continue
				}

				connMu.Lock()
				connections[id] = c
				connMu.Unlock()

				var writeMu sync.Mutex
				sendMsg := func(msgType string, payload any) {
					writeMu.Lock()
					defer writeMu.Unlock()
					msg := Message{Type: msgType, Payload: payload}
					if err := c.WriteJSON(msg); err != nil {
						log.Printf("Bot %d failed to send %s: %v", id, msgType, err)
					}
				}

				// 1. Join matchmaking
				sendMsg("FIND_MATCH", map[string]interface{}{})

				attackX, attackY := 0, 0

				fireNextAttack := func() {
					// simulate thinking time
					time.Sleep(100 * time.Millisecond)
					sendMsg("ATTACK", map[string]int{"x": attackX, "y": attackY})
					attackX++
					if attackX >= 10 {
						attackX = 0
						attackY++
					}
					if attackY >= 10 {
						attackY = 0 // should be over by now but wrap around just in case
					}
				}

				// Keep reading to consume packets and keep connection alive
				for {
					var msg Message
					err := c.ReadJSON(&msg)
					if err != nil {
						break
					}

					switch msg.Type {
					case "GAME_START":
						// Parse payload
						var payload GameStartPayload
						raw, _ := json.Marshal(msg.Payload)
						json.Unmarshal(raw, &payload)

						// Place ships
						sendMsg("PLACE_SHIPS", map[string]any{"ships": dummyShips})

						// If we go first, fire
						if payload.YourTurn {
							fireNextAttack()
						}

					case "ATTACK_RESULT":
						var payload AttackResultPayload
						raw, _ := json.Marshal(msg.Payload)
						json.Unmarshal(raw, &payload)

						if payload.YourTurn {
							fireNextAttack()
						}

					case "GAME_OVER", "OPPONENT_DISCONNECTED":
						// Game done, re-queue to simulate persistent load!
						log.Printf("Bot %d finished game, re-queueing", id)
						attackX, attackY = 0, 0
						sendMsg("FIND_MATCH", map[string]interface{}{})
					}
				}

				c.Close()
				connMu.Lock()
				connections[id] = nil
				connMu.Unlock()

				select {
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Second):
				}
			}
		}(i)

		// Small delay to prevent TCP port exhaustion / connection reset by peer
		select {
		case <-ctx.Done():
		case <-time.After(time.Duration(*delay) * time.Millisecond):
		}

		if i%100 == 0 && i > 0 {
			log.Printf("Spawned %d bots...", i)
		}
	}

	if ctx.Err() == nil {
		log.Printf("Successfully spawned all %d connections! Press Ctrl+C to stop.", *count)
	}
	wg.Wait()
}
