package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/e2e/fixtures/kube"
)

// TestApplyFindsEveryDeclaredResource asserts the entities after an apply are
// exactly the resources the configuration declares
func TestApplyFindsEveryDeclaredResource(t *testing.T) {
	c := applyKube(t, nil)

	require.Equal(t, kubeDeclaredIDs, kubeEntityIDs(t, c.Entities()))
}

// TestApplyReturnsRegisteredGoTypes asserts each block is decoded into the Go
// type registered for it, held as itself rather than a type generated from a
// schema
func TestApplyReturnsRegisteredGoTypes(t *testing.T) {
	c := applyKube(t, nil)
	entities := c.Entities()

	_, ok := kubeEntity(t, entities, "config_map.api").(*kube.ConfigMap)
	require.True(t, ok)

	_, ok = kubeEntity(t, entities, "deployment.api").(*kube.Deployment)
	require.True(t, ok)

	_, ok = kubeEntity(t, entities, "service.api").(*kube.Service)
	require.True(t, ok)

	_, ok = kubeEntity(t, entities, "ingress.api").(*kube.Ingress)
	require.True(t, ok)
}

// TestDecodeFillsRepeatedBlocksInOrder asserts a block that appears more than
// once is decoded into a slice, in the order it was written
func TestDecodeFillsRepeatedBlocksInOrder(t *testing.T) {
	d := decodedKubeDeployment(t)

	require.Len(t, d.Containers, 2)
	require.Equal(t, "api", d.Containers[0].Name)
	require.Equal(t, "proxy", d.Containers[1].Name)

	require.Len(t, d.Containers[0].Ports, 2)
	require.Equal(t, "http", d.Containers[0].Ports[0].Name)
	require.Equal(t, 8080, d.Containers[0].Ports[0].ContainerPort)
	require.Equal(t, "metrics", d.Containers[0].Ports[1].Name)
	require.Equal(t, 9090, d.Containers[0].Ports[1].ContainerPort)

	require.Len(t, d.Containers[1].Ports, 1)
	require.Equal(t, 8081, d.Containers[1].Ports[0].ContainerPort)
}

// TestDecodeFillsNestedBlocks asserts blocks nested inside a nested block are
// decoded, resources holds limits and requests
func TestDecodeFillsNestedBlocks(t *testing.T) {
	d := decodedKubeDeployment(t)

	api := d.Containers[0]
	require.NotNil(t, api.Resources)
	require.Equal(t, "500m", api.Resources.Limits.CPU)
	require.Equal(t, "512Mi", api.Resources.Limits.Memory)
	require.Equal(t, "100m", api.Resources.Requests.CPU)
	require.Equal(t, "128Mi", api.Resources.Requests.Memory)

	require.Len(t, api.VolumeMounts, 1)
	require.Equal(t, "config", api.VolumeMounts[0].Name)
	require.Equal(t, "/etc/api", api.VolumeMounts[0].Path)

	require.Len(t, d.Volumes, 1)
	require.Equal(t, "config", d.Volumes[0].Name)
}

// TestDecodeLeavesOmittedBlockNil asserts a block the configuration leaves
// out is nil or empty, the proxy container sets no resources, env or mounts
func TestDecodeLeavesOmittedBlockNil(t *testing.T) {
	d := decodedKubeDeployment(t)

	require.Nil(t, d.Containers[1].Resources)
	require.Empty(t, d.Containers[1].Env)
	require.Empty(t, d.Containers[1].VolumeMounts)
}

// TestDecodeResolvesValuesReadFromAnotherResourcesMap asserts values read out
// of a map attribute of another resource are resolved
func TestDecodeResolvesValuesReadFromAnotherResourcesMap(t *testing.T) {
	d := decodedKubeDeployment(t)

	env := d.Containers[0].Env
	require.Len(t, env, 3)
	require.Equal(t, "DB_HOST", env[0].Name)
	require.Equal(t, "postgres.default.svc", env[0].Value)
	require.Equal(t, "LOG_LEVEL", env[1].Name)
	require.Equal(t, "info", env[1].Value)

	require.Equal(t, "config_map.api", d.Volumes[0].ConfigMap)
}

// TestDecodeResolvesVariables asserts a variable is read both as a value of
// its own and inside an interpolated string
func TestDecodeResolvesVariables(t *testing.T) {
	d := decodedKubeDeployment(t)

	require.Equal(t, 3, d.Replicas)
	require.Equal(t, "ghcr.io/example/api:1.2.0", d.Containers[0].Image)
}

// TestDecodeResolvesReferenceIntoRepeatedBlockByPosition asserts the service
// names the deployment by id and reads its target port out of it, the port
// coming from a repeated block referenced by position
func TestDecodeResolvesReferenceIntoRepeatedBlockByPosition(t *testing.T) {
	service := decodeKube(t).Service
	require.NotNil(t, service)

	require.Equal(t, "deployment.api", service.Deployment)
	require.Equal(t, 80, service.Port)
	require.Equal(t, 8080, service.TargetPort)
}

// TestDecodeResolvesReferenceToAnotherResourcesAttributes asserts the ingress
// rule names the service it routes to by id, and reads its port
func TestDecodeResolvesReferenceToAnotherResourcesAttributes(t *testing.T) {
	ingress := decodeKube(t).Ingress
	require.NotNil(t, ingress)

	require.Equal(t, "api.example.com", ingress.Host)
	require.Len(t, ingress.Rules, 1)
	require.Equal(t, "/", ingress.Rules[0].Path)
	require.Equal(t, "service.api", ingress.Rules[0].Service)
	require.Equal(t, 80, ingress.Rules[0].Port)
}
