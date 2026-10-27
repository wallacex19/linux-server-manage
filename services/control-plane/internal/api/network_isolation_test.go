package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linux-server-manager/backend/internal/api"
	"github.com/linux-server-manager/backend/internal/config"
	"github.com/linux-server-manager/backend/internal/database"
)

func setupTestRouter(t *testing.T, env string, allowExternal bool) (*config.Config, *database.DB, http.Handler) {
	t.Helper()
	dbPath := t.TempDir() + "/test_net_iso.db"
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao conectar ao banco de teste: %v", err)
	}

	cfg := &config.Config{
		Host:                "127.0.0.1",
		Port:                8080,
		DatabasePath:        dbPath,
		Environment:         env,
		PublicURL:           "https://painel.exemplo.com",
		AllowExternalListen: allowExternal,
	}

	router, err := api.NewRouter(cfg, db)
	if err != nil {
		t.Fatalf("falha ao criar router: %v", err)
	}
	return cfg, db, router
}

// 1. Prova que uma requisição externa direta ao backend em produção é bloqueada e não alcança endpoints
func TestNetworkIsolation_DirectExternalRequest_BlockedInProduction(t *testing.T) {
	_, db, router := setupTestRouter(t, "production", false)
	defer db.Close()

	// Simula cliente conectando diretamente a partir de um IP público externo (não-loopback)
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	req.RemoteAddr = "198.51.100.25:45678" // IP público arbitrário

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado HTTP 403 Forbidden para requisição externa direta, obtido %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("resposta de erro não é JSON válido: %v", err)
	}
	if resp["error"] == "" {
		t.Fatalf("esperada mensagem de erro detalhada, obtido: %v", resp)
	}
}

// 2. Prova que o tráfego roteado via Caddy (loopback) é aceito normalmente
func TestNetworkIsolation_TrafficThroughCaddy_Accepted(t *testing.T) {
	_, db, router := setupTestRouter(t, "production", false)
	defer db.Close()

	// Requisição originada pelo Caddy na mesma máquina via IPv4 loopback
	reqIPv4 := httptest.NewRequest("GET", "/api/v1/health", nil)
	reqIPv4.RemoteAddr = "127.0.0.1:52134"
	recIPv4 := httptest.NewRecorder()
	router.ServeHTTP(recIPv4, reqIPv4)

	if recIPv4.Code != http.StatusOK {
		t.Fatalf("esperado HTTP 200 para tráfego loopback IPv4 do Caddy, obtido %d", recIPv4.Code)
	}

	// Requisição originada pelo Caddy via IPv6 loopback
	reqIPv6 := httptest.NewRequest("GET", "/api/v1/health", nil)
	reqIPv6.RemoteAddr = "[::1]:52134"
	recIPv6 := httptest.NewRecorder()
	router.ServeHTTP(recIPv6, reqIPv6)

	if recIPv6.Code != http.StatusOK {
		t.Fatalf("esperado HTTP 200 para tráfego loopback IPv6 do Caddy, obtido %d", recIPv6.Code)
	}
}

// 3. Prova que cabeçalhos X-Forwarded-For e X-Real-IP enviados diretamente por clientes externos não-confiáveis não alteram o IP
func TestNetworkIsolation_UntrustedClient_SpoofedHeadersIgnored(t *testing.T) {
	// Em modo development ou com external listen permitido, requisições diretas não alteram IP de auditoria
	var seenRemoteAddr string
	testHandler := api.NewTrustedProxyMiddleware(&config.Config{
		Environment:         "development",
		AllowExternalListen: true,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRemoteAddr = r.RemoteAddr
		w.WriteHeader(http.StatusOK)
	}))

	// Cliente externo não-confiável tentando forjar que é loopback ou IP de administrador
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	req.RemoteAddr = "203.0.113.10:39201"
	req.Header.Set("X-Forwarded-For", "127.0.0.1, 10.0.0.1")
	req.Header.Set("X-Real-IP", "127.0.0.1")

	rec := httptest.NewRecorder()
	testHandler.ServeHTTP(rec, req)

	// O RemoteAddr visto pela aplicação NÃO deve ter sido substituído por 127.0.0.1
	if seenRemoteAddr == "127.0.0.1" || seenRemoteAddr == "127.0.0.1:39201" {
		t.Fatalf("vulnerabilidade de spoofing detectada: seenRemoteAddr foi alterado para %s a partir de cliente não confiável", seenRemoteAddr)
	}
	if seenRemoteAddr != "203.0.113.10:39201" {
		t.Fatalf("esperado que seenRemoteAddr permaneça 203.0.113.10:39201, obtido %s", seenRemoteAddr)
	}
}

// 4. Prova que autenticação e autorização são estritamente independentes do IP recebido
func TestNetworkIsolation_AuthAndPermissions_IndependentOfIP(t *testing.T) {
	_, db, router := setupTestRouter(t, "development", true)
	defer db.Close()

	// Tentativa de acessar endpoint protegido sem credenciais, mesmo simulando vir de 127.0.0.1 via XFF
	req := httptest.NewRequest("GET", "/api/v1/system/info", nil)
	req.RemoteAddr = "127.0.0.1:40001"
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Deve ser rejeitado por falta de autenticação (401), comprovando que ser loopback/IP local não concede privilégio algum
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 Unauthorized para endpoint protegido sem token (IP não deve conceder privilégios), obtido %d", rec.Code)
	}
}

// 5. Prova que o cabeçalho X-Forwarded-For fornecido legitimamente pelo Caddy é processado para auditoria
func TestNetworkIsolation_CaddyForwardedIP_ProcessedForAuditing(t *testing.T) {
	// Inspeciona o que os handlers internos recebem
	var seenRemoteAddr string
	testHandler := api.NewTrustedProxyMiddleware(&config.Config{
		Environment: "production",
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRemoteAddr = r.RemoteAddr
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:58200" // Caddy conectando via loopback
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 127.0.0.1")

	rec := httptest.NewRecorder()
	testHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK para requisição legítima do Caddy com XFF, obtido %d", rec.Code)
	}
	// O IP de auditoria registrado no request deve ser o do cliente remoto reportado pelo proxy
	if seenRemoteAddr != "203.0.113.50:58200" && seenRemoteAddr != "203.0.113.50" {
		t.Fatalf("esperado que seenRemoteAddr reflita o cliente Caddy 203.0.113.50, obtido %s", seenRemoteAddr)
	}
}

// 6. Prova que um proxy reverso remoto explicitamente configurado em LSM_TRUSTED_PROXIES é aceito e auditado
func TestNetworkIsolation_RemoteTrustedProxy_AllowedAndAudited(t *testing.T) {
	var seenRemoteAddr string
	testHandler := api.NewTrustedProxyMiddleware(&config.Config{
		Environment:         "production",
		AllowExternalListen: true,
		TrustedProxies:      []string{"10.0.1.10", "192.168.1.0/24"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRemoteAddr = r.RemoteAddr
		w.WriteHeader(http.StatusOK)
	}))

	// Conexão direta vinda do proxy remoto confiável 10.0.1.10
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.1.10:48912"
	req.Header.Set("X-Forwarded-For", "203.0.113.99, 10.0.1.10")

	rec := httptest.NewRecorder()
	testHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK para requisição vinda de proxy remoto confiável, obtido %d", rec.Code)
	}
	if seenRemoteAddr != "203.0.113.99:48912" && seenRemoteAddr != "203.0.113.99" {
		t.Fatalf("esperado que seenRemoteAddr reflita o cliente 203.0.113.99, obtido %s", seenRemoteAddr)
	}

	// Conexão direta vinda de proxy dentro do bloco CIDR confiável 192.168.1.50
	reqCIDR := httptest.NewRequest("GET", "/test", nil)
	reqCIDR.RemoteAddr = "192.168.1.50:51234"
	reqCIDR.Header.Set("X-Forwarded-For", "198.51.100.77")
	recCIDR := httptest.NewRecorder()
	testHandler.ServeHTTP(recCIDR, reqCIDR)

	if recCIDR.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK para proxy dentro de bloco CIDR confiável, obtido %d", recCIDR.Code)
	}
	if seenRemoteAddr != "198.51.100.77:51234" && seenRemoteAddr != "198.51.100.77" {
		t.Fatalf("esperado que seenRemoteAddr reflita 198.51.100.77, obtido %s", seenRemoteAddr)
	}
}

// 7. Prova que um proxy ou cliente remoto NÃO configurado em LSM_TRUSTED_PROXIES é rejeitado em produção
func TestNetworkIsolation_RemoteUntrustedProxy_RejectedInProduction(t *testing.T) {
	testHandler := api.NewTrustedProxyMiddleware(&config.Config{
		Environment:         "production",
		AllowExternalListen: false,
		TrustedProxies:      []string{"10.0.1.10"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IP não presente na lista de proxies confiáveis
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "172.16.0.5:41234"
	req.Header.Set("X-Forwarded-For", "10.0.1.10") // Tentativa de spoofing

	rec := httptest.NewRecorder()
	testHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 Forbidden para requisição de proxy não autorizado em produção, obtido %d", rec.Code)
	}
}
