package state

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The container that held entities moved out of this package: storing entities
// and answering questions about them are different jobs, so what persists them
// exchanges a plain slice and offers nothing to construct. A test cannot assert
// that a reference fails to compile, so these guards read the sources instead,
// the way dependencies_test.go guards what this package may import.

// packageSource is one parsed source of this package, named so that a guard
// can say which file broke it.
type packageSource struct {
	name string
	file *ast.File
}

// packageSources parses every non test source of this package, so a guard can
// look at what the package declares rather than at what a caller happens to
// use. Test sources are excluded: a test may name a container of its own, it is
// the package's own surface that must offer none.
func packageSources(t *testing.T) []packageSource {
	t.Helper()

	root, err := filepath.Abs(".")
	require.NoError(t, err)

	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	files := []packageSource{}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}

		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, entry.Name()), nil, 0)
		require.NoError(t, err)

		files = append(files, packageSource{name: entry.Name(), file: file})
	}

	require.NotEmpty(t, files, "the guard scanned no sources, so it proves nothing")

	return files
}

// TestStatePackageExportsNoStateType asserts nothing in this package's public
// surface is an exported State type, so a caller has no container to hold.
func TestStatePackageExportsNoStateType(t *testing.T) {
	for _, source := range packageSources(t) {
		for _, declaration := range source.file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}

			for _, spec := range general.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				require.NotEqual(t, "State", typeSpec.Name.Name,
					"%s exports a State type, persistence exchanges a plain slice of entities", source.name)
			}
		}
	}
}

// TestStatePackageExportsNoNewStateFunction asserts nothing in this package's
// public surface is an exported NewState function, so a caller has nothing to
// construct before saving.
func TestStatePackageExportsNoNewStateFunction(t *testing.T) {
	for _, source := range packageSources(t) {
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil {
				continue
			}

			require.NotEqual(t, "NewState", function.Name.Name,
				"%s exports a NewState function, persistence exchanges a plain slice of entities", source.name)
		}
	}
}
