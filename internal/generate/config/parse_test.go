package configgen

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func TestParseMinimalProto(t *testing.T) {
	t.Parallel()

	schema, err := ParseProto(testdataPath("minimal.proto"), ParseOptions{
		GoPackage:     "config",
		RuntimeImport: "example.com/framework/runtime/config",
	})
	if err != nil {
		t.Fatalf("ParseProto() error = %v", err)
	}

	if schema.RootMessage == nil {
		t.Fatal("RootMessage is nil")
	}
	if schema.RootMessage.GoName != "Config" {
		t.Errorf("RootMessage.GoName = %q, want %q", schema.RootMessage.GoName, "Config")
	}
	if len(schema.RootMessage.Fields) != 2 {
		t.Fatalf("RootMessage.Fields len = %d, want 2", len(schema.RootMessage.Fields))
	}

	// Check app field.
	app := schema.RootMessage.Fields[0]
	if app.ProtoName != "app" {
		t.Errorf("field[0].ProtoName = %q, want %q", app.ProtoName, "app")
	}
	if !app.IsMessage {
		t.Error("field[0].IsMessage = false, want true")
	}
	if app.MessageRef == nil {
		t.Fatal("field[0].MessageRef is nil")
	}
	if len(app.MessageRef.Fields) != 2 {
		t.Fatalf("AppConfig fields len = %d, want 2", len(app.MessageRef.Fields))
	}

	// Check port field with default.
	port := app.MessageRef.Fields[0]
	if port.GoName != "Port" {
		t.Errorf("port.GoName = %q, want %q", port.GoName, "Port")
	}
	if port.GoType != "int32" {
		t.Errorf("port.GoType = %q, want %q", port.GoType, "int32")
	}
	if port.Default == nil {
		t.Fatal("port.Default is nil")
	}
	if port.Default.GoLiteral != "3000" {
		t.Errorf("port.Default.GoLiteral = %q, want %q", port.Default.GoLiteral, "3000")
	}

	// Check name field without default.
	name := app.MessageRef.Fields[1]
	if name.GoName != "Name" {
		t.Errorf("name.GoName = %q, want %q", name.GoName, "Name")
	}
	if name.Default != nil {
		t.Errorf("name.Default should be nil, got %v", name.Default)
	}

	// Check public field.
	if schema.PublicField == nil {
		t.Fatal("PublicField is nil")
	}
	if schema.PublicField.MessageRef == nil {
		t.Fatal("PublicField.MessageRef is nil")
	}
	if !schema.PublicField.MessageRef.IsPublic {
		t.Error("PublicConfig.IsPublic = false, want true")
	}
}

func TestParseFullProto(t *testing.T) {
	t.Parallel()

	schema, err := ParseProto(testdataPath("full.proto"), ParseOptions{
		GoPackage: "config",
	})
	if err != nil {
		t.Fatalf("ParseProto() error = %v", err)
	}

	// Root has 2 fields: app, public.
	if len(schema.RootMessage.Fields) != 2 {
		t.Fatalf("root fields = %d, want 2", len(schema.RootMessage.Fields))
	}

	// AppConfig has database nested message.
	app := schema.RootMessage.Fields[0].MessageRef
	if app == nil {
		t.Fatal("app MessageRef is nil")
	}
	if len(app.Fields) != 3 {
		t.Fatalf("AppConfig fields = %d, want 3", len(app.Fields))
	}

	// Database config.
	dbField := app.Fields[2]
	if !dbField.IsMessage {
		t.Fatal("database field should be a message")
	}
	db := dbField.MessageRef
	if db == nil {
		t.Fatal("database MessageRef is nil")
	}

	// Check secret annotation on dsn.
	dsn := db.Fields[0]
	if dsn.ProtoName != "dsn" {
		t.Errorf("dsn.ProtoName = %q, want %q", dsn.ProtoName, "dsn")
	}
	if !dsn.IsSecret {
		t.Error("dsn.IsSecret = false, want true")
	}

	// Check max_open_conns default.
	maxConns := db.Fields[1]
	if maxConns.GoName != "MaxOpenConns" {
		t.Errorf("max_open_conns.GoName = %q, want %q", maxConns.GoName, "MaxOpenConns")
	}
	if maxConns.Default == nil || maxConns.Default.GoLiteral != "25" {
		t.Errorf("max_open_conns default = %v, want 25", maxConns.Default)
	}

	// Check auto_migrate field type.
	autoMigrate := db.Fields[2]
	if autoMigrate.GoType != "bool" {
		t.Errorf("auto_migrate.GoType = %q, want %q", autoMigrate.GoType, "bool")
	}
	if autoMigrate.FlagType != "Bool" {
		t.Errorf("auto_migrate.FlagType = %q, want %q", autoMigrate.FlagType, "Bool")
	}

	// AppConfig is NOT public.
	if app.IsPublic {
		t.Error("AppConfig.IsPublic = true, want false")
	}

	// PublicConfig IS public.
	pub := schema.PublicField.MessageRef
	if !pub.IsPublic {
		t.Error("PublicConfig.IsPublic = false, want true")
	}

	// PublicAuthConfig is also public (nested under public).
	authField := pub.Fields[2]
	if authField.MessageRef == nil {
		t.Fatal("auth MessageRef is nil")
	}
	if !authField.MessageRef.IsPublic {
		t.Error("PublicAuthConfig.IsPublic = false, want true")
	}

	// JSON tags only on public fields.
	appName := pub.Fields[0]
	if appName.JSONTag != "appName" {
		t.Errorf("public app_name JSONTag = %q, want %q", appName.JSONTag, "appName")
	}
	portField := app.Fields[0]
	if portField.JSONTag != "" {
		t.Errorf("app port JSONTag = %q, want empty", portField.JSONTag)
	}

	// Repeated string default (scopes).
	auth := authField.MessageRef
	scopes := auth.Fields[2]
	if !scopes.IsRepeated {
		t.Error("scopes.IsRepeated = false, want true")
	}
	if scopes.GoType != "[]string" {
		t.Errorf("scopes.GoType = %q, want %q", scopes.GoType, "[]string")
	}
	if scopes.Default == nil {
		t.Fatal("scopes.Default is nil")
	}
	if !scopes.Default.IsSlice {
		t.Error("scopes.Default.IsSlice = false, want true")
	}

	// Descriptions from comments.
	if portField.Description == "" {
		t.Error("port.Description is empty, want comment text")
	}

	// AllMessages contains all messages.
	if len(schema.AllMessages) < 5 {
		t.Errorf("AllMessages len = %d, want >= 5", len(schema.AllMessages))
	}
}

func TestParseMissingFile(t *testing.T) {
	t.Parallel()

	_, err := ParseProto("/nonexistent/config.proto", ParseOptions{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseRejectsSecretInPublic(t *testing.T) {
	t.Parallel()

	_, err := ParseProto(testdataPath("secret_in_public.proto"), ParseOptions{})
	if err == nil {
		t.Fatal("expected error for secret field under public subtree")
	}
	if !strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "public") {
		t.Errorf("error = %q, want mention of secret and public", err.Error())
	}
}

func TestProtoNameToGoName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"port", "Port"},
		{"app_name", "AppName"},
		{"api_base_url", "APIBaseURL"},
		{"user_id", "UserID"},
		{"dsn", "DSN"},
		{"max_conn_idle_time", "MaxConnIdleTime"}, // "idle" must not become "IDle"
		{"client_id", "ClientID"},
		{"post_logout_redirect_path", "PostLogoutRedirectPath"},
		{"auto_migrate", "AutoMigrate"},
	}

	for _, tt := range tests {
		if got := protoNameToGoName(tt.input); got != tt.want {
			t.Errorf("protoNameToGoName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseDescriptions(t *testing.T) {
	t.Parallel()

	schema, err := ParseProto(testdataPath("minimal.proto"), ParseOptions{})
	if err != nil {
		t.Fatalf("ParseProto() error = %v", err)
	}

	port := schema.RootMessage.Fields[0].MessageRef.Fields[0]
	if port.Description != "HTTP server port" {
		t.Errorf("port.Description = %q, want %q", port.Description, "HTTP server port")
	}
}
