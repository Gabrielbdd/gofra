package runtimeauth

import (
	"fmt"

	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// ClaimMapperFunc extracts a [User] from validated access token claims. The
// default mapper handles ZITADEL's claim format; callers can override it via
// [WithClaimMapper].
type ClaimMapperFunc func(claims *oidc.AccessTokenClaims) (User, error)

// defaultClaimMapper extracts user fields from ZITADEL JWT access token
// claims. It reads the standard "sub" claim for the user ID.
func defaultClaimMapper(claims *oidc.AccessTokenClaims) (User, error) {
	if claims.Subject == "" {
		return User{}, fmt.Errorf("runtimeauth: missing sub claim")
	}
	return User{
		ID: claims.Subject,
	}, nil
}
