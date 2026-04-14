package runtimeauth

import (
	"encoding/json"
	"fmt"
)

// ClaimMapperFunc extracts a [User] from raw JWT claims. The default mapper
// handles ZITADEL's claim format; callers can override it via
// [WithClaimMapper].
type ClaimMapperFunc func(claims json.RawMessage) (User, error)

// defaultClaimMapper extracts user fields from ZITADEL JWT access token
// claims. It reads the standard "sub" claim for the user ID.
func defaultClaimMapper(raw json.RawMessage) (User, error) {
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return User{}, fmt.Errorf("runtimeauth: unmarshal claims: %w", err)
	}
	if claims.Sub == "" {
		return User{}, fmt.Errorf("runtimeauth: missing sub claim")
	}
	return User{
		ID: claims.Sub,
	}, nil
}
