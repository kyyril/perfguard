package runner

import (
	"fmt"
	"os"
	"strings"
)

// allowedEnvKeys defines the set of safe environment variables allowed to be passed
// to benchmark processes. All tokens, credentials, and secrets are excluded.
var allowedEnvKeys = map[string]bool{
	"PATH":        true,
	"SYSTEMROOT":  true,
	"WINDIR":      true,
	"COMSPEC":     true,
	"TEMP":        true,
	"TMP":         true,
	"TMPDIR":      true,
	"HOME":        true,
	"USERPROFILE": true,
	"HOMEDRIVE":   true,
	"HOMEPATH":    true,
	"GOPATH":      true,
	"GOROOT":      true,
	"GOCACHE":     true,
	"GOPROXY":     true,
	"GOSUMDB":     true,
	"GONOSUMDB":   true,
	"GOPRIVATE":   true,
	"LANG":        true,
	"LC_ALL":      true,
	"LC_CTYPE":    true,
	"TZ":          true,
	"TERM":        true,
}

// buildEnvironment constructs an isolated environment from os.Environ()
// using an allowlist of safe system keys.
func buildEnvironment(cpu int, extraEnv []string) []string {
	var env []string

	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 0 {
			continue
		}
		key := strings.ToUpper(parts[0])

		// Reject any potential secrets regardless of name
		if isSecretKey(key) {
			continue
		}

		if allowedEnvKeys[key] {
			env = append(env, entry)
		}
	}

	if cpu > 0 {
		env = append(env, fmt.Sprintf("GOMAXPROCS=%d", cpu))
	}

	for _, entry := range extraEnv {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 0 {
			continue
		}
		if !isSecretKey(strings.ToUpper(parts[0])) {
			env = append(env, entry)
		}
	}

	return env
}

func isSecretKey(key string) bool {
	secretKeywords := []string{
		"TOKEN", "SECRET", "PASSWORD", "PASS", "KEY",
		"AUTH", "CREDENTIAL", "PRIVATE", "APIKEY",
	}
	for _, kw := range secretKeywords {
		if strings.Contains(key, kw) {
			return true
		}
	}
	return false
}
