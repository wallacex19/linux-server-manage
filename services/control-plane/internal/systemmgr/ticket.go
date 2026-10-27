package systemmgr

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type ProcessKillTicket struct {
	NodeID       string `json:"node_id"`
	OperatorUID  int    `json:"operator_uid"`
	OperatorUser string `json:"operator_username"`
	IsAdmin      bool   `json:"is_admin"`
	PID          int    `json:"pid"`
	Signal       string `json:"signal"`
	Nonce        string `json:"nonce"`
	ExpiresAt    int64  `json:"expires_at"` // Unix timestamp in seconds
	Signature    string `json:"signature"`  // HMAC-SHA256 hex string
}

func CreateProcessKillTicket(nodeID, agentToken string, operatorUID int, operatorUsername string, isAdmin bool, pid int, signal string) (ProcessKillTicket, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return ProcessKillTicket{}, fmt.Errorf("falha ao gerar nonce criptográfico: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)
	expiresAt := time.Now().Add(30 * time.Second).Unix()

	canonical := CanonicalTicketMessage(nodeID, operatorUID, operatorUsername, isAdmin, pid, signal, nonce, expiresAt)
	mac := hmac.New(sha256.New, []byte(agentToken))
	mac.Write([]byte(canonical))
	sig := hex.EncodeToString(mac.Sum(nil))

	return ProcessKillTicket{
		NodeID:       nodeID,
		OperatorUID:  operatorUID,
		OperatorUser: operatorUsername,
		IsAdmin:      isAdmin,
		PID:          pid,
		Signal:       signal,
		Nonce:        nonce,
		ExpiresAt:    expiresAt,
		Signature:    sig,
	}, nil
}

func CanonicalTicketMessage(nodeID string, operatorUID int, operatorUsername string, isAdmin bool, pid int, signal, nonce string, expiresAt int64) string {
	return fmt.Sprintf("LSM-KILL-v1:%s:%d:%s:%t:%d:%s:%s:%d", nodeID, operatorUID, operatorUsername, isAdmin, pid, signal, nonce, expiresAt)
}
