package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"databit.com.br/gofra/examples/basic/config"
	basicweb "databit.com.br/gofra/examples/basic/web"
	"databit.com.br/gofra/runtimeconfig"
)

func main() {
	cfg := config.Default()

	mux := http.NewServeMux()
	mux.Handle(runtimeconfig.DefaultPath, config.PublicConfigHandler(cfg))
	mux.Handle("/", basicweb.Handler())

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	slog.Info("starting basic example", "addr", addr, "runtime_config_path", runtimeconfig.DefaultPath)

	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server stopped", "error", err)
	}
}
