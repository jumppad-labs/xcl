package xcl_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// charmPrefix is the module path prefix of the charmbracelet libraries the
// examples' pretty printer is built on
const charmPrefix = "github.com/charmbracelet/"

// requireGo skips the test when the go tool is not on the PATH
func requireGo(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("the go tool is not on the PATH")
	}
}

// goList runs go list with args and returns the lines it prints
func goList(t *testing.T, args ...string) []string {
	t.Helper()

	command := exec.Command("go", append([]string{"list"}, args...)...)
	output, err := command.Output()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		require.Failf(t, "go list failed", "go list %s: %s", strings.Join(args, " "), exitErr.Stderr)
	}
	require.NoError(t, err)

	return strings.Fields(string(output))
}

// libraryPackages returns every package in the module except the examples,
// the packages an application that uses xcl can import
func libraryPackages(t *testing.T) []string {
	t.Helper()

	packages := []string{}
	for _, pkg := range goList(t, "./...") {
		if strings.Contains(pkg, "/example/") {
			continue
		}

		packages = append(packages, pkg)
	}

	require.NotEmpty(t, packages)

	return packages
}

// TestLibraryDoesNotDependOnCharm asserts no package outside the examples
// depends on the charmbracelet libraries, so they never become a dependency
// of an application that uses xcl
func TestLibraryDoesNotDependOnCharm(t *testing.T) {
	requireGo(t)

	dependencies := goList(t, append([]string{"-deps"}, libraryPackages(t)...)...)
	require.NotEmpty(t, dependencies)

	for _, dependency := range dependencies {
		require.False(t, strings.HasPrefix(dependency, charmPrefix), "the library depends on %s", dependency)
	}
}

// TestPrettyLogDependsOnCharmLog asserts the dependency listing does show the
// charm logger where it is used, so the check above can not pass because the
// listing misses it
func TestPrettyLogDependsOnCharmLog(t *testing.T) {
	requireGo(t)

	dependencies := goList(t, "-deps", "./example/prettylog")

	require.Contains(t, dependencies, "github.com/charmbracelet/log")
}
