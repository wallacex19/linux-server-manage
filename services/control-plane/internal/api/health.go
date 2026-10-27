package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/linux-server-manager/backend/internal/database"
)

type HealthResponse struct {
	Status      string    `json:"status"`
	Timestamp   time.Time `json:"timestamp"`
	Environment string    `json:"environment"`
	Database    string    `json:"database"`
	Version     string    `json:"version"`
}

type HealthHandler struct {
	db  *database.DB
	env string
}

func NewHealthHandler(db *database.DB, env string) *HealthHandler {
	return &HealthHandler{
		db:  db,
		env: env,
	}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	dbStatus := "connected"
	if h.db != nil {
		if err := h.db.Ping(); err != nil {
			dbStatus = "unreachable"
		}
	} else {
		dbStatus = "not_configured"
	}

	resp := HealthResponse{
		Status:      "healthy",
		Timestamp:   time.Now().UTC(),
		Environment: h.env,
		Database:    dbStatus,
		Version:     "0.1.0",
	}

	w.Header().Set("Content-Type", "application/json")
	if dbStatus == "unreachable" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *HealthHandler) Version(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"name":    "Linux Server Manager - Backend Central",
		"version": "0.1.0",
	})
}
