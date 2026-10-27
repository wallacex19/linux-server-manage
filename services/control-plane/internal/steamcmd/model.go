package steamcmd

import "time"

type SteamCMDStatus struct {
	Installed       bool   `json:"installed"`
	Path            string `json:"path"`
	Version         string `json:"version"`
	Libs32BitReady  bool   `json:"libs_32bit_ready"`
	OsDistribution  string `json:"os_distribution"`
	InstallGuidance string `json:"install_guidance"`
}

type GameServer struct {
	ID           string    `json:"id"`
	NodeID       string    `json:"node_id"`
	Name         string    `json:"name"`
	ServiceType  string    `json:"service_type"`
	AppID        string    `json:"app_id"`
	InstallPath  string    `json:"install_path"`
	SystemUser   string    `json:"system_user"`
	SessionName  string    `json:"session_name"`
	Status       string    `json:"status"` // "stopped", "running", "updating", "error"
	AutoRestart  bool      `json:"auto_restart"`
	StartCommand string    `json:"start_command"`
	Port         int       `json:"port"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateServerRequest struct {
	Name         string `json:"name"`
	ServiceType  string `json:"service_type"`
	AppID        string `json:"app_id"`
	InstallPath  string `json:"install_path"`
	StartCommand string `json:"start_command"`
	Port         int    `json:"port"`
	AutoRestart  bool   `json:"auto_restart"`
}

type InstallUpdateRequest struct {
	Validate bool   `json:"validate"`
	Beta     string `json:"beta,omitempty"`
}

type PopularGamePreset struct {
	Name         string `json:"name"`
	AppID        string `json:"app_id"`
	ServiceType  string `json:"service_type"`
	DefaultPort  int    `json:"default_port"`
	DefaultArgs  string `json:"default_args"`
	Anonymous    bool   `json:"anonymous"`
	Description  string `json:"description"`
}

var PopularPresets = []PopularGamePreset{
	{
		Name:        "Counter-Strike 2",
		AppID:       "730",
		ServiceType: "cs2",
		DefaultPort: 27015,
		DefaultArgs: "./game/bin/linuxsteamrt64/cs2 -dedicated +map de_dust2",
		Anonymous:   true,
		Description: "Servidor dedicado oficial Valve Source 2",
	},
	{
		Name:        "Team Fortress 2",
		AppID:       "232250",
		ServiceType: "tf2",
		DefaultPort: 27015,
		DefaultArgs: "./srcds_run -game tf +map ctf_2fort +maxplayers 24",
		Anonymous:   true,
		Description: "Servidor clássico Valve multiplayer",
	},
	{
		Name:        "Valheim",
		AppID:       "896660",
		ServiceType: "valheim",
		DefaultPort: 2456,
		DefaultArgs: "./valheim_server.x86_64 -name 'Meu Valheim' -port 2456 -world 'Mundo1' -password 'senha123'",
		Anonymous:   true,
		Description: "Servidor cooperativo de sobrevivência viking",
	},
	{
		Name:        "Palworld",
		AppID:       "2394010",
		ServiceType: "palworld",
		DefaultPort: 8211,
		DefaultArgs: "./PalServer.sh -port=8211 -players=32",
		Anonymous:   true,
		Description: "Servidor dedicado de mundo aberto",
	},
	{
		Name:        "Project Zomboid",
		AppID:       "380870",
		ServiceType: "zomboid",
		DefaultPort: 16261,
		DefaultArgs: "./start-server.sh -servername MeuServidor",
		Anonymous:   true,
		Description: "Sobrevivência zumbi multiplayer isométrica",
	},
	{
		Name:        "Rust",
		AppID:       "258550",
		ServiceType: "rust",
		DefaultPort: 28015,
		DefaultArgs: "./RustDedicated -batchmode +server.port 28015 +server.level 'Procedural Map'",
		Anonymous:   true,
		Description: "Servidor de sobrevivência e PvP",
	},
}
