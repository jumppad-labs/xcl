package xcl_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// exampleMains returns the main.go of every example program, a directory
// under example without one, such as the shared prettylog package, is not a
// program and is left out
func exampleMains(t *testing.T) []string {
	t.Helper()

	mains, err := filepath.Glob(filepath.Join("example", "*", "main.go"))
	require.NoError(t, err)
	require.NotEmpty(t, mains, "no example programs found, the checks would prove nothing")

	return mains
}

// parseGoFile parses the Go source file at path
func parseGoFile(t *testing.T, path string) *ast.File {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	require.NoError(t, err)

	return file
}

// callsTo returns every call in node to a function or method named name,
// whatever it is qualified with
func callsTo(node ast.Node, name string) []*ast.CallExpr {
	calls := []*ast.CallExpr{}

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			if fun.Sel.Name == name {
				calls = append(calls, call)
			}
		case *ast.Ident:
			if fun.Name == name {
				calls = append(calls, call)
			}
		}

		return true
	})

	return calls
}

// qualifiedCallsTo returns every call in node to qualifier.name, i.e.
// prettylog.Handler
func qualifiedCallsTo(node ast.Node, qualifier, name string) []*ast.CallExpr {
	calls := []*ast.CallExpr{}
	for _, call := range callsTo(node, name) {
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		ident, ok := selector.X.(*ast.Ident)
		if ok && ident.Name == qualifier {
			calls = append(calls, call)
		}
	}

	return calls
}

// mainFunc returns the declaration of func main in file
func mainFunc(t *testing.T, file *ast.File) *ast.FuncDecl {
	t.Helper()

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == "main" {
			return fn
		}
	}

	require.Fail(t, "no func main found")
	return nil
}

// TestExamplesPassTheirHandlerToWithEventHandlerOnce asserts every example
// configures xcl's events in exactly one place, handing over the handler run
// was given rather than building one of its own
func TestExamplesPassTheirHandlerToWithEventHandlerOnce(t *testing.T) {
	for _, path := range exampleMains(t) {
		file := parseGoFile(t, path)

		calls := callsTo(file, "WithEventHandler")
		require.Len(t, calls, 1, "%s must call WithEventHandler exactly once", path)
		require.Len(t, calls[0].Args, 1, "%s calls WithEventHandler with %d arguments", path, len(calls[0].Args))

		argument, ok := calls[0].Args[0].(*ast.Ident)
		require.True(t, ok, "%s passes an expression to WithEventHandler, pass the handler run was given", path)
		require.Equal(t, "handler", argument.Name, "%s passes %s to WithEventHandler", path, argument.Name)
	}
}

// TestExamplesMainSetsUpPrettyLogOnce asserts every example's main builds the
// shared pretty printer exactly once, the one line an application needs
func TestExamplesMainSetsUpPrettyLogOnce(t *testing.T) {
	for _, path := range exampleMains(t) {
		file := parseGoFile(t, path)

		calls := qualifiedCallsTo(mainFunc(t, file), "prettylog", "Handler")
		require.Len(t, calls, 1, "%s main must call prettylog.Handler exactly once", path)
	}
}

// TestExamplesDoNotImportLogger asserts no example program reaches for the
// logger package, everything it reports goes through the event handler
func TestExamplesDoNotImportLogger(t *testing.T) {
	for _, path := range exampleMains(t) {
		file := parseGoFile(t, path)

		for _, imp := range file.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			require.NoError(t, err)

			require.NotEqual(t, "github.com/jumppad-labs/xcl/logger", importPath, "%s imports the logger package", path)
		}
	}
}

// TestExamplesCreatePluginRegistryWithoutArguments asserts every example
// creates its registry with no arguments, the registry needs no logger
func TestExamplesCreatePluginRegistryWithoutArguments(t *testing.T) {
	registries := 0

	for _, path := range exampleMains(t) {
		file := parseGoFile(t, path)

		for _, call := range callsTo(file, "NewPluginRegistry") {
			require.Empty(t, call.Args, "%s passes arguments to NewPluginRegistry", path)
			registries++
		}
	}

	require.NotZero(t, registries, "no NewPluginRegistry calls found, the check would prove nothing")
}

// examplePluginSources returns the source files of the example plugins, the
// in-process and external ones of the plugin example and the person plugin
func examplePluginSources(t *testing.T) []string {
	t.Helper()

	sources := []string{}
	for _, pattern := range []string{
		filepath.Join("example", "plugin", "internal", "*.go"),
		filepath.Join("example", "plugin", "external", "*.go"),
		filepath.Join("plugins", "example", "pkg", "person", "*.go"),
	} {
		matches, err := filepath.Glob(pattern)
		require.NoError(t, err)
		require.NotEmpty(t, matches, "no sources match %s", pattern)

		sources = append(sources, matches...)
	}

	return sources
}

// logMethods are the methods a plugin logs through
var logMethods = []string{"Debug", "Info", "Warn", "Error"}

// TestExamplePluginsDoNotLogResourceOrEventDetails asserts no example plugin
// names the resource or the event in what it logs, the logger a provider is
// given during a call is already bound to both
func TestExamplePluginsDoNotLogResourceOrEventDetails(t *testing.T) {
	logCalls := 0

	for _, path := range examplePluginSources(t) {
		file := parseGoFile(t, path)

		for _, method := range logMethods {
			for _, call := range callsTo(file, method) {
				if _, ok := call.Fun.(*ast.SelectorExpr); !ok {
					continue
				}

				logCalls++

				for _, arg := range call.Args {
					literal, ok := arg.(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						continue
					}

					value, err := strconv.Unquote(literal.Value)
					require.NoError(t, err)

					require.NotEqual(t, "resource", value, "%s passes \"resource\" to %s", path, method)
					require.NotEqual(t, "event", value, "%s passes \"event\" to %s", path, method)
				}
			}
		}
	}

	require.NotZero(t, logCalls, "no log calls found in the example plugins, the check would prove nothing")
}
