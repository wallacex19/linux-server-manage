package systemmgr

type ProcessInfo struct {
	PID           int     `json:"pid"`
	User          string  `json:"user"`
	UID           int     `json:"uid"`
	Name          string  `json:"name"`
	Command       string  `json:"command"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryBytes   uint64  `json:"memory_bytes"`
	MemoryPercent float64 `json:"memory_percent"`
	State         string  `json:"state"`
}

type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LoadState   string `json:"load_state"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
	CanControl  bool   `json:"can_control"`
}

type ServiceActionRequest struct {
	Name   string `json:"name"`
	Action string `json:"action"` // "start", "stop", "restart"
}

type KillProcessRequest struct {
	PID    int    `json:"pid"`
	Signal string `json:"signal"` // "SIGTERM", "SIGKILL"
}
