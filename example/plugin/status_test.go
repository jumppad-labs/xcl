package main

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/types"
)

// plainRenderer returns a renderer for output that is not a terminal, which
// draws without colour
func plainRenderer() *lipgloss.Renderer {
	return lipgloss.NewRenderer(&bytes.Buffer{})
}

// colourRenderer returns a renderer that draws with ANSI colours, as it does
// for a terminal
func colourRenderer() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(&bytes.Buffer{})
	r.SetColorProfile(termenv.ANSI)

	return r
}

func testNodes() []statusNode {
	return []statusNode{
		{
			ID:      "template.welcome",
			Name:    "welcome",
			Type:    "template",
			Links:   []string{"variable.output_dir", "docker.network.app.meta.name", "docker.container.web.ip_address"},
			Status:  types.StatusCreated,
			Details: []string{"./build/rendered/welcome.txt"},
		},
		{
			ID:      "docker.container.web",
			Name:    "web",
			Type:    "docker",
			Links:   []string{"docker.network.app.meta.name"},
			Status:  types.StatusCreated,
			Details: []string{"nginx:1.27-alpine", "10.42.0.2", "6febfc7f7781"},
		},
		{
			ID:      "docker.network.app",
			Name:    "app",
			Type:    "docker",
			Status:  types.StatusCreated,
			Details: []string{"10.42.0.0/24", "9864dfda95ff"},
		},
	}
}

func TestRenderStatusNestsEachResourceUnderItsDeepestDependency(t *testing.T) {
	printed := renderStatus(plainRenderer(), testNodes())

	expected := "● docker.network.app  10.42.0.0/24 · 9864dfda95ff\n" +
		"└── ● docker.container.web  nginx:1.27-alpine · 10.42.0.2 · 6febfc7f7781\n" +
		"    └── ● template.welcome  ./build/rendered/welcome.txt\n"
	require.Equal(t, expected, printed)
}

func TestRenderStatusPrintsIndependentResourcesAsSeparateTrees(t *testing.T) {
	nodes := []statusNode{
		{ID: "docker.network.zeta", Name: "zeta", Type: "docker", Status: types.StatusCreated},
		{ID: "docker.network.alpha", Name: "alpha", Type: "docker", Status: types.StatusCreated},
	}

	printed := renderStatus(plainRenderer(), nodes)

	require.Equal(t, "● docker.network.alpha\n● docker.network.zeta\n", printed)
}

func TestRenderStatusIgnoresLinksToEntitiesItDoesNotShow(t *testing.T) {
	nodes := []statusNode{
		{ID: "template.welcome", Name: "welcome", Type: "template", Links: []string{"variable.output_dir"}, Status: types.StatusCreated},
	}

	printed := renderStatus(plainRenderer(), nodes)

	require.Equal(t, "● template.welcome\n", printed)
}

func TestRenderStatusWithNoResourcesPrintsNothingApplied(t *testing.T) {
	printed := renderStatus(plainRenderer(), []statusNode{})

	require.Equal(t, "nothing applied\n", printed)
}

func TestRenderStatusColoursACreatedResourceGreen(t *testing.T) {
	nodes := []statusNode{{ID: "docker.network.app", Name: "app", Type: "docker", Status: types.StatusCreated}}

	printed := renderStatus(colourRenderer(), nodes)

	require.Contains(t, printed, "\x1b[32m●")
}

func TestRenderStatusColoursAFailedResourceRed(t *testing.T) {
	nodes := []statusNode{{ID: "docker.network.app", Name: "app", Type: "docker", Status: types.StatusFailed}}

	printed := renderStatus(colourRenderer(), nodes)

	require.Contains(t, printed, "\x1b[31m●")
}

func TestShortIDKeepsTheFirstTwelveCharacters(t *testing.T) {
	require.Equal(t, "9864dfda95ff", shortID("9864dfda95ff6b27252a148483d15d487526332c2e23572c9e3a79ada6908568"))
}

func TestShortIDLeavesAShortIDAlone(t *testing.T) {
	require.Equal(t, "abc", shortID("abc"))
}
