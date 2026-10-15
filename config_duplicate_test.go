package xcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plain"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/stretchr/testify/require"
)

// duplicateAgentConfig declares agent.claude twice
const duplicateAgentConfig = `
agent "claude" {
  models = ["opus"]
}

agent "claude" {
  models = ["haiku"]
}
`

// duplicateAgentInFileConfig declares agent.claude again, in a second file of
// the same directory
const duplicateAgentInFileConfig = `
agent "claude" {
  models = ["haiku"]
}
`

func TestValidateRefusesABlockWithoutItsLabel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	local := registry.NewLocal()
	registry.Register[plain.Agent](local, "agent")

	path := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(path, []byte(`agent { models = ["opus"] }`), 0o644))

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)

	err = c.Validate(path)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Equal(t, "Missing label", d.Summary)
	require.Equal(t, `agent takes a label: write it as agent "<label>" { … }.`, d.Detail)
}

func TestValidateRefusesAnEntityDeclaredTwice(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	local := registry.NewLocal()
	registry.Register[plain.Agent](local, "agent")

	path := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(path, []byte(duplicateAgentConfig), 0o644))

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)

	err = c.Validate(path)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Equal(t, "Duplicate block", d.Summary)
	require.Contains(t, d.Detail, "agent.claude is already declared at")
	require.Equal(t, 6, d.Subject.Start.Line)
}

func TestValidateRefusesAnEntityDeclaredInTwoFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	local := registry.NewLocal()
	registry.Register[plain.Agent](local, "agent")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.xcl"), []byte(duplicateAgentInFileConfig), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.xcl"), []byte(duplicateAgentInFileConfig), 0o644))

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)

	err = c.Validate(dir)
	require.Error(t, err)

	d := requireDiagnostic(t, err)
	require.Equal(t, "Duplicate block", d.Summary)
	require.Equal(t, filepath.Join(dir, "b.xcl"), d.Subject.Filename)
	require.Contains(t, d.Detail, filepath.Join(dir, "a.xcl"))
}
