package parser

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/types"
)

const (
	diffUnknownRefBeforeConfig    = "../test_fixtures/config/diff/unknown_ref/before/main.xcl"
	diffUnknownRefAfterConfig     = "../test_fixtures/config/diff/unknown_ref/after/main.xcl"
	diffUnknownChainConfig        = "../test_fixtures/config/diff/unknown_chain/main.xcl"
	diffUnknownCollectionConfig   = "../test_fixtures/config/diff/unknown_collection/main.xcl"
	diffUnknownOutputConfig       = "../test_fixtures/config/diff/unknown_output"
	diffUnknownNetworkOneID       = "resource.network.one"
	diffUnknownNetworkTwoID       = "resource.network.two"
	diffUnknownUserID             = "resource.container.user"
	diffUnknownFollowerID         = "resource.container.follower"
	diffUnknownAppID              = "resource.container.app"
	diffUnknownRootNetworkID      = "resource.network.root"
	diffUnknownInnerNetworkID     = "module.net.resource.network.inner"
	diffUnknownFromOutputID       = "resource.container.from_output"
	diffUnknownFromVariableID     = "module.app.resource.container.from_variable"
	diffUnknownFixedNetworkName   = "fixed"
	diffUnknownDefaultContainer   = "hello world"
	diffUnknownNetworkNamePath    = "network[0].name"
	diffUnknownAssignedAddressKey = "assigned_address"
)

// diffResourceAt returns the resource the diff lists for the address, failing
// the test when the address is not listed
func diffResourceAt(t *testing.T, result *diff.Diff, address string) diff.Resource {
	t.Helper()

	for _, resource := range result.Resources {
		if resource.Address == address {
			return resource
		}
	}

	require.FailNow(t, "address not listed", "expected %s in the diff, listed: %v", address, diffAddresses(result))
	return diff.Resource{}
}

// changeAt returns the change the resource lists at the written path, failing
// the test when there is none
func changeAt(t *testing.T, resource diff.Resource, path string) diff.Change {
	t.Helper()

	for _, change := range resource.Changes {
		if change.Path.String() == path {
			return change
		}
	}

	require.FailNow(t, "path not listed", "expected a change at %s for %s", path, resource.Address)
	return diff.Change{}
}

// applyUnknownRefBefore applies the before configuration, which saves user
// with a fixed network name, and clears the plugin's calls
func applyUnknownRefBefore(t *testing.T) *lifecycleHarness {
	t.Helper()

	h := setupLifecycle(t)
	h.applyAndSave(t, diffUnknownRefBeforeConfig)
	h.plugin.ResetCalls()

	return h
}

func TestDiffReportsSavedResourceReferencingNewComputedValueAsUpdate(t *testing.T) {
	h := applyUnknownRefBefore(t)

	result := h.runDiff(t, diffUnknownRefAfterConfig)

	user := diffResourceAt(t, result, diffUnknownUserID)
	require.Equal(t, diff.ActionUpdate, user.Action)
	require.Equal(t, []diff.Change{
		{
			Path:    diff.Path{}.Attribute("network").Index(0).Attribute("name"),
			Before:  diffUnknownFixedNetworkName,
			After:   nil,
			Unknown: true,
		},
	}, user.Changes)
}

func TestDiffOfSavedResourceReferencingNewComputedValueCountsEachAction(t *testing.T) {
	h := applyUnknownRefBefore(t)

	result := h.runDiff(t, diffUnknownRefAfterConfig)

	require.Equal(t, []string{diffUnknownUserID, diffUnknownNetworkTwoID}, diffAddresses(result))
	require.Equal(t, diff.ActionCreate, diffActionFor(t, result, diffUnknownNetworkTwoID))
	require.Equal(t, diff.Summary{Create: 1, Update: 1, Unchanged: 1}, result.Summary)
}

func TestDiffNeverReadsResourceHoldingUnknownValue(t *testing.T) {
	h := applyUnknownRefBefore(t)

	h.runDiff(t, diffUnknownRefAfterConfig)

	require.Empty(t, callsFor(h.plugin.GetCalls(), diffUnknownUserID))
	require.NotContains(t, h.plugin.GetReadResources(), diffUnknownUserID)
	require.NotContains(t, h.plugin.GetChangedCalls(), diffUnknownUserID+"->"+diffUnknownUserID)
}

func TestDiffReadsOnlyResourcesWithoutUnknownValues(t *testing.T) {
	h := applyUnknownRefBefore(t)

	h.runDiff(t, diffUnknownRefAfterConfig)

	// network one is saved and fully known, network two is new and user
	// holds an unknown
	require.Equal(t, []string{diffUnknownNetworkOneID}, h.plugin.GetReadResources())
	require.Equal(t, []string{
		"read " + diffUnknownNetworkOneID,
		"changed " + diffUnknownNetworkOneID,
	}, h.plugin.GetCalls())
}

func TestDiffWithUnknownValuesMakesNoCreateUpdateOrDestroyCalls(t *testing.T) {
	h := applyUnknownRefBefore(t)

	h.runDiff(t, diffUnknownRefAfterConfig)

	require.Empty(t, h.plugin.GetCreatedResources())
	require.Empty(t, h.plugin.GetUpdatedResources())
	require.Empty(t, h.plugin.GetDestroyedResources())
}

func TestDiffChangesHoldPlainValuesNeverCtyValues(t *testing.T) {
	h := applyUnknownRefBefore(t)

	result := h.runDiff(t, diffUnknownChainConfig)
	require.NotEmpty(t, result.Resources)

	for _, resource := range result.Resources {
		for _, change := range resource.Changes {
			_, beforeIsCty := change.Before.(cty.Value)
			_, afterIsCty := change.After.(cty.Value)

			require.False(t, beforeIsCty, "%s %s before is a cty value", resource.Address, change.Path)
			require.False(t, afterIsCty, "%s %s after is a cty value", resource.Address, change.Path)
		}
	}

	_, err := json.Marshal(result)
	require.NoError(t, err)
}

func TestDiffOfCreatedResourceReferencingCreatedComputedValueMarksOnlyThatValueUnknown(t *testing.T) {
	h := setupLifecycle(t)

	result := h.runDiff(t, lifecycleComputedRefConfig)

	user := diffResourceAt(t, result, computedRefUserID)
	require.Equal(t, diff.ActionCreate, user.Action)
	require.Equal(t, []diff.Change{
		{
			Path:  diff.Path{}.Attribute("default"),
			After: diffUnknownDefaultContainer,
		},
		{
			Path:    diff.Path{}.Attribute("network").Index(0).Attribute("name"),
			Unknown: true,
		},
	}, user.Changes)
}

func TestDiffOfCreatedResourceKeepsTheReferencedResourceConcrete(t *testing.T) {
	h := setupLifecycle(t)

	result := h.runDiff(t, lifecycleComputedRefConfig)

	network := diffResourceAt(t, result, lifecycleNetworkID)
	require.Equal(t, diff.ActionCreate, network.Action)
	require.Equal(t, []diff.Change{
		{
			Path:  diff.Path{}.Attribute("subnet"),
			After: "10.0.0.0/16",
		},
	}, network.Changes)
}

func TestDiffOfConfigurationWithNothingSavedCallsNoProvider(t *testing.T) {
	h := setupLifecycle(t)

	h.runDiff(t, lifecycleComputedRefConfig)

	require.Empty(t, h.plugin.GetCalls())
}

func TestDiffReportsUnknownMapEntryOnItsOwn(t *testing.T) {
	h := setupLifecycle(t)

	result := h.runDiff(t, diffUnknownCollectionConfig)

	app := diffResourceAt(t, result, diffUnknownAppID)

	known := changeAt(t, app, `env["A"]`)
	require.Equal(t, diff.Change{
		Path:  diff.Path{}.Attribute("env").Key("A"),
		After: "x",
	}, known)

	unknown := changeAt(t, app, `env["B"]`)
	require.Equal(t, diff.Change{
		Path:    diff.Path{}.Attribute("env").Key("B"),
		Unknown: true,
	}, unknown)
}

func TestDiffReportsUnknownListElementOnItsOwn(t *testing.T) {
	h := setupLifecycle(t)

	result := h.runDiff(t, diffUnknownCollectionConfig)

	app := diffResourceAt(t, result, diffUnknownAppID)

	require.Equal(t, diff.Change{
		Path:  diff.Path{}.Attribute("command").Index(0),
		After: "run",
	}, changeAt(t, app, "command[0]"))
	require.Equal(t, diff.Change{
		Path:    diff.Path{}.Attribute("command").Index(1),
		Unknown: true,
	}, changeAt(t, app, "command[1]"))
	require.Equal(t, diff.Change{
		Path:  diff.Path{}.Attribute("command").Index(2),
		After: "--verbose",
	}, changeAt(t, app, "command[2]"))
}

func TestDiffNeverReportsTheWholeCollectionHoldingAnUnknown(t *testing.T) {
	h := setupLifecycle(t)

	result := h.runDiff(t, diffUnknownCollectionConfig)

	app := diffResourceAt(t, result, diffUnknownAppID)
	for _, change := range app.Changes {
		require.NotEqual(t, "env", change.Path.String())
		require.NotEqual(t, "command", change.Path.String())
	}
}

func TestDiffPassesUnknownThroughModuleOutput(t *testing.T) {
	h := setupLifecycle(t)

	result := h.runDiff(t, diffUnknownOutputConfig)

	fromOutput := diffResourceAt(t, result, diffUnknownFromOutputID)
	require.Equal(t, diff.ActionCreate, fromOutput.Action)
	require.Equal(t, diff.Change{
		Path:    diff.Path{}.Attribute("network").Index(0).Attribute("name"),
		Unknown: true,
	}, changeAt(t, fromOutput, diffUnknownNetworkNamePath))
}

func TestDiffPassesUnknownThroughModuleVariable(t *testing.T) {
	h := setupLifecycle(t)

	result := h.runDiff(t, diffUnknownOutputConfig)

	fromVariable := diffResourceAt(t, result, diffUnknownFromVariableID)
	require.Equal(t, diff.ActionCreate, fromVariable.Action)
	require.Equal(t, diff.Change{
		Path:    diff.Path{}.Attribute("network").Index(0).Attribute("name"),
		Unknown: true,
	}, changeAt(t, fromVariable, diffUnknownNetworkNamePath))
}

func TestDiffThroughModulesListsEveryNewResource(t *testing.T) {
	h := setupLifecycle(t)

	result := h.runDiff(t, diffUnknownOutputConfig)

	require.Equal(t, []string{
		diffUnknownFromVariableID,
		diffUnknownInnerNetworkID,
		diffUnknownFromOutputID,
		diffUnknownRootNetworkID,
	}, diffAddresses(result))
	require.Equal(t, diff.Summary{Create: 4}, result.Summary)
}

func TestDiffThroughModulesOfAppliedConfigurationHasNoUnknowns(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, diffUnknownOutputConfig)
	h.plugin.ResetCalls()

	result := h.runDiff(t, diffUnknownOutputConfig)

	// every provider id is saved, so nothing is unknown and every resource
	// is read
	require.Empty(t, result.Resources)
	require.Equal(t, diff.Summary{Unchanged: 4}, result.Summary)
	require.Equal(t, sorted(
		diffUnknownFromOutputID,
		diffUnknownFromVariableID,
		diffUnknownInnerNetworkID,
		diffUnknownRootNetworkID,
	), sortedCopy(h.plugin.GetReadResources()))
}

func TestDiffPassesRecordedUnknownOfSavedResourceToItsDependents(t *testing.T) {
	h := applyUnknownRefBefore(t)

	result := h.runDiff(t, diffUnknownChainConfig)

	// user is saved, not pending: follower's name is unknown only because
	// user's own name was recorded as unknown
	follower := diffResourceAt(t, result, diffUnknownFollowerID)
	require.Equal(t, diff.ActionCreate, follower.Action)
	require.Equal(t, diff.Change{
		Path:    diff.Path{}.Attribute("network").Index(0).Attribute("name"),
		Unknown: true,
	}, changeAt(t, follower, diffUnknownNetworkNamePath))
}

func TestDiffKeepsKnownValuesOfSavedResourceForItsDependents(t *testing.T) {
	h := applyUnknownRefBefore(t)

	result := h.runDiff(t, diffUnknownChainConfig)

	follower := diffResourceAt(t, result, diffUnknownFollowerID)
	require.Equal(t, diff.Change{
		Path:  diff.Path{}.Attribute("default"),
		After: diffUnknownDefaultContainer,
	}, changeAt(t, follower, "default"))
}

func TestApplyAfterDiffWithUnknownValuesCreatesAndUpdatesWithProviderID(t *testing.T) {
	h := applyUnknownRefBefore(t)

	h.runDiff(t, diffUnknownRefAfterConfig)
	h.plugin.ResetCalls()

	st := h.applyAndSave(t, diffUnknownRefAfterConfig)

	require.Equal(t, []string{diffUnknownNetworkTwoID}, h.plugin.GetCreatedResources())
	require.Equal(t, []string{diffUnknownUserID}, h.plugin.GetUpdatedResources())

	network := findResource[structs.Network](t, st.GetResources(), diffUnknownNetworkTwoID)
	require.Equal(t, "id-two", network.ProviderID)

	user := findResource[structs.Container](t, st.GetResources(), diffUnknownUserID)
	require.Len(t, user.Networks, 1)
	require.Equal(t, "id-two", user.Networks[0].Name)
	require.Equal(t, types.StatusUpdated, user.Meta.Status)
}

func TestContextValueMakesComputedFieldsOfPendingEntityUnknown(t *testing.T) {
	recorder := newDiffRecorder()
	recorder.markPending(diffUnknownNetworkTwoID)

	network := &structs.Network{Subnet: "10.1.0.0/16"}
	network.Meta = types.Meta{ID: diffUnknownNetworkTwoID}

	value := cty.ObjectVal(map[string]cty.Value{
		"subnet":      cty.StringVal("10.1.0.0/16"),
		"provider_id": cty.StringVal(""),
		"observed":    cty.StringVal(""),
	})

	got := recorder.contextValue(network, value)

	require.Equal(t, cty.StringVal("10.1.0.0/16"), got.GetAttr("subnet"))
	require.False(t, got.GetAttr("provider_id").IsKnown())
	require.False(t, got.GetAttr("observed").IsKnown())
}

func TestContextValueLeavesEntityThatIsNotPendingAsItIs(t *testing.T) {
	recorder := newDiffRecorder()

	network := &structs.Network{Subnet: "10.0.0.0/16", ProviderID: "id-one"}
	network.Meta = types.Meta{ID: diffUnknownNetworkOneID}

	value := cty.ObjectVal(map[string]cty.Value{
		"subnet":      cty.StringVal("10.0.0.0/16"),
		"provider_id": cty.StringVal("id-one"),
		"observed":    cty.StringVal(""),
	})

	got := recorder.contextValue(network, value)

	require.True(t, got.RawEquals(value))
}

func TestContextValueMakesComputedFieldOfEveryNestedBlockUnknown(t *testing.T) {
	recorder := newDiffRecorder()
	recorder.markPending(diffUnknownUserID)

	container := &structs.Container{}
	container.Meta = types.Meta{ID: diffUnknownUserID}

	attachment := func(name string) cty.Value {
		return cty.ObjectVal(map[string]cty.Value{
			"name":                        cty.StringVal(name),
			diffUnknownAssignedAddressKey: cty.StringVal(""),
		})
	}

	value := cty.ObjectVal(map[string]cty.Value{
		"default": cty.StringVal(diffUnknownDefaultContainer),
		"network": cty.ListVal([]cty.Value{attachment("a"), attachment("b")}),
	})

	got := recorder.contextValue(container, value)

	networks := got.GetAttr("network").AsValueSlice()
	require.Len(t, networks, 2)
	require.Equal(t, cty.StringVal("a"), networks[0].GetAttr("name"))
	require.False(t, networks[0].GetAttr(diffUnknownAssignedAddressKey).IsKnown())
	require.Equal(t, cty.StringVal("b"), networks[1].GetAttr("name"))
	require.False(t, networks[1].GetAttr(diffUnknownAssignedAddressKey).IsKnown())
	require.Equal(t, cty.StringVal(diffUnknownDefaultContainer), got.GetAttr("default"))
}

func TestContextValueMakesRecordedUnknownPathOfSavedEntityUnknown(t *testing.T) {
	recorder := newDiffRecorder()
	recorder.recordUnknown(diffUnknownUserID, []diff.Path{
		diff.Path{}.Attribute("network").Index(0).Attribute("name"),
	})

	container := &structs.Container{}
	container.Meta = types.Meta{ID: diffUnknownUserID}

	value := cty.ObjectVal(map[string]cty.Value{
		"default": cty.StringVal(diffUnknownDefaultContainer),
		"network": cty.ListVal([]cty.Value{
			cty.ObjectVal(map[string]cty.Value{
				"name":                        cty.StringVal(""),
				diffUnknownAssignedAddressKey: cty.StringVal("assigned-fixed"),
			}),
		}),
	})

	got := recorder.contextValue(container, value)

	network := got.GetAttr("network").Index(cty.NumberIntVal(0))
	require.False(t, network.GetAttr("name").IsKnown())
	require.Equal(t, cty.StringVal("assigned-fixed"), network.GetAttr(diffUnknownAssignedAddressKey))
	require.Equal(t, cty.StringVal(diffUnknownDefaultContainer), got.GetAttr("default"))
}

func TestUnknownAtPathMakesMapEntryUnknown(t *testing.T) {
	value := cty.ObjectVal(map[string]cty.Value{
		"env": cty.MapVal(map[string]cty.Value{
			"A": cty.StringVal("x"),
			"B": cty.StringVal(""),
		}),
	})

	got := unknownAtPath(value, diff.Path{}.Attribute("env").Key("B"))

	env := got.GetAttr("env")
	require.Equal(t, cty.StringVal("x"), env.Index(cty.StringVal("A")))
	require.False(t, env.Index(cty.StringVal("B")).IsKnown())
}

func TestUnknownAtPathLeavesValueAsItIsForPathLeadingNowhere(t *testing.T) {
	value := cty.ObjectVal(map[string]cty.Value{
		"command": cty.ListVal([]cty.Value{cty.StringVal("run")}),
	})

	got := unknownAtPath(value, diff.Path{}.Attribute("command").Index(5))

	require.True(t, got.RawEquals(value))
}

func TestUnknownAtPathKeepsMarks(t *testing.T) {
	value := cty.ObjectVal(map[string]cty.Value{
		"secret": cty.StringVal("hidden").Mark("sensitive"),
	})

	got := unknownAtPath(value, diff.Path{}.Attribute("secret"))

	secret := got.GetAttr("secret")
	require.False(t, secret.IsKnown())
	require.True(t, secret.HasMark("sensitive"))
}

func TestUnknownPathsSplitsObjectUntilEachUnknownStandsAlone(t *testing.T) {
	value := cty.ObjectVal(map[string]cty.Value{
		"a": cty.StringVal("known"),
		"b": cty.UnknownVal(cty.String),
		"c": cty.ListVal([]cty.Value{cty.StringVal("x"), cty.UnknownVal(cty.String)}),
	})

	paths := unknownPaths(value, diff.Path{}.Attribute("root"), nil)

	require.Equal(t, []diff.Path{
		diff.Path{}.Attribute("root").Attribute("b"),
		diff.Path{}.Attribute("root").Attribute("c").Index(1),
	}, paths)
}

func TestUnknownPathsStepsIntoGoMapFieldByKey(t *testing.T) {
	// an object literal assigned to a Go map field is stepped into by key
	value := cty.ObjectVal(map[string]cty.Value{
		"A": cty.StringVal("x"),
		"B": cty.UnknownVal(cty.String),
	})

	paths := unknownPaths(value, diff.Path{}.Attribute("env"), namedFieldType(containerType, "env"))

	require.Equal(t, []diff.Path{diff.Path{}.Attribute("env").Key("B")}, paths)
}

func TestUnknownPathsReportsWhollyKnownValueAsNothing(t *testing.T) {
	value := cty.ObjectVal(map[string]cty.Value{
		"a": cty.StringVal("known"),
	})

	paths := unknownPaths(value, diff.Path{}.Attribute("root"), nil)

	require.Empty(t, paths)
}

func TestUnknownPathsReportsSetHoldingUnknownAsWhole(t *testing.T) {
	value := cty.SetVal([]cty.Value{cty.StringVal("x"), cty.UnknownVal(cty.String)})

	paths := unknownPaths(value, diff.Path{}.Attribute("dns"), nil)

	require.Equal(t, []diff.Path{diff.Path{}.Attribute("dns")}, paths)
}

func TestWithPlaceholdersReplacesEveryUnknownWithZeroValueOfItsType(t *testing.T) {
	value := cty.ObjectVal(map[string]cty.Value{
		"name":    cty.UnknownVal(cty.String),
		"count":   cty.UnknownVal(cty.Number),
		"enabled": cty.UnknownVal(cty.Bool),
		"list":    cty.UnknownVal(cty.List(cty.String)),
		"known":   cty.StringVal("kept"),
	})

	got := withPlaceholders(value)

	require.True(t, got.IsWhollyKnown())
	require.Equal(t, cty.StringVal(""), got.GetAttr("name"))
	require.True(t, got.GetAttr("count").RawEquals(cty.Zero))
	require.Equal(t, cty.False, got.GetAttr("enabled"))
	require.True(t, got.GetAttr("list").RawEquals(cty.ListValEmpty(cty.String)))
	require.Equal(t, cty.StringVal("kept"), got.GetAttr("known"))
}

func TestWithPlaceholdersKeepsMarksOfTheReplacedUnknown(t *testing.T) {
	value := cty.UnknownVal(cty.String).Mark("sensitive")

	got := withPlaceholders(value)

	require.True(t, got.IsKnown())
	require.True(t, got.HasMark("sensitive"))
}
