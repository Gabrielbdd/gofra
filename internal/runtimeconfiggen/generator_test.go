package runtimeconfiggen_test

import (
	"strings"
	"testing"

	"databit.com.br/gofra/internal/runtimeconfiggen"
)

func TestRenderGoBinderStub(t *testing.T) {
	t.Parallel()

	output, err := runtimeconfiggen.RenderGoBinderStub(runtimeconfiggen.GoBinderParams{
		PackageName:   "config",
		RuntimeImport: "databit.com.br/gofra/examples/basic/runtime/v1",
	})
	if err != nil {
		t.Fatalf("RenderGoBinderStub() error = %v", err)
	}

	text := string(output)
	if !strings.Contains(text, "func BindPublicConfig") {
		t.Fatalf("output missing binder function:\n%s", text)
	}
	if !strings.Contains(text, `runtimev1 "databit.com.br/gofra/examples/basic/runtime/v1"`) {
		t.Fatalf("output missing runtime import:\n%s", text)
	}
}

func TestRenderTSLoader(t *testing.T) {
	t.Parallel()

	output, err := runtimeconfiggen.RenderTSLoader(runtimeconfiggen.TSLoaderParams{})
	if err != nil {
		t.Fatalf("RenderTSLoader() error = %v", err)
	}

	text := string(output)
	if !strings.Contains(text, "__GOFRA_CONFIG__") {
		t.Fatalf("output missing global name:\n%s", text)
	}
	if !strings.Contains(text, "export const runtimeConfig = loadRuntimeConfig();") {
		t.Fatalf("output missing exported runtime config:\n%s", text)
	}
}
