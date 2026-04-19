// Package zitadelsecret reads a ZITADEL Personal Access Token from a file
// path or environment variable. File takes precedence; whitespace is trimmed.
//
// This package is consumer-facing. The generated starter does not import it.
package zitadelsecret

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrNotFound is returned when neither FilePath nor EnvVar yields a value.
var ErrNotFound = errors.New("zitadelsecret: no PAT found in file or env")

// Source describes where a PAT may come from. FilePath is tried first; on
// missing file (ENOENT) the reader falls through to EnvVar. Other filesystem
// errors are returned as-is.
type Source struct {
	FilePath string
	EnvVar   string
}

// Read resolves the PAT from src. Returned values have leading and trailing
// whitespace trimmed. An empty file is treated as a hard error — if the file
// exists but yields no token, callers almost certainly want to know rather
// than silently fall through to the environment.
func Read(src Source) (string, error) {
	if src.FilePath != "" {
		data, err := os.ReadFile(src.FilePath)
		switch {
		case err == nil:
			token := strings.TrimSpace(string(data))
			if token == "" {
				return "", fmt.Errorf("zitadelsecret: %q is empty", src.FilePath)
			}
			return token, nil
		case errors.Is(err, os.ErrNotExist):
			// Fall through to env.
		default:
			return "", fmt.Errorf("zitadelsecret: read %q: %w", src.FilePath, err)
		}
	}
	if src.EnvVar != "" {
		if v := strings.TrimSpace(os.Getenv(src.EnvVar)); v != "" {
			return v, nil
		}
	}
	return "", ErrNotFound
}
