package runtimeconfig

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	DefaultPath       = "/_gofra/config.js"
	DefaultGlobalName = "__GOFRA_CONFIG__"
)

type HandlerOptions struct {
	GlobalName string
}

func Handler[T any](resolver Resolver[T]) http.Handler {
	return HandlerWithOptions(resolver, HandlerOptions{})
}

func HandlerWithOptions[T any](resolver Resolver[T], opts HandlerOptions) http.Handler {
	globalName := opts.GlobalName
	if globalName == "" {
		globalName = DefaultGlobalName
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
		default:
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		if resolver == nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		value, err := resolver.Resolve(r.Context(), r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		payload, err := json.Marshal(value)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}

		fmt.Fprintf(w, "window.%s = %s;\n", globalName, payload)
	})
}
