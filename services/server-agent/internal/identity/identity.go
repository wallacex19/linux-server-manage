package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type NodeIdentity struct {
	Fingerprint string
	PublicKey   ed25519.PublicKey
	PrivateKey  ed25519.PrivateKey
	NodeID      string // Mantido para compatibilidade com testes unitários
}

// LoadOrCreateIdentity carrega ou gera par de chaves Ed25519 exclusivo do nó
func LoadOrCreateIdentity(keyPath string) (*NodeIdentity, error) {
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("falha ao criar diretório para identidade: %w", err)
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// Gera novo par de chaves Ed25519
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("falha ao gerar par de chaves: %w", err)
		}

		if err := os.WriteFile(keyPath, priv, 0600); err != nil {
			return nil, fmt.Errorf("falha ao salvar chave privada: %w", err)
		}

		fp := deriveFingerprint(pub)
		return &NodeIdentity{
			Fingerprint: fp,
			NodeID:      fp,
			PublicKey:   pub,
			PrivateKey:  priv,
		}, nil
	}

	// Carrega chave existente
	// #nosec G304 -- keyPath é caminho canônico configurado para chave privada Ed25519
	privBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler chave privada existente: %w", err)
	}

	var priv ed25519.PrivateKey
	if len(privBytes) == ed25519.PrivateKeySize {
		priv = ed25519.PrivateKey(privBytes)
	} else if len(privBytes) == ed25519.SeedSize {
		priv = ed25519.NewKeyFromSeed(privBytes)
	} else {
		return nil, fmt.Errorf("tamanho inválido da chave privada: %d bytes (esperado %d ou %d)", len(privBytes), ed25519.PrivateKeySize, ed25519.SeedSize)
	}

	pub := priv.Public().(ed25519.PublicKey)
	fp := deriveFingerprint(pub)

	return &NodeIdentity{
		Fingerprint: fp,
		NodeID:      fp,
		PublicKey:   pub,
		PrivateKey:  priv,
	}, nil
}

// deriveFingerprint calcula o fingerprint único a partir do hash SHA-256 da chave pública
func deriveFingerprint(pub ed25519.PublicKey) string {
	hash := sha256.Sum256(pub)
	return hex.EncodeToString(hash[:16]) // 32 caracteres hex
}

// PublicKeyHex retorna a chave pública codificada em hexadecimal (64 caracteres)
func (n *NodeIdentity) PublicKeyHex() string {
	return hex.EncodeToString(n.PublicKey)
}

// SignMessage assina um payload arbitrário com a chave privada do nó
func (n *NodeIdentity) SignMessage(message []byte) []byte {
	return ed25519.Sign(n.PrivateKey, message)
}

// VerifySignature verifica a assinatura de uma mensagem usando a chave pública informada
func VerifySignature(pub ed25519.PublicKey, message, sig []byte) bool {
	return ed25519.Verify(pub, message, sig)
}
