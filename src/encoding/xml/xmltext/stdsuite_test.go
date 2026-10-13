//go:build goexperiment.xmlv2

package xmltext

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// stdSuiteInputs returns every distinct XML-looking string constant in the
// frozen encoding/xml test files (inputs of the Token/RawToken/syntax/parse
// error/char/autoclose/entity tests, feeds, paths, marshal fixtures). The
// Marshal/Unmarshal/Encoder tests also need a binder and writer, which do not
// exist yet; their *inputs* are still tokenized here.
func stdSuiteInputs(t testing.TB) []string {
	files, err := filepath.Glob("../*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no legacy test files: %v", err)
	}
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if strings.Contains(s, "<") && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	var fold func(e ast.Expr) (string, bool)
	fold = func(e ast.Expr) (string, bool) {
		switch v := e.(type) {
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				return "", false
			}
			s, err := strconv.Unquote(v.Value)
			return s, err == nil
		case *ast.BinaryExpr:
			if v.Op != token.ADD {
				return "", false
			}
			l, ok1 := fold(v.X)
			r, ok2 := fold(v.Y)
			return l + r, ok1 && ok2
		case *ast.ParenExpr:
			return fold(v.X)
		}
		return "", false
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if e, ok := n.(ast.Expr); ok {
				if s, ok := fold(e); ok {
					add(s)
					if _, bin := e.(*ast.BinaryExpr); bin {
						return false // folded whole; skip the pieces
					}
				}
			}
			return true
		})
	}
	return out
}

// TestStdSuiteInputsAgainstLegacy runs every such input through the frozen
// v1 decoder and xmltext under each v1-equivalent profile (Strict=true,
// Strict=false, Strict=false with HTML AutoClose/Entity) and requires equal
// acceptance and tokens (rejections need the same token prefix).
func TestStdSuiteInputsAgainstLegacy(t *testing.T) {
	inputs := stdSuiteInputs(t)
	t.Logf("%d distinct XML-looking constants from the encoding/xml tests", len(inputs))
	cfgs := map[string]legacyCfg{
		"strict": strictCfg,
		"loose":  looseCfg,
		"html":   {autoClose: htmlAuto, entity: htmlEntity},
	}
	bad, bothOK, bothReject := 0, 0, 0
	for name, c := range cfgs {
		for _, in := range inputs {
			_, werr := legacyRun(in, c)
			if _, gerr := readAll2(NewDecoderBytes([]byte(in), c.options()...)); werr == nil && gerr == nil {
				bothOK++
			} else if werr != nil && gerr != nil {
				bothReject++
			}
			if d := diffLegacy(t, in, c); d != "" {
				bad++
				t.Errorf("[%s] %q\n%s", name, in, d)
			}
		}
	}
	t.Logf("%d comparisons: %d accepted by both, %d rejected by both, %d differences", len(inputs)*len(cfgs), bothOK, bothReject, bad)
	if bothOK < 100 || bothReject < 50 {
		t.Errorf("suspiciously few accepted (%d) or rejected (%d) inputs", bothOK, bothReject)
	}
}
