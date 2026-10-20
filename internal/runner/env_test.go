package runner

import (
	"strings"
	"testing"
)

func TestBuildEnvironmentAllowlist(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("GITHUB_TOKEN", "ghp_secret1234567890")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "aws_secret_key_value")
	t.Setenv("MY_DATABASE_PASSWORD", "dbpass123")
	t.Setenv("SOME_API_KEY", "key_secret")

	env := buildEnvironment(4, []string{"SAFE_VAR=hello", "FORBIDDEN_TOKEN=bad"})

	joined := strings.Join(env, "\n")

	// Secrets must NOT be present
	forbidden := []string{
		"GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY", "MY_DATABASE_PASSWORD",
		"SOME_API_KEY", "FORBIDDEN_TOKEN",
	}
	for _, f := range forbidden {
		if strings.Contains(joined, f) {
			t.Errorf("expected secret %s to be stripped from environment", f)
		}
	}

	// Safe variables must be present
	if !strings.Contains(joined, "PATH=") {
		t.Error("expected PATH in environment")
	}
	if !strings.Contains(joined, "GOMAXPROCS=4") {
		t.Error("expected GOMAXPROCS=4 in environment")
	}
	if !strings.Contains(joined, "SAFE_VAR=hello") {
		t.Error("expected SAFE_VAR=hello in environment")
	}
}
