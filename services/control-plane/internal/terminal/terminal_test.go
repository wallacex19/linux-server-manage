package terminal

import (
	"testing"

	"github.com/linux-server-manager/backend/internal/auth"
)

func TestTmuxServiceIsolationAndLifecycle(t *testing.T) {
	service := NewTmuxService()

	user1 := &auth.LinuxUserIdentity{Username: "user1", UID: 1001}
	user2 := &auth.LinuxUserIdentity{Username: "user2", UID: 1002}

	// 1. Cria sessão para user1
	sess1, err := service.CreateSession(user1, "game-valheim", "Valheim")
	if err != nil {
		t.Fatalf("falha ao criar sessão para user1: %v", err)
	}
	if sess1.Name != "game-valheim" {
		t.Errorf("esperado nome 'game-valheim', obtido '%s'", sess1.Name)
	}

	// 2. Lista sessões para user1
	user1Sessions, err := service.ListSessions(user1)
	if err != nil {
		t.Fatalf("erro ao listar sessões de user1: %v", err)
	}
	if len(user1Sessions) != 1 {
		t.Errorf("esperado 1 sessão para user1, obtido %d", len(user1Sessions))
	}

	// 3. User2 não deve enxergar a sessão de user1 (isolamento obrigatório)
	user2Sessions, err := service.ListSessions(user2)
	if err != nil {
		t.Fatalf("erro ao listar sessões de user2: %v", err)
	}
	for _, s := range user2Sessions {
		if s.Name == "game-valheim" {
			t.Fatal("violação de segurança: user2 enxergou a sessão de user1!")
		}
	}

	// 4. Encerra sessão de user1
	if err := service.KillSession(user1, "game-valheim"); err != nil {
		t.Fatalf("falha ao encerrar sessão: %v", err)
	}

	user1SessionsAfter, _ := service.ListSessions(user1)
	if len(user1SessionsAfter) != 0 {
		t.Errorf("esperado 0 sessões após kill, obtido %d", len(user1SessionsAfter))
	}
}
