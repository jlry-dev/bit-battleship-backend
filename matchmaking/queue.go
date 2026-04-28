package matchmaking

import (
	"battleship-backend/db"
	"battleship-backend/game"
	"battleship-backend/models"
	"battleship-backend/store"
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
)

func isAlive(p *models.Player) bool {
	select {
	case <-p.Disconnected:
		return false
	default:
		return true
	}
}

// Matchmaker handles player queueing and room creation.
type Matchmaker struct {
	queue      chan *models.Player // buffered channel size 100
	store      *store.Store
	db         *db.DB
	workers    int
	queuedIDs  map[string]bool // track who is already in queue
	queueMutex sync.Mutex
}

func NewMatchmaker(s *store.Store, d *db.DB, workers int) *Matchmaker {
	return &Matchmaker{
		queue:      make(chan *models.Player, 100),
		store:      s,
		db:         d,
		workers:    workers,
		queuedIDs:  make(map[string]bool),
	}
}

// Start spawns a single goroutine to pair people up.
// Multiple workers reading twice from the same channel causes them to steal players from each other!
func (m *Matchmaker) Start() {
	go func() {
		log.Println("matchmaker started")
		for {
			// block until we get 2 players
			p1 := <-m.queue
			p2 := <-m.queue

			// Verify connections are still alive before spinning up a room
			alive1 := isAlive(p1)
			alive2 := isAlive(p2)

			m.queueMutex.Lock()
			delete(m.queuedIDs, p1.ID)
			delete(m.queuedIDs, p2.ID)
			m.queueMutex.Unlock()

			if !alive1 && !alive2 {
				// both dead, do nothing
				log.Printf("Matchmaker: Both players disconnected while in queue. Discarding.")
				continue
			} else if !alive1 {
				log.Printf("Matchmaker: p1 disconnected, returning p2 to queue.")
				m.Enqueue(p2)
				continue
			} else if !alive2 {
				log.Printf("Matchmaker: p2 disconnected, returning p1 to queue.")
				m.Enqueue(p1)
				continue
			}

			// create a random room id (8 bytes, hex string)
			b := make([]byte, 8)
			_, err := rand.Read(b)
			if err != nil {
				log.Printf("crypto/rand failed: %v", err)
				continue
			}
			roomID := hex.EncodeToString(b)

				// spin up the room
				room := game.NewGameRoom(roomID, p1, p2, m.store, m.db)
				m.store.AddRoom(room)

				// tell the store which room they're in so we can route their messages
				m.store.RegisterPlayerRoom(p1.ID, roomID)
				m.store.RegisterPlayerRoom(p2.ID, roomID)

				// let them know they matched!
				game.SendToPlayer(p1, "GAME_START", models.GameStartPayload{
					RoomID:       roomID,
					YourTurn:     true, // p1 goes first
					OpponentName: p2.Name,
				})
				game.SendToPlayer(p2, "GAME_START", models.GameStartPayload{
					RoomID:       roomID,
					YourTurn:     false,
					OpponentName: p1.Name,
				})

				// fire up the room's event loop
				go room.Run()
			}
		}()
}

// Enqueue drops a player in the waiting line
func (m *Matchmaker) Enqueue(p *models.Player) {
	m.queueMutex.Lock()
	if m.queuedIDs[p.ID] {
		m.queueMutex.Unlock()
		log.Printf("player %s is already in queue", p.ID)
		return
	}
	m.queuedIDs[p.ID] = true
	m.queueMutex.Unlock()

	select {
	case m.queue <- p:
		log.Printf("player %s jumped in the matchmaking queue", p.ID)
	default:
		m.queueMutex.Lock()
		delete(m.queuedIDs, p.ID)
		m.queueMutex.Unlock()
		log.Printf("queue is full, dropping player %s", p.ID)
	}
}

// QueueLength tells us how many people are waiting (for the metrics page)
func (m *Matchmaker) QueueLength() int {
	return len(m.queue)
}
