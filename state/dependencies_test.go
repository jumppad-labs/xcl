package state

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// addressPackage is the package that knows how an entity is addressed. Storage
// holds entities and answers nothing about addresses, so it must not reach for
// it, and a test cannot assert that an import fails to compile. The guard reads
// the sources instead, the way the examples guard what they may import
const addressPackage = "github.com/jumppad-labs/xcl/internal/resources"

// TestStateSourcesDoNotImportTheAddressPackage asserts no source of this
// package imports the address package, or anything else whose path names
// addresses. Test sources are excluded: a test may name an address to describe
// what it expects, it is the package itself that must not resolve one
func TestStateSourcesDoNotImportTheAddressPackage(t *testing.T) {
	root, err := filepath.Abs(".")
	require.NoError(t, err)

	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	scanned := 0

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}

		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, entry.Name()), nil, parser.ImportsOnly)
		require.NoError(t, err)

		scanned++

		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			require.NoError(t, err)

			require.NotEqual(t, addressPackage, path,
				"%s imports the address package, storage holds entities and resolves no addresses", entry.Name())

			require.NotContains(t, strings.ToLower(path), "address",
				"%s imports an address package, storage holds entities and resolves no addresses", entry.Name())

			require.NotContains(t, strings.ToLower(path), "fqrn",
				"%s imports an address package, storage holds entities and resolves no addresses", entry.Name())
		}
	}

	require.NotEmpty(t, scanned, "the guard scanned no sources, so it proves nothing")
}
