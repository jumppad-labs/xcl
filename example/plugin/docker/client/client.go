// Package client is the Docker Engine client the Docker plugin's providers
// use to create real networks and containers.
//
// The providers never use the Docker SDK client directly. They hold a Docker,
// a narrow interface that mirrors only the SDK methods they call. The real SDK
// client satisfies it unchanged, and the provider unit tests use a mock
// generated from it by Mockery, so they run without a Docker engine.
package client

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// pingTimeout is how long Ping waits for a Docker engine to answer
const pingTimeout = 5 * time.Second

// Docker is the part of the Docker Engine SDK client the providers use. Each
// method has the same signature as the SDK's, so *client.Client satisfies it.
type Docker interface {
	Ping(ctx context.Context) (types.Ping, error)

	NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error)
	NetworkInspect(ctx context.Context, networkID string, options network.InspectOptions) (network.Inspect, error)
	NetworkRemove(ctx context.Context, networkID string) error
	NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error

	ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)

	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
}

// the real SDK client satisfies Docker without an adapter
var _ Docker = (*dockerclient.Client)(nil)

// New returns a Docker Engine SDK client configured from the environment:
// DOCKER_HOST, or the default socket when it is not set. The API version is
// negotiated with the engine, so older engines work too.
func New() (Docker, error) {
	return newSDKClient()
}

func newSDKClient() (*dockerclient.Client, error) {
	c, err := dockerclient.NewClientWithOpts(dockerclient.WithHostFromEnv(), dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("unable to create Docker client: %w", err)
	}

	return c, nil
}

// Ping reports whether a Docker engine is reachable, it returns an error when
// none answers. The application calls it before applying its configuration,
// and the tests that need a real engine call it to decide whether to skip.
func Ping(ctx context.Context) error {
	c, err := newSDKClient()
	if err != nil {
		return fmt.Errorf("no Docker engine reachable: %w", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if _, err := c.Ping(ctx); err != nil {
		return fmt.Errorf("no Docker engine reachable: %w", err)
	}

	return nil
}
