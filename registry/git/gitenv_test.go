package git

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestGitEnv_DropsGitVariablesAndHardens(t *testing.T) {
	t.Setenv("GIT_DIR", "/somewhere/.git")
	t.Setenv("GIT_SSH_COMMAND", "ssh -i /secret/key")
	t.Setenv("GIT_ALLOW_PROTOCOL", "ext")
	t.Setenv("EVENT_SPEC_PROBE", "kept")

	env := gitEnv()

	for _, kv := range env {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(key) {
		case "GIT_TERMINAL_PROMPT", "GIT_ALLOW_PROTOCOL":
			// Added back below; checked by value.
		default:
			if strings.HasPrefix(strings.ToUpper(key), "GIT_") {
				t.Errorf("gitEnv kept %s", key)
			}
		}
	}

	// The two hardening defaults replace, rather than inherit, whatever the
	// caller had: a caller-supplied GIT_ALLOW_PROTOCOL=ext must not survive.
	for _, want := range []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ALLOW_PROTOCOL=file:git:http:https:ssh",
		"EVENT_SPEC_PROBE=kept",
	} {
		if !slices.Contains(env, want) {
			t.Errorf("gitEnv missing %s", want)
		}
	}
}

func TestStrippedExecVarsInEnv(t *testing.T) {
	for _, key := range strippedExecVars {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	if got := strippedExecVarsInEnv(); len(got) != 0 {
		t.Fatalf("want no stripped vars reported, got %v", got)
	}

	t.Setenv("GIT_SSH_COMMAND", "ssh -i /secret/key")
	t.Setenv("GIT_ASKPASS", "/secret/askpass")

	got := strippedExecVarsInEnv()
	if !slices.Contains(got, "GIT_SSH_COMMAND") || !slices.Contains(got, "GIT_ASKPASS") {
		t.Errorf("got %v, want it to name GIT_SSH_COMMAND and GIT_ASKPASS", got)
	}
	for _, name := range got {
		if strings.Contains(name, "secret") {
			t.Errorf("%q leaks a variable value; report names only", name)
		}
	}
}
