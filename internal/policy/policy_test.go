package policy

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

// forbidden imports, anywhere in the shipped binary.
var forbidden = map[string]string{
	"net":       "I3: no network",
	"net/http":  "I3: no network",
	"net/rpc":   "I3: no network",
	"os/exec":   "I4: never runs other programs",
	"os/user":   "I1: reads /etc/passwd",
	"plugin":    "no code loading",
	"io/ioutil": "use internal/audit",
}

// fileFuncs touch the filesystem. Only internal/audit may call them.
var fileFuncs = map[string]bool{
	"Open": true, "OpenFile": true, "ReadFile": true, "WriteFile": true, "Create": true,
	"CreateTemp": true, "ReadDir": true, "Remove": true, "RemoveAll": true, "Rename": true,
	"Mkdir": true, "MkdirAll": true, "MkdirTemp": true, "Stat": true, "Lstat": true,
	"Readlink": true, "Chmod": true, "Chown": true, "Lchown": true, "Symlink": true,
	"Link": true, "Truncate": true, "DirFS": true, "Chdir": true, "Chtimes": true,
}

// syscallFileFuncs are the raw equivalents.
var syscallFileFuncs = map[string]bool{
	"Open": true, "Openat": true, "Creat": true, "Unlink": true, "Unlinkat": true, "Rmdir": true,
	"Mkdir": true, "Mkdirat": true, "Rename": true, "Renameat": true, "Chmod": true, "Chown": true,
	"Link": true, "Symlink": true, "Truncate": true, "Getdents": true, "ReadDirent": true, "Exec": true,
	"ForkExec": true, "StartProcess": true, "Socket": true, "Connect": true,
}

func sources(t *testing.T) map[string][]string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	pkgs := map[string][]string{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "testdata") {
			return filepath.SkipDir
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			rel, _ := filepath.Rel(root, filepath.Dir(p))
			pkgs[filepath.ToSlash(rel)] = append(pkgs[filepath.ToSlash(rel)], p)
		}
		return nil
	})
	if len(pkgs) < 5 {
		t.Fatalf("found only %d packages; is the walk broken?", len(pkgs))
	}
	return pkgs
}

func TestInvariants(t *testing.T) {
	fset := token.NewFileSet()
	for pkg, files := range sources(t) {
		for _, path := range files {
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly|parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				name, _ := strconv.Unquote(imp.Path.Value)
				if why, bad := forbidden[name]; bad || strings.HasPrefix(name, "net/") {
					t.Errorf("%s imports %s (%s)", path, name, why)
				}
				if name == "unsafe" && pkg != "internal/sandbox" && pkg != "internal/term" {
					t.Errorf("%s imports unsafe; only the sandbox and terminal code may", path)
				}
			}
			full, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(full, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				x, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				pos := fset.Position(sel.Pos())
				switch {
				case x.Name == "os" && fileFuncs[sel.Sel.Name] && pkg != "internal/audit":
					t.Errorf("%s: os.%s outside internal/audit", pos, sel.Sel.Name)
				case x.Name == "syscall" && syscallFileFuncs[sel.Sel.Name] &&
					!(pkg == "internal/sandbox" && sel.Sel.Name == "Open"):
					t.Errorf("%s: syscall.%s is not allowed", pos, sel.Sel.Name)
				}
				return true
			})
		}
	}
}

// The hook scripts must never eval anything but their own static text.
func TestHooksDontEvalOutput(t *testing.T) {
	for _, name := range []string{"bash.sh", "zsh.zsh"} {
		data, err := os.ReadFile(filepath.Join("..", "shell", name))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "#") || !strings.Contains(code, "eval") {
				continue
			}
			// The only evals rename the player's own not-found handler.
			if strings.Contains(code, `eval "_tenant_cnf_orig${_tenant_fn#`) ||
				strings.Contains(code, `eval "command_not_found_handle${fn#`) {
				continue
			}
			t.Errorf("%s:%d: unexpected eval: %s", name, i+1, code)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "print -s") || strings.Contains(line, "history -s") || strings.Contains(line, "fc -W") || strings.Contains(line, "history -w") {
				t.Errorf("%s:%d: writes history: %s", name, i+1, line)
			}
		}
	}
}
