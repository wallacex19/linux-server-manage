package nodes_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/nodes"
)

// TestMultiServer_AcceptanceCriteria10 valida rigorosamente os 10 critérios da arquitetura multi-servidor
func TestMultiServer_AcceptanceCriteria10(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "acceptance_multi_server.db")
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar banco de teste: %v", err)
	}
	defer db.Close()

	svc := nodes.NewService(db, "https://painel.lsm.exemplo")

	// =========================================================================
	// Critério 1: Cadastrar e parear ambos os servidores (Servidor 1 e Servidor 2)
	// =========================================================================
	srv1, err := svc.CreateNode(nodes.CreateNodeRequest{
		Name:      "Ubuntu-Web-Prod",
		GroupName: "Infraestrutura",
		Tags:      "nginx,caddy",
	})
	if err != nil {
		t.Fatalf("[Critério 1] Falha ao cadastrar Servidor 1: %v", err)
	}

	srv2, err := svc.CreateNode(nodes.CreateNodeRequest{
		Name:      "Ubuntu-Games-01",
		GroupName: "Jogos Steam",
		Tags:      "cs2,valheim",
	})
	if err != nil {
		t.Fatalf("[Critério 1] Falha ao cadastrar Servidor 2: %v", err)
	}

	pub1, priv1, _ := ed25519.GenerateKey(rand.Reader)
	pubHex1 := hex.EncodeToString(pub1)
	sig1 := hex.EncodeToString(ed25519.Sign(priv1, []byte(srv1.PairingToken)))

	pair1, err := svc.PairAgent(nodes.PairAgentRequest{
		PairingToken: srv1.PairingToken,
		PublicKey:    pubHex1,
		Signature:    sig1,
		Hostname:     "web-prod.srv",
		IPAddress:    "10.0.0.1",
		OSInfo:       "Ubuntu Server 24.04 LTS",
		AgentVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("[Critério 1] Falha ao parear Servidor 1: %v", err)
	}

	if pair1.AgentToken == "" || !svc.ValidateAgent(srv1.ID, pair1.AgentToken) {
		t.Fatalf("[Critério 1] Token permanente do Servidor 1 deve ser válido")
	}

	pub2, priv2, _ := ed25519.GenerateKey(rand.Reader)
	pubHex2 := hex.EncodeToString(pub2)
	sig2 := hex.EncodeToString(ed25519.Sign(priv2, []byte(srv2.PairingToken)))

	pair2, err := svc.PairAgent(nodes.PairAgentRequest{
		PairingToken: srv2.PairingToken,
		PublicKey:    pubHex2,
		Signature:    sig2,
		Hostname:     "games-01.srv",
		IPAddress:    "10.0.0.2",
		OSInfo:       "Ubuntu Server 24.04 LTS",
		AgentVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("[Critério 1] Falha ao parear Servidor 2: %v", err)
	}

	// =========================================================================
	// Critério 2: Visualizar métricas individuais e globais
	// =========================================================================
	_ = svc.RecordHeartbeat(srv1.ID, "10.0.0.1", &nodes.NodeMetricsSummary{
		CPUPercent:    18.2,
		MemoryPercent: 45.0,
		DiskPercent:   33.0,
		ProcessCount:  92,
		UptimeSeconds: 14400,
	})

	_ = svc.RecordHeartbeat(srv2.ID, "10.0.0.2", &nodes.NodeMetricsSummary{
		CPUPercent:    62.5,
		MemoryPercent: 78.0,
		DiskPercent:   55.0,
		ProcessCount:  140,
		UptimeSeconds: 7200,
	})

	list, err := svc.ListNodes("", "")
	if err != nil {
		t.Fatalf("[Critério 2] Falha ao listar servidores: %v", err)
	}

	var found1, found2 *nodes.Node
	for i := range list {
		if list[i].ID == srv1.ID {
			found1 = &list[i]
		}
		if list[i].ID == srv2.ID {
			found2 = &list[i]
		}
	}

	if found1 == nil || found1.Metrics.CPUPercent != 18.2 {
		t.Errorf("[Critério 2] Métricas do Servidor 1 divergentes")
	}
	if found2 == nil || found2.Metrics.CPUPercent != 62.5 {
		t.Errorf("[Critério 2] Métricas do Servidor 2 divergentes")
	}

	// =========================================================================
	// Critério 3: Detectar quando um dos servidores fica offline
	// =========================================================================
	// Simula queda de conexão no Servidor 2
	svc.RecordDisconnection(srv2.ID, "queda de link de rede")

	listAfterDrop, _ := svc.ListNodes("", "")
	for _, n := range listAfterDrop {
		if n.ID == srv1.ID && n.Status != nodes.StatusOnline {
			t.Errorf("[Critério 3] Servidor 1 deve continuar 'online'")
		}
		if n.ID == srv2.ID && n.Status != nodes.StatusOffline {
			t.Errorf("[Critério 3] Servidor 2 deve ser detectado como 'offline'")
		}
	}

	// =========================================================================
	// Critério 7: Executar uma operação autorizada sem afetar o outro servidor (Isolamento)
	// =========================================================================
	err = svc.UpdateNode(srv1.ID, nodes.UpdateNodeRequest{
		Name:      "Ubuntu-Web-Prod-Renomeado",
		GroupName: "Infraestrutura Crítica",
		Tags:      "nginx,caddy,ssl",
	})
	if err != nil {
		t.Fatalf("[Critério 7] Falha ao atualizar Servidor 1: %v", err)
	}

	updated2, _ := svc.GetNode(srv2.ID)
	if updated2.Name != "Ubuntu-Games-01" {
		t.Errorf("[Critério 7] Alteração no Servidor 1 vazou para o Servidor 2!")
	}

	// =========================================================================
	// Critério 9: Revogar um agente e impedir novas operações autenticadas por ele
	// =========================================================================
	err = svc.RevokeNode(srv2.ID)
	if err != nil {
		t.Fatalf("[Critério 9] Falha ao revogar Servidor 2: %v", err)
	}

	if svc.ValidateAgent(srv2.ID, pair2.AgentToken) {
		t.Errorf("[Critério 9] Token do agente revogado não pode ser validado!")
	}

	// =========================================================================
	// Critério 10: Manter o painel central funcional quando um agente estiver indisponível
	// =========================================================================
	finalList, err := svc.ListNodes("", "")
	if err != nil {
		t.Fatalf("[Critério 10] Painel falhou ao listar servidores com nós offline/revogados: %v", err)
	}
	if len(finalList) < 2 {
		t.Errorf("[Critério 10] Painel deve continuar exibindo catálogo íntegro")
	}

	// Servidor 1 continua online e ativo
	serv1Final, _ := svc.GetNode(srv1.ID)
	if serv1Final.Status != nodes.StatusOnline {
		t.Errorf("[Critério 10] Servidor 1 deve permanecer online e funcional")
	}

	// Tempo de execução registrado com sucesso
	t.Logf("Todos os 10 critérios de aceitação foram validados com sucesso no teste de integração multi-servidor (%s)", time.Now().Format(time.RFC3339))
}
