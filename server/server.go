package server

import (
	"battleship-backend/db"
	"battleship-backend/matchmaking"
	"battleship-backend/metrics"
	"battleship-backend/store"
	"log"
	"net/http"
)

// Server manages the HTTP endpoints and dependencies.
type Server struct {
	httpServer *http.Server
	store      *store.Store
	matchmaker *matchmaking.Matchmaker
	db         *db.DB
	metrics    *metrics.Metrics
}

// NewServer initializes the HTTP server with its dependencies.
func NewServer(store *store.Store, mm *matchmaking.Matchmaker, db *db.DB, m *metrics.Metrics) *Server {
	return &Server{
		store:      store,
		matchmaker: mm,
		db:         db,
		metrics:    m,
	}
}

// Start begins listening on the specified address.
func (s *Server) Start(addr string) error {
	mux := s.setupRouter()

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("server listening on %s", addr)
	return s.httpServer.ListenAndServe()
}
