package parser

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/events"
)

const (
	updateCredentialBeforePassword = "diff-before-pw-3a9c"
	updateCredentialAfterPassword  = "diff-after-pw-8e1f"
)

// applyCredentialBefore applies and saves the credential before
// configuration, then clears the plugin's calls
func applyCredentialBefore(t *testing.T) *lifecycleHarness {
	t.Helper()

	h := setupLifecycle(t)
	h.applyAndSave(t, decideCredentialBeforeConfig)
	h.plugin.ResetCalls()

	return h
}

// lockedBuffer is a bytes.Buffer that can be written from several goroutines,
// as the walker emits events concurrently
type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func TestUpdateIsToldEditedSettingWithPreviousAndNewValues(t *testing.T) {
	h := applyUpdateRefBefore(t)
	h.plugin.SetChangedResult(diffUpdateRefAppID, entity.Update)

	h.applyAndSave(t, diffUpdateRefEditedConfig)

	require.Contains(t, h.plugin.GetUpdatedResources(), diffUpdateRefAppID)
	require.Equal(t, []entity.PropertyChange{
		{
			Path:   entity.Path{}.Attribute("subnet"),
			Before: "10.0.0.0/16",
			After:  "10.9.0.0/16",
		},
	}, h.plugin.GetUpdateChanges(diffUpdateRefAppID))
}

func TestUpdateIsToldNoSettingsWhenOnlyDependencyIsReplaced(t *testing.T) {
	h := applyUpdateRefBefore(t)
	h.plugin.SetChangedResult(diffUpdateRefAppID, entity.Replace)
	h.plugin.SetChangedResult(diffUpdateRefNamedID, entity.Update)

	h.applyAndSave(t, diffUpdateRefBeforeConfig)

	// named takes app's name, which replacing app leaves as it was
	require.Contains(t, h.plugin.GetUpdatedResources(), diffUpdateRefNamedID)
	require.Empty(t, h.plugin.GetUpdateChanges(diffUpdateRefNamedID))
}

func TestUpdateIsToldDependencyIsReplaced(t *testing.T) {
	h := applyUpdateRefBefore(t)
	h.plugin.SetChangedResult(diffUpdateRefAppID, entity.Replace)
	h.plugin.SetChangedResult(diffUpdateRefNamedID, entity.Update)

	h.applyAndSave(t, diffUpdateRefBeforeConfig)

	require.Equal(t, []entity.DependencyChange{
		{Address: diffUpdateRefAppID, Change: entity.Replace},
	}, h.plugin.GetUpdateDependencies(diffUpdateRefNamedID))
}

func TestUpdateIsToldTheDependenciesTheDecisionWasToldAbout(t *testing.T) {
	h := applyUpdateRefBefore(t)
	h.plugin.SetChangedResult(diffUpdateRefAppID, entity.Replace)
	h.plugin.SetChangedResult(diffUpdateRefNamedID, entity.Update)

	h.applyAndSave(t, diffUpdateRefBeforeConfig)

	require.NotEmpty(t, h.plugin.GetChangedDependencies(diffUpdateRefNamedID))
	require.Equal(t, h.plugin.GetChangedDependencies(diffUpdateRefNamedID), h.plugin.GetUpdateDependencies(diffUpdateRefNamedID))
}

func TestUpdateIsToldRealValueFromDependencyReplacedInSameApply(t *testing.T) {
	h := applyUpdateRefBefore(t)
	h.plugin.SetChangedResult(diffUpdateRefAppID, entity.Replace)
	h.plugin.SetChangedResult(diffUpdateRefUserID, entity.Update)

	h.applyAndSave(t, diffUpdateRefBeforeConfig)

	// the decision was told user's network name is not yet known
	require.Equal(t, []entity.PropertyChange{
		{
			Path:    entity.Path{}.Attribute("network").Index(0).Attribute("name"),
			Before:  "id-app",
			After:   nil,
			Unknown: true,
		},
	}, h.plugin.GetChangedChanges(diffUpdateRefUserID))

	// by the update app has been created again and has set its provider id,
	// which is the same as before but is still listed, now known
	require.Equal(t, []entity.PropertyChange{
		{
			Path:   entity.Path{}.Attribute("network").Index(0).Attribute("name"),
			Before: "id-app",
			After:  "id-app",
		},
	}, h.plugin.GetUpdateChanges(diffUpdateRefUserID))
}

func TestUpdateIsToldRealValuesOfEditedSensitiveSetting(t *testing.T) {
	h := applyCredentialBefore(t)
	h.plugin.SetChangedResult(decideCredentialID, entity.Update)

	h.applyAndSave(t, decideCredentialAfterConfig)

	require.Equal(t, []string{decideCredentialID}, h.plugin.GetUpdatedResources())
	require.Equal(t, []entity.PropertyChange{
		{
			Path:      entity.Path{}.Attribute("password"),
			Before:    updateCredentialBeforePassword,
			After:     updateCredentialAfterPassword,
			Sensitive: true,
		},
	}, h.plugin.GetUpdateChanges(decideCredentialID))
}

func TestEventsOfUpdateOfEditedSensitiveSettingCarryNoRealValueAtRawData(t *testing.T) {
	h := applyCredentialBefore(t)
	h.plugin.SetChangedResult(decideCredentialID, entity.Update)

	collector := &eventCollector{}
	p := h.newParserWithEventData(t, collector.collect, events.DataRaw)

	_, err := p.Apply(context.Background(), decideCredentialAfterConfig)
	require.NoError(t, err)
	require.Equal(t, []string{decideCredentialID}, h.plugin.GetUpdatedResources())

	updateData := ""
	for _, event := range collector.all() {
		if event.ResourceID == decideCredentialID && event.Operation == events.OperationUpdate {
			updateData += string(event.Data)
		}

		written := fmt.Sprintf("%s %v %v", string(event.Data), event.Meta, event.Error)
		require.NotContains(t, written, updateCredentialBeforePassword)
		require.NotContains(t, written, updateCredentialAfterPassword)
	}

	// the update events do carry the credential, with its password masked
	require.Contains(t, updateData, `"password":{"xcl_masked":"redact","value":"(sensitive)"}`)
}

func TestEventsOfUpdateOfEditedSensitiveSettingCarryNoRealValueAtProcessedData(t *testing.T) {
	h := applyCredentialBefore(t)
	h.plugin.SetChangedResult(decideCredentialID, entity.Update)

	collector := &eventCollector{}
	p := h.newParserWithEventData(t, collector.collect, events.DataProcessed)

	_, err := p.Apply(context.Background(), decideCredentialAfterConfig)
	require.NoError(t, err)
	require.Equal(t, []string{decideCredentialID}, h.plugin.GetUpdatedResources())

	updateData := ""
	for _, event := range collector.all() {
		if event.ResourceID == decideCredentialID && event.Operation == events.OperationUpdate {
			updateData += string(event.Data)
		}

		written := fmt.Sprintf("%s %v %v", string(event.Data), event.Meta, event.Error)
		require.NotContains(t, written, updateCredentialBeforePassword)
		require.NotContains(t, written, updateCredentialAfterPassword)
	}

	// the update events do carry the credential, with its password masked
	require.Contains(t, updateData, `"password":{"xcl_masked":"redact","value":"(sensitive)"}`)
}

func TestLogsOfUpdateOfEditedSensitiveSettingCarryNoRealValue(t *testing.T) {
	h := applyCredentialBefore(t)
	h.plugin.SetChangedResult(decideCredentialID, entity.Update)

	logs := &lockedBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	p := h.newParserWithEventData(t, events.Emit(events.SlogHandler(logger)), events.DataRaw)

	_, err := p.Apply(context.Background(), decideCredentialAfterConfig)
	require.NoError(t, err)
	require.Equal(t, []string{decideCredentialID}, h.plugin.GetUpdatedResources())

	written := logs.String()
	require.Contains(t, written, decideCredentialID, "the logs hold nothing about the credential")
	require.NotContains(t, written, updateCredentialBeforePassword)
	require.NotContains(t, written, updateCredentialAfterPassword)
}

func TestPlanShowsEditedSensitiveSettingHidden(t *testing.T) {
	h := applyCredentialBefore(t)
	h.plugin.SetChangedResult(decideCredentialID, entity.Update)

	result := h.runDiff(t, decideCredentialAfterConfig)

	credential := diffResourceAt(t, result, decideCredentialID)
	require.Equal(t, diff.ActionUpdate, credential.Action)
	require.Len(t, credential.Changes, 1)
	require.Equal(t, "password", credential.Changes[0].Path.String())
	require.True(t, credential.Changes[0].Sensitive)
	require.Nil(t, credential.Changes[0].Before)
	require.Nil(t, credential.Changes[0].After)
}

func TestPlanDecisionAndUpdateListTheSameSettingsForPlainEdit(t *testing.T) {
	h := applyUpdateRefBefore(t)
	h.plugin.SetChangedResult(diffUpdateRefAppID, entity.Update)

	result := h.runDiff(t, diffUpdateRefEditedConfig)
	planned := diffChangePaths(diffResourceAt(t, result, diffUpdateRefAppID).Changes)
	h.plugin.ResetCalls()

	h.applyAndSave(t, diffUpdateRefEditedConfig)

	require.Equal(t, []string{"subnet"}, planned)
	require.Equal(t, planned, changePaths(h.plugin.GetChangedChanges(diffUpdateRefAppID)))
	require.Equal(t, planned, changePaths(h.plugin.GetUpdateChanges(diffUpdateRefAppID)))
}

func TestPlanDecisionAndUpdateListTheSameSettingsForSensitiveEdit(t *testing.T) {
	h := applyCredentialBefore(t)
	h.plugin.SetChangedResult(decideCredentialID, entity.Update)

	result := h.runDiff(t, decideCredentialAfterConfig)
	planned := diffChangePaths(diffResourceAt(t, result, decideCredentialID).Changes)
	h.plugin.ResetCalls()

	h.applyAndSave(t, decideCredentialAfterConfig)

	require.Equal(t, []string{"password"}, planned)
	require.Equal(t, planned, changePaths(h.plugin.GetChangedChanges(decideCredentialID)))
	require.Equal(t, planned, changePaths(h.plugin.GetUpdateChanges(decideCredentialID)))
}

func TestPlanDecisionAndUpdateListTheSameSettingsForValueNotYetKnown(t *testing.T) {
	h := applyUpdateRefBefore(t)
	h.plugin.SetChangedResult(diffUpdateRefAppID, entity.Replace)
	h.plugin.SetChangedResult(diffUpdateRefUserID, entity.Update)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)
	planned := diffChangePaths(diffResourceAt(t, result, diffUpdateRefUserID).Changes)
	h.plugin.ResetCalls()

	h.applyAndSave(t, diffUpdateRefBeforeConfig)

	require.Equal(t, []string{"network[0].name"}, planned)
	require.Equal(t, planned, changePaths(h.plugin.GetChangedChanges(diffUpdateRefUserID)))
	require.Equal(t, planned, changePaths(h.plugin.GetUpdateChanges(diffUpdateRefUserID)))
}

func TestPlanNamesTheReplacedDependencyBehindAnUpdate(t *testing.T) {
	h := applyUpdateRefBefore(t)
	h.plugin.SetChangedResult(diffUpdateRefAppID, entity.Replace)
	h.plugin.SetChangedResult(diffUpdateRefNamedID, entity.Update)

	result := h.runDiff(t, diffUpdateRefBeforeConfig)

	named := diffResourceAt(t, result, diffUpdateRefNamedID)
	require.Equal(t, diff.ActionUpdate, named.Action)
	require.Equal(t, []diff.Dependency{
		{Address: diffUpdateRefAppID, Action: diff.ActionReplace},
	}, named.Dependencies)
}

func TestPlanNamesNoDependenciesForAnUpdateWithoutChangingDependencies(t *testing.T) {
	h := applyUpdateRefBefore(t)
	h.plugin.SetChangedResult(diffUpdateRefAppID, entity.Update)

	result := h.runDiff(t, diffUpdateRefEditedConfig)

	app := diffResourceAt(t, result, diffUpdateRefAppID)
	require.Equal(t, diff.ActionUpdate, app.Action)
	require.Empty(t, app.Dependencies)
}
