package config

import (
	"fmt"

	runtimev1 "databit.com.br/gofra/examples/basic/runtime/v1"
)

// BindPublicConfig mirrors the generated binder shape that gofra-gen-runtimeconfig
// will eventually own. For now it is checked in manually so the example app can
// dogfood the runtime-config handler contract end to end.
func BindPublicConfig(cfg *Config) (*runtimev1.RuntimeConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config: nil *Config")
	}

	return &runtimev1.RuntimeConfig{
		APIBaseURL: "",
		Auth: runtimev1.AuthConfig{
			Issuer:                 cfg.Auth.Issuer,
			ClientID:               cfg.Auth.ClientID,
			Scopes:                 append([]string(nil), cfg.Auth.Scopes...),
			RedirectPath:           cfg.Auth.RedirectPath,
			PostLogoutRedirectPath: cfg.Auth.PostLogoutRedirectPath,
		},
	}, nil
}
