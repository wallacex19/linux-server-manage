package terminal

import "time"

type PersistentSession struct {
	Name         string    `json:"name"`
	ID           string    `json:"id"`
	WindowsCount int       `json:"windows_count"`
	CreatedAt    time.Time `json:"created_at"`
	IsAttached   bool      `json:"is_attached"`
	IsGameServer bool      `json:"is_game_server"`
	GameName     string    `json:"game_name,omitempty"`
}

type CreateSessionRequest struct {
	Name     string `json:"name"`
	GameName string `json:"game_name,omitempty"`
}

type WindowResizeMsg struct {
	Type string `json:"type"` // "resize"
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}
