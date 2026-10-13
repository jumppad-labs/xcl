package xclviper

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl"
	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type runner struct {
	Name     string            `xcl:"name,label"`
	Capacity int               `xcl:"capacity,optional"`
	Tasks    []string          `xcl:"tasks,optional"`
	Labels   map[string]string `xcl:"labels,optional"`
}

type agent struct {
	Name    string   `xcl:"name,label"`
	Enabled *bool    `xcl:"enabled,optional"`
	Models  []string `xcl:"models,optional"`
}

type database struct {
	Engine   string                  `xcl:"engine,label"`
	Name     string                  `xcl:"name,label"`
	Port     int                     `xcl:"port,optional"`
	Password types.Sensitive[string] `xcl:"password,optional"`
}

type override struct {
	Name   string   `xcl:"name,label"`
	Models []string `xcl:"models,optional"`
}

type organization struct {
	Name      string     `xcl:"name,label"`
	Capacity  int        `xcl:"capacity,optional"`
	Overrides []override `xcl:"agent,block"`
}

// read returns a Viper for the test's types that has read file from testdata.
func read(t *testing.T, file string, options ...Option) (*viper.Viper, error) {
	t.Helper()

	v := New(append([]Option{
		Type[runner]("runner"),
		Type[agent]("agent"),
		Type[database]("database", "postgres"),
		Type[organization]("organization"),
	}, options...)...)

	path, err := filepath.Abs(filepath.Join("testdata", file))
	require.NoError(t, err)

	v.SetConfigFile(path)

	return v, v.ReadInConfig()
}

func TestReadsBlocksIntoKeys(t *testing.T) {
	v, err := read(t, "runner.xcl")
	require.NoError(t, err)

	require.Equal(t, 3, v.GetInt("runner.nas.capacity"))
	require.Equal(t, []string{"document"}, v.GetStringSlice("runner.nas.tasks"))
	require.Equal(t, "lan", v.GetString("runner.nas.labels.network"))
	require.Equal(t, []string{"opus", "sonnet"}, v.GetStringSlice("agent.claude.models"))
}

func TestReadsASubtypeIntoItsKey(t *testing.T) {
	v, err := read(t, "runner.xcl")
	require.NoError(t, err)

	require.Equal(t, 5432, v.GetInt("database.postgres.main.port"))
}

func TestKeysLabelledNestedBlocksByLabel(t *testing.T) {
	v, err := read(t, "runner.xcl")
	require.NoError(t, err)

	require.Equal(t, []string{"sonnet"}, v.GetStringSlice("organization.hidden.agent.claude.models"))
}

func TestKeepsAPointerSetToFalse(t *testing.T) {
	v, err := read(t, "runner.xcl")
	require.NoError(t, err)

	require.True(t, v.IsSet("agent.ollama.enabled"))
	require.False(t, v.GetBool("agent.ollama.enabled"))
}

func TestLeavesOutAnUnsetPointer(t *testing.T) {
	v, err := read(t, "runner.xcl")
	require.NoError(t, err)

	require.False(t, v.IsSet("agent.claude.enabled"))
}

func TestLeavesOutADisabledBlock(t *testing.T) {
	v, err := read(t, "runner.xcl")
	require.NoError(t, err)

	require.False(t, v.IsSet("agent.retired"))
}

func TestRevealsASensitiveValue(t *testing.T) {
	v, err := read(t, "runner.xcl")
	require.NoError(t, err)

	require.Equal(t, "hunter2", v.GetString("database.postgres.main.password"))
}

func TestLetsTheEnvironmentOverrideAKey(t *testing.T) {
	t.Setenv("RUNNER_RUNNER_NAS_CAPACITY", "8")

	v, err := read(t, "runner.xcl")
	require.NoError(t, err)

	v.SetEnvPrefix("RUNNER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	require.Equal(t, 8, v.GetInt("runner.nas.capacity"))
}

func TestNamesTheFileItReadInAnError(t *testing.T) {
	_, err := read(t, "broken.xcl")

	require.ErrorContains(t, err, "broken.xcl:2")
	require.ErrorContains(t, err, `An argument named "modls" is not expected here`)
}

func TestKeepsTheSourceOfTheFileItReadInAnError(t *testing.T) {
	_, err := read(t, "broken.xcl")

	var d *xclerrors.Diagnostic
	require.ErrorAs(t, err, &d)
	require.NotNil(t, d.Source)
	require.Contains(t, d.Source.Lines, `  modls = ["opus"]`)
}

func TestShowsTheSourceInTheReportOfAnError(t *testing.T) {
	_, err := read(t, "broken.xcl")

	var d *xclerrors.Diagnostic
	require.ErrorAs(t, err, &d)

	report := string(highlight.Text.Render(xcl.DiagnosticPieces(xclerrors.Diagnostics{d}, 0)))
	require.Contains(t, report, `modls = ["opus"]`)
}

func TestReadsConfigurationGivenAsBytes(t *testing.T) {
	v := New(Type[agent]("agent"))
	v.SetConfigType(Extension)

	err := v.ReadConfig(strings.NewReader(`agent "claude" { models = ["opus"] }`))
	require.NoError(t, err)

	require.Equal(t, []string{"opus"}, v.GetStringSlice("agent.claude.models"))
}

func TestStillReadsOtherFormats(t *testing.T) {
	v, err := read(t, "runner.yaml")
	require.NoError(t, err)

	require.Equal(t, 7, v.GetInt("capacity"))
}

func TestPassesOptionsToViper(t *testing.T) {
	v, err := read(t, "runner.xcl", With(viper.KeyDelimiter("::")))
	require.NoError(t, err)

	require.Equal(t, 3, v.GetInt("runner::nas::capacity"))
}

func TestPanicsForATypeXclCannotDeclare(t *testing.T) {
	type withMeta struct {
		Meta string `xcl:"meta,optional"`
	}

	require.Panics(t, func() {
		New(Type[withMeta]("thing"))
	})
}
