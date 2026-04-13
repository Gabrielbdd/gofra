package configgen

import (
	"strings"
	"text/template"
)

var templateFuncs = template.FuncMap{
	"structTag":   structTag,
	"flagPath":    flagPath,
	"join":        strings.Join,
	"hasDefault":  func(f FieldInfo) bool { return f.Default != nil },
	"hasPublic":   func(s *ConfigSchema) bool { return s.PublicField != nil },
	"notMessage":  func(f FieldInfo) bool { return !f.IsMessage },
	"notSecret":   func(f FieldInfo) bool { return !f.IsSecret },
	"notRepeated": func(f FieldInfo) bool { return !f.IsRepeated },
}

// structTag builds the Go struct tag string for a field.
func structTag(f FieldInfo) string {
	parts := []string{
		`koanf:"` + f.KoanfTag + `"`,
		`yaml:"` + f.KoanfTag + `"`,
	}
	if f.JSONTag != "" {
		parts = append(parts, `json:"` + f.JSONTag + `"`)
	}
	return "`" + strings.Join(parts, " ") + "`"
}

// flagPath builds the dotted flag name for a field given a prefix.
// e.g., prefix="app", name="port" => "app.port"
func flagPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
