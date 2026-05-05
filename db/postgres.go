package db

import (
	"battleship-backend/models"
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// NewDB establishes a connection to the PostgreSQL database.
func NewDB(ctx context.Context, connString string) (*DB, error) {
	// pgxpool is used to provide connection pooling and better concurrency support.
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}

	// Verify the connection.
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	log.Printf("connected to the database")
	return &DB{pool: pool}, nil
}

// Migrate initializes the database schema.
func (d *DB) Migrate(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS games (
		id          TEXT PRIMARY KEY,
		winner_id   TEXT,
		player1_id  TEXT NOT NULL,
		player2_id  TEXT NOT NULL,
		move_count  INTEGER,
		started_at  TIMESTAMPTZ,
		finished_at TIMESTAMPTZ
	);`

	_, err := d.pool.Exec(ctx, query)
	if err != nil {
		log.Printf("failed to run migration: %v", err)
		return err
	}
	log.Printf("db migration done")
	return nil
}

// SaveGame persists the match result to the database.
func (d *DB) SaveGame(ctx context.Context, state *models.GameState) error {
	query := `
	INSERT INTO games (id, winner_id, player1_id, player2_id, move_count, started_at, finished_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	// move count isn't explicitly tracked in state right now
	moves := 0
	for _, b := range state.Boards {
		for i := 0; i < 10; i++ {
			for j := 0; j < 10; j++ {
				if b.Grid[i][j] == models.Hit || b.Grid[i][j] == models.Miss {
					moves++
				}
			}
		}
	}

	_, err := d.pool.Exec(ctx, query,
		state.RoomID,
		state.Winner,
		state.Players[0].ID,
		state.Players[1].ID,
		moves,
		state.StartedAt,
		state.FinishedAt,
	)

	if err != nil {
		log.Printf("failed to save game %s: %v", state.RoomID, err)
		return err
	}

	log.Printf("saved game %s successfully!", state.RoomID)
	return nil
}

// Close cleans up the pool when we shut down.
func (d *DB) Close() {
	if d.pool != nil {
		d.pool.Close()
	}
}
