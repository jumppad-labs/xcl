package xcl

import (
	"runtime/debug"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jumppad-labs/xcl/events"
	"github.com/stretchr/testify/require"
)

// fourIndependentNetworksConfig declares four networks, none depends on
// another, and a container that depends on network.one
const fourIndependentNetworksConfig = `
resource "network" "one" {
  subnet = "10.0.1.0/24"
}

resource "network" "two" {
  subnet = "10.0.2.0/24"
}

resource "network" "three" {
  subnet = "10.0.3.0/24"
}

resource "network" "four" {
  subnet = "10.0.4.0/24"
}

resource "container" "attached" {
  network {
    name = resource.network.one.meta.name
  }
}
`

// receiverPanic is the distinctive value a test receiver panics with
type receiverPanic struct {
	reason string
}

// thePanic is the value panickingReceiver panics with
var thePanic = &receiverPanic{reason: "the receiver gave up"}

// panickingReceiver panics with thePanic on the first event it is given
func panickingReceiver(e Event) {
	panic(thePanic)
}

// applyRecoveringPanic calls Apply and returns the value it panicked with and
// the stack at the point it was recovered, or nil and nil when it returned
func applyRecoveringPanic(f *deliveryFixture) (recovered any, stack []byte) {
	defer func() {
		recovered = recover()
		if recovered != nil {
			stack = debug.Stack()
		}
	}()

	f.config.Apply(f.configFile)

	return nil, nil
}

func TestReceiverPanicIsRaisedFromApplyWithSameValue(t *testing.T) {
	value := &receiverPanic{reason: "create started"}

	receiver := func(e Event) {
		if e.Operation == events.OperationCreate && e.Phase == events.PhaseStart {
			panic(value)
		}
	}

	f := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(receiver))

	recovered, _ := applyRecoveringPanic(f)

	require.NotNil(t, recovered)
	require.Same(t, value, recovered)
}

func TestReceiverPanicStackContainsReceiver(t *testing.T) {
	f := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(panickingReceiver))

	recovered, stack := applyRecoveringPanic(f)

	require.Same(t, thePanic, recovered)
	require.Contains(t, string(stack), "panickingReceiver")
}

func TestReceiverPanicRecordsOnlyCreatedResourcesInState(t *testing.T) {
	value := &receiverPanic{reason: "third create started"}

	createStarts := 0
	receiver := func(e Event) {
		if e.Operation == events.OperationCreate && e.Phase == events.PhaseStart {
			createStarts++
			if createStarts == 3 {
				panic(value)
			}
		}
	}

	f := setupDeliveryConfig(t, fourIndependentNetworksConfig, WithEventHandler(receiver))

	// hold every create long enough for the creates to overlap and still be
	// in progress when the receiver panics
	f.plugin.SetCreateHook(func(id string) {
		time.Sleep(200 * time.Millisecond)
	})

	recovered, _ := applyRecoveringPanic(f)
	require.Same(t, value, recovered)

	completed := []string{}
	for _, c := range f.plugin.GetCallTimes() {
		require.NoError(t, c.Err)
		completed = append(completed, c.Call[len("create "):])
	}
	sort.Strings(completed)

	// at least the three creates the receiver saw start were completed, and
	// the state holds exactly the completed creates
	require.GreaterOrEqual(t, len(completed), 3)
	require.Equal(t, completed, savedIDs(t, f.store))
}

func TestReceiverPanicStartsNoProviderCallAfterPanic(t *testing.T) {
	value := &receiverPanic{reason: "first create started"}

	firstStarted := make(chan struct{})
	var closeFirstStarted sync.Once

	var panicked time.Time
	receiver := func(e Event) {
		if e.ResourceID == "resource.network.first" && e.Operation == events.OperationCreate && e.Phase == events.PhaseStart {
			<-firstStarted
			panicked = time.Now()
			panic(value)
		}
	}

	f := setupDeliveryConfig(t, dependentChainConfig, WithEventHandler(receiver))

	// network.first signals that its create has started, then holds the
	// create long enough for the panic to stop the apply before it returns
	f.plugin.SetCreateHook(func(id string) {
		if id != "resource.network.first" {
			return
		}

		closeFirstStarted.Do(func() { close(firstStarted) })
		time.Sleep(300 * time.Millisecond)
	})

	recovered, _ := applyRecoveringPanic(f)
	require.Same(t, value, recovered)

	calls := f.plugin.GetCallTimes()
	require.Len(t, calls, 1)
	require.Equal(t, "create resource.network.first", calls[0].Call)

	for _, c := range calls {
		require.False(t, c.Started.After(panicked), "%s started after the receiver panicked", c.Call)
	}

	require.Equal(t, []string{"create resource.network.first"}, f.plugin.GetCalls())
}
