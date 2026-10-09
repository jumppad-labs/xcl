package parser

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/types"
)

func TestPlanChangesHidesSensitiveValuesWithoutReveal(t *testing.T) {
	saved := &structs.Credential{Password: types.NewSensitive("old-secret")}
	configured := &structs.Credential{Password: types.NewSensitive("new-secret")}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, true)

	changes := planChanges(revealed, false)

	require.Len(t, changes, 1)
	require.Equal(t, "password", changes[0].Path.String())
	require.True(t, changes[0].Sensitive)
	require.Nil(t, changes[0].Before)
	require.Nil(t, changes[0].After)
	requireNoSecret(t, changes, "old-secret")
	requireNoSecret(t, changes, "new-secret")
}

func TestPlanChangesDoesNotAlterTheRevealedChanges(t *testing.T) {
	saved := &structs.Credential{Password: types.NewSensitive("old-secret")}
	configured := &structs.Credential{Password: types.NewSensitive("new-secret")}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, true)

	planChanges(revealed, false)

	require.Equal(t, "old-secret", revealed[0].Before)
	require.Equal(t, "new-secret", revealed[0].After)
}

func TestPlanChangesKeepsSensitiveValuesWithReveal(t *testing.T) {
	saved := &structs.Credential{Password: types.NewSensitive("old-secret")}
	configured := &structs.Credential{Password: types.NewSensitive("new-secret")}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, true)

	changes := planChanges(revealed, true)

	require.Len(t, changes, 1)
	require.Equal(t, "password", changes[0].Path.String())
	require.True(t, changes[0].Sensitive)
	require.Equal(t, "old-secret", changes[0].Before)
	require.Equal(t, "new-secret", changes[0].After)
}

func TestPlanChangesEqualsResourceChangesWithoutReveal(t *testing.T) {
	saved := &structs.Credential{
		Username: "admin",
		Password: types.NewSensitive("old-secret"),
		Pin:      types.NewSensitive(1234),
	}
	configured := &structs.Credential{
		Username: "root",
		Password: types.NewSensitive("new-secret"),
		Pin:      types.NewSensitive(1234),
	}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, true)

	changes := planChanges(revealed, false)

	expected := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)
	require.Len(t, expected, 2)
	require.Equal(t, expected, changes)
}

func TestPluginChangesKeepsRealSensitiveValues(t *testing.T) {
	saved := &structs.Credential{Password: types.NewSensitive("old-secret")}
	configured := &structs.Credential{Password: types.NewSensitive("new-secret")}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, true)

	changes, err := pluginChanges(revealed)

	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, "password", changes[0].Path.String())
	require.True(t, changes[0].Sensitive)
	require.False(t, changes[0].Unknown)
	require.Equal(t, "old-secret", changes[0].Before)
	require.Equal(t, "new-secret", changes[0].After)
}

func TestPluginChangesNormalisesAnIntToFloat64(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{MaxRestartCount: 1}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{MaxRestartCount: 3}}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, true)

	changes, err := pluginChanges(revealed)

	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, "max_restart_count", changes[0].Path.String())
	require.Equal(t, float64(1), changes[0].Before)
	require.Equal(t, float64(3), changes[0].After)
}

func TestPluginChangesNormalisesABlockToAMap(t *testing.T) {
	saved := &structs.Container{}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Ports: []structs.Port{{Local: 8443, Remote: 443}},
	}}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, true)

	changes, err := pluginChanges(revealed)

	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, "port[0]", changes[0].Path.String())
	require.Nil(t, changes[0].Before)
	require.Equal(t, map[string]any{"local": float64(8443), "remote": float64(443)}, changes[0].After)
}

func TestPluginChangesNormalisesAListInsideABlockToASliceOfAny(t *testing.T) {
	saved := &structs.Container{}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Resources: &structs.Resources{CPU: 2, CPUPin: []int{0, 1}},
	}}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, true)

	changes, err := pluginChanges(revealed)

	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, "resources", changes[0].Path.String())
	require.Nil(t, changes[0].Before)
	require.Equal(t, map[string]any{
		"cpu":     float64(2),
		"cpu_pin": []any{float64(0), float64(1)},
		"memory":  float64(0),
		"user":    "",
	}, changes[0].After)
}

func TestPlanChangesUnknownChangeHasNoAfter(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Default: "old"}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Default: ""}}
	unknown := []diff.Path{diff.Path{}.Attribute("default")}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, unknown, true)

	changes := planChanges(revealed, false)

	require.Len(t, changes, 1)
	require.Equal(t, "default", changes[0].Path.String())
	require.True(t, changes[0].Unknown)
	require.Equal(t, "old", changes[0].Before)
	require.Nil(t, changes[0].After)
}

func TestPluginChangesUnknownChangeHasNoAfter(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Default: "old"}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Default: ""}}
	unknown := []diff.Path{diff.Path{}.Attribute("default")}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, unknown, true)

	changes, err := pluginChanges(revealed)

	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, "default", changes[0].Path.String())
	require.True(t, changes[0].Unknown)
	require.Equal(t, "old", changes[0].Before)
	require.Nil(t, changes[0].After)
}

func TestResolveUnknownGivesAnUnknownChangeItsRealValue(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app", IPAddress: "10.0.0.5"}},
	}}
	decided := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app", IPAddress: ""}},
	}}
	unknown := []diff.Path{diff.Path{}.Attribute("network").Index(0).Attribute("ip_address")}
	revealed := resourceChanges(diff.ActionUpdate, saved, decided, nil, unknown, true)
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app", IPAddress: "10.0.0.9"}},
	}}

	changes := resolveUnknown(revealed, configured)

	require.Len(t, changes, 1)
	require.Equal(t, "network[0].ip_address", changes[0].Path.String())
	require.False(t, changes[0].Unknown)
	require.False(t, changes[0].Sensitive)
	require.Equal(t, "10.0.0.5", changes[0].Before)
	require.Equal(t, "10.0.0.9", changes[0].After)
}

func TestResolveUnknownKeepsAChangeWhoseRealValueEqualsTheSavedValue(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Default: "same"}}
	decided := &structs.Container{ContainerBase: structs.ContainerBase{Default: ""}}
	unknown := []diff.Path{diff.Path{}.Attribute("default")}
	revealed := resourceChanges(diff.ActionUpdate, saved, decided, nil, unknown, true)
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Default: "same"}}

	changes := resolveUnknown(revealed, configured)

	require.Len(t, changes, 1)
	require.Equal(t, "default", changes[0].Path.String())
	require.False(t, changes[0].Unknown)
	require.Equal(t, "same", changes[0].Before)
	require.Equal(t, "same", changes[0].After)
}

func TestResolveUnknownLeavesKnownChangesAlone(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Default: "old", MaxRestartCount: 1}}
	decided := &structs.Container{ContainerBase: structs.ContainerBase{Default: "", MaxRestartCount: 2}}
	unknown := []diff.Path{diff.Path{}.Attribute("default")}
	revealed := resourceChanges(diff.ActionUpdate, saved, decided, nil, unknown, true)
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Default: "new", MaxRestartCount: 2}}

	changes := resolveUnknown(revealed, configured)

	require.Len(t, changes, 2)
	require.Equal(t, "default", changes[0].Path.String())
	require.Equal(t, "new", changes[0].After)
	require.Equal(t, "max_restart_count", changes[1].Path.String())
	require.False(t, changes[1].Unknown)
	require.Equal(t, 1, changes[1].Before)
	require.Equal(t, 2, changes[1].After)
}

func TestResolveUnknownMarksAChangeSensitiveWhenItsRealValueIsSensitive(t *testing.T) {
	saved := &structs.Credential{}
	decided := &structs.Credential{}
	unknown := []diff.Path{diff.Path{}.Attribute("pin")}
	revealed := resourceChanges(diff.ActionUpdate, saved, decided, nil, unknown, true)
	require.Len(t, revealed, 1)
	require.False(t, revealed[0].Sensitive)
	configured := &structs.Credential{Pin: types.NewSensitive(1234)}

	changes := resolveUnknown(revealed, configured)

	require.Len(t, changes, 1)
	require.Equal(t, "pin", changes[0].Path.String())
	require.False(t, changes[0].Unknown)
	require.True(t, changes[0].Sensitive)
	require.Equal(t, 1234, changes[0].After)
}

func TestResolvedSensitiveChangeIsHiddenInThePlanView(t *testing.T) {
	saved := &structs.Credential{}
	decided := &structs.Credential{}
	unknown := []diff.Path{diff.Path{}.Attribute("pin")}
	revealed := resourceChanges(diff.ActionUpdate, saved, decided, nil, unknown, true)
	configured := &structs.Credential{Pin: types.NewSensitive(1234)}

	changes := planChanges(resolveUnknown(revealed, configured), false)

	require.Len(t, changes, 1)
	require.True(t, changes[0].Sensitive)
	require.Nil(t, changes[0].After)
	requireNoSecret(t, changes, "1234")
}

func TestPluginChangesListsTheSamePathsAsPlanChangesInTheSameOrder(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{
		Default:  "old",
		Networks: []structs.NetworkAttachment{{Name: "app", IPAddress: "10.0.0.5"}},
		Env:      map[string]string{"A": "1"},
	}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Default:  "new",
		Networks: []structs.NetworkAttachment{{Name: "web", IPAddress: ""}},
		Env:      map[string]string{"A": "2", "B": "3"},
		Ports:    []structs.Port{{Local: 80, Remote: 8080}},
	}}
	unknown := []diff.Path{diff.Path{}.Attribute("network").Index(0).Attribute("ip_address")}
	revealed := resourceChanges(diff.ActionUpdate, saved, configured, nil, unknown, true)

	plan := planChanges(revealed, false)
	plugin, err := pluginChanges(revealed)

	require.NoError(t, err)
	planPaths := []string{}
	for _, change := range plan {
		planPaths = append(planPaths, change.Path.String())
	}
	pluginPaths := []string{}
	for _, change := range plugin {
		pluginPaths = append(pluginPaths, change.Path.String())
	}
	require.Equal(t, []string{"default", "network[0].name", "network[0].ip_address", `env["A"]`, `env["B"]`, "port[0]"}, planPaths)
	require.Equal(t, planPaths, pluginPaths)
}

func TestValueAtReturnsTheValueAtAPath(t *testing.T) {
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app"}},
	}}

	value := valueAt(reflect.ValueOf(configured), diff.Path{}.Attribute("network").Index(0).Attribute("name"))

	require.True(t, value.IsValid())
	require.Equal(t, "app", value.Interface())
}

func TestValueAtReturnsTheZeroValueForAPathThatLeadsNowhere(t *testing.T) {
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app"}},
	}}

	missingAttribute := valueAt(reflect.ValueOf(configured), diff.Path{}.Attribute("no_such_setting"))
	indexPastTheEnd := valueAt(reflect.ValueOf(configured), diff.Path{}.Attribute("network").Index(3).Attribute("name"))
	missingKey := valueAt(reflect.ValueOf(configured), diff.Path{}.Attribute("env").Key("MISSING"))

	require.False(t, missingAttribute.IsValid())
	require.False(t, indexPastTheEnd.IsValid())
	require.False(t, missingKey.IsValid())
}
