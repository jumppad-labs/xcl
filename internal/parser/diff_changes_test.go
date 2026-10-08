package parser

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/types"
)

// changesLogin is a block holding a sensitive leaf, so a test can add or
// remove a whole value that contains a secret
type changesLogin struct {
	User   string                  `xcl:"user"`
	Secret types.Sensitive[string] `xcl:"secret"`
}

// changesVault is a resource with a list of blocks holding sensitive leaves
type changesVault struct {
	types.ResourceBase `xcl:",remain"`

	Logins []changesLogin `xcl:"login,block"`
}

// requireNoSecret fails when the secret can be read anywhere in the JSON form
// of the changes
func requireNoSecret(t *testing.T, changes []diff.Change, secret string) {
	t.Helper()

	encoded, err := json.Marshal(changes)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), secret)
}

func TestResourceChangesUpdateOfOneLeafIsOneEntryWithBeforeAndAfter(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Default: "one", Privileged: false}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Default: "two", Privileged: false}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, "default", changes[0].Path.String())
	require.Equal(t, "one", changes[0].Before)
	require.Equal(t, "two", changes[0].After)
	require.False(t, changes[0].Unknown)
	require.False(t, changes[0].Sensitive)
}

func TestResourceChangesUpdateOfIdenticalResourcesHasNoChanges(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{
		Default: "one",
		Env:     map[string]string{"LOG_LEVEL": "debug"},
		Ports:   []structs.Port{{Local: 8080, Remote: 80}},
		RunAs:   &structs.User{User: "root", Group: "wheel"},
	}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Default: "one",
		Env:     map[string]string{"LOG_LEVEL": "debug"},
		Ports:   []structs.Port{{Local: 8080, Remote: 80}},
		RunAs:   &structs.User{User: "root", Group: "wheel"},
	}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Empty(t, changes)
}

func TestResourceChangesReplaceComparesLikeUpdate(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{MaxRestartCount: 1}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{MaxRestartCount: 3}}

	changes := resourceChanges(diff.ActionReplace, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, "max_restart_count", changes[0].Path.String())
	require.Equal(t, 1, changes[0].Before)
	require.Equal(t, 3, changes[0].After)
}

func TestResourceChangesDeleteHasNoChanges(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Default: "one"}}

	changes := resourceChanges(diff.ActionDelete, saved, nil, nil, nil, false)

	require.Empty(t, changes)
}

func TestResourceChangesNeverReportsComputedNetworkFields(t *testing.T) {
	saved := &structs.Network{Subnet: "10.0.0.0/16", ProviderID: "abc", Observed: "old"}
	configured := &structs.Network{Subnet: "10.0.0.0/16", ProviderID: "def", Observed: "new"}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Empty(t, changes)
}

func TestResourceChangesNeverReportsComputedFieldInNestedBlock(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{
		Build:    &structs.Build{Context: "./", ImageID: "sha256:old"},
		Networks: []structs.NetworkAttachment{{Name: "app", AssignedAddress: "10.0.0.2"}},
	}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Build:    &structs.Build{Context: "./", ImageID: "sha256:new"},
		Networks: []structs.NetworkAttachment{{Name: "app", AssignedAddress: "10.0.0.3"}},
	}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Empty(t, changes)
}

func TestResourceChangesNestedBlockLeafExtendsPath(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{RunAs: &structs.User{User: "root", Group: "wheel"}}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{RunAs: &structs.User{User: "app", Group: "wheel"}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, "run_as.user", changes[0].Path.String())
	require.Equal(t, "root", changes[0].Before)
	require.Equal(t, "app", changes[0].After)
}

func TestResourceChangesListBlockLeafExtendsPathWithIndex(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app"}},
	}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "web"}},
	}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, "network[0].name", changes[0].Path.String())
	require.Equal(t, "app", changes[0].Before)
	require.Equal(t, "web", changes[0].After)
}

func TestResourceChangesListComparesStrictlyByIndex(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Ports: []structs.Port{
		{Local: 80, Remote: 80},
		{Local: 443, Remote: 443},
		{Local: 8080, Remote: 8080},
	}}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Ports: []structs.Port{
		{Local: 443, Remote: 443},
		{Local: 8080, Remote: 8080},
	}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 5)

	require.Equal(t, "port[0].local", changes[0].Path.String())
	require.Equal(t, 80, changes[0].Before)
	require.Equal(t, 443, changes[0].After)

	require.Equal(t, "port[0].remote", changes[1].Path.String())
	require.Equal(t, 80, changes[1].Before)
	require.Equal(t, 443, changes[1].After)

	require.Equal(t, "port[1].local", changes[2].Path.String())
	require.Equal(t, 443, changes[2].Before)
	require.Equal(t, 8080, changes[2].After)

	require.Equal(t, "port[1].remote", changes[3].Path.String())
	require.Equal(t, 443, changes[3].Before)
	require.Equal(t, 8080, changes[3].After)

	require.Equal(t, "port[2]", changes[4].Path.String())
	require.Equal(t, map[string]any{"local": 8080, "remote": 8080}, changes[4].Before)
	require.Nil(t, changes[4].After)
}

func TestResourceChangesAddedListElementHasOnlyAfter(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Ports: []structs.Port{{Local: 80, Remote: 80}}}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Ports: []structs.Port{
		{Local: 80, Remote: 80},
		{Local: 8443, Remote: 443},
	}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, "port[1]", changes[0].Path.String())
	require.Nil(t, changes[0].Before)
	require.Equal(t, map[string]any{"local": 8443, "remote": 443}, changes[0].After)
}

func TestResourceChangesRemovedListElementHasOnlyBefore(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{DNS: []string{"1.1.1.1", "8.8.8.8"}}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{DNS: []string{"1.1.1.1"}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, "dns[1]", changes[0].Path.String())
	require.Equal(t, "8.8.8.8", changes[0].Before)
	require.Nil(t, changes[0].After)
}

func TestResourceChangesAddedBlockElementOmitsComputedField(t *testing.T) {
	saved := &structs.Container{}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app", AssignedAddress: "10.0.0.2"}},
	}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, "network[0]", changes[0].Path.String())
	require.Equal(t, map[string]any{"id": 0, "name": "app", "ip_address": "", "aliases": nil}, changes[0].After)
}

func TestResourceChangesMapChangedKeyHasBeforeAndAfter(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Env: map[string]string{"LOG_LEVEL": "info", "MODE": "a"}}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Env: map[string]string{"LOG_LEVEL": "debug", "MODE": "a"}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, `env["LOG_LEVEL"]`, changes[0].Path.String())
	require.Equal(t, "info", changes[0].Before)
	require.Equal(t, "debug", changes[0].After)
}

func TestResourceChangesMapAddedKeyHasOnlyAfter(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Env: map[string]string{"MODE": "a"}}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Env: map[string]string{"LOG_LEVEL": "debug", "MODE": "a"}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, `env["LOG_LEVEL"]`, changes[0].Path.String())
	require.Nil(t, changes[0].Before)
	require.Equal(t, "debug", changes[0].After)
}

func TestResourceChangesMapRemovedKeyHasOnlyBefore(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Env: map[string]string{"LOG_LEVEL": "debug", "MODE": "a"}}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Env: map[string]string{"MODE": "a"}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, `env["LOG_LEVEL"]`, changes[0].Path.String())
	require.Equal(t, "debug", changes[0].Before)
	require.Nil(t, changes[0].After)
}

func TestResourceChangesNilAndEmptyMapAreEqual(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Env: nil}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Env: map[string]string{}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Empty(t, changes)
}

func TestResourceChangesNilAndEmptyListAreEqual(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Ports: []structs.Port{}, DNS: nil}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Ports: nil, DNS: []string{}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Empty(t, changes)
}

func TestResourceChangesCreateListsNonZeroFieldsWithOnlyAfter(t *testing.T) {
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Default: "hello world",
		Env:     map[string]string{"LOG_LEVEL": "debug"},
		Ports:   []structs.Port{{Local: 8080, Remote: 80}},
		RunAs:   &structs.User{User: "root", Group: "wheel"},
	}}

	changes := resourceChanges(diff.ActionCreate, nil, configured, nil, nil, false)

	require.Len(t, changes, 4)

	require.Equal(t, "default", changes[0].Path.String())
	require.Nil(t, changes[0].Before)
	require.Equal(t, "hello world", changes[0].After)

	require.Equal(t, "env", changes[1].Path.String())
	require.Nil(t, changes[1].Before)
	require.Equal(t, map[string]any{"LOG_LEVEL": "debug"}, changes[1].After)

	require.Equal(t, "port", changes[2].Path.String())
	require.Nil(t, changes[2].Before)
	require.Equal(t, []any{map[string]any{"local": 8080, "remote": 80}}, changes[2].After)

	require.Equal(t, "run_as", changes[3].Path.String())
	require.Nil(t, changes[3].Before)
	require.Equal(t, map[string]any{"user": "root", "group": "wheel"}, changes[3].After)
}

func TestResourceChangesCreateOmitsZeroFieldsNotSetInBody(t *testing.T) {
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Privileged: false, MaxRestartCount: 0}}

	changes := resourceChanges(diff.ActionCreate, nil, configured, nil, nil, false)

	require.Empty(t, changes)
}

func TestResourceChangesCreateListsZeroFieldSetInBody(t *testing.T) {
	body := parseResourceBody(t, `privileged = false`)
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Privileged: false}}

	changes := resourceChanges(diff.ActionCreate, nil, configured, body, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, "privileged", changes[0].Path.String())
	require.Nil(t, changes[0].Before)
	require.Equal(t, false, changes[0].After)
}

func TestResourceChangesCreateOmitsComputedFields(t *testing.T) {
	configured := &structs.Network{Subnet: "10.0.0.0/16", ProviderID: "abc", Observed: "seen"}

	changes := resourceChanges(diff.ActionCreate, nil, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, "subnet", changes[0].Path.String())
	require.Equal(t, "10.0.0.0/16", changes[0].After)
}

func TestResourceChangesChangedSensitiveLeafHasNoValues(t *testing.T) {
	saved := &structs.Credential{Password: types.NewSensitive("old-secret")}
	configured := &structs.Credential{Password: types.NewSensitive("new-secret")}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 1)
	require.Equal(t, "password", changes[0].Path.String())
	require.True(t, changes[0].Sensitive)
	require.Nil(t, changes[0].Before)
	require.Nil(t, changes[0].After)
	requireNoSecret(t, changes, "old-secret")
	requireNoSecret(t, changes, "new-secret")
}

func TestResourceChangesUnchangedSensitiveLeafHasNoChange(t *testing.T) {
	saved := &structs.Credential{Password: types.NewSensitive("secret"), Pin: types.NewSensitive(1234)}
	configured := &structs.Credential{Password: types.NewSensitive("secret"), Pin: types.NewSensitive(1234)}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Empty(t, changes)
}

func TestResourceChangesRevealedSensitiveLeafHasValuesAndIsStillSensitive(t *testing.T) {
	saved := &structs.Credential{Pin: types.NewSensitive(1234)}
	configured := &structs.Credential{Pin: types.NewSensitive(5678)}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, true)

	require.Len(t, changes, 1)
	require.Equal(t, "pin", changes[0].Path.String())
	require.True(t, changes[0].Sensitive)
	require.Equal(t, 1234, changes[0].Before)
	require.Equal(t, 5678, changes[0].After)
}

func TestResourceChangesCreateOfSensitiveFieldsHidesSecrets(t *testing.T) {
	configured := &structs.Credential{
		Username: "admin",
		Password: types.NewSensitive("top-secret"),
		Pin:      types.NewSensitive(9876),
	}

	changes := resourceChanges(diff.ActionCreate, nil, configured, nil, nil, false)

	require.Len(t, changes, 3)

	require.Equal(t, "username", changes[0].Path.String())
	require.Equal(t, "admin", changes[0].After)
	require.False(t, changes[0].Sensitive)

	require.Equal(t, "password", changes[1].Path.String())
	require.True(t, changes[1].Sensitive)
	require.Nil(t, changes[1].Before)
	require.Nil(t, changes[1].After)

	require.Equal(t, "pin", changes[2].Path.String())
	require.True(t, changes[2].Sensitive)
	require.Nil(t, changes[2].Before)
	require.Nil(t, changes[2].After)

	requireNoSecret(t, changes, "top-secret")
	requireNoSecret(t, changes, "9876")
}

func TestResourceChangesAddedWholeValueSplitsSensitiveLeafOut(t *testing.T) {
	saved := &changesVault{}
	configured := &changesVault{Logins: []changesLogin{{User: "admin", Secret: types.NewSensitive("hidden-value")}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 2)

	require.Equal(t, "login[0].user", changes[0].Path.String())
	require.Nil(t, changes[0].Before)
	require.Equal(t, "admin", changes[0].After)
	require.False(t, changes[0].Sensitive)

	require.Equal(t, "login[0].secret", changes[1].Path.String())
	require.True(t, changes[1].Sensitive)
	require.Nil(t, changes[1].Before)
	require.Nil(t, changes[1].After)

	requireNoSecret(t, changes, "hidden-value")
}

func TestResourceChangesRemovedWholeValueSplitsSensitiveLeafOut(t *testing.T) {
	saved := &changesVault{Logins: []changesLogin{{User: "admin", Secret: types.NewSensitive("hidden-value")}}}
	configured := &changesVault{}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	require.Len(t, changes, 2)

	require.Equal(t, "login[0].user", changes[0].Path.String())
	require.Equal(t, "admin", changes[0].Before)
	require.Nil(t, changes[0].After)

	require.Equal(t, "login[0].secret", changes[1].Path.String())
	require.True(t, changes[1].Sensitive)
	require.Nil(t, changes[1].Before)
	require.Nil(t, changes[1].After)

	requireNoSecret(t, changes, "hidden-value")
}

func TestResourceChangesRevealedAddedWholeValueShowsSensitiveLeaf(t *testing.T) {
	saved := &changesVault{}
	configured := &changesVault{Logins: []changesLogin{{User: "admin", Secret: types.NewSensitive("hidden-value")}}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, true)

	require.Len(t, changes, 2)

	require.Equal(t, "login[0].user", changes[0].Path.String())
	require.Equal(t, "admin", changes[0].After)

	require.Equal(t, "login[0].secret", changes[1].Path.String())
	require.True(t, changes[1].Sensitive)
	require.Equal(t, "hidden-value", changes[1].After)
}

func TestResourceChangesUnknownPathOnUpdateHasSavedBeforeAndNoAfter(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app", IPAddress: "10.0.0.5"}},
	}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app", IPAddress: ""}},
	}}
	unknown := []diff.Path{diff.Path{}.Attribute("network").Index(0).Attribute("ip_address")}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, unknown, false)

	require.Len(t, changes, 1)
	require.Equal(t, "network[0].ip_address", changes[0].Path.String())
	require.True(t, changes[0].Unknown)
	require.Equal(t, "10.0.0.5", changes[0].Before)
	require.Nil(t, changes[0].After)
}

func TestResourceChangesUnknownPathIsReportedEvenWhenValuesAreEqual(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{Default: "same"}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{Default: "same"}}
	unknown := []diff.Path{diff.Path{}.Attribute("default")}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, unknown, false)

	require.Len(t, changes, 1)
	require.Equal(t, "default", changes[0].Path.String())
	require.True(t, changes[0].Unknown)
	require.Equal(t, "same", changes[0].Before)
	require.Nil(t, changes[0].After)
}

func TestResourceChangesUnknownPathOnCreateHasNoValues(t *testing.T) {
	configured := &structs.Network{}
	unknown := []diff.Path{diff.Path{}.Attribute("subnet")}

	changes := resourceChanges(diff.ActionCreate, nil, configured, nil, unknown, false)

	require.Len(t, changes, 1)
	require.Equal(t, "subnet", changes[0].Path.String())
	require.True(t, changes[0].Unknown)
	require.Nil(t, changes[0].Before)
	require.Nil(t, changes[0].After)
}

func TestResourceChangesAddedBlockElementSplitsUnknownOut(t *testing.T) {
	saved := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app"}},
	}}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app"}, {Name: "web"}},
	}}
	unknown := []diff.Path{diff.Path{}.Attribute("network").Index(1).Attribute("ip_address")}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, unknown, false)

	require.Len(t, changes, 2)

	require.Equal(t, "network[1].name", changes[0].Path.String())
	require.Nil(t, changes[0].Before)
	require.Equal(t, "web", changes[0].After)
	require.False(t, changes[0].Unknown)

	require.Equal(t, "network[1].ip_address", changes[1].Path.String())
	require.True(t, changes[1].Unknown)
	require.Nil(t, changes[1].Before)
	require.Nil(t, changes[1].After)
}

func TestResourceChangesCreatedBlockSplitsUnknownOut(t *testing.T) {
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Networks: []structs.NetworkAttachment{{Name: "app", Aliases: []string{"api"}}},
	}}
	unknown := []diff.Path{diff.Path{}.Attribute("network").Index(0).Attribute("ip_address")}

	changes := resourceChanges(diff.ActionCreate, nil, configured, nil, unknown, false)

	require.Len(t, changes, 3)

	require.Equal(t, "network[0].name", changes[0].Path.String())
	require.Equal(t, "app", changes[0].After)

	require.Equal(t, "network[0].ip_address", changes[1].Path.String())
	require.True(t, changes[1].Unknown)
	require.Nil(t, changes[1].After)

	require.Equal(t, "network[0].aliases", changes[2].Path.String())
	require.Equal(t, []any{"api"}, changes[2].After)
}

func TestResourceChangesValuesMarshalAsConfiguration(t *testing.T) {
	saved := &structs.Container{}
	configured := &structs.Container{ContainerBase: structs.ContainerBase{
		Ports: []structs.Port{{Local: 8443, Remote: 443}},
	}}

	changes := resourceChanges(diff.ActionUpdate, saved, configured, nil, nil, false)

	encoded, err := json.Marshal(changes)
	require.NoError(t, err)
	require.JSONEq(t, `[ { "path": "port[0]", "after": { "local": 8443, "remote": 443 } } ]`, string(encoded))
}
