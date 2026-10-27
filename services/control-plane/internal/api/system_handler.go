package api

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"
)

var serverStartTime = time.Now()

type SystemInfoResponse struct {
	Hostname      string `json:"hostname"`
	OS            string `json:"os"`
	Architecture  string `json:"architecture"`
	CPUCores      int    `json:"cpu_cores"`
	GoVersion     string `json:"go_version"`
	UptimeSeconds uint64 `json:"uptime_seconds"`
}

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

func (s *SystemHandler) GetInfo(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	uptime := uint64(time.Since(serverStartTime).Seconds())

	resp := SystemInfoResponse{
		Hostname:      hostname,
		OS:            runtime.GOOS,
		Architecture:  runtime.GOARCH,
		CPUCores:      runtime.NumCPU(),
		GoVersion:     runtime.Version(),
		UptimeSeconds: uptime,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
