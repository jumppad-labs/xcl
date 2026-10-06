package xcl

import (
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/internal/xcl/hclwrite"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// encodeReferencesSecretValue is the value of the variable the sensitive
// fields refer to. It must never appear in text written without
// RevealSensitive.
const encodeReferencesSecretValue = "references-secret-value"

// encodeReferencesConfig declares the reference shapes the encode fixture
// does not: a reference inside a template, a reference on a field that holds
// computed neighbours, a reference that resolves disabled to false, and a
// sensitive field written as a bare reference and as a template
const encodeReferencesConfig = `
variable "region" {
  default = "us-east"
}

variable "subnet" {
  default = "10.0.0.0/16"
}

variable "off" {
  default = false
}

variable "db_password" {
  default = "` + encodeReferencesSecretValue + `"
}

resource "database" "templated" {
  location = "${variable.region}-a"
  port     = 5432
}

resource "network" "referenced" {
  subnet   = variable.subnet
  disabled = variable.off
}

resource "secret" "bare" {
  username = "admin"
  password = variable.db_password
}

resource "secret" "templated" {
  username = "admin"
  password = "${variable.db_password}-suffix"
}
`

const (
	encodeReferencesTemplatedDatabaseID = "resource.database.templated"
	encodeReferencesNetworkID           = "resource.network.referenced"
	encodeReferencesBareSecretID        = "resource.secret.bare"
	encodeReferencesTemplatedSecretID   = "resource.secret.templated"
)

func TestEncodeEntityShowsReferencesWhenAsked(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	out, err := EncodeEntity(database, ShowReferences())
	require.NoError(t, err)

	expected := `resource "database" "main" {
  location = variable.region
  port     = 5432

  timeouts {
    connect = 30
    read    = 60
  }
}
`

	require.Equal(t, expected, string(out))
}

func TestEncodeEntityShowsTemplateAsWritten(t *testing.T) {
	c, _, _ := applyEncodeSensitiveConfig(t, encodeReferencesConfig)

	database := encodeEntityByID(t, c, encodeReferencesTemplatedDatabaseID)

	out, err := EncodeEntity(database, ShowReferences())
	require.NoError(t, err)

	expected := `resource "database" "templated" {
  location = "${variable.region}-a"
  port     = 5432
}
`

	require.Equal(t, expected, string(out))
	require.NotContains(t, string(out), "us-east-a")
}

func TestEncodeEntityShowsNestedBlockReferencesAsWritten(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container, ShowReferences())
	require.NoError(t, err)

	text := string(out)

	expectedFirst := `  network {
    name       = resource.network.main.meta.name
    ip_address = "10.0.0.10"
  }`
	expectedSecond := `  network {
    name       = resource.network.main.meta.name
    ip_address = "10.0.0.11"
  }`

	require.Contains(t, text, expectedFirst)
	require.Contains(t, text, expectedSecond)
	require.Equal(t, 2, strings.Count(text, "network {"), "expected two network blocks, got:\n%s", text)
	require.NotContains(t, text, `name       = "main"`)
}

func TestEncodeEntityShowsResolvedValuesByDefault(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	database := encodeEntityByID(t, c, encodeDatabaseID)

	out, err := EncodeEntity(database)
	require.NoError(t, err)

	expected := `resource "database" "main" {
  location = "us-east"
  port     = 5432

  timeouts {
    connect = 30
    read    = 60
  }
}
`

	require.Equal(t, expected, string(out))
	require.NotContains(t, string(out), "variable.region")
}

func TestEncodeEntityShowsResolvedNestedBlockValuesByDefault(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container)
	require.NoError(t, err)

	text := string(out)

	require.Equal(t, 2, strings.Count(text, `name       = "main"`), "expected both network names resolved, got:\n%s", text)
	require.NotContains(t, text, "resource.network.main.meta.name")
}

func TestEncodeSavedEntityMatchesEncodeEntityWithReferences(t *testing.T) {
	c, reg, statePath := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	fromEntity, err := EncodeEntity(container, ShowReferences())
	require.NoError(t, err)

	record := encodeSavedRecordByID(t, statePath, encodeContainerID)

	fromSaved, err := EncodeSavedEntity(reg, record, ShowReferences())
	require.NoError(t, err)

	require.Equal(t, fromEntity, fromSaved)
	require.Contains(t, string(fromSaved), "resource.network.main.meta.name")
}

func TestEncodeSavedEntityMatchesEncodeEntityWithoutReferences(t *testing.T) {
	c, reg, statePath := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	fromEntity, err := EncodeEntity(container)
	require.NoError(t, err)

	record := encodeSavedRecordByID(t, statePath, encodeContainerID)

	fromSaved, err := EncodeSavedEntity(reg, record)
	require.NoError(t, err)

	require.Equal(t, fromEntity, fromSaved)
	require.NotContains(t, string(fromSaved), "resource.network.main.meta.name")
}

func TestEncodeEntityShowsReferencesWithComputed(t *testing.T) {
	c, _, _ := applyEncodeSensitiveConfig(t, encodeReferencesConfig)

	network := encodeEntityByID(t, c, encodeReferencesNetworkID)

	out, err := EncodeEntity(network, ShowReferences(), IncludeComputed())
	require.NoError(t, err)

	expected := `resource "network" "referenced" {
  subnet      = variable.subnet
  provider_id = "id-referenced" # set by the provider
}
`

	require.Equal(t, expected, string(out))
}

func TestEncodeEntityWithReferencesIsDeterministic(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	first, err := EncodeEntity(container, ShowReferences())
	require.NoError(t, err)

	for range 10 {
		again, err := EncodeEntity(container, ShowReferences())
		require.NoError(t, err)

		require.Equal(t, first, again)
	}
}

func TestEncodeEntityWithReferencesIsFormatterStable(t *testing.T) {
	c, _, _ := applyEncodeFixture(t)

	container := encodeEntityByID(t, c, encodeContainerID)

	out, err := EncodeEntity(container, ShowReferences())
	require.NoError(t, err)

	require.Equal(t, out, hclwrite.Format(out))
}

func TestEncodeEntityWithTemplateReferenceIsFormatterStable(t *testing.T) {
	c, _, _ := applyEncodeSensitiveConfig(t, encodeReferencesConfig)

	database := encodeEntityByID(t, c, encodeReferencesTemplatedDatabaseID)

	out, err := EncodeEntity(database, ShowReferences())
	require.NoError(t, err)

	require.Equal(t, out, hclwrite.Format(out))
}

func TestEncodeEntityKeepsTrimmedDisabledWithReferences(t *testing.T) {
	c, _, _ := applyEncodeSensitiveConfig(t, encodeReferencesConfig)

	network := encodeEntityByID(t, c, encodeReferencesNetworkID)

	// the reference must have been recorded, or this test would pass without
	// showReferences ever seeing the trimmed field
	meta, err := types.GetMeta(network)
	require.NoError(t, err)
	require.Equal(t, "variable.off", meta.References["disabled"])

	out, err := EncodeEntity(network, ShowReferences())
	require.NoError(t, err)

	expected := `resource "network" "referenced" {
  subnet = variable.subnet
}
`

	require.Equal(t, expected, string(out))
	require.NotContains(t, string(out), "disabled")
}

func TestEncodeEntityShowsBareReferenceForSensitiveField(t *testing.T) {
	c, _, _ := applyEncodeSensitiveConfig(t, encodeReferencesConfig)

	secret := encodeEntityByID(t, c, encodeReferencesBareSecretID)

	out, err := EncodeEntity(secret, ShowReferences())
	require.NoError(t, err)

	expected := `resource "secret" "bare" {
  username = "admin"
  password = variable.db_password
}
`

	require.Equal(t, expected, string(out))
	require.NotContains(t, string(out), encodeReferencesSecretValue)
	require.NotContains(t, string(out), types.SensitiveMarker)
}

func TestEncodeEntityKeepsMarkerForSensitiveTemplate(t *testing.T) {
	c, _, _ := applyEncodeSensitiveConfig(t, encodeReferencesConfig)

	secret := encodeEntityByID(t, c, encodeReferencesTemplatedSecretID)

	out, err := EncodeEntity(secret, ShowReferences())
	require.NoError(t, err)

	expected := `resource "secret" "templated" {
  username = "admin"
  password = "(sensitive)"
}
`

	require.Equal(t, expected, string(out))
	require.NotContains(t, string(out), encodeReferencesSecretValue)
	require.NotContains(t, string(out), "variable.db_password")
}

func TestEncodeSavedEntityKeepsMarkerForSensitiveTemplate(t *testing.T) {
	_, reg, statePath := applyEncodeSensitiveConfig(t, encodeReferencesConfig)

	record := encodeSavedRecordByID(t, statePath, encodeReferencesTemplatedSecretID)

	out, err := EncodeSavedEntity(reg, record, ShowReferences())
	require.NoError(t, err)

	require.Regexp(t, `password\s+= "\(sensitive\)"`, string(out))
	require.NotContains(t, string(out), encodeReferencesSecretValue)
	require.NotContains(t, string(out), "variable.db_password")
}

func TestEncodeEntityShowsSensitiveTemplateWhenRevealed(t *testing.T) {
	c, _, _ := applyEncodeSensitiveConfig(t, encodeReferencesConfig)

	secret := encodeEntityByID(t, c, encodeReferencesTemplatedSecretID)

	out, err := EncodeEntity(secret, ShowReferences(), RevealSensitive())
	require.NoError(t, err)

	expected := `resource "secret" "templated" {
  username = "admin"
  password = "${variable.db_password}-suffix"
}
`

	require.Equal(t, expected, string(out))
	require.NotContains(t, string(out), types.SensitiveMarker)
}
