package registry

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/events"
)

// eventRecorder records every event emitted to it. Plugins log from their own
// goroutines, so recording is guarded by a mutex.
type eventRecorder struct {
	mutex  sync.Mutex
	events []events.Event
}

func (r *eventRecorder) emit(e events.Event) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.events = append(r.events, e)
}

// recorded returns a copy of the events recorded so far
func (r *eventRecorder) recorded() []events.Event {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return append([]events.Event{}, r.events...)
}

// lifecycleEvents returns the events in recorded for operation that are not
// log messages, the start, success and error events of the operation
func lifecycleEvents(recorded []events.Event, operation string) []events.Event {
	found := []events.Event{}
	for _, e := range recorded {
		if e.Operation == operation && e.Phase != events.PhaseLog {
			found = append(found, e)
		}
	}

	return found
}

// eventsWithPhase returns the events in recorded with the given phase
func eventsWithPhase(recorded []events.Event, phase string) []events.Event {
	found := []events.Event{}
	for _, e := range recorded {
		if e.Phase == phase {
			found = append(found, e)
		}
	}

	return found
}

// logsWithMessage returns the log events in recorded with the given message
func logsWithMessage(recorded []events.Event, message string) []events.Event {
	found := []events.Event{}
	for _, e := range recorded {
		if e.Phase == events.PhaseLog && e.Meta[events.KeyMessage] == message {
			found = append(found, e)
		}
	}

	return found
}

// stopHosts stops every plugin host r has loaded
func stopHosts(r *PluginRegistry) {
	for _, host := range r.GetPluginHosts() {
		host.Stop()
	}
}

// requireLinuxProc skips the test when processes cannot be inspected through
// /proc
func requireLinuxProc(t *testing.T) {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skip("inspecting running processes needs Linux /proc")
	}
}

// processesRunning returns how many running processes were started from the
// binary at path, read from the /proc/<pid>/exe link of every process
func processesRunning(t *testing.T, path string) int {
	t.Helper()

	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Failed to resolve %s: %v", path, err)
	}

	links, err := filepath.Glob("/proc/[0-9]*/exe")
	if err != nil {
		t.Fatalf("Failed to list processes: %v", err)
	}

	count := 0
	for _, link := range links {
		target, err := os.Readlink(link)
		if err != nil {
			// the process exited, or belongs to another user
			continue
		}

		if target == absolute {
			count++
		}
	}

	return count
}
