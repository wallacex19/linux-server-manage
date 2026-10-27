package nodes

import "time"

type NodeStatus string

const (
	StatusOnline     NodeStatus = "online"
	StatusOffline    NodeStatus = "offline"
	StatusConnecting NodeStatus = "connecting"
	StatusUnpaired   NodeStatus = "unpaired"
	StatusRevoked    NodeStatus = "revoked"
	StatusError      NodeStatus = "error"
)

type Node struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Hostname         string              `json:"hostname"`
	IPAddress        string              `json:"ip_address"`
	PublicKey        string              `json:"public_key,omitempty"`
	Status           NodeStatus          `json:"status"`
	GroupName        string              `json:"group_name"`
	Tags             string              `json:"tags"`
	AgentVersion     string              `json:"agent_version"`
	PairingToken     string              `json:"pairing_token,omitempty"`
	PairingExpiresAt *time.Time          `json:"pairing_expires_at,omitempty"`
	RevokedAt        *time.Time          `json:"revoked_at,omitempty"`
	LastHeartbeat    *time.Time          `json:"last_heartbeat,omitempty"`
	LastError        string              `json:"last_error,omitempty"`
	OSInfo           string              `json:"os_info"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	InstallCommand   string              `json:"install_command,omitempty"`
	Metrics          *NodeMetricsSummary `json:"metrics,omitempty"`
}

type NodeMetricsSummary struct {
	CPUPercent       float64   `json:"cpu_percent"`
	MemoryPercent    float64   `json:"memory_percent"`
	DiskPercent      float64   `json:"disk_percent"`
	ProcessCount     int       `json:"process_count"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	GameServersCount int       `json:"game_servers_count"`
	CollectedAt      time.Time `json:"collected_at"`
	IsStale          bool      `json:"is_stale"`
}

type CreateNodeRequest struct {
	Name      string `json:"name"`
	GroupName string `json:"group_name"`
	Tags      string `json:"tags"`
}

type UpdateNodeRequest struct {
	Name      string `json:"name"`
	GroupName string `json:"group_name"`
	Tags      string `json:"tags"`
}

type PairAgentRequest struct {
	NodeID       string `json:"node_id,omitempty"`
	PairingToken string `json:"pairing_token"`
	PublicKey    string `json:"public_key"`
	Signature    string `json:"signature"`
	Hostname     string `json:"hostname"`
	IPAddress    string `json:"ip_address"`
	OSInfo       string `json:"os_info"`
	AgentVersion string `json:"agent_version"`
}

type AgentPairResponse struct {
	NodeID     string `json:"node_id"`
	AgentToken string `json:"agent_token"`
	PanelURL   string `json:"panel_url"`
}

type NodeEventLog struct {
	ID           int64     `json:"id"`
	NodeID       string    `json:"node_id"`
	Event        string    `json:"event"`
	IPAddress    string    `json:"ip_address"`
	ErrorMessage string    `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}
