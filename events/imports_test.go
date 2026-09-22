package events

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEventsPackageImportsOnlyTheStandardLibrary asserts every non-test file
// of the package imports only standard library packages, whose import paths
// have no dot in their first element
func TestEventsPackageImportsOnlyTheStandardLibrary(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	fileSet := token.NewFileSet()
	checked := 0

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fileSet, name, nil, parser.ImportsOnly)
		require.NoError(t, err)

		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			require.NoError(t, err)

			first := strings.SplitN(path, "/", 2)[0]
			require.NotContains(t, first, ".", "%s imports %s, which is not in the standard library", name, path)
		}

		checked++
	}

	require.Greater(t, checked, 0)
}
