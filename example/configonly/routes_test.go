package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/plugins/registry"
)

// loadTestConfig loads one of the test configurations under testdata through
// the program's own loader. It passes no options, so the test keeps no state
// and receives no events.
func loadTestConfig(t *testing.T, name string) *appConfig {
	t.Helper()

	cfg, err := loadConfig(filepath.Join("testdata", name), registry.NewPluginRegistry())
	require.NoError(t, err)

	return cfg
}

func TestIngressRoutesReportsEveryIngressPath(t *testing.T) {
	cfg := loadTestConfig(t, "routes")

	routes, err := ingressRoutes(cfg)
	require.NoError(t, err)

	require.Equal(t, []Route{
		{
			Host:        "shop.test",
			Path:        "/",
			Service:     "service.web",
			ServicePort: 80,
			Deployment:  "deployment.web",
			Container:   "web",
			PortName:    "http",
			Port:        3000,
		},
		{
			Host:        "shop.test",
			Path:        "/api",
			Service:     "service.api",
			ServicePort: 9090,
			Deployment:  "deployment.api",
			Container:   "api",
			PortName:    "grpc",
			Port:        7000,
		},
	}, routes)
}

func TestIngressRoutesFailsForUnknownService(t *testing.T) {
	cfg := loadTestConfig(t, "unknown_service")

	_, err := ingressRoutes(cfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "ingress ingress.shop path /")
	require.ErrorContains(t, err, `service "service.missing" not found`)
}

func TestIngressRoutesFailsForUnknownDeployment(t *testing.T) {
	cfg := loadTestConfig(t, "unknown_deployment")

	_, err := ingressRoutes(cfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "ingress ingress.shop path /")
	require.ErrorContains(t, err, `deployment "deployment.missing" of service service.web not found`)
}

func TestIngressRoutesFailsForUnreachableTargetPort(t *testing.T) {
	cfg := loadTestConfig(t, "unknown_target_port")

	_, err := ingressRoutes(cfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "ingress ingress.shop path /")
	require.ErrorContains(t, err, "no container in deployment.web exposes port 1234")
}

func TestRouteStringNamesEveryHop(t *testing.T) {
	route := Route{
		Host:        "api.example.com",
		Path:        "/",
		Service:     "service.api",
		ServicePort: 80,
		Deployment:  "deployment.api",
		Container:   "api",
		PortName:    "http",
		Port:        8080,
	}

	require.Equal(t, "api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)", route.String())
}
