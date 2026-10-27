package security

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

type LocalAccountInfo struct {
	Username string   `json:"username"`
	UID      int      `json:"uid"`
	GID      int      `json:"gid"`
	HomeDir  string   `json:"home_dir"`
	IsAdmin  bool     `json:"is_admin"`
	Groups   []string `json:"groups"`
}

type AccountChecker struct {
	hostname  string
	isTest    bool
	mockUsers map[string]*LocalAccountInfo
}

// NewAccountChecker cria o verificador de contas para o runtime oficial de produção.
// Consulta exclusivamente as contas reais do sistema operacional Linux via PAM/kernel.
// Não possui contas simuladas embutidas nem rotas de bypass.
func NewAccountChecker(hostname string) *AccountChecker {
	if hostname == "" {
		if h, err := os.Hostname(); err == nil {
			hostname = h
		} else {
			hostname = "linux-host"
		}
	}

	return &AccountChecker{
		hostname: hostname,
		isTest:   false,
	}
}

// NewTestAccountChecker cria um verificador simulado com injeção explícita para testes unitários.
func NewTestAccountChecker(hostname string, mocks map[string]*LocalAccountInfo) *AccountChecker {
	if hostname == "" {
		hostname = "test-host"
	}
	m := make(map[string]*LocalAccountInfo)
	for k, v := range mocks {
		m[k] = v
	}
	return &AccountChecker{
		hostname:  hostname,
		isTest:    true,
		mockUsers: m,
	}
}

// CheckLocalAccount verifica se o usuário autenticado no painel existe localmente neste servidor Linux
func (c *AccountChecker) CheckLocalAccount(username string) (*LocalAccountInfo, error) {
	// Se for verificador de teste, consulta exclusivamente os mocks fornecidos pelo teste
	if c.isTest {
		if u, exists := c.mockUsers[username]; exists {
			return u, nil
		}
		return nil, fmt.Errorf("a conta '%s' não existe no servidor remoto '%s' (acesso negado)", username, c.hostname)
	}

	// Runtime real: Consulta obrigatória e exclusiva na base de contas do sistema operacional Linux
	u, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("a conta '%s' não existe no servidor remoto '%s' (acesso negado)", username, c.hostname)
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	isAdmin := uid == 0
	groupNames := []string{}

	if groupIDs, err := u.GroupIds(); err == nil {
		for _, gID := range groupIDs {
			if g, err := user.LookupGroupId(gID); err == nil {
				groupNames = append(groupNames, g.Name)
				if g.Name == "sudo" || g.Name == "wheel" || g.Name == "admin" {
					isAdmin = true
				}
			}
		}
	}

	return &LocalAccountInfo{
		Username: u.Username,
		UID:      uid,
		GID:      gid,
		HomeDir:  u.HomeDir,
		IsAdmin:  isAdmin,
		Groups:   groupNames,
	}, nil
}
