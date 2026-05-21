package spec

import (
	"fmt"
	"os"
	"strings"
)

// ResolveSecret resolves a credential value based on its declared secret type.
// Supported types:
//
//	"env_var"  — value is an env var name or "${VAR}" reference; reads from the environment.
//	"file"     — value is a file path; reads and trims the file contents.
//	"inline"   — value is the literal secret (dev only).
//	""         — treated as "inline".
func ResolveSecret(value, secretType string) (string, error) {
	switch secretType {
	case "", "inline":
		return value, nil
	case "env_var":
		name := value
		if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
			name = value[2 : len(value)-1]
		}
		v := os.Getenv(name)
		if v == "" {
			return "", fmt.Errorf("env var %q is not set", name)
		}
		return v, nil
	case "file":
		data, err := os.ReadFile(value)
		if err != nil {
			return "", fmt.Errorf("read secret file %q: %w", value, err)
		}
		return strings.TrimSpace(string(data)), nil
	default:
		return "", fmt.Errorf("unknown secret type %q; expected env_var, file, or inline", secretType)
	}
}
