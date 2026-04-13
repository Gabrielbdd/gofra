// Package configgen generates Go config code from a proto schema.
package configgen

// ConfigSchema is the parsed representation of a user's config.proto.
type ConfigSchema struct {
	// GoPackage is the Go package name for the generated code (e.g., "config").
	GoPackage string

	// RuntimeImport is the import path for the framework's runtime/config package.
	RuntimeImport string

	// RootMessage is the top-level Config message.
	RootMessage *MessageInfo

	// AllMessages contains all messages in topological order (dependencies first).
	AllMessages []*MessageInfo

	// PublicField points to the "public" field on the root message, or nil if absent.
	PublicField *FieldInfo
}

// MessageInfo represents a proto message mapped to a Go struct.
type MessageInfo struct {
	// ProtoName is the proto message name (e.g., "AppConfig").
	ProtoName string

	// GoName is the Go struct name (e.g., "AppConfig").
	GoName string

	// Description is the leading comment from the proto source.
	Description string

	// Fields contains all fields in proto field-number order.
	Fields []FieldInfo

	// IsPublic is true if this message is within the public subtree.
	IsPublic bool
}

// FieldInfo represents a single proto field mapped to a Go struct field.
type FieldInfo struct {
	// ProtoName is the snake_case proto field name (e.g., "app_name").
	ProtoName string

	// GoName is the Go field name (e.g., "AppName").
	GoName string

	// KoanfTag is the koanf/yaml struct tag value (snake_case).
	KoanfTag string

	// JSONTag is the json struct tag value (camelCase). Only set for public fields.
	JSONTag string

	// GoType is the Go type string (e.g., "string", "int32", "[]string", "AppConfig").
	GoType string

	// FlagType is the pflag registration method name (e.g., "String", "Int32", "Bool").
	// Empty for message fields.
	FlagType string

	// Description is the leading comment from the proto source.
	Description string

	// IsMessage is true if the field type is a nested message.
	IsMessage bool

	// IsRepeated is true for repeated (slice) fields.
	IsRepeated bool

	// IsSecret is true if gofra.config.v1.field.secret is set.
	IsSecret bool

	// Default holds the parsed default value, or nil if no default is annotated.
	Default *DefaultInfo

	// MessageRef points to the nested message's info, if IsMessage is true.
	MessageRef *MessageInfo
}

// DefaultInfo holds a parsed default value ready for Go code emission.
type DefaultInfo struct {
	// GoLiteral is the ready-to-emit Go literal (e.g., `3000`, `"myapp"`, `true`).
	GoLiteral string

	// IsSlice is true when the default is for a repeated field ([]string{...}).
	IsSlice bool
}
