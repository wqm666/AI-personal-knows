package port

import "context"

type Identity struct {
	OwnerID string
	Name    string
}

type IdentityProvider interface {
	Resolve(ctx context.Context) (*Identity, error)
}

type ctxKey struct{}

func ContextWithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func IdentityFromContext(ctx context.Context) *Identity {
	id, _ := ctx.Value(ctxKey{}).(*Identity)
	if id == nil {
		return &Identity{OwnerID: "default", Name: "default"}
	}
	return id
}
