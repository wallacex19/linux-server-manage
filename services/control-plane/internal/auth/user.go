package auth

import "context"

type LinuxUserIdentity struct {
	Username string   `json:"username"`
	UID      int      `json:"uid"`
	GID      int      `json:"gid"`
	Groups   []string `json:"groups"`
	HomeDir  string   `json:"home_dir"`
	Shell    string   `json:"shell"`
	IsAdmin  bool     `json:"is_admin"`
}

type contextKey string

const UserContextKey = contextKey("lsm_user")

func WithUser(ctx context.Context, user *LinuxUserIdentity) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

func UserFromContext(ctx context.Context) *LinuxUserIdentity {
	val, ok := ctx.Value(UserContextKey).(*LinuxUserIdentity)
	if !ok {
		return nil
	}
	return val
}
