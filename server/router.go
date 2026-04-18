package server

import (
	"battleship-backend/models"
	"battleship-backend/ws"
	"encoding/json"
	"net/http"
	"runtime"
)

// corsMiddleware slaps the right headers on so the frontend doesn't yell at us
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// setupRouter creates the mux and registers all our endpoints
func (s *Server) setupRouter() http.Handler {
	mux := http.NewServeMux()

	wsHandler := ws.Handler{
		Store:      s.store,
		Matchmaker: s.matchmaker,
		Metrics:    s.metrics,
	}

	// websocket endpoint
	mux.HandleFunc("/ws", wsHandler.HandleWS)

	// basic health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// metric endpoint for grading/monitoring
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		s.metrics.Mu.Lock()
		mps := s.metrics.MessagesPerSec
		s.metrics.Mu.Unlock()

		payload := models.MetricsPayload{
			ActiveRooms:    s.store.RoomCount(),
			ConnectedUsers: s.store.UserCount(),
			Goroutines:     runtime.NumGoroutine(), // built-in runtime check!
			QueueLength:    s.matchmaker.QueueLength(),
			MessagesPerSec: mps,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	})

	// stress test endpoint is exactly the same as metrics but I guess they wanted a specific path
	mux.HandleFunc("/stress-info", func(w http.ResponseWriter, r *http.Request) {
		s.metrics.Mu.Lock()
		mps := s.metrics.MessagesPerSec
		s.metrics.Mu.Unlock()

		payload := models.MetricsPayload{
			ActiveRooms:    s.store.RoomCount(),
			ConnectedUsers: s.store.UserCount(),
			Goroutines:     runtime.NumGoroutine(),
			QueueLength:    s.matchmaker.QueueLength(),
			MessagesPerSec: mps,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	})

	return corsMiddleware(mux)
}
