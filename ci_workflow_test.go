package xcl

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The workflow builds the module against the version go.mod declares as its
// minimum. Nothing else proves that path: the newest toolchain satisfies the
// go directive, so an ordinary build says nothing about the floor and a break
// in it stays silent until a consumer on an older Go reports it.
//
// These guards exist because the pin had drifted below the declared minimum
// once already, which no test would have caught.

const workflowPath = ".github/workflows/go.yml"

// goDirective returns the minimum Go version go.mod declares.
func goDirective(t *testing.T) string {
	t.Helper()

	mod, err := os.ReadFile("go.mod")
	require.NoError(t, err)

	m := regexp.MustCompile(`(?m)^go (\d+\.\d+(?:\.\d+)?)$`).FindStringSubmatch(string(mod))
	require.Len(t, m, 2, "go.mod should declare a go directive")

	return m[1]
}

// version returns a Go version as comparable parts, so 1.9 sorts below 1.22.
func version(t *testing.T, v string) []int {
	t.Helper()

	parts := strings.Split(v, ".")
	out := make([]int, 3)
	for i := range out {
		if i >= len(parts) {
			break
		}

		n, err := strconv.Atoi(parts[i])
		require.NoError(t, err, "version %q should be numeric", v)
		out[i] = n
	}

	return out
}

func below(a, b []int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}

	return false
}

func TestWorkflowBuildsAgainstTheMinimumSupportedGo(t *testing.T) {
	wf, err := os.ReadFile(workflowPath)
	require.NoError(t, err)

	floor := goDirective(t)

	require.Contains(t, string(wf), "GOTOOLCHAIN: go"+floor,
		"a job must pin GOTOOLCHAIN to the minimum go.mod declares, otherwise Go may silently use a newer toolchain")
}

func TestWorkflowPinsNoGoVersionBelowTheDeclaredMinimum(t *testing.T) {
	wf, err := os.ReadFile(workflowPath)
	require.NoError(t, err)

	floor := version(t, goDirective(t))

	pins := regexp.MustCompile(`go-version:\s*'?"?(\d+\.\d+(?:\.\d+)?)'?"?`).FindAllStringSubmatch(string(wf), -1)
	require.NotEmpty(t, pins, "the workflow should pin at least one Go version")

	for _, p := range pins {
		require.False(t, below(version(t, p[1]), floor),
			"workflow pins Go %s, below the %s that go.mod declares as the minimum", p[1], goDirective(t))
	}
}

func TestWorkflowRunsOnEveryChange(t *testing.T) {
	wf, err := os.ReadFile(workflowPath)
	require.NoError(t, err)

	require.Contains(t, string(wf), "push:", "the workflow should run on push")
	require.Contains(t, string(wf), "pull_request:", "the workflow should run on pull requests, so a change is checked before it lands")
}

func TestGoModDeclaresNoToolchainDirective(t *testing.T) {
	mod, err := os.ReadFile("go.mod")
	require.NoError(t, err)

	require.NotRegexp(t, `(?m)^toolchain `, string(mod),
		"a toolchain directive would override the GOTOOLCHAIN the minimum-version job sets, and defeat it")
}
