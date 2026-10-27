package security_test

import (
	"strings"
	"testing"

	"github.com/linux-server-manager/agent/internal/security"
)

func TestAccountChecker_ExistingAndNonExistingUser(t *testing.T) {
	checker := security.NewTestAccountChecker("servidor-jogos-01", map[string]*security.LocalAccountInfo{
		"jogador_linux": {
			Username: "jogador_linux",
			UID:      1005,
			GID:      1005,
			HomeDir:  "/home/jogador_linux",
			IsAdmin:  false,
			Groups:   []string{"jogador_linux"},
		},
	})

	// 1. Usuário existente deve retornar com sucesso e permissões reais
	acc, err := checker.CheckLocalAccount("jogador_linux")
	if err != nil {
		t.Fatalf("esperado sucesso para usuário existente: %v", err)
	}
	if acc.UID != 1005 {
		t.Errorf("esperado UID 1005, obtido %d", acc.UID)
	}
	if acc.IsAdmin {
		t.Errorf("esperado IsAdmin = false")
	}

	// 2. Usuário que não existe neste nó remoto deve ser negado com mensagem explícita
	_, err = checker.CheckLocalAccount("usuario_inexistente")
	if err == nil {
		t.Fatalf("esperado erro ao consultar usuário que não existe no nó")
	}

	expectedMsg := "a conta 'usuario_inexistente' não existe no servidor remoto 'servidor-jogos-01' (acesso negado)"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("esperado mensagem contendo '%s', obtido '%s'", expectedMsg, err.Error())
	}
}

// TestAccountChecker_ProductionConstructorDoesNotUseMocks comprova que o construtor oficial de produção
// não possui usuários mockados e consulta exclusivamente o SO
func TestAccountChecker_ProductionConstructorDoesNotUseMocks(t *testing.T) {
	prodChecker := security.NewAccountChecker("servidor-producao")

	// Usuário simulado arbitrário nunca deve ser aprovado pelo construtor de produção
	_, err := prodChecker.CheckLocalAccount("conta_mock_inventada_12345")
	if err == nil {
		t.Fatal("esperado erro ao consultar conta inexistente no verificador de produção")
	}
}
