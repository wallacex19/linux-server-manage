package auth

import (
	"context"
	"testing"
	"time"
)

func TestDevAuthenticator(t *testing.T) {
	auth := NewDevAuthenticator()
	ctx := context.Background()

	// 1. Sucesso admin
	user, err := auth.Authenticate(ctx, "admin", "adminpassword")
	if err != nil {
		t.Fatalf("erro ao autenticar admin: %v", err)
	}
	if !user.IsAdmin {
		t.Error("esperado IsAdmin=true para o usuário admin")
	}
	if user.UID != 1000 {
		t.Errorf("esperado UID 1000, obtido %d", user.UID)
	}

	// 2. Sucesso operador comum
	userOp, err := auth.Authenticate(ctx, "operador", "userpassword")
	if err != nil {
		t.Fatalf("erro ao autenticar operador: %v", err)
	}
	if userOp.IsAdmin {
		t.Error("esperado IsAdmin=false para o usuário operador")
	}

	// 3. Falha de senha
	_, err = auth.Authenticate(ctx, "admin", "wrongpass")
	if err != ErrInvalidCredentials {
		t.Errorf("esperado ErrInvalidCredentials, obtido: %v", err)
	}

	// 4. Usuário inexistente
	_, err = auth.Authenticate(ctx, "naoexiste", "qualquer")
	if err != ErrUserNotFound {
		t.Errorf("esperado ErrUserNotFound, obtido: %v", err)
	}
}

func TestSessionStore(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)

	user := &LinuxUserIdentity{
		Username: "admin",
		UID:      1000,
		IsAdmin:  true,
	}

	sess, err := store.Create(user)
	if err != nil {
		t.Fatalf("falha ao criar sessão: %v", err)
	}

	if len(sess.Token) != 64 {
		t.Errorf("esperado token hex de 64 caracteres, obtido %d", len(sess.Token))
	}

	retrieved, exists := store.Get(sess.Token)
	if !exists {
		t.Fatal("sessão deveria existir")
	}
	if retrieved.User.Username != "admin" {
		t.Errorf("esperado username 'admin', obtido '%s'", retrieved.User.Username)
	}

	store.Revoke(sess.Token)
	_, existsAfterRevoke := store.Get(sess.Token)
	if existsAfterRevoke {
		t.Fatal("sessão não deveria existir após revogação")
	}
}

func TestLoginRateLimiter(t *testing.T) {
	limiter := NewLoginRateLimiter(3, 100*time.Millisecond)
	key := "127.0.0.1:baduser"

	if !limiter.IsAllowed(key) {
		t.Fatal("deve permitir inicialmente")
	}

	limiter.RecordFailure(key)
	limiter.RecordFailure(key)
	if !limiter.IsAllowed(key) {
		t.Fatal("deve permitir 2 falhas quando o máximo é 3")
	}

	limiter.RecordFailure(key) // 3ª falha -> bloqueia
	if limiter.IsAllowed(key) {
		t.Fatal("deve bloquear após atingir o limite máximo de falhas")
	}

	time.Sleep(120 * time.Millisecond) // espera lockout expirar
	if !limiter.IsAllowed(key) {
		t.Fatal("deve desbloquear após o tempo de lockout")
	}
}
