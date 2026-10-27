package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/database"
)

type AuthHandler struct {
	authenticator auth.Authenticator
	sessionStore  *auth.SessionStore
	rateLimiter   *auth.LoginRateLimiter
	db            *database.DB
	isSecureCookie bool
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string                  `json:"token"`
	User  *auth.LinuxUserIdentity `json:"user"`
}

func NewAuthHandler(authenticator auth.Authenticator, sessions *auth.SessionStore, db *database.DB, isProd bool) *AuthHandler {
	return &AuthHandler{
		authenticator:  authenticator,
		sessionStore:   sessions,
		rateLimiter:    auth.NewLoginRateLimiter(5, 0),
		db:             db,
		isSecureCookie: isProd,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON inválido"}`, http.StatusBadRequest)
		return
	}

	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = strings.Split(forwarded, ",")[0]
	}
	rateKey := clientIP + ":" + strings.ToLower(req.Username)

	if !h.rateLimiter.IsAllowed(rateKey) {
		h.logAudit(req.Username, "login_blocked_rate_limit", "auth", "", clientIP, "Tentativas excessivas")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Muitas tentativas de login falhas. Tente novamente em alguns minutos."})
		return
	}

	user, err := h.authenticator.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		h.rateLimiter.RecordFailure(rateKey)
		h.logAudit(req.Username, "login_failed", "auth", "", clientIP, "Credenciais inválidas")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Usuário ou senha Linux incorretos"})
		return
	}

	h.rateLimiter.RecordSuccess(rateKey)

	sess, err := h.sessionStore.Create(user)
	if err != nil {
		http.Error(w, `{"error":"falha ao criar sessão"}`, http.StatusInternalServerError)
		return
	}

	// Define cookie seguro HttpOnly
	http.SetCookie(w, &http.Cookie{
		Name:     "lsm_session",
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		Secure:   h.isSecureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	h.logAudit(user.Username, "login_success", "auth", user.Username, clientIP, "Login efetuado com sucesso")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(LoginResponse{
		Token: sess.Token,
		User:  user,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token != "" {
		h.sessionStore.Revoke(token)
	}

	// Remove cookie expirando-o
	http.SetCookie(w, &http.Cookie{
		Name:     "lsm_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	user := auth.UserFromContext(r.Context())
	username := "anonymous"
	if user != nil {
		username = user.Username
	}
	h.logAudit(username, "logout", "auth", "", r.RemoteAddr, "Sessão encerrada")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "sessão encerrada"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "não autenticado"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(user)
}

func (h *AuthHandler) logAudit(actor, action, resType, resID, ip, details string) {
	if h.db == nil {
		return
	}
	query := `INSERT INTO audit_logs (actor, action, resource_type, resource_id, ip_address, details) VALUES (?, ?, ?, ?, ?, ?)`
	_, _ = h.db.Exec(query, actor, action, resType, resID, ip, details)
}

func extractToken(r *http.Request) string {
	// 1. Tenta Cookie
	if cookie, err := r.Cookie("lsm_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	// 2. Tenta Authorization Bearer
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}
