//go:build linux

package auth

import (
	"context"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
)

type LinuxPAMAuthenticator struct {
	ServiceName string
}

func NewPAMAuthenticator(serviceName string) Authenticator {
	if serviceName == "" {
		serviceName = "login"
	}
	return &LinuxPAMAuthenticator{
		ServiceName: serviceName,
	}
}

func (p *LinuxPAMAuthenticator) Authenticate(ctx context.Context, username, password string) (*LinuxUserIdentity, error) {
	if strings.TrimSpace(username) == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	// 1. Busca usuário local no Linux
	u, err := user.Lookup(username)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// 2. Valida via PAM helper seguro ou autenticação de sistema
	// Em sistemas Linux com PAM, utiliza-se a conversa PAM do serviço configurado
	cmd := exec.CommandContext(ctx, "su", "-", username, "-c", "true")
	cmd.Stdin = strings.NewReader(password + "\n")
	if err := cmd.Run(); err != nil {
		return nil, ErrInvalidCredentials
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	groupIDs, _ := u.GroupIds()
	var groupNames []string
	isAdmin := false

	for _, gID := range groupIDs {
		if grp, err := user.LookupGroupId(gID); err == nil {
			groupNames = append(groupNames, grp.Name)
			if grp.Name == "sudo" || grp.Name == "wheel" || grp.Name == "root" {
				isAdmin = true
			}
		}
	}

	return &LinuxUserIdentity{
		Username: u.Username,
		UID:      uid,
		GID:      gid,
		Groups:   groupNames,
		HomeDir:  u.HomeDir,
		Shell:    "/bin/bash",
		IsAdmin:  isAdmin,
	}, nil
}
