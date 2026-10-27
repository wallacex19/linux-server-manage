package system_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/linux-server-manager/agent/internal/system"
)

func makeTestTicket(nodeID, agentToken string, operatorUID int, operatorUser string, isAdmin bool, pid int, signal, nonce string, expiresAt int64) *system.ProcessKillTicket {
	canonical := fmt.Sprintf("LSM-KILL-v1:%s:%d:%s:%t:%d:%s:%s:%d",
		nodeID, operatorUID, operatorUser, isAdmin, pid, signal, nonce, expiresAt)
	mac := hmac.New(sha256.New, []byte(agentToken))
	mac.Write([]byte(canonical))
	sig := hex.EncodeToString(mac.Sum(nil))

	return &system.ProcessKillTicket{
		NodeID:       nodeID,
		OperatorUID:  operatorUID,
		OperatorUser: operatorUser,
		IsAdmin:      isAdmin,
		PID:          pid,
		Signal:       signal,
		Nonce:        nonce,
		ExpiresAt:    expiresAt,
		Signature:    sig,
	}
}

// 1. Proibição estrita de finalizar PID 1 (init)
func TestKillProcess_PID1_Protection(t *testing.T) {
	err := system.KillProcess(1, "SIGTERM", 0, "root", "", true, nil, "token123", "node-01")
	if err == nil {
		t.Fatal("esperado erro ao tentar finalizar PID 1, obtido nil")
	}
}

// 2. Rejeição de sinal inválido
func TestKillProcess_InvalidSignal(t *testing.T) {
	err := system.KillProcess(os.Getpid(), "SIGBOGUS", 1000, "user", "", false, nil, "token123", "node-01")
	if err == nil {
		t.Fatal("esperado erro ao tentar enviar sinal desconhecido/inválido")
	}
}

// 3. Processo inexistente
func TestKillProcess_NonExistentPID(t *testing.T) {
	err := system.KillProcess(9876543, "SIGTERM", 1000, "user", "", false, nil, "token123", "node-01")
	if err == nil {
		t.Fatal("esperado erro para PID inexistente")
	}
}

// 4. Ticket expirado deve ser rejeitado
func TestKillProcess_Ticket_Expired(t *testing.T) {
	expired := time.Now().Add(-60 * time.Second).Unix()
	ticket := makeTestTicket("node-01", "token123", 1001, "operador", false, os.Getpid(), "SIGTERM", "nonce-exp-01", expired)

	err := system.KillProcess(os.Getpid(), "SIGTERM", 1001, "operador", "", false, ticket, "token123", "node-01")
	if err == nil || err.Error() != "autorização expirada" {
		t.Fatalf("esperado erro 'autorização expirada', obtido: %v", err)
	}
}

// 5. Ticket adulterado ou com assinatura incorreta deve ser rejeitado
func TestKillProcess_Ticket_Tampered(t *testing.T) {
	validExp := time.Now().Add(60 * time.Second).Unix()
	ticket := makeTestTicket("node-01", "token123", 1001, "operador", false, os.Getpid(), "SIGTERM", "nonce-tamper-01", validExp)
	ticket.Signature = "deadbeefcafebabe00112233445566778899aabbccddeeff"

	err := system.KillProcess(os.Getpid(), "SIGTERM", 1001, "operador", "", false, ticket, "token123", "node-01")
	if err == nil || err.Error() != "autorização adulterada ou assinatura inválida" {
		t.Fatalf("esperado erro 'autorização adulterada ou assinatura inválida', obtido: %v", err)
	}
}

// 6. Prevenção contra repetição (Replay Attack) com mesmo nonce
func TestKillProcess_Ticket_Replay(t *testing.T) {
	validExp := time.Now().Add(60 * time.Second).Unix()
	ticket := makeTestTicket("node-01", "token123", 1001, "operador", false, 9999991, "SIGTERM", "nonce-replay-unique-01", validExp)

	// Primeira validação
	err1 := system.VerifyTicket(ticket, "node-01", "token123")
	if err1 != nil {
		t.Fatalf("primeira verificação deveria ter sucesso, obtido: %v", err1)
	}

	// Segunda validação com o mesmo nonce deve falhar por replay
	err2 := system.VerifyTicket(ticket, "node-01", "token123")
	if err2 == nil || err2.Error() != "autorização já utilizada / replay detectado" {
		t.Fatalf("esperado erro de replay detectado, obtido: %v", err2)
	}
}

// 7. Divergência de Node ID
func TestKillProcess_Ticket_NodeIDMismatch(t *testing.T) {
	validExp := time.Now().Add(60 * time.Second).Unix()
	ticket := makeTestTicket("node-attacker-99", "token123", 1001, "operador", false, os.Getpid(), "SIGTERM", "nonce-node-01", validExp)

	err := system.VerifyTicket(ticket, "node-victim-01", "token123")
	if err == nil {
		t.Fatal("esperado erro de divergência de Node ID")
	}
}

// 8. Teste de execução de processo do mesmo usuário e listagem
func TestProcesses_ListAndCurrentProcess(t *testing.T) {
	procs, err := system.ListProcesses()
	if err != nil {
		t.Fatalf("falha ao listar processos: %v", err)
	}
	if len(procs) == 0 {
		t.Fatal("esperado lista não vazia de processos")
	}

	currentPID := os.Getpid()
	foundCurrent := false
	for _, p := range procs {
		if p.PID == currentPID {
			foundCurrent = true
			if p.Status == "" && p.State == "" {
				t.Errorf("status do processo atual não preenchido")
			}
			break
		}
	}
	if !foundCurrent && runtime.GOOS == "linux" {
		t.Errorf("processo atual PID %d não encontrado na listagem /proc", currentPID)
	}
}

// 9. Processo filho: sinalizar processo de teste com ticket legítimo
func TestKillProcess_ChildProcess_Lifecycle(t *testing.T) {
	// Cria um processo filho que aguarda
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "timeout", "10")
	} else {
		cmd = exec.Command("sleep", "10")
	}

	if err := cmd.Start(); err != nil {
		t.Skipf("ambiente não suporta subprocesso: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	childPID := cmd.Process.Pid
	validExp := time.Now().Add(30 * time.Second).Unix()
	ticket := makeTestTicket("node-01", "token123", os.Getuid(), "testuser", true, childPID, "SIGTERM", fmt.Sprintf("nonce-child-%d", childPID), validExp)

	err := system.KillProcess(childPID, "SIGTERM", os.Getuid(), "testuser", "", true, ticket, "token123", "node-01")
	if err != nil {
		t.Fatalf("falha ao finalizar processo filho autorizado: %v", err)
	}
}

// 10. Operadores não-administradores não podem sinalizar processos de root (UID 0)
func TestKillProcess_UID0_Protection_ForNonAdmin(t *testing.T) {
	validExp := time.Now().Add(30 * time.Second).Unix()
	// PID 1 pertence ao root (UID 0)
	ticket := makeTestTicket("node-01", "token123", 1001, "operador", false, 1, "SIGTERM", "nonce-root-test", validExp)

	err := system.KillProcess(1, "SIGTERM", 1001, "operador", "", false, ticket, "token123", "node-01")
	if err == nil {
		t.Fatal("esperado erro ao tentar sinalizar processo root por operador comum")
	}
}
