package testutil_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// guardSkipDirs are directory names the guard never descends into: sibling
// worktrees carry a full second copy of the module, and node_modules/sdk hold
// generated or vendored sources that are not part of this Go module.
var guardSkipDirs = map[string]bool{
	".git":         true,
	".worktrees":   true,
	"node_modules": true,
	"sdk":          true,
}

// guardAllowedFiles are the module-relative test files permitted to exec git
// directly, because they are the ones testing the helper itself.
var guardAllowedFiles = map[string]bool{
	"testutil/gitenv_test.go": true,
}

// gitExecCall is one `exec.Command("git", …)` call site found by the guard.
type gitExecCall struct {
	File string
	Line int
}

func (c gitExecCall) String() string { return c.File + ":" + strconv.Itoa(c.Line) }

// TestNoUnguardedGitExec fails if a test file execs git without going through
// testutil.GitCommand. An unguarded git inherits GIT_DIR from the environment,
// which overrides cmd.Dir and retargets the contributor's own repository when
// the tests run from a git hook (#184).
func TestNoUnguardedGitExec(t *testing.T) {
	root := moduleRoot(t)

	var offenders []gitExecCall
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && guardSkipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if guardAllowedFiles[rel] {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		calls, err := findGitExecCalls(rel, src)
		if err != nil {
			return err
		}
		offenders = append(offenders, calls...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk module: %v", err)
	}

	for _, call := range offenders {
		t.Errorf("%s: execs git directly; use testutil.GitCommand or testutil.RunGit", call)
	}
}

// TestFindGitExecCalls proves the detector fires, so a green
// TestNoUnguardedGitExec means something.
func TestFindGitExecCalls(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []int
	}{
		{
			name: "exec.Command with git",
			src: `package p
import "os/exec"
func f() { _ = exec.Command("git", "status") }
`,
			want: []int{3},
		},
		{
			name: "exec.CommandContext with git",
			src: `package p
import (
	"context"
	"os/exec"
)
func f() { _ = exec.CommandContext(context.Background(), "git", "init") }
`,
			want: []int{6},
		},
		{
			name: "exec.LookPath is not a call site",
			src: `package p
import "os/exec"
func f() { _, _ = exec.LookPath("git") }
`,
			want: nil,
		},
		{
			name: "another binary is fine",
			src: `package p
import "os/exec"
func f() { _ = exec.Command("go", "test") }
`,
			want: nil,
		},
		{
			name: "git as a plain string is fine",
			src: `package p
func f() string { return "git" }
`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, err := findGitExecCalls("fixture_test.go", []byte(tt.src))
			if err != nil {
				t.Fatalf("findGitExecCalls: %v", err)
			}
			var got []int
			for _, c := range calls {
				got = append(got, c.Line)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want lines %v", calls, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("call %d at line %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// findGitExecCalls parses src and reports every exec.Command or
// exec.CommandContext whose command literal is "git".
func findGitExecCalls(name string, src []byte) ([]gitExecCall, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}

	var calls []gitExecCall
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "exec" {
			return true
		}

		var nameArg int
		switch sel.Sel.Name {
		case "Command":
			nameArg = 0
		case "CommandContext":
			nameArg = 1
		default:
			return true
		}
		if len(call.Args) <= nameArg {
			return true
		}
		if isStringLit(call.Args[nameArg], "git") {
			calls = append(calls, gitExecCall{File: name, Line: fset.Position(call.Pos()).Line})
		}
		return true
	})
	return calls, nil
}

// isStringLit reports whether expr is the string literal want.
func isStringLit(expr ast.Expr, want string) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(lit.Value)
	return err == nil && value == want
}

// moduleRoot walks up from the working directory to the directory holding
// go.mod.
func moduleRoot(tb testing.TB) string {
	tb.Helper()
	dir, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
