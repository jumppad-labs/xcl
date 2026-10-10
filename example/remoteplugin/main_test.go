package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// runCommand runs the program with args and returns its exit code and what it
// wrote to stdout and stderr
func runCommand(args ...string) (int, string, string) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	code := run(args, stdout, stderr)

	return code, stdout.String(), stderr.String()
}

func TestRunWithoutACommandPrintsUsage(t *testing.T) {
	code, _, stderr := runCommand()

	require.Equal(t, 2, code)
	require.Contains(t, stderr, "usage: xcl-docker <command>")
}

func TestRunWithAnUnknownCommandFails(t *testing.T) {
	code, _, stderr := runCommand("launch")

	require.Equal(t, 2, code)
	require.Contains(t, stderr, `unknown command "launch"`)
}

func TestApplyWithoutAPathFails(t *testing.T) {
	code, _, stderr := runCommand("apply")

	require.Equal(t, 2, code)
	require.Contains(t, stderr, "apply needs the path of the configuration to apply")
}

func TestApplyWithAnUnknownFlagFails(t *testing.T) {
	code, _, _ := runCommand("apply", "--plugin", "./build/docker-plugin", "./config")

	require.Equal(t, 2, code)
}

const testKey = "-----BEGIN PGP PUBLIC KEY BLOCK-----\ntest\n-----END PGP PUBLIC KEY BLOCK-----\n"

func TestFetchKeyReturnsThePublishedKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testKey))
	}))
	defer server.Close()

	key, err := fetchKey(server.URL)

	require.NoError(t, err)
	require.Equal(t, testKey, key)
}

func TestFetchKeyFailsWhenTheKeyIsMissing(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	_, err := fetchKey(server.URL)

	require.ErrorContains(t, err, "404 Not Found")
}

func TestFetchKeyFailsWhenTheServerIsUnreachable(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	_, err := fetchKey(url)

	require.ErrorContains(t, err, "fetching release key "+url)
}

func TestFetchKeyReadsNoMoreThanOneMebibyte(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("k"), 2<<20))
	}))
	defer server.Close()

	key, err := fetchKey(server.URL)

	require.NoError(t, err)
	require.Len(t, key, 1<<20)
}
