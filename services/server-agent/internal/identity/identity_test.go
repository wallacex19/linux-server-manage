package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNodeIdentityLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lsm_agent_test_*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPath := filepath.Join(tempDir, "agent.key")

	// 1. Gera nova identidade
	id1, err := LoadOrCreateIdentity(keyPath)
	if err != nil {
		t.Fatalf("erro ao criar identidade: %v", err)
	}

	if id1.NodeID == "" {
		t.Fatal("NodeID nao deve ser vazio")
	}

	if len(id1.PublicKeyHex()) != 64 {
		t.Fatalf("esperado tamanho de chave hex 64, obtido %d", len(id1.PublicKeyHex()))
	}

	// 2. Testa assinatura e validação
	msg := []byte("ping-heartbeat-payload")
	sig := id1.SignMessage(msg)

	if !VerifySignature(id1.PublicKey, msg, sig) {
		t.Fatal("falha ao verificar assinatura com chave publica valida")
	}

	tamperedMsg := []byte("tampered-payload")
	if VerifySignature(id1.PublicKey, tamperedMsg, sig) {
		t.Fatal("assinatura invalida nao deveria ter sido aprovada")
	}

	// 3. Carrega a chave existente e verifica se mantém o mesmo NodeID
	id2, err := LoadOrCreateIdentity(keyPath)
	if err != nil {
		t.Fatalf("erro ao recarregar identidade: %v", err)
	}

	if id2.NodeID != id1.NodeID {
		t.Fatalf("esperado mesmo NodeID %s, obtido %s", id1.NodeID, id2.NodeID)
	}
}
