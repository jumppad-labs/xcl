package diff_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
)

// designExample builds the example result from the diff design
func designExample() *diff.Diff {
	ports := diff.Path{}.Attribute("ports")

	return &diff.Diff{
		Summary: diff.Summary{Create: 1, Update: 2, Replace: 1, Delete: 1, Unchanged: 4},
		Resources: []diff.Resource{
			{
				Address: "resource.container.api",
				Action:  diff.ActionUpdate,
				Changes: []diff.Change{
					{Path: ports.Index(0).Attribute("host"), Before: 8080, After: 9090},
					{Path: ports.Index(2), After: map[string]any{"host": 443, "local": 8443}},
					{Path: diff.Path{}.Attribute("env").Key("DB_PASSWORD"), Sensitive: true},
				},
			},
			{
				Address: "resource.container.cache",
				Action:  diff.ActionUpdate,
			},
			{
				Address: "resource.container.web",
				Action:  diff.ActionCreate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("image"), After: "nginx"},
					{Path: diff.Path{}.Attribute("db_host"), Unknown: true},
				},
			},
			{
				Address: "resource.network.app",
				Action:  diff.ActionReplace,
			},
			{
				Address: "resource.postgres.old",
				Action:  diff.ActionDelete,
			},
		},
	}
}

const designExampleJSON = `{
  "summary": { "create": 1, "update": 2, "replace": 1, "delete": 1, "unchanged": 4 },
  "resources": [
    { "address": "resource.container.api", "action": "update",
      "changes": [ { "path": "ports[0].host", "before": 8080, "after": 9090 },
                   { "path": "ports[2]", "after": { "host": 443, "local": 8443 } },
                   { "path": "env[\"DB_PASSWORD\"]", "sensitive": true } ] },
    { "address": "resource.container.cache", "action": "update" },
    { "address": "resource.container.web", "action": "create",
      "changes": [ { "path": "image", "after": "nginx" },
                   { "path": "db_host", "unknown": true } ] },
    { "address": "resource.network.app", "action": "replace" },
    { "address": "resource.postgres.old", "action": "delete" }
  ]
}`

func TestDiffMarshalJSONProducesDesignExample(t *testing.T) {
	encoded, err := json.Marshal(designExample())
	require.NoError(t, err)

	require.JSONEq(t, designExampleJSON, string(encoded))
}

func TestChangeMarshalJSONOmitsAbsentBeforeAndAfter(t *testing.T) {
	encoded, err := json.Marshal(diff.Change{Path: diff.Path{}.Attribute("db_host"), Unknown: true})
	require.NoError(t, err)

	require.JSONEq(t, `{ "path": "db_host", "unknown": true }`, string(encoded))
}

func TestResourceMarshalJSONOmitsAbsentChanges(t *testing.T) {
	encoded, err := json.Marshal(diff.Resource{Address: "resource.postgres.old", Action: diff.ActionDelete})
	require.NoError(t, err)

	require.JSONEq(t, `{ "address": "resource.postgres.old", "action": "delete" }`, string(encoded))
}

func TestReplaceResourceJSONIncludesReasonAndDependencies(t *testing.T) {
	encoded, err := json.Marshal(diff.Resource{
		Address:      "resource.container.web",
		Action:       diff.ActionReplace,
		Reason:       diff.ReplaceDependency,
		ReplacedDeps: []string{"resource.network.app", "resource.volume.data"},
	})
	require.NoError(t, err)

	require.JSONEq(t, `{ "address": "resource.container.web", "action": "replace", "reason": "dependency",
	  "replaced_dependencies": [ "resource.network.app", "resource.volume.data" ] }`, string(encoded))
}

func TestReplaceResourceJSONOmitsDependenciesWhenTheProviderDecided(t *testing.T) {
	encoded, err := json.Marshal(diff.Resource{
		Address: "resource.container.web",
		Action:  diff.ActionReplace,
		Reason:  diff.ReplaceProvider,
	})
	require.NoError(t, err)

	require.JSONEq(t, `{ "address": "resource.container.web", "action": "replace", "reason": "provider" }`, string(encoded))
}

func TestUpdateResourceJSONOmitsReason(t *testing.T) {
	encoded, err := json.Marshal(diff.Resource{
		Address: "resource.container.api",
		Action:  diff.ActionUpdate,
		Changes: []diff.Change{
			{Path: diff.Path{}.Attribute("image"), Before: "old", After: "new"},
		},
	})
	require.NoError(t, err)

	require.JSONEq(t, `{ "address": "resource.container.api", "action": "update",
	  "changes": [ { "path": "image", "before": "old", "after": "new" } ] }`, string(encoded))
}

func TestReplaceReasonsHaveDesignNames(t *testing.T) {
	require.Equal(t, "failed", string(diff.ReplaceFailed))
	require.Equal(t, "provider", string(diff.ReplaceProvider))
	require.Equal(t, "dependency", string(diff.ReplaceDependency))
}

func TestApplicationReadsSummaryFromDiff(t *testing.T) {
	result := designExample()

	require.Equal(t, 1, result.Summary.Create)
	require.Equal(t, 2, result.Summary.Update)
	require.Equal(t, 1, result.Summary.Replace)
	require.Equal(t, 1, result.Summary.Delete)
	require.Equal(t, 4, result.Summary.Unchanged)
}

func TestApplicationReadsResourceAddressesAndActionsFromDiff(t *testing.T) {
	result := designExample()

	require.Len(t, result.Resources, 5)
	require.Equal(t, "resource.container.api", result.Resources[0].Address)
	require.Equal(t, diff.ActionUpdate, result.Resources[0].Action)
	require.Equal(t, "resource.container.cache", result.Resources[1].Address)
	require.Equal(t, diff.ActionUpdate, result.Resources[1].Action)
	require.Equal(t, "resource.container.web", result.Resources[2].Address)
	require.Equal(t, diff.ActionCreate, result.Resources[2].Action)
	require.Equal(t, "resource.network.app", result.Resources[3].Address)
	require.Equal(t, diff.ActionReplace, result.Resources[3].Action)
	require.Equal(t, "resource.postgres.old", result.Resources[4].Address)
	require.Equal(t, diff.ActionDelete, result.Resources[4].Action)
}

func TestApplicationReadsChangesFromResource(t *testing.T) {
	api := designExample().Resources[0]

	require.Len(t, api.Changes, 3)

	require.Equal(t, "ports[0].host", api.Changes[0].Path.String())
	require.Equal(t, 8080, api.Changes[0].Before)
	require.Equal(t, 9090, api.Changes[0].After)

	require.Equal(t, "ports[2]", api.Changes[1].Path.String())
	require.Nil(t, api.Changes[1].Before)
	require.Equal(t, map[string]any{"host": 443, "local": 8443}, api.Changes[1].After)

	require.Equal(t, `env["DB_PASSWORD"]`, api.Changes[2].Path.String())
	require.True(t, api.Changes[2].Sensitive)
	require.Nil(t, api.Changes[2].Before)
	require.Nil(t, api.Changes[2].After)
}

func TestApplicationReadsUnknownFlagFromChange(t *testing.T) {
	web := designExample().Resources[2]

	require.Equal(t, "db_host", web.Changes[1].Path.String())
	require.True(t, web.Changes[1].Unknown)
	require.Nil(t, web.Changes[1].After)
}

func TestActionsHaveDesignNames(t *testing.T) {
	require.Equal(t, "create", string(diff.ActionCreate))
	require.Equal(t, "update", string(diff.ActionUpdate))
	require.Equal(t, "replace", string(diff.ActionReplace))
	require.Equal(t, "delete", string(diff.ActionDelete))
}

func TestChangedSumsTheFourActionCounts(t *testing.T) {
	result := designExample()

	require.Equal(t, 5, result.Changed())
}

func TestChangedExcludesUnchanged(t *testing.T) {
	result := &diff.Diff{Summary: diff.Summary{Unchanged: 7}}

	require.Equal(t, 0, result.Changed())
}

func TestChangedCountsEachActionOnce(t *testing.T) {
	result := &diff.Diff{Summary: diff.Summary{Create: 3, Update: 5, Replace: 7, Delete: 11, Unchanged: 13}}

	require.Equal(t, 26, result.Changed())
}

func TestChangedOfNilDiffIsZero(t *testing.T) {
	var result *diff.Diff

	require.Equal(t, 0, result.Changed())
}
