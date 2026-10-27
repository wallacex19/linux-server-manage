//go:build linux

package auth

// NewDefaultAuthenticator retorna o autenticador oficial para Linux (PAM).
// Não permite ativação de autenticadores simulados por variáveis de ambiente.
// Testes unitários que necessitarem de simulação devem instanciar e injetar
// explicitamente NewDevAuthenticator().
func NewDefaultAuthenticator() Authenticator {
	return NewPAMAuthenticator("lsm")
}
