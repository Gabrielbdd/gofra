package configgen

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	_ "embed"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	gofraconfig "databit.com.br/gofra/internal/generate/config/gofraconfig"
)

//go:embed annotations.proto
var annotationsProtoSource string

// ParseOptions configures the proto parser.
type ParseOptions struct {
	// GoPackage is the Go package name for generated code (default: "config").
	GoPackage string

	// RuntimeImport is the framework's runtime/config import path.
	RuntimeImport string
}

// ParseProto compiles a .proto file and extracts a ConfigSchema from it.
// The proto must define a top-level message named "Config".
func ParseProto(protoPath string, opts ParseOptions) (*ConfigSchema, error) {
	if opts.GoPackage == "" {
		opts.GoPackage = "config"
	}

	absPath, err := filepath.Abs(protoPath)
	if err != nil {
		return nil, fmt.Errorf("configgen: abs path: %w", err)
	}

	protoDir := filepath.Dir(absPath)
	fileName := filepath.Base(absPath)

	compiler := protocompile.Compiler{
		Resolver: protocompile.CompositeResolver{
			// Well-known types from the Go protobuf global registry.
			registryResolver{},
			// Framework annotations (embedded) + user proto files (disk).
			&protocompile.SourceResolver{
				Accessor: sourceAccessor(protoDir),
			},
		},
		SourceInfoMode: protocompile.SourceInfoStandard,
		Reporter: reporter.NewReporter(
			func(err reporter.ErrorWithPos) error { return err },
			func(reporter.ErrorWithPos) {},
		),
	}

	files, err := compiler.Compile(context.Background(), fileName)
	if err != nil {
		return nil, fmt.Errorf("configgen: compile %s: %w", protoPath, err)
	}

	fd := files[0]

	configMsg := fd.Messages().ByName("Config")
	if configMsg == nil {
		return nil, fmt.Errorf("configgen: %s does not define a message named Config", protoPath)
	}

	schema := &ConfigSchema{
		GoPackage:     opts.GoPackage,
		RuntimeImport: opts.RuntimeImport,
	}

	// Build message map first so we can resolve references.
	msgMap := map[protoreflect.FullName]*MessageInfo{}
	buildMessages(fd, msgMap)

	// Identify the public field.
	for i := range configMsg.Fields().Len() {
		f := configMsg.Fields().Get(i)
		if string(f.Name()) == "public" && f.Kind() == protoreflect.MessageKind {
			markPublic(f.Message(), msgMap)
			break
		}
	}

	// Convert the root message.
	root := convertMessage(configMsg, msgMap, fd)
	schema.RootMessage = root

	// Collect all messages in order.
	var allMsgs []*MessageInfo
	collectMessages(root, &allMsgs, map[string]bool{})

	// Reverse so dependencies come first.
	for i, j := 0, len(allMsgs)-1; i < j; i, j = i+1, j-1 {
		allMsgs[i], allMsgs[j] = allMsgs[j], allMsgs[i]
	}
	schema.AllMessages = allMsgs

	// Find the public field reference.
	for i := range root.Fields {
		if root.Fields[i].ProtoName == "public" && root.Fields[i].IsMessage {
			schema.PublicField = &root.Fields[i]
			break
		}
	}

	return schema, nil
}

// registryResolver resolves well-known protos (e.g., google/protobuf/descriptor.proto)
// from the Go protobuf global registry.
type registryResolver struct{}

func (registryResolver) FindFileByPath(path string) (protocompile.SearchResult, error) {
	fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		return protocompile.SearchResult{}, err
	}
	return protocompile.SearchResult{Desc: fd}, nil
}

func sourceAccessor(protoDir string) func(path string) (io.ReadCloser, error) {
	return func(path string) (io.ReadCloser, error) {
		// Framework annotations proto (embedded).
		if path == "gofra/config/v1/annotations.proto" {
			return io.NopCloser(strings.NewReader(annotationsProtoSource)), nil
		}

		// Try the proto directory on disk.
		full := filepath.Join(protoDir, path)
		if f, err := os.Open(full); err == nil {
			return f, nil
		}

		return nil, os.ErrNotExist
	}
}

func buildMessages(fd protoreflect.FileDescriptor, out map[protoreflect.FullName]*MessageInfo) {
	for i := range fd.Messages().Len() {
		msg := fd.Messages().Get(i)
		buildMessageRecursive(msg, out)
	}
}

func buildMessageRecursive(msg protoreflect.MessageDescriptor, out map[protoreflect.FullName]*MessageInfo) {
	info := &MessageInfo{
		ProtoName:   string(msg.Name()),
		GoName:      string(msg.Name()),
		Description: extractComment(msg),
	}
	out[msg.FullName()] = info

	for i := range msg.Messages().Len() {
		buildMessageRecursive(msg.Messages().Get(i), out)
	}
}

func markPublic(msg protoreflect.MessageDescriptor, msgMap map[protoreflect.FullName]*MessageInfo) {
	if info, ok := msgMap[msg.FullName()]; ok {
		info.IsPublic = true
	}
	for i := range msg.Fields().Len() {
		f := msg.Fields().Get(i)
		if f.Kind() == protoreflect.MessageKind {
			markPublic(f.Message(), msgMap)
		}
	}
}

func convertMessage(msg protoreflect.MessageDescriptor, msgMap map[protoreflect.FullName]*MessageInfo, fd protoreflect.FileDescriptor) *MessageInfo {
	info := msgMap[msg.FullName()]

	for i := range msg.Fields().Len() {
		f := msg.Fields().Get(i)
		fi := convertField(f, info.IsPublic, msgMap, fd)
		info.Fields = append(info.Fields, fi)
	}

	return info
}

func convertField(f protoreflect.FieldDescriptor, parentIsPublic bool, msgMap map[protoreflect.FullName]*MessageInfo, fd protoreflect.FileDescriptor) FieldInfo {
	fi := FieldInfo{
		ProtoName:  string(f.Name()),
		GoName:     protoNameToGoName(string(f.Name())),
		KoanfTag:   string(f.Name()),
		Description: extractFieldComment(f, fd),
		IsRepeated: f.IsList(),
	}

	if f.Kind() == protoreflect.MessageKind {
		fi.IsMessage = true
		fi.GoType = string(f.Message().Name())
		if ref, ok := msgMap[f.Message().FullName()]; ok {
			fi.MessageRef = ref
			// Recursively convert the nested message.
			convertMessage(f.Message(), msgMap, fd)
		}
	} else {
		fi.GoType = protoKindToGoType(f.Kind(), f.IsList())
		fi.FlagType = protoKindToFlagType(f.Kind(), f.IsList())
	}

	// JSON tag only for fields in the public subtree.
	if parentIsPublic {
		fi.JSONTag = string(f.JSONName())
	}

	// Read gofra annotations.
	readFieldAnnotations(f, &fi)

	return fi
}

func readFieldAnnotations(f protoreflect.FieldDescriptor, fi *FieldInfo) {
	opts, ok := f.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return
	}

	// protocompile compiles annotations.proto from source, producing dynamic
	// message types for extensions. Re-marshal and unmarshal the options so
	// the Go protobuf runtime resolves extensions using our generated Go types.
	raw, err := proto.Marshal(opts)
	if err != nil {
		return
	}
	resolved := &descriptorpb.FieldOptions{}
	if err := proto.Unmarshal(raw, resolved); err != nil {
		return
	}

	if !proto.HasExtension(resolved, gofraconfig.E_Field) {
		return
	}

	ext := proto.GetExtension(resolved, gofraconfig.E_Field)
	fc, ok := ext.(*gofraconfig.FieldConfig)
	if !ok || fc == nil {
		return
	}

	if fc.Secret != nil && *fc.Secret {
		fi.IsSecret = true
	}

	if fc.DefaultValue != nil {
		fi.Default = convertDefault(fc.DefaultValue, f)
	}
}

func convertDefault(dv *gofraconfig.DefaultValue, f protoreflect.FieldDescriptor) *DefaultInfo {
	switch v := dv.Kind.(type) {
	case *gofraconfig.DefaultValue_StringValue:
		return &DefaultInfo{GoLiteral: fmt.Sprintf("%q", v.StringValue)}
	case *gofraconfig.DefaultValue_Int32Value:
		return &DefaultInfo{GoLiteral: fmt.Sprintf("%d", v.Int32Value)}
	case *gofraconfig.DefaultValue_Int64Value:
		return &DefaultInfo{GoLiteral: fmt.Sprintf("%d", v.Int64Value)}
	case *gofraconfig.DefaultValue_BoolValue:
		return &DefaultInfo{GoLiteral: fmt.Sprintf("%t", v.BoolValue)}
	case *gofraconfig.DefaultValue_DoubleValue:
		return &DefaultInfo{GoLiteral: fmt.Sprintf("%g", v.DoubleValue)}
	case *gofraconfig.DefaultValue_StringList:
		if v.StringList == nil || len(v.StringList.Values) == 0 {
			return &DefaultInfo{GoLiteral: "[]string{}", IsSlice: true}
		}
		parts := make([]string, len(v.StringList.Values))
		for i, s := range v.StringList.Values {
			parts[i] = fmt.Sprintf("%q", s)
		}
		return &DefaultInfo{
			GoLiteral: fmt.Sprintf("[]string{%s}", strings.Join(parts, ", ")),
			IsSlice:   true,
		}
	}
	return nil
}

func extractComment(msg protoreflect.MessageDescriptor) string {
	locs := msg.ParentFile().SourceLocations()

	// Try ByDescriptor first.
	if loc := locs.ByDescriptor(msg); loc.LeadingComments != "" {
		return cleanComment(loc.LeadingComments)
	}

	// Fallback: scan all locations for a matching path.
	for i := range locs.Len() {
		loc := locs.Get(i)
		if loc.LeadingComments != "" && locs.ByPath(loc.Path).LeadingComments != "" {
			// Match by full name via the path.
			desc := resolveDescriptorByPath(msg.ParentFile(), loc.Path)
			if desc != nil && desc.FullName() == msg.FullName() {
				return cleanComment(loc.LeadingComments)
			}
		}
	}

	return ""
}

func extractFieldComment(f protoreflect.FieldDescriptor, _ protoreflect.FileDescriptor) string {
	locs := f.ParentFile().SourceLocations()

	// Try ByDescriptor first.
	if loc := locs.ByDescriptor(f); loc.LeadingComments != "" {
		return cleanComment(loc.LeadingComments)
	}

	// Fallback: scan all locations for a matching path.
	for i := range locs.Len() {
		loc := locs.Get(i)
		if loc.LeadingComments != "" {
			desc := resolveDescriptorByPath(f.ParentFile(), loc.Path)
			if desc != nil && desc.FullName() == f.FullName() {
				return cleanComment(loc.LeadingComments)
			}
		}
	}

	return ""
}

// resolveDescriptorByPath follows a source_code_info path to find the descriptor.
func resolveDescriptorByPath(fd protoreflect.FileDescriptor, path protoreflect.SourcePath) protoreflect.Descriptor {
	if len(path) < 2 {
		return nil
	}

	// path[0]=4 means message_type, path[1] is the message index.
	if path[0] != 4 || int(path[1]) >= fd.Messages().Len() {
		return nil
	}
	msg := fd.Messages().Get(int(path[1]))

	if len(path) == 2 {
		return msg
	}

	// path[2]=2 means field, path[3] is the field index.
	if len(path) >= 4 && path[2] == 2 {
		if int(path[3]) >= msg.Fields().Len() {
			return nil
		}
		return msg.Fields().Get(int(path[3]))
	}

	// path[2]=3 means nested_type (nested message), path[3] is the index.
	if len(path) >= 4 && path[2] == 3 {
		if int(path[3]) >= msg.Messages().Len() {
			return nil
		}
		nested := msg.Messages().Get(int(path[3]))
		if len(path) == 4 {
			return nested
		}
		// Deeper nesting not handled.
	}

	return nil
}

func cleanComment(s string) string {
	s = strings.TrimSpace(s)
	// Remove trailing period for flag descriptions.
	s = strings.TrimSuffix(s, ".")
	return s
}

func collectMessages(msg *MessageInfo, out *[]*MessageInfo, seen map[string]bool) {
	if seen[msg.GoName] {
		return
	}
	seen[msg.GoName] = true
	*out = append(*out, msg)

	for _, f := range msg.Fields {
		if f.IsMessage && f.MessageRef != nil {
			collectMessages(f.MessageRef, out, seen)
		}
	}
}

func protoNameToGoName(name string) string {
	var b strings.Builder
	upper := true
	for _, r := range name {
		if r == '_' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			b.WriteRune(r)
		}
	}

	result := b.String()

	// Common Go naming fixes.
	result = strings.ReplaceAll(result, "Id", "ID")
	result = strings.ReplaceAll(result, "Url", "URL")
	result = strings.ReplaceAll(result, "Dsn", "DSN")
	result = strings.ReplaceAll(result, "Api", "API")

	return result
}

func protoKindToGoType(kind protoreflect.Kind, repeated bool) string {
	var base string
	switch kind {
	case protoreflect.StringKind:
		base = "string"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		base = "int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		base = "int64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		base = "uint32"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		base = "uint64"
	case protoreflect.BoolKind:
		base = "bool"
	case protoreflect.FloatKind:
		base = "float32"
	case protoreflect.DoubleKind:
		base = "float64"
	default:
		base = "string"
	}
	if repeated {
		return "[]" + base
	}
	return base
}

func protoKindToFlagType(kind protoreflect.Kind, repeated bool) string {
	if repeated {
		return "" // repeated fields not supported as flags
	}
	switch kind {
	case protoreflect.StringKind:
		return "String"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "Int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "Int64"
	case protoreflect.BoolKind:
		return "Bool"
	case protoreflect.FloatKind:
		return "Float32"
	case protoreflect.DoubleKind:
		return "Float64"
	default:
		return "String"
	}
}
