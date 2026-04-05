package game

import (
	"battleship-backend/models"
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// RoomStore is an interface to prevent import cycles with the store package.
type RoomStore interface {
	DeleteRoom(id string)
}

// DBSave is an interface for persisting the game state.
type DBSave interface {
	SaveGame(ctx context.Context, state *models.GameState) error
}

type InboundMessage struct {
	PlayerID string
	Message  models.Message
}

type GameRoom struct {
	ID        string
	State     *models.GameState
	Inbox     chan InboundMessage
	Done      chan struct{}
	closeOnce sync.Once
	store     RoomStore // using interface instead of *store.Store directly to prevent import cycle
	db        DBSave    // we need this to persist the result when the game is over
}

// NewGameRoom initializes a new game room.
func NewGameRoom(id string, p1, p2 *models.Player, s RoomStore, db DBSave) *GameRoom {
	return &GameRoom{
		ID:    id,
		State: NewGameState(id, p1, p2),
		Inbox: make(chan InboundMessage, 64), // buffer it so we don't block
		Done:  make(chan struct{}),
		store: s,
		db:    db,
	}
}

// Close safely shuts down the room, terminating the Run goroutine
func (r *GameRoom) Close() {
	r.closeOnce.Do(func() {
		close(r.Done)
	})
}

// Run executes the main event loop for the game room.
func (r *GameRoom) Run() {
	defer func() {
		// when we exit, tell the store to forget us
		r.store.DeleteRoom(r.ID)
		log.Printf("room %s has shut down", r.ID)
	}()

	log.Printf("room %s has started up with players %s and %s", r.ID, r.State.Players[0].ID, r.State.Players[1].ID)

	idleTimeout := time.NewTimer(10 * time.Minute)
	defer idleTimeout.Stop()

	for {
		select {
		case <-r.Done:
			// a player disconnected or the game ended
			return
		case msg := <-r.Inbox:
			if !idleTimeout.Stop() {
				select {
				case <-idleTimeout.C:
				default:
				}
			}
			idleTimeout.Reset(10 * time.Minute)
			r.handleMessage(msg)
		case <-idleTimeout.C:
			log.Printf("room %s idle timeout, shutting down", r.ID)
			r.Close()
		}
	}
}

// handleMessage routes our incoming socket JSON to the right place
func (r *GameRoom) handleMessage(msg InboundMessage) {
	switch msg.Message.Type {
	case "PLACE_SHIPS":
		r.handlePlaceShips(msg.PlayerID, msg.Message.Payload)
	case "ATTACK":
		r.handleAttack(msg.PlayerID, msg.Message.Payload)
	case "PLAYER_DISCONNECTED":
		r.handleDisconnect(msg.PlayerID)
	default:
		log.Printf("unknown message type %s from player %s in room %s", msg.Message.Type, msg.PlayerID, r.ID)
	}
}

func (r *GameRoom) handleDisconnect(playerID string) {
	// Find the opponent and tell them
	idx := PlayerIndex(r.State, playerID)
	if idx != -1 {
		opponentIdx := 1 - idx
		opponent := r.State.Players[opponentIdx]
		SendToPlayer(opponent, "OPPONENT_DISCONNECTED", models.ErrorPayload{
			Message: "Your opponent disconnected.",
		})
	}

	// Close the room! This terminates the Run() loop safely.
	r.Close()
}

func (r *GameRoom) handlePlaceShips(playerID string, raw json.RawMessage) {
	if r.State.Phase != models.PhaseWaiting {
		// ships can only be placed during the setup phase
		sendError(r, playerID, "You can only place ships during the setup phase")
		return
	}

	idx := PlayerIndex(r.State, playerID)
	if idx == -1 {
		return
	}

	var payload models.PlaceShipsPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		sendError(r, playerID, "bad json payload for PLACE_SHIPS")
		return
	}

	board := r.State.Boards[idx]
	if len(board.Ships) > 0 {
		sendError(r, playerID, "Ships are already placed.")
		return
	}

	if err := PlaceShips(board, payload.Ships); err != nil {
		sendError(r, playerID, err.Error())
		return
	}

	// see if both players are ready
	if BothPlayersPlaced(r.State) {
		SetPhase(r.State, models.PhaseBattle)
		
		// actually, need to send specific messages because YourTurn is different for p2
		p1 := r.State.Players[0]
		p2 := r.State.Players[1]
		
		SendToPlayer(p1, "GAME_START", models.GameStartPayload{
			RoomID:       r.State.RoomID,
			YourTurn:     true,
			OpponentName: p2.Name,
		})
		SendToPlayer(p2, "GAME_START", models.GameStartPayload{
			RoomID:       r.State.RoomID,
			YourTurn:     false,
			OpponentName: p1.Name,
		})
	}
}

func (r *GameRoom) handleAttack(playerID string, raw json.RawMessage) {
	if r.State.Phase != models.PhaseBattle {
		sendError(r, playerID, "you can only attack during the battle phase")
		return
	}

	idx := PlayerIndex(r.State, playerID)
	if idx == -1 {
		return
	}

	// is it actually their turn?
	if r.State.Turns != idx {
		sendError(r, playerID, "It is not your turn.")
		return
	}

	var payload models.AttackPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		sendError(r, playerID, "bad json payload for ATTACK")
		return
	}

	// attack the OTHER guy's board
	opponentIdx := 1 - idx
	opponentBoard := r.State.Boards[opponentIdx]

	hit, sunk, sunkShip, err := Attack(opponentBoard, payload.X, payload.Y)
	if err != nil {
		sendError(r, playerID, err.Error())
		return
	}

	// send the result to BOTH players so they can update their UI
	res := models.AttackResultPayload{
		X:             payload.X,
		Y:             payload.Y,
		Hit:           hit,
		Sunk:          sunk,
		SunkShip:      sunkShip.Name,
		SunkShipCells: sunkShip.Cells,
	}

	// turn switches only if we didn't get a hit (continuous turn on hit!)
	if !hit {
		SwitchTurn(r.State)
	}

	// player 1 gets the payload with YourTurn = (state.Turns == 0)
	res.YourTurn = (r.State.Turns == 0)
	res.AttackedMyBoard = (opponentIdx == 0)
	SendToPlayer(r.State.Players[0], "ATTACK_RESULT", res)

	res.YourTurn = (r.State.Turns == 1)
	res.AttackedMyBoard = (opponentIdx == 1)
	SendToPlayer(r.State.Players[1], "ATTACK_RESULT", res)

	// did they win?!
	if AllSunk(opponentBoard) {
		SetPhase(r.State, models.PhaseOver)
		r.State.Winner = playerID
		now := time.Now()
		r.State.FinishedAt = &now

		SendToPlayer(r.State.Players[0], "GAME_OVER", models.GameOverPayload{
			Winner: playerID,
			YouWon: r.State.Players[0].ID == playerID,
		})
		SendToPlayer(r.State.Players[1], "GAME_OVER", models.GameOverPayload{
			Winner: playerID,
			YouWon: r.State.Players[1].ID == playerID,
		})

		// attempt to save the game to the database
		if r.db != nil {
			go func() { // async so we don't block
				if err := r.db.SaveGame(context.Background(), r.State); err != nil {
					log.Printf("failed to save game to db: %v", err)
				}
			}()
		}

		// kill the room
		close(r.Done)
	}
}

// helper to send an error message to a single player
func sendError(r *GameRoom, playerID string, msg string) {
	idx := PlayerIndex(r.State, playerID)
	if idx != -1 {
		SendToPlayer(r.State.Players[idx], "ERROR", models.ErrorPayload{
			Message: msg,
		})
	}
}
