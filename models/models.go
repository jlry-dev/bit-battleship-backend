package models

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

type Player struct {
	ID           string
	Name         string
	Conn         *websocket.Conn
	Send         chan []byte // buffered channel for non-blocking writes
	Disconnected chan struct{}
}

// Ship represents a piece on the game board.
type Ship struct {
	Name  string `json:"name"`
	Size  int    `json:"size"`
	Cells []Cell `json:"cells"`
}

// Cell is a basic x/y coordinate on the grid
type Cell struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Board holds the grid state and the ships that are placed on it
type Board struct {
	Grid  [10][10]CellState
	Ships []Ship
}

type CellState int

const (
	Empty CellState = iota
	ShipCell
	Hit
	Miss
)

// GameState contains all information for an active match.
type GameState struct {
	RoomID     string
	Players    [2]*Player
	Boards     [2]*Board
	Turns      int // 0 for player 1, 1 for player 2
	Phase      GamePhase
	Winner     string
	StartedAt  time.Time
	FinishedAt *time.Time // pointer so it can be nil until game is actually done
}

type GamePhase int

const (
	PhaseWaiting GamePhase = iota
	PhasePlacement
	PhaseBattle
	PhaseOver
)

// Message is what we parse from incoming websockets
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// OutboundMessage is what we shoot back to the client
type OutboundMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// PlaceShipsPayload is for the initial setup phase
type PlaceShipsPayload struct {
	Ships []Ship `json:"ships"`
}

// AttackPayload contains the coordinates for an attack.
type AttackPayload struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// AttackResultPayload tells the client what happened after they clicked
type AttackResultPayload struct {
	X               int    `json:"x"`
	Y               int    `json:"y"`
	Hit             bool   `json:"hit"`
	Sunk            bool   `json:"sunk"`
	SunkShip        string `json:"sunk_ship,omitempty"`       // empty if we didn't sink anything
	SunkShipCells   []Cell `json:"sunk_ship_cells,omitempty"` // coordinates of the sunk ship
	YourTurn        bool   `json:"your_turn"`                 // is it your turn next?
	AttackedMyBoard bool   `json:"attacked_my_board"`         // did this attack hit YOUR board?
}

type GameStartPayload struct {
	RoomID       string `json:"room_id"`
	YourTurn     bool   `json:"your_turn"`
	OpponentName string `json:"opponent_name"`
}

type GameOverPayload struct {
	Winner string `json:"winner"`
	YouWon bool   `json:"you_won"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}

// MetricsPayload contains current system statistics.
type MetricsPayload struct {
	ActiveRooms    int   `json:"active_rooms"`
	ConnectedUsers int   `json:"connected_users"`
	Goroutines     int   `json:"goroutines"`
	QueueLength    int   `json:"queue_length"` // required for stress testing metrics
	MessagesPerSec int64 `json:"messages_per_sec"`
}
