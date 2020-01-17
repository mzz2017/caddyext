package directives_test

import (
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/mzz2017/caddyext/directives"
)

// directiveSrc is the minimum source required for the directives manager to parse. We could
// embed the entire directives source here if needed, but we really just want to test the loading
// of import paths and the order of which the directives are inserted into direciveOrder.
var directivesSkeleton = []byte(`
package caddy

import (
	"github.com/mholt/caddy/caddy/https"
	"github.com/mholt/caddy/caddy/parse"
	"github.com/mholt/caddy/caddy/setup"
	"github.com/mholt/caddy/middleware"
)

var directiveOrder = []directive{}
`)

// tmpDirectives creates a temporary directives file, invokes f, then deletes the file from disk.
func tmpDirectives(f func(file string, err error)) {
	tmp, err := ioutil.TempFile(os.TempDir(), "caddyext_directives_test")
	if err == nil {
		defer os.Remove(tmp.Name())
		_, err = tmp.Write(directivesSkeleton)
	}

	if err == nil {
		f(tmp.Name(), nil)
	} else {
		f("", err)
	}
}

// importPaths returns a list of import paths for the given go source file
func importPaths(src string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		return nil, err
	}

	imps := astutil.Imports(fset, f)
	paths := make([]string, len(imps[0]))

	for i, imp := range imps[0] {
		if path := imp.Path; path != nil {
			paths[i], _ = strconv.Unquote(path.Value)
		}
	}

	return paths, nil
}

// TestAddDirectivesAfterSave tests the behavior of adding a directives to Caddy after its directives
// have been modified.
func TestAddDirectivesAfterSave(t *testing.T) {
	tmpDirectives(func(file string, err error) {
		if err != nil {
			t.Fatal(err)
		}

		// Create directives from file
		m, err := directives.NewFrom(file)
		if err != nil {
			t.Fatalf("Unexpected error creating directives manager:", err)
		}

		// Add directive, save and reload
		m.AddDirective("directive1", "github.com/mikepulaski/directive1")
		m.Save()

		m, err = directives.NewFrom(file)
		if err != nil {
			t.Fatalf("Unexpected error reloading directives manager:", err)
		}

		// Add directive, save and reload
		m.AddDirective("directive2", "github.com/mikepulaski/directive2")
		m.Save()

		m, err = directives.NewFrom(file)
		if err != nil {
			t.Fatalf("Unexpected error reloading directives manager:", err)
		}

		// Check directives
		if n := len(m.List()); n == 0 {
			t.Errorf("Expected two directives; have none")
		} else if n > 2 {
			t.Errorf("Expected two directives; have %d: %s", n, m.List())
		} else {
			directives := m.List()
			if actual, expected := directives[0].Name, "directive1"; actual != expected {
				t.Errorf("Unexpected Directive.Name: %s != %s", actual, expected)
			}
			if actual, expected := directives[1].Name, "directive2"; actual != expected {
				t.Errorf("Unexpected Directive.Name: %s != %s", actual, expected)
			}
		}

		// Check actual import paths
		imports, err := importPaths(file)
		if err != nil {
			t.Fatalf("Error getting import paths for output: %s", err)
		}

		directiveImports := []string{}
		for _, path := range imports {
			if strings.HasPrefix(path, "github.com/mholt/caddy") {
				continue
			}

			directiveImports = append(directiveImports, path)
		}

		if n := len(directiveImports); n == 0 {
			t.Errorf("Expected two directive import paths; have none")
		} else if n > 2 {
			t.Errorf("Expected two directive import paths; have %d: %s", n, directiveImports)
		} else {
			// Imports are prepended; check imports in reverse order
			if actual, expected := directiveImports[0], "github.com/mikepulaski/directive2"; actual != expected {
				t.Errorf("Unexpected import path: %s != %s", actual, expected)
			}
			if actual, expected := directiveImports[1], "github.com/mikepulaski/directive1"; actual != expected {
				t.Errorf("Unexpected import path: %s != %s", actual, expected)
			}
		}
	})
}
