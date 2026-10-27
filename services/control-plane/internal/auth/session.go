package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type Session struct {
	Token     string             `json:"token"`
	User      *LinuxUserIdentity `json:"user"`
	CreatedAt time.Time          `json:"created_at"`
	ExpiresAt time.Time          `json:"expires_at"`
}

type SessionStore struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	durationTTL time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	s := &SessionStore{
		sessions:    make(map[string]*Session),
		durationTTL: ttl,
	}

	// Limpeza periódica de sessões expiradas a cada 15 minutos
	go s.cleanupLoop()

	return s
}

func (s *SessionStore) Create(user *LinuxUserIdentity) (*Session, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("falha ao gerar token de sessão criptográfico: %w", err)
	}
	token := hex.EncodeToString(bytes)

	now := time.Now().UTC()
	sess := &Session{
		Token:     token,
		User:      user,
		CreatedAt: now,
		ExpiresAt: now.Add(s.durationTTL),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = sess

	return sess, nil
}

func (s *SessionStore) Get(token string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, exists := s.sessions[token]
	if !exists {
		return nil, false
	}

	if time.Now().UTC().After(sess.ExpiresAt) {
		return nil, false
	}

	return sess, true
}

func (s *SessionStore) Revoke(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func (s *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now().UTC()
		for token, sess := range s.sessions {
			if now.After(sess.ExpiresAt) {
				delete(s.sessions, token)
			}
		}
		s.mu.Unlock()
	}
}
