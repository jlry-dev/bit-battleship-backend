package store

import (
	"battleship-backend/game"
	"sync"
	"sync/atomic"
)

// Store manages all active game rooms.
type Store struct {
	mu           sync.RWMutex
	rooms        map[string]*game.GameRoom
	userCount    int64
	playerToRoom map[string]string // playerID -> roomID
}

func NewStore() *Store {
	return &Store{
		rooms:        make(map[string]*game.GameRoom),
		playerToRoom: make(map[string]string),
	}
}

// AddRoom adds a new room to the store.
func (s *Store) AddRoom(room *game.GameRoom) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rooms[room.ID] = room
}

// GetRoom retrieves a room by its ID.
func (s *Store) GetRoom(id string) (*game.GameRoom, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	room, ok := s.rooms[id]
	return room, ok
}

// DeleteRoom removes a room from the map completely
func (s *Store) DeleteRoom(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rooms, id)
}

// RoomCount returns the total number of active rooms.
func (s *Store) RoomCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.rooms)
}

func (s *Store) IncrementUsers() {
	atomic.AddInt64(&s.userCount, 1)
}

func (s *Store) DecrementUsers() {
	atomic.AddInt64(&s.userCount, -1)
}

func (s *Store) UserCount() int {
	return int(atomic.LoadInt64(&s.userCount))
}

// RegisterPlayerRoom maps a player to a room ID.
func (s *Store) RegisterPlayerRoom(playerID, roomID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playerToRoom[playerID] = roomID
}

// GetRoomByPlayer finds which room a player is sitting in
func (s *Store) GetRoomByPlayer(playerID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	roomID, ok := s.playerToRoom[playerID]
	return roomID, ok
}

// UnregisterPlayer removes the player from the mapping so we don't leak memory
func (s *Store) UnregisterPlayer(playerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.playerToRoom, playerID)
}
