package game

import (
	"battleship-backend/models"
	"time"
)

// NewGameState creates a fresh game with 2 players that just matched
func NewGameState(roomID string, p1, p2 *models.Player) *models.GameState {
	return &models.GameState{
		RoomID:    roomID,
		Players:   [2]*models.Player{p1, p2},
		Boards:    [2]*models.Board{NewBoard(), NewBoard()},
		Turns:     0, // p1 goes first usually
		Phase:     models.PhaseWaiting,
		StartedAt: time.Now(),
	}
}

// SetPhase transitions the game to a new phase
func SetPhase(state *models.GameState, phase models.GamePhase) {
	state.Phase = phase
}

// BothPlayersPlaced returns true if both players have their fleets setup
func BothPlayersPlaced(state *models.GameState) bool {
	// a bit hacky but if both boards have ships, we assume they placed them
	// since placing 0 ships isn't allowed anyway
	return len(state.Boards[0].Ships) > 0 && len(state.Boards[1].Ships) > 0
}

// PlayerIndex tells us if a user is player 0 or 1
// Returns -1 if they aren't in this room (which shouldn't happen but whatever)
func PlayerIndex(state *models.GameState, playerID string) int {
	if state.Players[0].ID == playerID {
		return 0
	}
	if state.Players[1].ID == playerID {
		return 1
	}
	return -1
}

// SwitchTurn flips the turn index so the other guy can shoot
func SwitchTurn(state *models.GameState) {
	if state.Turns == 0 {
		state.Turns = 1
	} else {
		state.Turns = 0
	}
}
