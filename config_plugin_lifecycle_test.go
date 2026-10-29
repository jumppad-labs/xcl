package xcl

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/plugins"
)

// An external plugin's process runs only while an operation uses it. xcl
// stops it when the operation is done and starts it again for the next, so a
// program never stops a plugin itself. These tests use the external
// subtypeless fixture plugin.

// externalHost returns the host of the one external plugin c has loaded
func externalHost(t *testing.T, c *Config) *plugins.GRPCPluginHost {
	t.Helper()

	hosts := c.catalog.GetPluginHosts()
	require.Len(t, hosts, 1)

	host, ok := hosts[0].(*plugins.GRPCPluginHost)
	require.True(t, ok, "expected an external plugin host, got %T", hosts[0])

	return host
}

// externalWidgetConfig returns a Config getting the external subtypeless
// plugin binary from a local registry, whose plugin processes are stopped
// when the test ends
func externalWidgetConfig(t *testing.T, binary string) *Config {
	t.Helper()

	c, err := NewConfig(WithRegistry(externalWidgetRegistry(binary)))
	require.NoError(t, err)
	stopPluginHosts(t, c)

	return c
}

func TestApplyStopsTheExternalPluginWhenDone(t *testing.T) {
	binary := buildSubtypelessPlugin(t)
	isolateHome(t)

	f := newPersonConfig(t, externalWidgetRegistry(binary), subtypelessFixture(t, "subtypeless"), t.TempDir())

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	require.False(t, externalHost(t, f.config).Running())
}

// twoWidgets declares a widget that depends on another, so its create
// starts only once the first widget's create has succeeded and further
// events follow the first widget's create success event
const twoWidgets = `widget "main" {
  size = 3

  part {
    name = "wheel"
  }
}

widget "second" {
  size = widget.main.size

  part {
    name = "axle"
  }
}
`

func TestRenderFromHandlerDuringApplyKeepsTheExternalPluginRunning(t *testing.T) {
	binary := buildSubtypelessPlugin(t)
	isolateHome(t)

	var c *Config

	var renderErr error
	var text []byte
	runningAfterRender := false
	rendered := false

	// With a buffer of one event the apply cannot finish, and so cannot
	// release its use of the plugins, while the handler is busy with the
	// first widget's create success: the second widget's events are still
	// to be delivered.
	handler := func(e Event) {
		if e.Operation != "create" || e.Phase != "success" || e.ResourceID != "widget.main" {
			return
		}

		text, renderErr = c.EncodeSavedEntity(e.Data)
		runningAfterRender = externalHost(t, c).Running()
		rendered = true
	}

	f := setupPersonConfig(t, externalWidgetRegistry(binary), t.TempDir(), twoWidgets,
		WithEventData(EventDataProcessed),
		WithEventBufferSize(1),
		WithEventHandler(handler),
	)
	c = f.config

	err := c.Apply(f.configFile)
	require.NoError(t, err)

	require.True(t, rendered, "no create success event for widget.main was delivered")
	require.NoError(t, renderErr)
	require.Contains(t, string(text), `widget "main"`)
	require.True(t, runningAfterRender, "the external plugin was stopped by a render during the apply")

	// the plugin is stopped once the apply is done, as without the render
	require.False(t, externalHost(t, c).Running())
}

func TestLoadAfterApplyStartsTheExternalPluginAgainAndStopsIt(t *testing.T) {
	binary := buildSubtypelessPlugin(t)
	isolateHome(t)

	f := newPersonConfig(t, externalWidgetRegistry(binary), subtypelessFixture(t, "subtypeless"), t.TempDir())

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	err = f.config.Load()
	require.NoError(t, err)

	widget, err := Find[subtypelessWidget](f.config, "widget.main")
	require.NoError(t, err)
	require.Equal(t, "wheel,axle", widget.PartNames)

	require.False(t, externalHost(t, f.config).Running())
}

func TestUseKeepsTheExternalPluginRunningUntilTheLastOperationIsDone(t *testing.T) {
	binary := buildSubtypelessPlugin(t)
	isolateHome(t)

	c := externalWidgetConfig(t, binary)

	first, err := c.catalog.Use(nil)
	require.NoError(t, err)

	second, err := c.catalog.Use(nil)
	require.NoError(t, err)

	host := externalHost(t, c)
	require.True(t, host.Running())

	first()
	require.True(t, host.Running())

	second()
	require.False(t, host.Running())
}

func TestUseStartsAStoppedExternalPluginAgain(t *testing.T) {
	binary := buildSubtypelessPlugin(t)
	isolateHome(t)

	c := externalWidgetConfig(t, binary)

	done, err := c.catalog.Use(nil)
	require.NoError(t, err)
	done()

	done, err = c.catalog.Use(nil)
	require.NoError(t, err)
	defer done()

	require.True(t, externalHost(t, c).Running())
}

func TestUseDoneTwiceReleasesOnce(t *testing.T) {
	binary := buildSubtypelessPlugin(t)
	isolateHome(t)

	c := externalWidgetConfig(t, binary)

	first, err := c.catalog.Use(nil)
	require.NoError(t, err)

	second, err := c.catalog.Use(nil)
	require.NoError(t, err)
	defer second()

	first()
	first()

	require.True(t, externalHost(t, c).Running())
}

func TestUseFailsWhenTheExternalPluginCannotStartAgain(t *testing.T) {
	binary := buildSubtypelessPlugin(t)
	isolateHome(t)

	c := externalWidgetConfig(t, binary)

	done, err := c.catalog.Use(nil)
	require.NoError(t, err)
	done()

	err = os.Remove(binary)
	require.NoError(t, err)

	done, err = c.catalog.Use(nil)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPluginLoad)
	require.Nil(t, done)
}
