package e2e_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/e2e/fixtures/kube"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/types"
)

// kubeDeclaredIDs is every resource the kube configuration declares, sorted
var kubeDeclaredIDs = []string{
	"config_map.api",
	"deployment.api",
	"ingress.api",
	"output.api_url",
	"secret.db",
	"service.api",
	"variable.image_tag",
	"variable.replicas",
}

// kubeAppConfig is an application's view of the kube configuration, filled
// by one Decode call: a slice field receives every block of its registered
// type in the order written, a pointer field the one block of its type
type kubeAppConfig struct {
	ConfigMaps  []*kube.ConfigMap
	Deployments []*kube.Deployment
	Service     *kube.Service
	Ingress     *kube.Ingress
}

// applyKube applies the kube configuration with events going to handler, a
// nil handler leaves xcl silent, and returns the applied Config
func applyKube(t *testing.T, handler xcl.EventHandler) *xcl.Config {
	t.Helper()

	t.Setenv("DB_PASSWORD", testPassword)

	c := newKubeConfig(t, registry.NewPluginRegistry(), handler, t.TempDir(), testStateKey)
	require.NoError(t, c.Apply(kubeConfigDir))

	return c
}

// decodeKube applies the kube configuration and decodes it into a
// kubeAppConfig
func decodeKube(t *testing.T) kubeAppConfig {
	t.Helper()

	c := applyKube(t, nil)

	var cfg kubeAppConfig
	require.NoError(t, c.Decode(&cfg))

	return cfg
}

// decodedKubeDeployment returns the one deployment the kube configuration
// declares, as Decode returns it
func decodedKubeDeployment(t *testing.T) *kube.Deployment {
	t.Helper()

	cfg := decodeKube(t)
	require.Len(t, cfg.Deployments, 1)

	return cfg.Deployments[0]
}

// kubeEntity returns the entity in entities with the given id, failing the
// test when there is none
func kubeEntity(t *testing.T, entities []any, id string) any {
	t.Helper()

	entity, err := testutil.EntityByID(entities, id)
	require.NoError(t, err)

	return entity
}

// kubeEntityIDs returns the ids of entities, sorted
func kubeEntityIDs(t *testing.T, entities []any) []string {
	t.Helper()

	ids := []string{}
	for _, entity := range entities {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	sort.Strings(ids)
	return ids
}

// kubeLifecycleEvents applies, decodes and destroys the kube configuration,
// as an application using it would, and returns every event reported
func kubeLifecycleEvents(t *testing.T) []xcl.Event {
	t.Helper()

	recorder := &testutil.EventRecorder{}
	c := applyKube(t, recorder.Record)

	var cfg kubeAppConfig
	require.NoError(t, c.Decode(&cfg))
	require.NoError(t, c.Destroy())

	return recorder.Events()
}

// kubeEventsWithOperation returns the events in recorded for operation, in
// the order they were reported
func kubeEventsWithOperation(recorded []xcl.Event, operation string) []xcl.Event {
	found := []xcl.Event{}
	for _, e := range recorded {
		if e.Operation == operation {
			found = append(found, e)
		}
	}

	return found
}
