package httpx

import "context"

type setupCodeVerifiedKey struct{}

// WithSetupCodeVerified marks a request as having a valid setup code
func WithSetupCodeVerified(ctx context.Context) context.Context {
	return context.WithValue(ctx, setupCodeVerifiedKey{}, true)
}

// SetupCodeVerified reports whether the request has a valid setup code
func SetupCodeVerified(ctx context.Context) bool {
	verified, _ := ctx.Value(setupCodeVerifiedKey{}).(bool)
	return verified
}
