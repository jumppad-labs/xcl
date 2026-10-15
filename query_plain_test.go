package xcl

import (
	"path/filepath"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plain"
	"github.com/jumppad-labs/xcl/registry"
	statemocks "github.com/jumppad-labs/xcl/state/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupPlainConfig applies the plain fixture with its types registered as
// plain structs, none of which embeds types.ResourceBase.
func setupPlainConfig(t *testing.T) *Config {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	ss := &statemocks.MockStateStore{}
	ss.On("Exists").Return(false)
	ss.On("Load").Return(nil, nil)
	ss.On("Save", mock.Anything).Return(nil)

	local := registry.NewLocal()
	local.RegisterType(&plain.Agent{}, "agent")
	local.RegisterType(&plain.Database{}, "database", "postgres")
	local.RegisterType(&plain.Organization{}, "organization")
	local.RegisterType(&plain.Job{}, "job")
	local.RegisterType(&plain.Unlabelled{}, "unlabelled")
	local.RegisterType(&plain.Service{}, "service")

	c, err := NewConfig(WithRegistry(local), WithStateStore(ss))
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/plain/main.xcl")
	require.NoError(t, err)

	require.NoError(t, c.Apply(path))

	return c
}

func TestFindReturnsAPlainType(t *testing.T) {
	c := setupPlainConfig(t)

	agent, err := Find[plain.Agent](c, "agent.claude")
	require.NoError(t, err)
	require.Equal(t, []string{"opus", "sonnet"}, agent.Models)
}

func TestFindFillsAPlainTypesNameLabel(t *testing.T) {
	c := setupPlainConfig(t)

	agent, err := Find[plain.Agent](c, "agent.claude")
	require.NoError(t, err)
	require.Equal(t, "claude", agent.Name)
}

func TestFindFillsAPlainTypesSubtypeAndNameLabels(t *testing.T) {
	c := setupPlainConfig(t)

	database, err := Find[plain.Database](c, "database.postgres.main")
	require.NoError(t, err)
	require.Equal(t, "postgres", database.Engine)
	require.Equal(t, "main", database.Name)
	require.Equal(t, 5432, database.Port)
}

func TestFindKeepsAPointerSetToFalse(t *testing.T) {
	c := setupPlainConfig(t)

	agent, err := Find[plain.Agent](c, "agent.ollama")
	require.NoError(t, err)
	require.NotNil(t, agent.Enabled)
	require.False(t, *agent.Enabled)
}

func TestFindLeavesAnUnsetPointerNil(t *testing.T) {
	c := setupPlainConfig(t)

	agent, err := Find[plain.Agent](c, "agent.claude")
	require.NoError(t, err)
	require.Nil(t, agent.Enabled)
}

func TestFindFillsTheLabelsOfNestedBlocksOfAPlainType(t *testing.T) {
	c := setupPlainConfig(t)

	organization, err := Find[plain.Organization](c, "organization.hidden")
	require.NoError(t, err)
	require.Len(t, organization.Overrides, 1)
	require.Equal(t, "claude", organization.Overrides[0].Name)
	require.Equal(t, []string{"sonnet"}, organization.Overrides[0].Models)
}

func TestFindResolvesAReferenceToAPlainType(t *testing.T) {
	c := setupPlainConfig(t)

	job, err := Find[plain.Job](c, "job.deploy")
	require.NoError(t, err)
	require.Equal(t, "make deploy port=5432", job.Command)
}

func TestFindFillsTheDependsOnOfMeta(t *testing.T) {
	c := setupPlainConfig(t)

	job, err := Find[plain.Job](c, "job.deploy")
	require.NoError(t, err)
	require.Equal(t, []string{"job.build"}, job.Meta.DependsOn)
}

func TestFindFillsTheDisabledFlagOfMeta(t *testing.T) {
	c := setupPlainConfig(t)

	job, err := Find[plain.Job](c, "job.off")
	require.NoError(t, err)
	require.True(t, job.Meta.Disabled)
}

func TestFindFillsAPlainTypesMetaField(t *testing.T) {
	c := setupPlainConfig(t)

	job, err := Find[plain.Job](c, "job.deploy")
	require.NoError(t, err)
	require.Equal(t, "job.deploy", job.Meta.ID)
	require.Equal(t, "job", job.Meta.Type)
	require.Equal(t, "deploy", job.Meta.Name)
	require.Equal(t, "main.xcl", filepath.Base(job.Meta.File))
	require.Equal(t, 25, job.Meta.Line)
}

func TestAReferenceToAPlainTypeFillsItsMeta(t *testing.T) {
	c := setupPlainConfig(t)

	service, err := Find[plain.Service](c, "service.api")
	require.NoError(t, err)
	require.Equal(t, "database.postgres.main", service.Database.Meta.ID)
	require.Equal(t, "main.xcl", filepath.Base(service.Database.Meta.File))
}

func TestAReferenceToAPlainTypeFillsItsLabelsAndAttributes(t *testing.T) {
	c := setupPlainConfig(t)

	service, err := Find[plain.Service](c, "service.api")
	require.NoError(t, err)
	require.Equal(t, "postgres", service.Database.Engine)
	require.Equal(t, "main", service.Database.Name)
	require.Equal(t, 5432, service.Database.Port)
}

func TestAListOfReferencesToAPlainTypeFillsEachMeta(t *testing.T) {
	c := setupPlainConfig(t)

	service, err := Find[plain.Service](c, "service.api")
	require.NoError(t, err)
	require.Len(t, service.Databases, 1)
	require.Equal(t, "database.postgres.main", service.Databases[0].Meta.ID)
}

func TestFindReturnsAPlainTypeWithoutLabelFields(t *testing.T) {
	c := setupPlainConfig(t)

	unlabelled, err := Find[plain.Unlabelled](c, "unlabelled")
	require.NoError(t, err)
	require.Contains(t, unlabelled.Value, "kept")
}

func TestAReferenceReadsAPlainTypesLabels(t *testing.T) {
	c := setupPlainConfig(t)

	unlabelled, err := Find[plain.Unlabelled](c, "unlabelled")
	require.NoError(t, err)
	require.Equal(t, "kept claude postgres", unlabelled.Value)
}

func TestConfigurationCannotSetAPlainTypesLabel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	local := registry.NewLocal()
	local.RegisterType(&plain.Agent{}, "agent")

	c, err := NewConfig(WithRegistry(local))
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/plain_label_set/main.xcl")
	require.NoError(t, err)

	err = c.Apply(path)
	require.ErrorContains(t, err, `"name"`)
}

func TestFindReturnsACopyOfAPlainEntity(t *testing.T) {
	c := setupPlainConfig(t)

	agent, err := Find[plain.Agent](c, "agent.claude")
	require.NoError(t, err)
	agent.Models[0] = "changed"
	agent.Name = "renamed"

	again, err := Find[plain.Agent](c, "agent.claude")
	require.NoError(t, err)
	require.Equal(t, "claude", again.Name)
}

func TestFindRefusesAPlainEntityAsAnotherType(t *testing.T) {
	c := setupPlainConfig(t)

	_, err := Find[plain.Job](c, "agent.claude")

	var mismatch *xclerrors.TypeMismatchError
	require.ErrorAs(t, err, &mismatch)
}

func TestFindByTypeReturnsPlainTypes(t *testing.T) {
	c := setupPlainConfig(t)

	agents, err := FindByType[plain.Agent](c, "agent")
	require.NoError(t, err)
	require.Len(t, agents, 2)
}

func TestFindOneReturnsAPlainType(t *testing.T) {
	c := setupPlainConfig(t)

	organization, err := FindOne[plain.Organization](c, "organization")
	require.NoError(t, err)
	require.Equal(t, "hidden", organization.Name)
}

func TestAllReturnsEveryEntityOfAPlainType(t *testing.T) {
	c := setupPlainConfig(t)

	jobs, err := All[plain.Job](c)
	require.NoError(t, err)
	require.Len(t, jobs, 3)
}

func TestAllRefusesAPlainTypeThatIsOnlyNested(t *testing.T) {
	c := setupPlainConfig(t)

	_, err := All[plain.Override](c)

	require.ErrorIs(t, err, xclerrors.ErrNotAnEntity)

	var notRegistered *xclerrors.NotRegisteredError
	require.ErrorAs(t, err, &notRegistered)
	require.True(t, notRegistered.MayBeNested)
}

func TestDecodeFillsPlainTypes(t *testing.T) {
	c := setupPlainConfig(t)

	var decoded struct {
		Agents       []*plain.Agent
		Database     *plain.Database
		Organization *plain.Organization
	}
	require.NoError(t, Decode(c, &decoded))

	require.Len(t, decoded.Agents, 2)
	require.Equal(t, "main", decoded.Database.Name)
	require.Equal(t, "hidden", decoded.Organization.Name)
}

func TestAsTurnsAStoredPlainEntityIntoItsType(t *testing.T) {
	c := setupPlainConfig(t)

	stored, err := c.FindOneEntity("database", "postgres")
	require.NoError(t, err)

	database, err := As[plain.Database](stored)
	require.NoError(t, err)
	require.Equal(t, "main", database.Name)
}
