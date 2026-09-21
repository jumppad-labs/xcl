package xcl

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/stretchr/testify/require"
)

// The lookup surface replaced a helper that was constructed for one Go type
// and answered only for that type. These tests hold the two properties that
// made the replacement worth making: one configuration answers for as many
// types as the caller asks about, and the helper it replaced is gone rather
// than merely unused.

// A lookup names its type at the call, not at a construction, so a caller asks
// a configuration about whatever type they like, one after another, with
// nothing to set up in between.

func TestConsecutiveLookupsOfDifferentTypesAllSucceed(t *testing.T) {
	c := setupFindConfig(t)

	database, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)
	require.Equal(t, "resource.database.main", database.Meta.ID)

	app, err := Find[registered.App](c, "resource.app.web")
	require.NoError(t, err)
	require.Equal(t, "resource.app.web", app.Meta.ID)

	consumers, err := FindByType[registered.Consumer](c, "resource", "consumer")
	require.NoError(t, err)
	require.Len(t, consumers, 1)
	require.Equal(t, "resource.consumer.reader", consumers[0].Meta.ID)
}

// vendoredPrefixes are the trees vendored into this repository, they are
// nobody's code to change here and are not scanned.
var vendoredPrefixes = []string{
	filepath.Join("internal", "xcl"),
	filepath.Join("internal", "cty"),
}

// TestNoCodeReferencesTheSupersededQuerier asserts the superseded helper is
// gone from the repository rather than left behind unused: no call to its
// constructor, and no mention of the type itself, anywhere a caller could copy
// it from. A test cannot assert that code fails to compile, so the guard reads
// the sources instead, the way the examples guard what they may import.
func TestNoCodeReferencesTheSupersededQuerier(t *testing.T) {
	root, err := filepath.Abs(".")
	require.NoError(t, err)

	scanned := 0

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)

		relative, err := filepath.Rel(root, path)
		require.NoError(t, err)

		if d.IsDir() {
			if relative == "." {
				return nil
			}

			if strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}

			for _, vendored := range vendoredPrefixes {
				if relative == vendored {
					return fs.SkipDir
				}
			}

			return nil
		}

		if filepath.Ext(path) != ".go" {
			return nil
		}

		// Honour build constraints. query_methods_go127.go holds generic
		// methods, which the Go 1.25 parser cannot read at all, and on that
		// toolchain the file is not part of the build either — so a guard
		// over "the files this build contains" must skip it rather than
		// choke on it.
		included, err := build.Default.MatchFile(filepath.Dir(path), filepath.Base(path))
		require.NoError(t, err)

		if !included {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		require.NoError(t, err)

		scanned++

		// the names are matched as identifiers rather than as text, so the
		// guard does not trip over its own description of what it forbids
		ast.Inspect(file, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}

			require.NotEqual(t, "NewQuerier", ident.Name,
				"%s still constructs the superseded querier, use the lookup surface in query.go", relative)

			require.NotEqual(t, "Querier", ident.Name,
				"%s still names the superseded querier, use the lookup surface in query.go", relative)

			return true
		})

		return nil
	})
	require.NoError(t, err)

	require.NotEmpty(t, scanned, "the guard scanned no sources, so it proves nothing")
}
