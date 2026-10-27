package api_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/linux-server-manager/backend/internal/api"
	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/nodes"
)

func TestAgent_FullLifecycle_Pairing_WSS_Heartbeat_Commands_Revoke(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_full_lifecycle.db")
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao conectar no banco: %v", err)
	}
	defer db.Close()

	nodesSvc := nodes.NewService(db, "http://localhost")
	agentHub := nodes.NewAgentHub(nodesSvc)

	agentHandler := api.NewAgentHandler(nodesSvc, agentHub)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/agent/pair", agentHandler.Pair)
	mux.HandleFunc("/api/v1/agent/connect", agentHandler.Connect)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Criação do nó no Control Plane
	createdNode, err := nodesSvc.CreateNode(nodes.CreateNodeRequest{
		Name:      "Integration-Worker-01",
		GroupName: "Production",
		Tags:      "worker,queue",
	})
	if err != nil {
		t.Fatalf("falha ao cadastrar nó: %v", err)
	}
	if createdNode.PairingToken == "" {
		t.Fatal("token de pareamento gerado vazio")
	}

	// 2. Chaves Ed25519 do Agente
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("falha ao gerar par Ed25519: %v", err)
	}
	pubKeyHex := hex.EncodeToString(pubKey)
	validSig := hex.EncodeToString(ed25519.Sign(privKey, []byte(createdNode.PairingToken)))

	// 3. Teste de rejeição com assinatura forjada/inválida
	badPairReq := nodes.PairAgentRequest{
		PairingToken: createdNode.PairingToken,
		PublicKey:    pubKeyHex,
		Signature:    "deadbeefcafebabe0102030405060708deadbeefcafebabe0102030405060708deadbeefcafebabe0102030405060708deadbeefcafebabe0102030405060708",
		Hostname:     "worker-01",
		IPAddress:    "192.168.1.100",
		OSInfo:       "Ubuntu 24.04",
		AgentVersion: "1.1.0",
	}
	badPayload, _ := json.Marshal(badPairReq)
	badResp, err := http.Post(ts.URL+"/api/v1/agent/pair", "application/json", strings.NewReader(string(badPayload)))
	if err != nil {
		t.Fatalf("erro ao enviar requisição com assinatura forjada: %v", err)
	}
	if badResp.StatusCode != http.StatusBadRequest && badResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("esperado 400 ou 401 para assinatura forjada, obtido %d", badResp.StatusCode)
	}
	badResp.Body.Close()

	// 4. Pareamento válido com assinatura criptográfica autêntica
	pairReq := nodes.PairAgentRequest{
		PairingToken: createdNode.PairingToken,
		PublicKey:    pubKeyHex,
		Signature:    validSig,
		Hostname:     "worker-01",
		IPAddress:    "192.168.1.100",
		OSInfo:       "Ubuntu 24.04 LTS",
		AgentVersion: "1.1.0",
	}
	pairPayload, _ := json.Marshal(pairReq)
	pairResp, err := http.Post(ts.URL+"/api/v1/agent/pair", "application/json", strings.NewReader(string(pairPayload)))
	if err != nil {
		t.Fatalf("erro ao enviar requisição de pareamento: %v", err)
	}
	if pairResp.StatusCode != http.StatusOK {
		t.Fatalf("esperado status 200 no pareamento, obtido: %d", pairResp.StatusCode)
	}
	var pairResult nodes.AgentPairResponse
	if err := json.NewDecoder(pairResp.Body).Decode(&pairResult); err != nil {
		t.Fatalf("falha ao decodificar resposta de pareamento: %v", err)
	}
	pairResp.Body.Close()

	if pairResult.NodeID != createdNode.ID {
		t.Errorf("NodeID na resposta diverge: esperado %s, obtido %s", createdNode.ID, pairResult.NodeID)
	}
	if pairResult.AgentToken == "" {
		t.Fatal("agent_token retornado vazio")
	}

	// 5. Teste de Replay: Reutilização do token de pareamento já consumido deve falhar
	replayResp, err := http.Post(ts.URL+"/api/v1/agent/pair", "application/json", strings.NewReader(string(pairPayload)))
	if err != nil {
		t.Fatalf("erro na requisição de replay: %v", err)
	}
	if replayResp.StatusCode != http.StatusUnauthorized && replayResp.StatusCode != http.StatusBadRequest {
		t.Errorf("token consumido deve ser rejeitado no replay, obtido: %d", replayResp.StatusCode)
	}
	replayResp.Body.Close()

	// 6. Conexão WebSocket com cabeçalhos de autenticação (sem credenciais em query strings)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/agent/connect"
	headers := make(http.Header)
	headers.Set("X-LSM-Node-ID", pairResult.NodeID)
	headers.Set("X-LSM-Agent-Token", pairResult.AgentToken)
	headers.Set("Authorization", "Bearer "+pairResult.AgentToken)

	wsDialer := websocket.DefaultDialer
	conn, resp, err := wsDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("falha ao conectar WebSocket do agente: %v (status: %v)", err, resp)
	}
	defer conn.Close()

	// Aguarda registro no Hub
	time.Sleep(100 * time.Millisecond)
	if !agentHub.IsNodeOnline(createdNode.ID) {
		t.Fatalf("esperado nó %s estar online após conexão WebSocket", createdNode.ID)
	}

	// 7. Envio de Heartbeat do Agente para o Control Plane
	hbMsg := map[string]interface{}{
		"id":   "hb-101",
		"type": "heartbeat",
		"payload": map[string]interface{}{
			"cpu_percent":    25.5,
			"memory_percent": 60.0,
			"disk_percent":   40.0,
			"uptime_seconds": 3600,
			"process_count":  120,
		},
		"timestamp": time.Now(),
	}
	if err := conn.WriteJSON(hbMsg); err != nil {
		t.Fatalf("falha ao enviar heartbeat pelo WebSocket: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	nodeAfterHB, err := nodesSvc.GetNode(createdNode.ID)
	if err != nil {
		t.Fatalf("falha ao consultar nó após heartbeat: %v", err)
	}
	if nodeAfterHB.Status != nodes.StatusOnline {
		t.Errorf("status do nó deveria ser online, obtido: %s", nodeAfterHB.Status)
	}
	if nodeAfterHB.Metrics == nil || nodeAfterHB.Metrics.CPUPercent != 25.5 || nodeAfterHB.Metrics.MemoryPercent != 60.0 {
		t.Errorf("métricas do heartbeat não registradas corretamente: %+v", nodeAfterHB.Metrics)
	}

	// 8. Envio de Comando do Control Plane -> Agente e Retorno de Resposta
	go func() {
		// Simula o agente respondendo ao comando
		_, msgBytes, readErr := conn.ReadMessage()
		if readErr != nil {
			return
		}
		var incoming map[string]interface{}
		_ = json.Unmarshal(msgBytes, &incoming)

		reqID, _ := incoming["id"].(string)
		action, _ := incoming["action"].(string)

		if action == "ping" {
			respMsg := map[string]interface{}{
				"id":        reqID,
				"type":      "command_resp",
				"payload":   map[string]string{"status": "pong", "node": createdNode.ID},
				"timestamp": time.Now(),
			}
			_ = conn.WriteJSON(respMsg)
		}
	}()

	cmdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmdRespRaw, err := agentHub.SendCommand(cmdCtx, createdNode.ID, "ping", map[string]string{"test": "true"})
	if err != nil {
		t.Fatalf("falha no envio/recebimento do comando através do hub: %v", err)
	}

	var cmdRespData map[string]interface{}
	if err := json.Unmarshal(cmdRespRaw, &cmdRespData); err != nil {
		t.Fatalf("falha ao deserializar resposta do comando: %v", err)
	}
	if cmdRespData["status"] != "pong" {
		t.Errorf("esperado resposta pong do comando, obtido: %v", cmdRespData["status"])
	}

	// 9. Revogação do Nó: Conexão ativa deve ser desconectada imediatamente
	err = nodesSvc.RevokeNode(createdNode.ID)
	if err != nil {
		t.Fatalf("falha ao revogar nó: %v", err)
	}
	agentHub.DisconnectNode(createdNode.ID, "nó revogado pelo administrador")

	time.Sleep(100 * time.Millisecond)
	if agentHub.IsNodeOnline(createdNode.ID) {
		t.Errorf("esperado nó estar offline após desconexão ativa por revogação")
	}

	// 10. Tentativa de reconexão do agente revogado deve ser rejeitada com 403 Forbidden
	conn2, resp2, err2 := wsDialer.Dial(wsURL, headers)
	if err2 == nil {
		conn2.Close()
		t.Fatalf("esperado erro ao tentar reconectar nó revogado")
	}
	if resp2 != nil && resp2.StatusCode != http.StatusForbidden {
		t.Errorf("esperado status 403 para nó revogado, obtido %d", resp2.StatusCode)
	}
}

func TestAgent_MultiNode_Isolation(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_multi_node_isolation.db")
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao conectar no banco: %v", err)
	}
	defer db.Close()

	nodesSvc := nodes.NewService(db, "http://localhost")
	agentHub := nodes.NewAgentHub(nodesSvc)

	agentHandler := api.NewAgentHandler(nodesSvc, agentHub)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/agent/connect", agentHandler.Connect)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Cria e ativa dois nós separados
	n1, _ := nodesSvc.CreateNode(nodes.CreateNodeRequest{Name: "Server-Alpha"})
	pub1, priv1, _ := ed25519.GenerateKey(rand.Reader)
	sig1 := hex.EncodeToString(ed25519.Sign(priv1, []byte(n1.PairingToken)))
	pair1, err := nodesSvc.PairAgent(nodes.PairAgentRequest{
		PairingToken: n1.PairingToken,
		PublicKey:    hex.EncodeToString(pub1),
		Signature:    sig1,
	})
	if err != nil {
		t.Fatalf("falha ao parear nó 1: %v", err)
	}

	n2, _ := nodesSvc.CreateNode(nodes.CreateNodeRequest{Name: "Server-Beta"})
	pub2, priv2, _ := ed25519.GenerateKey(rand.Reader)
	sig2 := hex.EncodeToString(ed25519.Sign(priv2, []byte(n2.PairingToken)))
	pair2, err := nodesSvc.PairAgent(nodes.PairAgentRequest{
		PairingToken: n2.PairingToken,
		PublicKey:    hex.EncodeToString(pub2),
		Signature:    sig2,
	})
	if err != nil {
		t.Fatalf("falha ao parear nó 2: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/agent/connect"

	// Conecta Nó 1
	h1 := make(http.Header)
	h1.Set("X-LSM-Node-ID", n1.ID)
	h1.Set("X-LSM-Agent-Token", pair1.AgentToken)
	conn1, _, err1 := websocket.DefaultDialer.Dial(wsURL, h1)
	if err1 != nil {
		t.Fatalf("falha ao conectar Nó 1: %v", err1)
	}
	defer conn1.Close()

	// Conecta Nó 2
	h2 := make(http.Header)
	h2.Set("X-LSM-Node-ID", n2.ID)
	h2.Set("X-LSM-Agent-Token", pair2.AgentToken)
	conn2, _, err2 := websocket.DefaultDialer.Dial(wsURL, h2)
	if err2 != nil {
		t.Fatalf("falha ao conectar Nó 2: %v", err2)
	}
	defer conn2.Close()

	time.Sleep(100 * time.Millisecond)

	// Canal para registrar mensagens recebidas pelo Nó 2
	n2Received := make(chan string, 5)
	go func() {
		for {
			_, msg, err := conn2.ReadMessage()
			if err != nil {
				return
			}
			n2Received <- string(msg)
		}
	}()

	// Nó 1 responde ao seu comando
	go func() {
		_, msg, err := conn1.ReadMessage()
		if err != nil {
			return
		}
		var m map[string]interface{}
		_ = json.Unmarshal(msg, &m)
		reqID, _ := m["id"].(string)
		_ = conn1.WriteJSON(map[string]interface{}{
			"id":      reqID,
			"type":    "command_resp",
			"payload": map[string]string{"server": "alpha"},
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	respRaw, err := agentHub.SendCommand(ctx, n1.ID, "test.isolation", nil)
	if err != nil {
		t.Fatalf("falha ao enviar comando para Nó 1: %v", err)
	}
	if !strings.Contains(string(respRaw), "alpha") {
		t.Errorf("resposta inesperada: %s", string(respRaw))
	}

	// Verifica se Nó 2 recebeu alguma coisa acidentalmente (deve ser 0)
	select {
	case msg := <-n2Received:
		t.Fatalf("FALHA DE ISOLAMENTO: Nó 2 recebeu comando destinado ao Nó 1: %s", msg)
	case <-time.After(300 * time.Millisecond):
		// Sucesso: Nó 2 não recebeu nada
	}
}
