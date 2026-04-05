package game

import (
	"battleship-backend/models"
	"errors"
)

// NewBoard initializes a 10x10 empty grid.
func NewBoard() *models.Board {
	b := &models.Board{}
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			b.Grid[i][j] = models.Empty
		}
	}
	return b
}

// PlaceShips validates and places ships on the board.
func PlaceShips(board *models.Board, ships []models.Ship) error {
	if len(ships) != 5 {
		return errors.New("Exactly 5 ships must be placed")
	}

	// keep track of which ones we saw so we don't get 5 carriers
	expected := map[string]int{
		"Carrier":    5,
		"Battleship": 4,
		"Cruiser":    3,
		"Submarine":  3,
		"Destroyer":  2,
	}

	seen := make(map[string]bool)

	for _, s := range ships {
		size, ok := expected[s.Name]
		if !ok {
			return errors.New("invalid ship name: " + s.Name)
		}
		if size != s.Size || len(s.Cells) != s.Size {
			return errors.New("ship size is wrong for " + s.Name)
		}
		if seen[s.Name] {
			return errors.New(s.Name + " is already placed")
		}
		seen[s.Name] = true

		// check if it's contiguous and fits on the board
		// contiguous means either all x are same and y is sequential, or vice versa
		xSame := true
		ySame := true
		for i := 0; i < len(s.Cells); i++ {
			c := s.Cells[i]
			// bounds check! don't let them place ships off the map
			if c.X < 0 || c.X > 9 || c.Y < 0 || c.Y > 9 {
				return errors.New("ship is out of bounds")
			}
			// overlap check!
			if board.Grid[c.Y][c.X] != models.Empty {
				return errors.New("ships can't overlap each other")
			}

			if i > 0 {
				prev := s.Cells[i-1]
				if c.X != prev.X {
					xSame = false
				}
				if c.Y != prev.Y {
					ySame = false
				}
				// check if adjacent
				dx := c.X - prev.X
				dy := c.Y - prev.Y
				if (dx != 1 && dx != -1) && (dy != 1 && dy != -1) {
					return errors.New("ship cells must be contiguous")
				}
				if (dx == 1 || dx == -1) && (dy == 1 || dy == -1) {
					return errors.New("diagonal ships are illegal")
				}
			}
		}

		if !xSame && !ySame {
			return errors.New("ship is crooked")
		}

		// all good, mark it on the grid
		for _, c := range s.Cells {
			board.Grid[c.Y][c.X] = models.ShipCell
		}
	}

	// save them to the board state
	board.Ships = ships
	return nil
}

// Attack shoots at the board, returning if we hit, sunk something, and what it was
func Attack(board *models.Board, x, y int) (hit bool, sunk bool, sunkShip models.Ship, err error) {
	if x < 0 || x > 9 || y < 0 || y > 9 {
		return false, false, models.Ship{}, errors.New("attack out of bounds")
	}

	cellState := board.Grid[y][x]
	if cellState == models.Hit || cellState == models.Miss {
		return false, false, models.Ship{}, errors.New("Cell has already been attacked")
	}

	if cellState == models.ShipCell {
		board.Grid[y][x] = models.Hit
		hit = true
		
		// did we sink it?
		for _, s := range board.Ships {
			// find the ship that covers this cell
			partOfThisShip := false
			for _, c := range s.Cells {
				if c.X == x && c.Y == y {
					partOfThisShip = true
					break
				}
			}
			if partOfThisShip {
				if isSunk(board, s) {
					sunk = true
					sunkShip = s
				}
				break
			}
		}
	} else {
		board.Grid[y][x] = models.Miss
		hit = false
	}

	return hit, sunk, sunkShip, nil
}

// isSunk helper function
func isSunk(board *models.Board, ship models.Ship) bool {
	for _, c := range ship.Cells {
		if board.Grid[c.Y][c.X] != models.Hit {
			return false // still floating!
		}
	}
	return true
}

// AllSunk returns true if the game is over and this board got completely destroyed
func AllSunk(board *models.Board) bool {
	for _, s := range board.Ships {
		if !isSunk(board, s) {
			return false
		}
	}
	return true
}
