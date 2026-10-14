package xclviper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// daemon is a singleton: it has no label fields, so its block has no labels
type daemon struct {
	Server string `xcl:"server"`
	Listen string `xcl:"listen,optional"`
}

type worker struct {
	Name              string `xcl:"name,label"`
	Model             string `xcl:"model"`
	MaxConcurrentRuns int    `xcl:"max_concurrent_runs,optional"`
}

type task struct {
	Name   string   `xcl:"name,label"`
	Agents []worker `xcl:"agents"`
}

// readDirectory returns a Viper for the directory test's types that has read
// testdata/directory with Read.
func readDirectory(t *testing.T) (*viper.Viper, error) {
	t.Helper()

	v := New(
		Type[daemon]("runner"),
		Type[worker]("agent"),
		Type[task]("task"),
	)

	path, err := filepath.Abs(filepath.Join("testdata", "directory"))
	require.NoError(t, err)

	return v, Read(v, path)
}

func TestReadsEveryFileInADirectory(t *testing.T) {
	v, err := readDirectory(t)
	require.NoError(t, err)

	require.Equal(t, "claude/opus", v.GetString("agent.claude.model"))
	require.Equal(t, "https://maalstrom.app", v.GetString("runner.server"))
}

func TestReadsASingletonUnderItsType(t *testing.T) {
	v, err := readDirectory(t)
	require.NoError(t, err)

	require.Equal(t, "https://maalstrom.app", v.GetString("runner.server"))
	require.Equal(t, "127.0.0.1:7660", v.GetString("runner.listen"))
}

func TestKeepsTheOrderOfAListOfReferences(t *testing.T) {
	v, err := readDirectory(t)
	require.NoError(t, err)

	agents, ok := v.Get("task.document.agents").([]any)
	require.True(t, ok, "agents is a %T", v.Get("task.document.agents"))
	require.Len(t, agents, 2)

	first := agents[0].(map[string]any)
	require.Equal(t, "local", first["name"])
	require.Equal(t, "ollama/qwen3:32b", first["model"])

	second := agents[1].(map[string]any)
	require.Equal(t, "claude", second["name"])
	require.Equal(t, 2, second["max_concurrent_runs"])
}

func TestUnmarshalsAListOfReferencesInOrder(t *testing.T) {
	v, err := readDirectory(t)
	require.NoError(t, err)

	var config struct {
		Task map[string]struct {
			Agents []struct {
				Name  string `mapstructure:"name"`
				Model string `mapstructure:"model"`
			} `mapstructure:"agents"`
		} `mapstructure:"task"`
	}
	require.NoError(t, v.Unmarshal(&config))

	agents := config.Task["document"].Agents
	require.Len(t, agents, 2)
	require.Equal(t, "local", agents[0].Name)
	require.Equal(t, "claude", agents[1].Name)
}

func TestReadsASingleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.xcl")
	require.NoError(t, os.WriteFile(path, []byte(`agent "claude" { models = ["opus"] }`), 0o644))

	v := New(Type[agent]("agent"))

	require.NoError(t, Read(v, path))
	require.Equal(t, []string{"opus"}, v.GetStringSlice("agent.claude.models"))
}

func TestNamesTheFileInADirectoryInAnError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "good.xcl"), []byte(`agent "local" {}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.xcl"), []byte("agent \"claude\" {\n  modls = [\"opus\"]\n}\n"), 0o644))

	v := New(Type[agent]("agent"))

	err := Read(v, dir)
	require.ErrorContains(t, err, "bad.xcl:2")
}
