package xcl_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// excludedDirectories are the module directories, relative to the module
// root, whose Go files are not library code this check governs
var excludedDirectories = []string{
	// vendored copy of an upstream package, not code xcl owns
	"internal/xcl",
	// vendored copy of the upstream cty package, not code xcl owns
	"internal/cty",
	// example applications, which are free to write output of their own
	"example",
	// Spektacular's project store, holds no library code
	".spektacular",
}

// excludedFixtureDirectoryNames are directory names that hold test data and
// fixtures wherever they appear in the module
var excludedFixtureDirectoryNames = []string{
	"testdata",
	"test_fixtures",
}

// excludedFiles are single library files, relative to the module root, that
// this check does not govern
var excludedFiles = []string{
	// an unused resource printer rather than logging, it writes only when
	// called explicitly and is out of scope for event based logging
	"logger/pretty_printer.go",
}

// moduleRoot returns the module root. The test runs in the root package
// directory, which is the module root, and the go.mod there proves it.
func moduleRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(".")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(root, "go.mod"))
	require.NoError(t, err, "the test must run in the module root, which holds go.mod")

	return root
}

// libraryFiles returns the path, relative to the module root, of every non
// test Go file in the module that is not excluded
func libraryFiles(t *testing.T, root string) []string {
	t.Helper()

	files := []string{}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)

		if entry.IsDir() {
			for _, excluded := range excludedDirectories {
				if relative == excluded {
					return filepath.SkipDir
				}
			}

			for _, name := range excludedFixtureDirectoryNames {
				if entry.Name() == name {
					return filepath.SkipDir
				}
			}

			return nil
		}

		if !strings.HasSuffix(relative, ".go") || strings.HasSuffix(relative, "_test.go") {
			return nil
		}

		for _, excluded := range excludedFiles {
			if relative == excluded {
				return nil
			}
		}

		files = append(files, relative)

		return nil
	})
	require.NoError(t, err)

	return files
}

// parseLibraryFile parses the imports and body of a library file
func parseLibraryFile(t *testing.T, root, relative string) *ast.File {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, relative), nil, parser.SkipObjectResolution)
	require.NoError(t, err, "parsing %s", relative)

	return file
}

// TestLibraryFilesAreFound asserts the walk finds library files at all, so the
// checks below cannot pass by looking at nothing
func TestLibraryFilesAreFound(t *testing.T) {
	root := moduleRoot(t)

	files := libraryFiles(t, root)

	require.Contains(t, files, "config.go")
	require.Contains(t, files, "internal/parser/parser.go")
}

// TestExcludedFilesAreNotChecked asserts the walk leaves out the listed
// exclusions, so an exclusion that stops matching is noticed
func TestExcludedFilesAreNotChecked(t *testing.T) {
	root := moduleRoot(t)

	files := libraryFiles(t, root)

	require.NotContains(t, files, "logger/pretty_printer.go")
	require.NotContains(t, files, "internal/xcl/diagnostic.go")
}

// TestLibraryFilesDoNotImportStandardLogger asserts no library file imports
// the standard library "log" package, xcl reports everything as events
func TestLibraryFilesDoNotImportStandardLogger(t *testing.T) {
	root := moduleRoot(t)

	// logImportAllowed are files that must name the standard library logger
	// type without using the global logger
	logImportAllowed := []string{
		// hclog.Logger's StandardLogger method returns a *log.Logger, the
		// adapter builds a private one that writes back into the adapter
		"plugins/hclog_adapter.go",
	}

	offenders := []string{}
	for _, relative := range libraryFiles(t, root) {
		file := parseLibraryFile(t, root, relative)

		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			require.NoError(t, err)

			if path == "log" && !slices.Contains(logImportAllowed, relative) {
				offenders = append(offenders, relative)
			}
		}
	}

	require.Empty(t, offenders, "library files import the standard library log package")
}

// TestLibraryFilesDoNotRedirectStandardLogger asserts no library file calls
// log.SetOutput, the standard library logger belongs to the application
func TestLibraryFilesDoNotRedirectStandardLogger(t *testing.T) {
	root := moduleRoot(t)

	offenders := []string{}
	for _, relative := range libraryFiles(t, root) {
		file := parseLibraryFile(t, root, relative)

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			receiver, ok := selector.X.(*ast.Ident)
			if ok && receiver.Name == "log" && selector.Sel.Name == "SetOutput" {
				offenders = append(offenders, relative)
			}

			return true
		})
	}

	require.Empty(t, offenders, "library files redirect the standard library logger")
}

// TestLibraryFilesDoNotPrintWithFmt asserts no library file calls
// fmt.Print, fmt.Printf or fmt.Println, xcl reports everything as events and
// never writes to the process's standard output itself
func TestLibraryFilesDoNotPrintWithFmt(t *testing.T) {
	root := moduleRoot(t)

	printFunctions := []string{"Print", "Printf", "Println"}

	offenders := []string{}
	for _, relative := range libraryFiles(t, root) {
		file := parseLibraryFile(t, root, relative)

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			receiver, ok := selector.X.(*ast.Ident)
			if ok && receiver.Name == "fmt" && slices.Contains(printFunctions, selector.Sel.Name) {
				offenders = append(offenders, relative+": fmt."+selector.Sel.Name)
			}

			return true
		})
	}

	require.Empty(t, offenders, "library files print to standard output with fmt")
}

// TestLibraryFilesDoNotNameStandardStreams asserts no library file refers to
// os.Stdout or os.Stderr, the process's streams belong to the application
func TestLibraryFilesDoNotNameStandardStreams(t *testing.T) {
	root := moduleRoot(t)

	streams := []string{"Stdout", "Stderr"}

	offenders := []string{}
	for _, relative := range libraryFiles(t, root) {
		file := parseLibraryFile(t, root, relative)

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			receiver, ok := selector.X.(*ast.Ident)
			if ok && receiver.Name == "os" && slices.Contains(streams, selector.Sel.Name) {
				offenders = append(offenders, relative+": os."+selector.Sel.Name)
			}

			return true
		})
	}

	require.Empty(t, offenders, "library files refer to the standard output or error stream")
}

// loggerMethods are the methods of logger.Logger, a type that declares all of
// them implements it
var loggerMethods = []string{"Debug", "Info", "Warn", "Error"}

// receiverTypeName returns the name of the type a method is declared on,
// without the pointer or type parameters
func receiverTypeName(expr ast.Expr) string {
	switch receiver := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(receiver.X)
	case *ast.IndexExpr:
		return receiverTypeName(receiver.X)
	case *ast.IndexListExpr:
		return receiverTypeName(receiver.X)
	case *ast.Ident:
		return receiver.Name
	}

	return ""
}

// hasLoggerSignature reports whether a method has the signature of the
// logger.Logger methods, (msg string, args ...any) with no results. Methods
// that only share a name, such as the gRPC host callback service's Debug,
// which takes a context and a request, do not implement logger.Logger.
func hasLoggerSignature(function *ast.FuncType) bool {
	if function.Results != nil && len(function.Results.List) > 0 {
		return false
	}

	parameters := []ast.Expr{}
	for _, field := range function.Params.List {
		// a field declares one parameter per name, or one unnamed parameter
		count := max(len(field.Names), 1)
		for range count {
			parameters = append(parameters, field.Type)
		}
	}

	if len(parameters) != 2 {
		return false
	}

	message, ok := parameters[0].(*ast.Ident)
	if !ok || message.Name != "string" {
		return false
	}

	variadic, ok := parameters[1].(*ast.Ellipsis)
	if !ok {
		return false
	}

	switch element := variadic.Elt.(type) {
	case *ast.Ident:
		return element.Name == "any"
	case *ast.InterfaceType:
		return len(element.Methods.List) == 0
	}

	return false
}

// loggerImplementationsAllowed are files whose types declare the logger
// methods for a reason other than logging themselves
var loggerImplementationsAllowed = []string{
	// GRPCLogger runs in an external plugin's process and forwards every
	// message to the host's event stream
	"plugins/grpc_clients.go",
	// hclogAdapter implements hclog.Logger, which has the same methods, and
	// forwards every message to an event logger
	"plugins/hclog_adapter.go",
}

// loggerImplementations returns every library type outside the logger
// package that declares all the logger.Logger methods, keyed
// "<package directory>.<type name>", with the files declaring those methods.
// The methods of a type are gathered across the files of its package.
func loggerImplementations(t *testing.T, root string) map[string][]string {
	t.Helper()

	// methods maps a type to the logger methods declared on it and the file
	// declaring each
	methods := map[string]map[string]string{}

	for _, relative := range libraryFiles(t, root) {
		if strings.HasPrefix(relative, "logger/") {
			continue
		}

		file := parseLibraryFile(t, root, relative)

		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || len(function.Recv.List) == 0 {
				continue
			}

			if !slices.Contains(loggerMethods, function.Name.Name) || !hasLoggerSignature(function.Type) {
				continue
			}

			typeName := filepath.ToSlash(filepath.Dir(relative)) + "." + receiverTypeName(function.Recv.List[0].Type)
			if methods[typeName] == nil {
				methods[typeName] = map[string]string{}
			}

			methods[typeName][function.Name.Name] = relative
		}
	}

	implementations := map[string][]string{}
	for typeName, declared := range methods {
		if len(declared) != len(loggerMethods) {
			continue
		}

		files := []string{}
		for _, relative := range declared {
			if !slices.Contains(files, relative) {
				files = append(files, relative)
			}
		}

		slices.Sort(files)
		implementations[typeName] = files
	}

	return implementations
}

// TestLoggerImplementationScanFindsAllowedTypes asserts the scan recognises
// the allowed logger implementations, so the check below cannot pass by
// recognising nothing
func TestLoggerImplementationScanFindsAllowedTypes(t *testing.T) {
	root := moduleRoot(t)

	implementations := loggerImplementations(t, root)

	require.Equal(t, []string{"plugins/grpc_clients.go"}, implementations["plugins.GRPCLogger"])
	require.Equal(t, []string{"plugins/hclog_adapter.go"}, implementations["plugins.hclogAdapter"])
}

// TestOnlyLoggerPackageImplementsLogger asserts no library type outside the
// logger package declares every logger.Logger method, so logging is only
// ever done through the event logger the logger package provides, apart from
// the allowed forwarding types
func TestOnlyLoggerPackageImplementsLogger(t *testing.T) {
	root := moduleRoot(t)

	offenders := []string{}
	for typeName, files := range loggerImplementations(t, root) {
		for _, relative := range files {
			if !slices.Contains(loggerImplementationsAllowed, relative) {
				offenders = append(offenders, typeName+" in "+relative)
			}
		}
	}

	slices.Sort(offenders)
	require.Empty(t, offenders, "library types outside the logger package implement logger.Logger")
}
