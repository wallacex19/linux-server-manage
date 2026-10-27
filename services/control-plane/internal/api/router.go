package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/config"
	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/files"
	"github.com/linux-server-manager/backend/internal/monitoring"
	"github.com/linux-server-manager/backend/internal/nodes"
	"github.com/linux-server-manager/backend/internal/steamcmd"
	"github.com/linux-server-manager/backend/internal/systemmgr"
	"github.com/linux-server-manager/backend/internal/terminal"
)

// RouterOptions define dependências opcionais que podem ser injetadas explicitamente em testes
type RouterOptions struct {
	Authenticator auth.Authenticator
	SystemManager *systemmgr.SystemManager
	SteamCMD      *steamcmd.Service
}

// NewRouter cria o roteador oficial para execução do Control Plane em produção.
// Sempre utiliza os serviços e autenticadores reais de produção adequados ao sistema operacional.
// Rejeita configurações inválidas ou inexistentes com erro explícito.
func NewRouter(cfg *config.Config, db *database.DB) (*chi.Mux, error) {
	return NewRouterWithOptions(cfg, db, RouterOptions{})
}

// NewRouterWithOptions permite injeção explícita de dependências para testes e homologação controlada.
// Rejeita configurações inválidas com erro explícito.
func NewRouterWithOptions(cfg *config.Config, db *database.DB, opts RouterOptions) (*chi.Mux, error) {
	if cfg == nil {
		return nil, errors.New("configuração do servidor é obrigatória")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuração do servidor rejeitada: %w", err)
	}

	r := chi.NewRouter()

	// Middlewares essenciais e de segurança
	r.Use(middleware.RequestID)
	r.Use(NewTrustedProxyMiddleware(cfg))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS estrito e Proteção contra CSRF
	r.Use(corsMiddleware)
	r.Use(csrfProtectionMiddleware)

	authenticator := opts.Authenticator
	if authenticator == nil {
		authenticator = auth.NewDefaultAuthenticator()
	}
	sessionStore := auth.NewSessionStore(24 * time.Hour)

	// Inicializa coletor e sampler de métricas (60 amostras no histórico, coleta a cada 3s)
	collector := monitoring.NewCollector()
	historyBuffer := monitoring.NewHistoryBuffer(60)
	sampler := monitoring.NewSampler(collector, historyBuffer, 3*time.Second)
	sampler.Start(context.Background())

	// Inicializa o subsistema de servidores distribuídos (Multi-Server)
	publicURL := cfg.PublicURL
	if publicURL == "" {
		publicURL = "http://localhost:8080"
	}
	nodesSvc := nodes.NewService(db, publicURL)
	agentHub := nodes.NewAgentHub(nodesSvc)

	healthHandler := NewHealthHandler(db, cfg.Environment)
	authHandler := NewAuthHandler(authenticator, sessionStore, db, cfg.Environment == "production")
	systemHandler := NewSystemHandler()
	metricsHandler := NewMetricsHandler(historyBuffer, collector)
	fileService := files.NewFileService("")
	filesHandler := NewFilesHandler(fileService, db, agentHub, nodesSvc)
	tmuxService := terminal.NewTmuxService()
	terminalHandler := terminal.NewTerminalHandler(sessionStore, tmuxService, db)

	systemMgr := opts.SystemManager
	if systemMgr == nil {
		systemMgr = systemmgr.NewSystemManager()
	}
	systemMgrHandler := NewSystemManagerHandler(systemMgr, db, agentHub, nodesSvc)

	steamcmdSvc := opts.SteamCMD
	if steamcmdSvc == nil {
		steamcmdSvc = steamcmd.NewService(db, "local-node")
	}
	steamcmdHandler := NewSteamCMDHandler(steamcmdSvc, db)
	nodesHandler := NewNodesHandler(nodesSvc, agentHub, db)
	agentHandler := NewAgentHandler(nodesSvc, agentHub)

	// Rotas da API REST v1
	r.Route("/api/v1", func(r chi.Router) {
		// Rotas públicas
		r.Get("/health", healthHandler.Health)
		r.Get("/version", healthHandler.Version)
		r.Post("/auth/login", authHandler.Login)

		// Endpoints públicos de comunicação e pareamento com agentes remotos
		r.Post("/agent/pair", agentHandler.Pair)
		r.Get("/agent/connect", agentHandler.Connect)

		// Rotas protegidas (exigem autenticação do usuário Linux)
		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(sessionStore))

			r.Post("/auth/logout", authHandler.Logout)
			r.Get("/auth/me", authHandler.Me)
			r.Get("/system/info", systemHandler.GetInfo)

			// Gerenciamento de Processos e Serviços (Etapa 5)
			r.Get("/system/processes", systemMgrHandler.ListProcesses)
			r.Post("/system/processes/kill", systemMgrHandler.KillProcess)
			r.Get("/system/services", systemMgrHandler.ListServices)
			r.With(RequireAdmin).Post("/system/services/action", systemMgrHandler.ControlService)
			r.With(RequireAdmin).Get("/system/services/logs", systemMgrHandler.GetServiceLogs)

			// Métricas e monitoramento em tempo real
			r.Get("/metrics/current", metricsHandler.GetCurrent)
			r.Get("/metrics/history", metricsHandler.GetHistory)

			// Gerenciador de Arquivos (Etapa 3)
			r.Route("/files", func(r chi.Router) {
				r.Get("/list", filesHandler.List)
				r.Get("/read", filesHandler.Read)
				r.Post("/save", filesHandler.Save)
				r.Post("/mkdir", filesHandler.Mkdir)
				r.Post("/create", filesHandler.Create)
				r.Post("/rename", filesHandler.Rename)
				r.Delete("/delete", filesHandler.Delete)
				r.Get("/download", filesHandler.Download)
				r.Post("/upload", filesHandler.Upload)
			})

			// Terminal Interativo e Sessões Persistentes (Etapa 4)
			r.Route("/terminal", func(r chi.Router) {
				r.Get("/sessions", terminalHandler.ListSessions)
				r.Post("/sessions", terminalHandler.CreateSession)
				r.Delete("/sessions", terminalHandler.KillSession)
				r.Get("/ws", terminalHandler.HandleWebSocket)
			})

			// SteamCMD e Servidores de Jogos Dedicados (Etapa 6)
			r.Get("/steamcmd/status", steamcmdHandler.GetStatus)
			r.Get("/steamcmd/presets", steamcmdHandler.GetPresets)
			r.Route("/games/servers", func(r chi.Router) {
				r.Get("/", steamcmdHandler.ListServers)
				r.Post("/", steamcmdHandler.CreateServer)
				r.Delete("/{id}", steamcmdHandler.DeleteServer)
				r.Post("/{id}/start", steamcmdHandler.StartServer)
				r.Post("/{id}/stop", steamcmdHandler.StopServer)
				r.Post("/{id}/update", steamcmdHandler.UpdateServer)
			})

			// Gerenciamento Centralizado de Servidores Linux (Multi-Server)
			r.Route("/nodes", func(r chi.Router) {
				r.Get("/", nodesHandler.ListNodes)
				r.Post("/", nodesHandler.CreateNode)
				r.Get("/{id}", nodesHandler.GetNode)
				r.Put("/{id}", nodesHandler.UpdateNode)
				r.Post("/{id}/revoke", nodesHandler.RevokeNode)
				r.Delete("/{id}", nodesHandler.DeleteNode)
			})

			// Rotas administrativas (restritas a membros de sudo/wheel/root)
			r.Group(func(r chi.Router) {
				r.Use(RequireAdmin)
				// Endpoints administrativos futuros (Fase 2)
			})
		})
	})

	return r, nil
}

func isAllowedOrigin(origin, host string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Host, host) {
		return true
	}
	reqHost, _, errReq := net.SplitHostPort(host)
	if errReq != nil {
		reqHost = host
	}
	origHost, _, errOrig := net.SplitHostPort(u.Host)
	if errOrig != nil {
		origHost = u.Host
	}
	isLocalReq := reqHost == "localhost" || reqHost == "127.0.0.1" || reqHost == "::1"
	isLocalOrig := origHost == "localhost" || origHost == "127.0.0.1" || origHost == "::1"
	return isLocalReq && isLocalOrig
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if isAllowedOrigin(origin, r.Host) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else {
				// Origem não autorizada: não concede credenciais nem reflete
				w.Header().Set("Access-Control-Allow-Origin", "null")
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Requested-With, X-Node-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// csrfProtectionMiddleware previne ataques CSRF em requisições mutativas de estado
func csrfProtectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Métodos seguros e de leitura não sofrem restrição CSRF
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Valida Origin se presente
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !isAllowedOrigin(origin, r.Host) {
				http.Error(w, `{"error":"CORS/CSRF: origem não autorizada"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Fallback para Referer se Origin não for enviado
		referer := r.Header.Get("Referer")
		if referer != "" {
			u, err := url.Parse(referer)
			if err != nil || !isAllowedOrigin(u.Scheme+"://"+u.Host, r.Host) {
				http.Error(w, `{"error":"CORS/CSRF: referer não autorizado"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Clientes nativos e de linha de comando sem Origin/Referer são autorizados
		next.ServeHTTP(w, r)
	})
}
