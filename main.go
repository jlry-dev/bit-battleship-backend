package main

import (
	"battleship-backend/db"
	"battleship-backend/matchmaking"
	"battleship-backend/metrics"
	"battleship-backend/server"
	"battleship-backend/store"
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

// loadEnv parses the .env file.
func loadEnv() {
	b, err := os.ReadFile(".env")
	if err != nil {
		// totally fine if it doesn't exist, might be set in docker or whatever
		return
	}
	
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

func main() {
	// 1. load the env file
	loadEnv()

	// 2. read configs
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	
	workersStr := os.Getenv("MATCHMAKER_WORKERS")
	workers := 4
	if workersStr != "" {
		if w, err := strconv.Atoi(workersStr); err == nil {
			workers = w
		}
	}

	ctx := context.Background()

	// 3. spin up the database
	database, err := db.NewDB(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(ctx); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// 4. store for all the active games
	s := store.NewStore()

	// 5. metrics for the stress test page
	m := metrics.NewMetrics()
	m.StartTicker()

	// 6. matchmaker to pair people up
	mm := matchmaking.NewMatchmaker(s, database, workers)
	mm.Start()

	// 7. setup and start the http server
	srv := server.NewServer(s, mm, database, m)

	// we do this in a goroutine so we can catch the shutdown signal
	go func() {
		if err := srv.Start(":" + port); err != nil {
			log.Fatalf("server crashed: %v", err)
		}
	}()

	// 8. graceful shutdown setup
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// block until we get a signal (like ctrl+c)
	<-stop

	log.Println("shutting down...")
	// db defer will close the pool
}
