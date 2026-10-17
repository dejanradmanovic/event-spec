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

// guardAllowedFiles are the module-relative files permitted to exec git
// directly: the helper itself, its test, and the registry resolver, which is
// production code and applies its own equivalent scrub in gitEnv().
var guardAllowedFiles = map[string]bool{
	"testutil/gitenv.go":       true,
	"testutil/gitenv_test.go":  true,
	"registry/git/resolver.go": true,
}

// gitExecCall is one `exec.Command("git", …)` call site found by the guard.
type gitExecCall struct {
	File string
	Line int
}

func (c gitExecCall) String() string { return c.File + ":" + strconv.Itoa(c.Line) }

// TestNoUnguardedGitExec fails if any Go file in the module execs git without
// going through testutil.GitCommand. It scans production code as well as
// tests: the bug in #184 shipped in registry/git/resolver.go, not in a test.
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
		if !strings.HasSuffix(d.Name(), ".go") {
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
		t.Errorf("%s: execs git directly, so it inherits GIT_DIR from any ancestor "+
			"git hook and can retarget the contributor's real repository (issue #184); "+
			"use testutil.GitCommand or testutil.RunGit", call)
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
			name: "aliased os/exec import",
			src: `package p
import xc "os/exec"
func f() { _ = xc.Command("git", "status") }
`,
			want: []int{3},
		},
		{
			name: "dot-imported os/exec",
			src: `package p
import . "os/exec"
func f() { _ = Command("git", "status") }
`,
			want: []int{3},
		},
		{
			name: "an unrelated package named exec is not os/exec",
			src: `package p
import "example.com/exec"
func f() { _ = exec.Command("git", "status") }
`,
			want: nil,
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
			calls, err := findGitExecCalls("fixture.go", []byte(tt.src))
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

// findGitExecCalls parses src and reports every os/exec Command or
// CommandContext call whose command literal is "git". The local name of the
// os/exec import is resolved per file, so an alias does not dodge the check.
func findGitExecCalls(name string, src []byte) ([]gitExecCall, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	pkgName, ok := execPackageName(file)
	if !ok {
		return nil, nil
	}

	var calls []gitExecCall
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, ok := execFuncName(call.Fun, pkgName)
		if !ok {
			return true
		}

		var nameArg int
		switch fn {
		case "Command":
			nameArg = 0
		case "CommandContext":
			nameArg = 1
		default:
			return true
		}
		if len(call.Args) > nameArg && isStringLit(call.Args[nameArg], "git") {
			calls = append(calls, gitExecCall{File: name, Line: fset.Position(call.Pos()).Line})
		}
		return true
	})
	return calls, nil
}

// execPackageName returns the identifier os/exec is bound to in file ("." for
// a dot import), and whether the file imports it at all.
func execPackageName(file *ast.File) (string, bool) {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != "os/exec" {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name, true
		}
		return "exec", true
	}
	return "", false
}

// execFuncName returns the os/exec function fun refers to, given the local
// name of the os/exec import.
func execFuncName(fun ast.Expr, pkgName string) (string, bool) {
	if pkgName == "." {
		ident, ok := fun.(*ast.Ident)
		if !ok {
			return "", false
		}
		return ident.Name, true
	}
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != pkgName {
		return "", false
	}
	return sel.Sel.Name, true
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
