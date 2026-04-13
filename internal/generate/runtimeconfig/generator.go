package runtimeconfiggen

import (
	"bytes"
	"fmt"
	"go/format"
	"text/template"
)

type GoBinderParams struct {
	PackageName   string
	RuntimeImport string
	RuntimeAlias  string
	RuntimeType   string
	ConfigType    string
	FunctionName  string
}

type TSLoaderParams struct {
	RuntimeImport string
	GlobalName    string
}

func RenderGoBinderStub(params GoBinderParams) ([]byte, error) {
	if params.PackageName == "" {
		params.PackageName = "config"
	}
	if params.RuntimeAlias == "" {
		params.RuntimeAlias = "runtimev1"
	}
	if params.RuntimeType == "" {
		params.RuntimeType = "RuntimeConfig"
	}
	if params.ConfigType == "" {
		params.ConfigType = "Config"
	}
	if params.FunctionName == "" {
		params.FunctionName = "BindPublicConfig"
	}
	if params.RuntimeImport == "" {
		return nil, fmt.Errorf("runtime import is required")
	}

	const source = `package {{ .PackageName }}

import (
	"fmt"

	{{ .RuntimeAlias }} "{{ .RuntimeImport }}"
)

// {{ .FunctionName }} is scaffolded output from gofra generate runtime-config.
// The proto descriptor-driven field mapping still needs to replace this stub.
func {{ .FunctionName }}(cfg *{{ .ConfigType }}) (*{{ .RuntimeAlias }}.{{ .RuntimeType }}, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config: nil *{{ .ConfigType }}")
	}

	return &{{ .RuntimeAlias }}.{{ .RuntimeType }}{}, nil
}
`

	var buf bytes.Buffer
	if err := template.Must(template.New("go-binder").Parse(source)).Execute(&buf, params); err != nil {
		return nil, err
	}

	return format.Source(buf.Bytes())
}

func RenderTSLoader(params TSLoaderParams) ([]byte, error) {
	if params.RuntimeImport == "" {
		params.RuntimeImport = "./runtime_config_pb"
	}
	if params.GlobalName == "" {
		params.GlobalName = "__GOFRA_CONFIG__"
	}

	const source = `import type { RuntimeConfig } from "{{ .RuntimeImport }}";

type RuntimeWindow = Window & {
  {{ .GlobalName }}?: unknown;
};

export function isRuntimeConfig(value: unknown): value is RuntimeConfig {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  return "auth" in value;
}

export function validateRuntimeConfig(value: unknown): RuntimeConfig {
  if (!isRuntimeConfig(value)) {
    throw new Error("missing or invalid runtime config");
  }

  return Object.freeze(value as RuntimeConfig);
}

export function loadRuntimeConfig(): RuntimeConfig {
  const value = (window as RuntimeWindow).{{ .GlobalName }};
  return validateRuntimeConfig(value);
}

export const runtimeConfig = loadRuntimeConfig();
`

	var buf bytes.Buffer
	if err := template.Must(template.New("ts-loader").Parse(source)).Execute(&buf, params); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
