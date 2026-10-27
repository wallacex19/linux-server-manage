//go:build !linux

package auth

import (
	"context"
	"errors"
)

// ErrUnsupportedPlatform indica que autenticação de produção do SO não está disponível fora do Linux
var ErrUnsupportedPlatform = errors.New("autenticação PAM de produção suportada exclusivamente no Linux; esta plataforma não possui autenticador de produção")

type unsupportedAuthenticator struct{}

func (u *unsupportedAuthenticator) Authenticate(ctx context.Context, username, password string) (*LinuxUserIdentity, error) {
	return nil, ErrUnsupportedPlatform
}

// NewDefaultAuthenticator em plataformas não-Linux NÃO utiliza autenticador simulado (DevAuthenticator) por padrão.
// O runtime de produção do LSM requer Linux com PAM.
// Testes que necessitarem de autenticador simulado devem instanciar e injetar
// explicitamente NewDevAuthenticator().
func NewDefaultAuthenticator() Authenticator {
	return &unsupportedAuthenticator{}
}
