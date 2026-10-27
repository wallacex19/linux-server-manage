package transport

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/linux-server-manager/agent/internal/config"
	"github.com/linux-server-manager/agent/internal/identity"
	"github.com/linux-server-manager/agent/internal/security"
)

func createTestIdentity(t *testing.T) *identity.NodeIdentity {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("falha ao gerar chave para teste: %v", err)
	}
	return &identity.NodeIdentity{
		NodeID:     "node-test-01",
		PublicKey:  pub,
		PrivateKey: priv,
	}
}

func TestClient_ExecuteAction_SecurityAndFiles(t *testing.T) {
	tempHome, err := os.MkdirTemp("", "lsm_agent_test_*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempHome)

	ident := createTestIdentity(t)
	cfg := config.Config{
		NodeID:   "node-test-01",
		Hostname: "test-node",
	}
	client := NewClient(cfg, ident)

	normalUser := &security.LocalAccountInfo{
		Username: "operador",
		UID:      1001,
		GID:      1001,
		HomeDir:  tempHome,
		IsAdmin:  false,
	}

	adminUser := &security.LocalAccountInfo{
		Username: "root",
		UID:      0,
		GID:      0,
		HomeDir:  tempHome,
		IsAdmin:  true,
	}

	// 1. Ação Ping
	res, err := client.executeAction("ping", CommandPayload{}, normalUser)
	if err != nil {
		t.Fatalf("falha ao executar ping: %v", err)
	}
	var pingData map[string]interface{}
	_ = json.Unmarshal(res, &pingData)
	if pingData["status"] != "pong" {
		t.Errorf("esperado status pong, obtido %v", pingData["status"])
	}

	// 2. Criação de arquivo seguro
	testFilePath := filepath.Join(tempHome, "teste.txt")
	_, err = client.executeAction("files.create", CommandPayload{Path: testFilePath}, normalUser)
	if err != nil {
		t.Fatalf("falha ao criar arquivo: %v", err)
	}

	// 3. Salvar conteúdo (anti-TOCTOU)
	_, err = client.executeAction("files.save", CommandPayload{Path: testFilePath, Content: "conteudo de teste"}, normalUser)
	if err != nil {
		t.Fatalf("falha ao salvar arquivo: %v", err)
	}

	// 4. Ler arquivo
	readRes, err := client.executeAction("files.read", CommandPayload{Path: testFilePath}, normalUser)
	if err != nil {
		t.Fatalf("falha ao ler arquivo: %v", err)
	}
	var readData map[string]interface{}
	_ = json.Unmarshal(readRes, &readData)
	if readData["content"] != "conteudo de teste" {
		t.Errorf("conteúdo lido diverge do gravado: %v", readData["content"])
	}

	// 5. Listar arquivos do diretório
	listRes, err := client.executeAction("files.list", CommandPayload{Path: tempHome}, normalUser)
	if err != nil {
		t.Fatalf("falha ao listar arquivos: %v", err)
	}
	var listData map[string]interface{}
	if err := json.Unmarshal(listRes, &listData); err != nil {
		t.Fatalf("esperado retorno de listagem de arquivos, obtido: %s", string(listRes))
	}
	entries, ok := listData["entries"].([]interface{})
	if !ok || len(entries) == 0 {
		t.Fatalf("esperado pelo menos uma entrada nos arquivos, obtido: %v", listData)
	}

	// 6. Tentativa de Path Traversal por usuário não-admin DEVE falhar
	_, err = client.executeAction("files.read", CommandPayload{Path: "../../../etc/shadow"}, normalUser)
	if err == nil {
		t.Errorf("esperado erro de acesso negado para path traversal fora da home")
	}

	// 7. Renomear arquivo
	newFilePath := filepath.Join(tempHome, "teste_renomeado.txt")
	_, err = client.executeAction("files.rename", CommandPayload{OldPath: testFilePath, NewPath: newFilePath}, normalUser)
	if err != nil {
		t.Fatalf("falha ao renomear arquivo: %v", err)
	}

	// 8. Deletar arquivo renomeado
	_, err = client.executeAction("files.delete", CommandPayload{Path: newFilePath}, normalUser)
	if err != nil {
		t.Fatalf("falha ao deletar arquivo: %v", err)
	}

	// 9. Teste de serviços: Usuário não-admin NÃO pode executar ações de alteração em serviços
	_, err = client.executeAction("system.service_action", CommandPayload{ServiceName: "nginx", Action: "restart"}, normalUser)
	if err == nil {
		t.Errorf("esperado erro ao tentar reiniciar serviço sem ser admin")
	}

	// 10. Listagem de processos (permitido para usuários locais)
	procRes, err := client.executeAction("system.processes", CommandPayload{}, normalUser)
	if err != nil {
		t.Fatalf("falha ao listar processos: %v", err)
	}
	if len(procRes) == 0 {
		t.Errorf("resposta de processos vazia")
	}

	// 11. Listagem de serviços (permitido para leitura)
	srvRes, err := client.executeAction("system.services", CommandPayload{}, normalUser)
	if err != nil {
		t.Fatalf("falha ao listar serviços: %v", err)
	}
	if len(srvRes) == 0 {
		t.Errorf("resposta de serviços vazia")
	}

	// 12. Validação de comando desconhecido
	_, err = client.executeAction("unknown.action", CommandPayload{}, adminUser)
	if err == nil {
		t.Errorf("esperado erro para comando desconhecido")
	}

	// 13. Proteção contra finalização de PID 1
	_, err = client.executeAction("system.process_kill", CommandPayload{PID: 1, Signal: "SIGTERM"}, adminUser)
	if err == nil {
		t.Errorf("esperado bloqueio ao tentar matar PID 1")
	}

	// 14. Bloqueio de exclusão em caminho crítico do sistema
	_, err = client.executeAction("files.delete", CommandPayload{Path: "/etc"}, adminUser)
	if err == nil {
		t.Errorf("esperado bloqueio para exclusão de /etc")
	}
}
