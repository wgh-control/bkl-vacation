package platform

import (
	"context"

	"github.com/wgh-control/bkl-vacation/internal/identity"
)

type principalKey struct{}

// WithPrincipal stores the authenticated person on the request context.
func WithPrincipal(ctx context.Context, p identity.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// Principal returns the person stored by Authenticated routes.
func Principal(ctx context.Context) (identity.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(identity.Principal)
	return p, ok
}
