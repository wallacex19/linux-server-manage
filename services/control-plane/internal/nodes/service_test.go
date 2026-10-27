package nodes_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/nodes"
)

func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_nodes.db")
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao conectar ao banco de teste: %v", err)
	}

	return db, func() {
		_ = db.Close()
	}
}

func TestNodeLifecycleAndPairing(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := nodes.NewService(db, "https://painel.central.exemplo")

	// 1. Cadastra Servidor A (Infraestrutura)
	nodeA, err := svc.CreateNode(nodes.CreateNodeRequest{
		Name:      "Servidor Alpha",
		GroupName: "Infraestrutura",
		Tags:      "ubuntu,producao",
	})
	if err != nil {
		t.Fatalf("falha ao criar nó: %v", err)
	}

	if nodeA.Status != nodes.StatusUnpaired {
		t.Errorf("esperado status 'unpaired', obtido '%s'", nodeA.Status)
	}
	if nodeA.PairingToken == "" {
		t.Errorf("esperado token de pareamento preenchido")
	}

	// 2. Cadastra Servidor B (Jogos)
	nodeB, err := svc.CreateNode(nodes.CreateNodeRequest{
		Name:      "Servidor Games",
		GroupName: "Jogos Steam",
		Tags:      "cs2,valheim",
	})
	if err != nil {
		t.Fatalf("falha ao criar segundo nó: %v", err)
	}

	pubDummy, _, _ := ed25519.GenerateKey(rand.Reader)
	pubDummyHex := hex.EncodeToString(pubDummy)

	// 3. Tentar parear com chave pública inválida deve falhar
	_, err = svc.PairAgent(nodes.PairAgentRequest{
		PairingToken: nodeA.PairingToken,
		PublicKey:    "chave-invalida-curta",
		Hostname:     "alpha-host",
	})
	if err == nil {
		t.Errorf("esperado erro ao tentar parear com chave pública inválida")
	}

	// 3.1 Tentar parear com assinatura falsa deve falhar
	fakeSig := hex.EncodeToString(make([]byte, 64))
	_, err = svc.PairAgent(nodes.PairAgentRequest{
		PairingToken: nodeA.PairingToken,
		PublicKey:    pubDummyHex,
		Signature:    fakeSig,
		Hostname:     "alpha-host",
	})
	if err == nil {
		t.Errorf("esperado erro ao tentar parear com assinatura falsa")
	}

	// 3.2 Tentar parear com token inválido deve falhar
	_, err = svc.PairAgent(nodes.PairAgentRequest{
		PairingToken: "LSM-PAIR-INVALIDO",
		PublicKey:    pubDummyHex,
		Hostname:     "alpha-host",
	})
	if err == nil || err != nodes.ErrInvalidOrExpiredToken {
		t.Errorf("esperado ErrInvalidOrExpiredToken, obtido %v", err)
	}

	// 4. Pareamento legítimo do Servidor A com chave Ed25519 e assinatura
	pubA, privA, _ := ed25519.GenerateKey(rand.Reader)
	pubAHex := hex.EncodeToString(pubA)
	sigA := hex.EncodeToString(ed25519.Sign(privA, []byte(nodeA.PairingToken)))

	pairResp, err := svc.PairAgent(nodes.PairAgentRequest{
		PairingToken: nodeA.PairingToken,
		PublicKey:    pubAHex,
		Signature:    sigA,
		Hostname:     "ubuntu-alpha-01",
		IPAddress:    "192.168.1.10",
		OSInfo:       "Ubuntu Server 24.04 LTS",
		AgentVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("falha no pareamento: %v", err)
	}

	if pairResp.NodeID != nodeA.ID {
		t.Errorf("esperado NodeID %s, obtido %s", nodeA.ID, pairResp.NodeID)
	}
	if pairResp.AgentToken == "" {
		t.Errorf("esperado AgentToken gerado")
	}

	// 5. Validação de token permanente do agente
	if !svc.ValidateAgent(nodeA.ID, pairResp.AgentToken) {
		t.Errorf("esperado token de agente válido")
	}

	// 6. Registro de Heartbeat com métricas
	err = svc.RecordHeartbeat(nodeA.ID, "192.168.1.10", &nodes.NodeMetricsSummary{
		CPUPercent:    15.5,
		MemoryPercent: 42.0,
		DiskPercent:   30.0,
		ProcessCount:  85,
		UptimeSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("falha ao registrar heartbeat: %v", err)
	}

	// 7. Listagem e verificação de métricas e status
	nodesList, err := svc.ListNodes("", "")
	if err != nil {
		t.Fatalf("falha ao listar nós: %v", err)
	}
	// Deve conter pelo menos o nó padrão local-node (da migração) + Node A + Node B
	if len(nodesList) < 2 {
		t.Errorf("esperado pelo menos 2 nós, obtido %d", len(nodesList))
	}

	var foundA, foundB *nodes.Node
	for i := range nodesList {
		if nodesList[i].ID == nodeA.ID {
			foundA = &nodesList[i]
		}
		if nodesList[i].ID == nodeB.ID {
			foundB = &nodesList[i]
		}
	}

	if foundA == nil || foundA.Status != nodes.StatusOnline {
		t.Errorf("esperado Servidor Alpha com status 'online', obtido %v", foundA)
	}
	if foundA.Metrics == nil || foundA.Metrics.CPUPercent != 15.5 {
		t.Errorf("esperado métricas corretas salvas em cache para nó A")
	}

	if foundB == nil || foundB.Status != nodes.StatusUnpaired {
		t.Errorf("esperado Servidor Games com status 'unpaired'")
	}

	// 8. Revogação de acesso
	err = svc.RevokeNode(nodeA.ID)
	if err != nil {
		t.Fatalf("falha ao revogar nó: %v", err)
	}

	if svc.ValidateAgent(nodeA.ID, pairResp.AgentToken) {
		t.Errorf("esperado que agente revogado NÃO seja mais validado")
	}

	// 9. Remoção do nó
	err = svc.DeleteNode(nodeB.ID)
	if err != nil {
		t.Fatalf("falha ao excluir nó B: %v", err)
	}
}

func TestSecurity_Ed25519Pairing_SignatureMandatoryAndImmutability(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := nodes.NewService(db, "http://localhost:8080")

	node, err := svc.CreateNode(nodes.CreateNodeRequest{
		Name:      "Node-Strict-Security",
		GroupName: "Testing",
	})
	if err != nil {
		t.Fatalf("falha ao criar nó: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("falha ao gerar par Ed25519: %v", err)
	}
	pubHex := hex.EncodeToString(pub)

	// 1. Assinatura ausente -> erro explícito
	_, err = svc.PairAgent(nodes.PairAgentRequest{
		PairingToken: node.PairingToken,
		PublicKey:    pubHex,
		Signature:    "",
		Hostname:     "test-host",
	})
	if err == nil || !strings.Contains(err.Error(), "assinatura criptográfica Ed25519 é obrigatória") {
		t.Errorf("esperado erro de assinatura obrigatória, obtido: %v", err)
	}

	// 2. Assinatura malformada (tamanho != 64 bytes)
	_, err = svc.PairAgent(nodes.PairAgentRequest{
		PairingToken: node.PairingToken,
		PublicKey:    pubHex,
		Signature:    "deadbeef",
		Hostname:     "test-host",
	})
	if err == nil || !strings.Contains(err.Error(), "malformada ou com tamanho incorreto") {
		t.Errorf("esperado erro de tamanho de assinatura, obtido: %v", err)
	}

	// 3. Node ID diverge do nó real
	canonicalMsg := fmt.Sprintf("LSM-PAIR-v1:%s:%s", node.ID, node.PairingToken)
	validSig := hex.EncodeToString(ed25519.Sign(priv, []byte(canonicalMsg)))

	_, err = svc.PairAgent(nodes.PairAgentRequest{
		NodeID:       "wrong-node-id-00000000",
		PairingToken: node.PairingToken,
		PublicKey:    pubHex,
		Signature:    validSig,
		Hostname:     "test-host",
	})
	if err == nil || !strings.Contains(err.Error(), "identificador do nó diverge") {
		t.Errorf("esperado erro de divergência de Node ID, obtido: %v", err)
	}

	// 4. Pareamento bem-sucedido com assinatura canônica válida
	pairResp, err := svc.PairAgent(nodes.PairAgentRequest{
		NodeID:       node.ID,
		PairingToken: node.PairingToken,
		PublicKey:    pubHex,
		Signature:    validSig,
		Hostname:     "test-host",
		IPAddress:    "10.0.0.50",
	})
	if err != nil {
		t.Fatalf("pareamento com assinatura canônica falhou: %v", err)
	}
	if pairResp.NodeID != node.ID {
		t.Errorf("esperado NodeID %s, obtido %s", node.ID, pairResp.NodeID)
	}

	// 5. Tentativa de Replay / Reuso do mesmo pairing token deve ser rejeitada
	_, err = svc.PairAgent(nodes.PairAgentRequest{
		NodeID:       node.ID,
		PairingToken: node.PairingToken,
		PublicKey:    pubHex,
		Signature:    validSig,
		Hostname:     "test-host",
	})
	if err == nil || err != nodes.ErrInvalidOrExpiredToken {
		t.Errorf("esperado ErrInvalidOrExpiredToken em tentativa de replay de token já consumido, obtido: %v", err)
	}
}
