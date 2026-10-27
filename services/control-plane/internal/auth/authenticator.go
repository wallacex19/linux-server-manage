package auth

import (
	"context"
	"errors"
)

var (
	ErrInvalidCredentials = errors.New("credenciais inválidas")
	ErrUserNotFound       = errors.New("usuário não encontrado no sistema operacional")
	ErrAccountLocked      = errors.New("conta bloqueada ou desativada no sistema operacional")
	ErrRateLimited        = errors.New("muitas tentativas de login; tente novamente mais tarde")
)

type Authenticator interface {
	Authenticate(ctx context.Context, username, password string) (*LinuxUserIdentity, error)
}
