package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/config"
)

// isTrustedProxy verifica se a conexão direta TCP provém do proxy reverso local (Caddy)
// ou de um proxy remoto explicitamente configurado em LSM_TRUSTED_PROXIES (IP ou CIDR).
func isTrustedProxy(remoteAddr string, cfg *config.Config) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host == "localhost"
	}
	if ip.IsLoopback() {
		return true
	}
	if cfg != nil {
		for _, trusted := range cfg.TrustedProxies {
			if strings.Contains(trusted, "/") {
				_, ipNet, err := net.ParseCIDR(trusted)
				if err == nil && ipNet.Contains(ip) {
					return true
				}
			} else {
				trustedIP := net.ParseIP(trusted)
				if trustedIP != nil && trustedIP.Equal(ip) {
					return true
				}
			}
		}
	}
	return false
}

// NewTrustedProxyMiddleware valida estritamente a cadeia de proxies reversos para auditoria e isolamento.
// Se a conexão direta vier de um proxy confiável (loopback ou LSM_TRUSTED_PROXIES), o cabeçalho X-Forwarded-For é avaliado.
// Se a conexão vier diretamente de um IP externo não autorizado em produção,
// o acesso direto é imediatamente bloqueado com 403 Forbidden para garantir o isolamento da porta administrativa.
// NOTA DE SEGURANÇA: O IP do cliente é utilizado exclusivamente para auditoria e logs,
// NUNCA como substituto de autenticação ou autorização.
func NewTrustedProxyMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			trusted := isTrustedProxy(r.RemoteAddr, cfg)
			if trusted {
				// Proxy confiável detectado; analisa cabeçalho X-Forwarded-For para logs
				xff := r.Header.Get("X-Forwarded-For")
				if xff != "" {
					parts := strings.Split(xff, ",")
					clientIP := strings.TrimSpace(parts[0])
					if parsed := net.ParseIP(clientIP); parsed != nil {
						_, port, _ := net.SplitHostPort(r.RemoteAddr)
						if port != "" {
							r.RemoteAddr = net.JoinHostPort(clientIP, port)
						} else {
							r.RemoteAddr = clientIP
						}
					}
				} else if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
					if parsed := net.ParseIP(xrip); parsed != nil {
						_, port, _ := net.SplitHostPort(r.RemoteAddr)
						if port != "" {
							r.RemoteAddr = net.JoinHostPort(xrip, port)
						} else {
							r.RemoteAddr = xrip
						}
					}
				}
			} else {
				// Conexão externa direta ao backend
				// Em ambiente de produção sem override explícito, bloqueia imediatamente o acesso direto não roteado pelo Caddy
				if cfg != nil && cfg.Environment == "production" && !cfg.AllowExternalListen {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error": "acesso direto externo ao backend proibido; o tráfego deve ser roteado pelo proxy reverso Caddy",
					})
					return
				}
			}

			// Se a conexão não vier de proxy confiável (em dev/test), X-Forwarded-For é ignorado,
			// mantendo o IP TCP direto inalterado e prevenindo spoofing.
			next.ServeHTTP(w, r)
		})
	}
}

// TrustedProxyMiddleware mantém compatibilidade com chamadores legados sem injeção de config
func TrustedProxyMiddleware(next http.Handler) http.Handler {
	return NewTrustedProxyMiddleware(nil)(next)
}

func RequireAuth(sessions *auth.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "sessão não autenticada"})
				return
			}

			sess, ok := sessions.Get(token)
			if !ok || sess.User == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "sessão expirada ou inválida"})
				return
			}

			ctx := auth.WithUser(r.Context(), sess.User)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.UserFromContext(r.Context())
		if user == nil || !user.IsAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "acesso restrito a administradores autorizados"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
