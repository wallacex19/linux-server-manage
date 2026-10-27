package auth

import (
	"context"
	"strings"
	"sync"
)

type DevAuthenticator struct {
	mu    sync.RWMutex
	users map[string]devUserEntry
}

type devUserEntry struct {
	password string
	identity LinuxUserIdentity
}

func NewDevAuthenticator() *DevAuthenticator {
	da := &DevAuthenticator{
		users: make(map[string]devUserEntry),
	}

	// Usuário Administrador de desenvolvimento (simula membro do grupo sudo no Linux)
	da.users["admin"] = devUserEntry{
		password: "adminpassword",
		identity: LinuxUserIdentity{
			Username: "admin",
			UID:      1000,
			GID:      1000,
			Groups:   []string{"admin", "sudo", "docker"},
			HomeDir:  "/home/admin",
			Shell:    "/bin/bash",
			IsAdmin:  true,
		},
	}

	// Usuário comum sem privilégios administrativos
	da.users["operador"] = devUserEntry{
		password: "userpassword",
		identity: LinuxUserIdentity{
			Username: "operador",
			UID:      1001,
			GID:      1001,
			Groups:   []string{"operador"},
			HomeDir:  "/home/operador",
			Shell:    "/bin/bash",
			IsAdmin:  false,
		},
	}

	return da
}

func (d *DevAuthenticator) Authenticate(ctx context.Context, username, password string) (*LinuxUserIdentity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	userLower := strings.ToLower(strings.TrimSpace(username))
	entry, exists := d.users[userLower]
	if !exists {
		return nil, ErrUserNotFound
	}

	if entry.password != password {
		return nil, ErrInvalidCredentials
	}

	idCopy := entry.identity
	return &idCopy, nil
}
