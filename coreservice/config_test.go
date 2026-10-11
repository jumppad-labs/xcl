package coreservice

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/state"
)

// databaseSchema is a schema v1 document declaring a type with a subtype,
// which configuration declares with two labels
const databaseSchema = `{"schema_version":1,"type":"database","subtype":"postgres","fields":[{"name":"version","type":"string"}]}`

const (
	// githubWidgetRepository is the repository the fake widget release is
	// published from, named after the fixture plugin binary
	githubWidgetRepository = "acme/xcl-plugin-widget"

	// githubWidgetTag is the release tag of the fake widget release
	githubWidgetTag = "v1.0.0"
)

// localRegistry wraps local in a Registry message
func localRegistry(local *corepb.LocalRegistry) *corepb.Registry {
	return &corepb.Registry{Kind: &corepb.Registry_Local{Local: local}}
}

// githubRegistry wraps gh in a Registry message
func githubRegistry(gh *corepb.GitHubRegistry) *corepb.Registry {
	return &corepb.Registry{Kind: &corepb.Registry_Github{Github: gh}}
}

// statePath is a request keeping state under a temporary directory and
// nothing else
func statePath(t *testing.T) *corepb.StateStore {
	t.Helper()

	return &corepb.StateStore{Kind: &corepb.StateStore_Path{Path: t.TempDir()}}
}

// widgetRequest is a request whose GitHub registry reads the widget plugin's
// release from the fake GitHub API at apiURL
func widgetRequest(t *testing.T, apiURL string) *corepb.CreateConfigRequest {
	t.Helper()

	return &corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{githubRegistry(&corepb.GitHubRegistry{
			Plugins:  []*corepb.GitHubPlugin{{Repository: githubWidgetRepository, Version: githubWidgetTag}},
			CacheDir: t.TempDir(),
			ApiUrl:   apiURL,
		})},
		StateStore: statePath(t),
	}
}

func TestCreateConfigDeclaresSchemaType(t *testing.T) {
	c, handle := appliedApp(t)

	entity := c.find(handle, "app.web")
	require.Contains(t, string(entity), `"image":"nginx"`)
	require.Contains(t, string(entity), `"meta"`)
}

func TestCreateConfigDeclaresSchemaTypeWithSubtype(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(&corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{localRegistry(&corepb.LocalRegistry{
			Types: []*corepb.DeclaredType{{Schema: []byte(databaseSchema)}},
		})},
		StateStore: statePath(t),
	})

	c.apply(handle, writeConfig(t, "database \"postgres\" \"main\" {\n  version = \"16\"\n}\n"))

	entity := c.find(handle, "database.postgres.main")
	require.Contains(t, string(entity), `"version":"16"`)
}

func TestCreateConfigInvalidSchemaIsInvalidArgument(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", &corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{localRegistry(&corepb.LocalRegistry{
			Types: []*corepb.DeclaredType{{Type: "app", Schema: []byte(`{"schema_version":1`)}},
		})},
	})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "0", out.GetDetails()["registry"])
	require.Contains(t, out.GetMessage(), "type app")
}

func TestCreateConfigRegistersExternalPlugin(t *testing.T) {
	binary := testutil.BuildFixturePlugin(t, "..")
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(&corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{localRegistry(&corepb.LocalRegistry{
			ExternalPlugins: []string{binary},
		})},
		StateStore: statePath(t),
	})

	c.apply(handle, writeConfig(t, widgetConfig))

	entity := c.find(handle, "widget.app")
	require.Contains(t, string(entity), `"size":1`)
}

func TestCreateConfigGitHubRegistryUsesAPIURL(t *testing.T) {
	binary := testutil.BuildFixturePlugin(t, "..")
	fake := testutil.NewFakeGitHubRelease(t, githubWidgetRepository, githubWidgetTag, binary)
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(widgetRequest(t, fake.URL()))

	c.apply(handle, writeConfig(t, widgetConfig))

	require.Greater(t, fake.Requests(), 0)

	entity := c.find(handle, "widget.app")
	require.Contains(t, string(entity), `"size":1`)
}

func TestCreateConfigGitHubTamperedReleaseFails(t *testing.T) {
	binary := testutil.BuildFixturePlugin(t, "..")
	fake := testutil.NewFakeGitHubRelease(t, githubWidgetRepository, githubWidgetTag, binary,
		testutil.TamperArchive(runtime.GOOS, runtime.GOARCH))
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(widgetRequest(t, fake.URL()))

	conf := c.s.lookup(handle)
	require.NotNil(t, conf)

	err := conf.cfg.Apply(writeConfig(t, widgetConfig))
	require.Error(t, err)
	require.True(t, errors.Is(err, xcl.ErrPluginVerification), "apply failed with %v", err)

	out := toProto(err)
	require.Equal(t, CodePluginVerification, out.GetCode())
	require.Equal(t, githubWidgetRepository, out.GetDetails()["repository"])
	require.Equal(t, githubWidgetTag, out.GetDetails()["version"])
}

func TestCreateConfigPassesVariables(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.OutputsResponse{}
	c.ok(handle, "Outputs", &corepb.OutputsRequest{}, resp)

	require.JSONEq(t, `{"output.where":"us"}`, string(resp.GetOutputsJson()))
}

func TestCreateConfigStatePathWritesState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	req := appRequest(t)
	req.StateStore = &corepb.StateStore{Kind: &corepb.StateStore_Path{Path: dir}}

	c := newTestClient(t)
	handle := c.createConfig(req)
	c.apply(handle, writeConfig(t, appConfig))

	require.FileExists(t, filepath.Join(dir, state.StateFileName))
}

func TestCreateConfigHostStateStoreAccepted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	resp := &corepb.CreateConfigResponse{}
	c.ok(0, "CreateConfig", &corepb.CreateConfigRequest{
		StateStore: &corepb.StateStore{Kind: &corepb.StateStore_Host{Host: true}},
	}, resp)

	require.NotZero(t, resp.GetHandle())
}

func TestCreateConfigDeclaresHostFunction(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	resp := &corepb.CreateConfigResponse{}
	c.ok(0, "CreateConfig", &corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{localRegistry(&corepb.LocalRegistry{
			Functions: []*corepb.DeclaredFunction{{
				Name:      "slugify",
				Signature: []byte(`{"params":[{"name":"title","type":"string"}],"return":"string"}`),
			}},
		})},
		StateStore: statePath(t),
	}, resp)

	require.NotZero(t, resp.GetHandle())
}

func TestCreateConfigStateMaskRedactIsRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	req := appRequest(t)
	req.StateMask = &corepb.Mask{Kind: &corepb.Mask_Redact{Redact: true}}

	out := c.fail(0, "CreateConfig", req)

	require.Equal(t, CodeMaskNotReversible, out.GetCode())
	require.NotEmpty(t, out.GetDetails()["masker"])
}

func TestCreateConfigStateMaskAESEncryptsState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	req := appRequest(t)
	req.StateStore = &corepb.StateStore{Kind: &corepb.StateStore_Path{Path: dir}}
	req.StateMask = &corepb.Mask{Kind: &corepb.Mask_Aes256GcmKey{Aes256GcmKey: []byte("0123456789abcdef0123456789abcdef")}}

	c := newTestClient(t)
	handle := c.createConfig(req)
	c.apply(handle, writeConfig(t, appConfig))

	saved, err := os.ReadFile(filepath.Join(dir, state.StateFileName))
	require.NoError(t, err)
	require.Contains(t, string(saved), "app.web")
	require.NotContains(t, string(saved), "s3cret")
}

func TestCreateConfigUnknownRegistryKindIsInvalidArgument(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", &corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{{}},
	})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "0", out.GetDetails()["registry"])
}

func TestCreateConfigBadVariablesJSONIsInvalidArgument(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", &corepb.CreateConfigRequest{
		VariablesJson: []byte(`["not", "an", "object"]`),
	})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "variables_json", out.GetDetails()["option"])
}

func TestCreateConfigNegativeEventBufferSizeIsInvalidArgument(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", &corepb.CreateConfigRequest{
		EventBufferSize: -1,
	})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "event_buffer_size", out.GetDetails()["option"])
}

func TestCreateConfigEventOptionsAccepted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	req := appRequest(t)
	req.Events = true
	req.EventBufferSize = 8
	req.EventData = corepb.EventDataLevel_EVENT_DATA_PROCESSED
	req.EventMask = &corepb.Mask{Kind: &corepb.Mask_HmacSha256Key{HmacSha256Key: []byte("hmac-key")}}

	handle := c.createConfig(req)

	conf := c.s.lookup(handle)
	require.NotNil(t, conf)
	require.True(t, conf.events)
}
