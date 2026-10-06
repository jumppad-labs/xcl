package eventstream

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/stretchr/testify/require"
)

// generousTimeout bounds every wait in these tests, it is far longer than
// any of the operations should take so a slow machine never fails a test
const generousTimeout = 5 * time.Second

// recorder collects every event delivered to its Record method, it is safe
// to read while Drain is still running. It adds this package's queries to the
// shared recorder
type recorder struct {
	testutil.EventRecorder
}

// regular returns the delivered events that are not blocked announcements
func (r *recorder) regular() []events.Event {
	var regular []events.Event
	for _, e := range r.Events() {
		if e.Phase != events.PhaseBlocked {
			regular = append(regular, e)
		}
	}

	return regular
}

// blocked returns the delivered blocked announcements
func (r *recorder) blocked() []events.Event {
	var blocked []events.Event
	for _, e := range r.Events() {
		if e.Phase == events.PhaseBlocked {
			blocked = append(blocked, e)
		}
	}

	return blocked
}

// waitForWaitingEmitters polls until count emitters are waiting for room in s
func waitForWaitingEmitters(t *testing.T, s *Stream, count int) {
	t.Helper()

	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()

		return s.waiting == count
	}, generousTimeout, time.Millisecond)
}

// startDrain runs Drain on its own goroutine and returns a channel that is
// closed when Drain returns
func startDrain(s *Stream, receiver events.Handler, done <-chan struct{}) <-chan struct{} {
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		s.Drain(receiver, done)
	}()

	return returned
}

// waitForClose fails the test when ch is not closed within generousTimeout
func waitForClose(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(generousTimeout):
		require.FailNow(t, "timed out waiting for "+what)
	}
}

func TestNewTreatsASizeBelowOneAsOne(t *testing.T) {
	s := New(0)

	require.Equal(t, 1, cap(s.queue))
}

func TestNewUsesTheGivenSize(t *testing.T) {
	s := New(7)

	require.Equal(t, 7, cap(s.queue))
}

func TestEmitReturnsImmediatelyWithRoomAndASlowReceiver(t *testing.T) {
	s := New(10)
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	receiver := func(e events.Event) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
	}
	done := make(chan struct{})
	returned := startDrain(s, receiver, done)

	s.Emit(events.Event{ResourceID: "first"})
	waitForClose(t, waitForSignal(entered), "the receiver to be entered")

	// the receiver is now held on release, so every further emit must fit
	// in the queue without the receiver doing anything
	emitted := make(chan struct{})
	go func() {
		defer close(emitted)
		for i := 0; i < 10; i++ {
			s.Emit(events.Event{ResourceID: fmt.Sprintf("event-%d", i)})
		}
	}()

	waitForClose(t, emitted, "emits into a queue with room to return")

	close(release)
	close(done)
	waitForClose(t, returned, "Drain to return")
}

// waitForSignal turns a single receive on signal into a closed channel
func waitForSignal(signal <-chan struct{}) <-chan struct{} {
	closed := make(chan struct{})
	go func() {
		<-signal
		close(closed)
	}()

	return closed
}

func TestEmitWithALimitOfOneAndASlowReceiverDeliversEveryEvent(t *testing.T) {
	s := New(1)
	rec := &recorder{}
	receiver := func(e events.Event) {
		time.Sleep(time.Millisecond)
		rec.Record(e)
	}
	done := make(chan struct{})
	returned := startDrain(s, receiver, done)

	for i := 0; i < 50; i++ {
		s.Emit(events.Event{ResourceID: fmt.Sprintf("event-%d", i)})
	}

	close(done)
	waitForClose(t, returned, "Drain to return")

	require.Len(t, rec.regular(), 50)
}

func TestASingleBlockedStretchProducesExactlyOneBlockedEvent(t *testing.T) {
	s := New(1)
	s.Emit(events.Event{ResourceID: "fills-the-queue"})

	var emitters sync.WaitGroup
	for i := 0; i < 3; i++ {
		emitters.Add(1)
		go func() {
			defer emitters.Done()
			s.Emit(events.Event{ResourceID: fmt.Sprintf("blocked-%d", i)})
		}()
	}

	// every emitter is now waiting together, one stretch
	waitForWaitingEmitters(t, s, 3)

	// hold the stretch open for a measurable time before draining
	time.Sleep(10 * time.Millisecond)

	rec := &recorder{}
	done := make(chan struct{})
	returned := startDrain(s, rec.Record, done)

	emitters.Wait()
	close(done)
	waitForClose(t, returned, "Drain to return")

	blocked := rec.blocked()
	require.Len(t, blocked, 1)
	require.Equal(t, events.OperationEvents, blocked[0].Operation)
	require.Equal(t, events.PhaseBlocked, blocked[0].Phase)
	require.Equal(t, events.SourceCore, blocked[0].Source)
	require.False(t, blocked[0].Time.IsZero())
	require.Greater(t, blocked[0].Duration, time.Duration(0))
	require.GreaterOrEqual(t, blocked[0].Duration, 10*time.Millisecond)
}

func TestTheBlockedEventDoesNotReduceTheDeliveredRegularEvents(t *testing.T) {
	s := New(1)
	s.Emit(events.Event{ResourceID: "fills-the-queue"})

	var emitters sync.WaitGroup
	for i := 0; i < 3; i++ {
		emitters.Add(1)
		go func() {
			defer emitters.Done()
			s.Emit(events.Event{ResourceID: fmt.Sprintf("blocked-%d", i)})
		}()
	}

	waitForWaitingEmitters(t, s, 3)

	rec := &recorder{}
	done := make(chan struct{})
	returned := startDrain(s, rec.Record, done)

	emitters.Wait()
	close(done)
	waitForClose(t, returned, "Drain to return")

	require.Len(t, rec.blocked(), 1)
	require.Len(t, rec.regular(), 4)
	require.Len(t, rec.Events(), 5)
}

func TestDeliveryOrderMatchesEmitOrderForOneEmitter(t *testing.T) {
	s := New(1)
	rec := &recorder{}
	done := make(chan struct{})
	returned := startDrain(s, rec.Record, done)

	for i := 0; i < 100; i++ {
		s.Emit(events.Event{ResourceID: fmt.Sprintf("event-%03d", i)})
	}

	close(done)
	waitForClose(t, returned, "Drain to return")

	regular := rec.regular()
	require.Len(t, regular, 100)
	for i, e := range regular {
		require.Equal(t, fmt.Sprintf("event-%03d", i), e.ResourceID)
	}
}

func TestTheReceiverIsNeverEnteredConcurrently(t *testing.T) {
	s := New(2)

	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	var delivered atomic.Int32
	receiver := func(e events.Event) {
		current := inFlight.Add(1)
		for {
			seen := maxInFlight.Load()
			if current <= seen || maxInFlight.CompareAndSwap(seen, current) {
				break
			}
		}

		// give a concurrent call the chance to overlap
		time.Sleep(50 * time.Microsecond)

		if e.Phase != events.PhaseBlocked {
			delivered.Add(1)
		}
		inFlight.Add(-1)
	}
	done := make(chan struct{})
	returned := startDrain(s, receiver, done)

	var emitters sync.WaitGroup
	for emitter := 0; emitter < 8; emitter++ {
		emitters.Add(1)
		go func() {
			defer emitters.Done()
			for i := 0; i < 50; i++ {
				s.Emit(events.Event{ResourceID: fmt.Sprintf("emitter-%d-event-%d", emitter, i)})
			}
		}()
	}

	emitters.Wait()
	close(done)
	waitForClose(t, returned, "Drain to return")

	require.Equal(t, int32(1), maxInFlight.Load())
	require.Equal(t, int32(400), delivered.Load())
}

func TestDrainDoesNotReturnBeforeDoneIsClosed(t *testing.T) {
	s := New(5)
	for i := 0; i < 5; i++ {
		s.Emit(events.Event{ResourceID: fmt.Sprintf("event-%d", i)})
	}

	rec := &recorder{}
	done := make(chan struct{})
	returned := startDrain(s, rec.Record, done)

	require.Eventually(t, func() bool {
		return len(rec.Events()) == 5
	}, generousTimeout, time.Millisecond)

	// the queue is empty but done is still open
	select {
	case <-returned:
		require.FailNow(t, "Drain returned before done was closed")
	case <-time.After(50 * time.Millisecond):
	}

	close(done)
	waitForClose(t, returned, "Drain to return")
}

func TestDrainDeliversEverythingQueuedBeforeReturning(t *testing.T) {
	s := New(5)
	for i := 0; i < 5; i++ {
		s.Emit(events.Event{ResourceID: fmt.Sprintf("event-%d", i)})
	}

	done := make(chan struct{})
	close(done)

	rec := &recorder{}
	s.Drain(rec.Record, done)

	all := rec.Events()
	require.Len(t, all, 5)
	require.Equal(t, "event-0", all[0].ResourceID)
	require.Equal(t, "event-1", all[1].ResourceID)
	require.Equal(t, "event-2", all[2].ResourceID)
	require.Equal(t, "event-3", all[3].ResourceID)
	require.Equal(t, "event-4", all[4].ResourceID)
}

func TestDiscardReleasesABlockedEmitter(t *testing.T) {
	s := New(1)
	s.Emit(events.Event{ResourceID: "fills-the-queue"})

	emitted := make(chan struct{})
	go func() {
		defer close(emitted)
		s.Emit(events.Event{ResourceID: "blocked"})
	}()

	waitForWaitingEmitters(t, s, 1)

	s.Discard()

	waitForClose(t, emitted, "the blocked emitter to be released")
}

func TestEmitAfterDiscardDropsTheEvent(t *testing.T) {
	s := New(5)
	s.Discard()

	s.Emit(events.Event{ResourceID: "dropped"})

	done := make(chan struct{})
	close(done)
	rec := &recorder{}
	s.Drain(rec.Record, done)

	require.Empty(t, rec.Events())
}

func TestDiscardCanBeCalledMoreThanOnce(t *testing.T) {
	s := New(1)

	s.Discard()
	s.Discard()

	require.True(t, s.discard.Load())
}

func TestEmitStampsTheTimeAndSourceDefaults(t *testing.T) {
	s := New(1)

	before := time.Now()
	s.Emit(events.Event{})
	after := time.Now()

	done := make(chan struct{})
	close(done)
	rec := &recorder{}
	s.Drain(rec.Record, done)

	all := rec.Events()
	require.Len(t, all, 1)
	require.Equal(t, events.SourceCore, all[0].Source)
	require.False(t, all[0].Time.IsZero())
	require.False(t, all[0].Time.Before(before))
	require.False(t, all[0].Time.After(after))
}

func TestEmitKeepsATimeAndSourceGivenByTheCaller(t *testing.T) {
	s := New(1)
	given := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	s.Emit(events.Event{Time: given, Source: "example"})

	done := make(chan struct{})
	close(done)
	rec := &recorder{}
	s.Drain(rec.Record, done)

	all := rec.Events()
	require.Len(t, all, 1)
	require.Equal(t, "example", all[0].Source)
	require.Equal(t, given, all[0].Time)
}
