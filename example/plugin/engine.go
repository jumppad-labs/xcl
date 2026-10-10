package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

// pingDocker reports whether a Docker engine is reachable, it returns an
// error when none answers. apply, plan and destroy call it before they load
// any plugin, and the tests that need a real engine call it to decide whether
// to skip.
//
// It sends GET /_ping, the request the Docker SDK's own Ping sends, to the
// engine DOCKER_HOST names, the default socket when it is not set. It uses
// the standard library rather than the Docker SDK, so the application reads
// state through the Docker plugin's entity types alone and its build includes
// no Docker library. It knows only unix:// and tcp:// addresses; for any
// other it returns nil, and the plugin reports an unreachable engine itself.
func pingDocker(ctx context.Context) error {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = defaultDockerHost
	}

	address, err := url.Parse(host)
	if err != nil {
		return fmt.Errorf("no Docker engine reachable: %w", err)
	}

	transport := &http.Transport{}
	pingURL := ""

	switch address.Scheme {
	case "unix":
		socket := address.Path
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socket)
		}
		pingURL = "http://docker/_ping"

	case "tcp":
		pingURL = "http://" + address.Host + "/_ping"

	default:
		return nil
	}

	defer transport.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
	if err != nil {
		return fmt.Errorf("no Docker engine reachable: %w", err)
	}

	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		return fmt.Errorf("no Docker engine reachable: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("no Docker engine reachable: %s answered %s", host, response.Status)
	}

	return nil
}

// defaultDockerHost is the engine pingDocker asks when DOCKER_HOST is not set
const defaultDockerHost = "unix:///var/run/docker.sock"

// pingTimeout is how long pingDocker waits for a Docker engine to answer
const pingTimeout = 5 * time.Second
