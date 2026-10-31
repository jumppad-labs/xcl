package plugins

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/testutil"
)

// environmentWithout is the test's environment without the variable name
func environmentWithout(name string) []string {
	environment := []string{}
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, name+"=") {
			continue
		}

		environment = append(environment, entry)
	}

	return environment
}

func TestHandshakeCookieIsXCLPlugin(t *testing.T) {
	require.Equal(t, "XCL_PLUGIN", HandshakeConfig.MagicCookieKey)
	require.Equal(t, "xcl", HandshakeConfig.MagicCookieValue)
	require.Equal(t, uint(1), HandshakeConfig.ProtocolVersion)
}

func TestPluginServesWhenXCLPluginIsSet(t *testing.T) {
	binary := testutil.BuildFixturePlugin(t, "..")

	command := exec.Command(binary)
	command.Env = append(environmentWithout("XCL_PLUGIN"), "XCL_PLUGIN=xcl")

	stdout, err := command.StdoutPipe()
	require.NoError(t, err)

	require.NoError(t, command.Start())
	t.Cleanup(func() {
		command.Process.Kill()
		command.Wait()
	})

	lines := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		lines <- line
	}()

	select {
	case line := <-lines:
		// go-plugin's handshake line: core protocol, app protocol, network
		// (unix or tcp), address, protocol and an optional server certificate
		fields := strings.Split(strings.TrimSpace(line), "|")
		require.GreaterOrEqual(t, len(fields), 5, "handshake line %q", line)
		require.Equal(t, "1", fields[0], "handshake line %q", line)
		require.Equal(t, "1", fields[1], "handshake line %q", line)
		require.Equal(t, "grpc", fields[4], "handshake line %q", line)
	case <-time.After(30 * time.Second):
		require.FailNow(t, "the plugin printed no handshake line")
	}
}

func TestPluginRefusesToServeWithoutXCLPlugin(t *testing.T) {
	binary := testutil.BuildFixturePlugin(t, "..")

	command := exec.Command(binary)
	command.Env = environmentWithout("XCL_PLUGIN")

	output, err := command.CombinedOutput()

	require.Error(t, err)
	require.Contains(t, string(output), "This binary is a plugin")
}

func TestPluginRefusesToServeWithWrongXCLPluginValue(t *testing.T) {
	binary := testutil.BuildFixturePlugin(t, "..")

	command := exec.Command(binary)
	command.Env = append(environmentWithout("XCL_PLUGIN"), "XCL_PLUGIN=other")

	output, err := command.CombinedOutput()

	require.Error(t, err)
	require.Contains(t, string(output), "This binary is a plugin")
}
