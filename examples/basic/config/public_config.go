package config

import (
	"net/http"

	runtimev1 "databit.com.br/gofra/examples/basic/runtime/v1"
	"databit.com.br/gofra/runtimeconfig"
)

func NewPublicConfigResolver(
	cfg *Config,
	opts ...runtimeconfig.Option[runtimev1.RuntimeConfig],
) runtimeconfig.Resolver[runtimev1.RuntimeConfig] {
	return runtimeconfig.NewResolver(cfg, BindPublicConfig, opts...)
}

func PublicConfigHandler(
	cfg *Config,
	opts ...runtimeconfig.Option[runtimev1.RuntimeConfig],
) http.Handler {
	return runtimeconfig.Handler(NewPublicConfigResolver(cfg, opts...))
}
