// Package eventstream delivers the events of one operation to the
// application's receiver. Emitting never waits for the receiver, only for
// space in a bounded queue, and no event is ever dropped. The receiver is
// called one event at a time, in emit order, from a drain loop the caller
// runs on its own goroutine.
package eventstream

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/jumppad-labs/xcl/events"
)

// Stream is a bounded queue of undelivered events for one operation
type Stream struct {
	queue chan events.Event

	// wake tells Drain a blocked announcement is waiting, it holds at most
	// one signal
	wake chan struct{}

	// mu guards waiting, stretchStart and blocked
	mu sync.Mutex

	// waiting is the number of emitters waiting for space in queue
	waiting int

	// stretchStart is when the first emitter of the current blocked
	// stretch started waiting
	stretchStart time.Time

	// blocked is the announcement of the last blocked stretch, delivered
	// ahead of queue so it never takes a queue slot
	blocked *events.Event

	discard     atomic.Bool
	discardCh   chan struct{}
	discardOnce sync.Once
}

// New returns a Stream that holds at most size undelivered events, a size
// below 1 is treated as 1
func New(size int) *Stream {
	if size < 1 {
		size = 1
	}

	return &Stream{
		queue:     make(chan events.Event, size),
		wake:      make(chan struct{}, 1),
		discardCh: make(chan struct{}),
	}
}

// Emit queues e for delivery. It returns at once when the queue has room,
// otherwise it waits for room and never drops e. A zero Time is stamped with
// the current time and an empty Source with events.SourceCore. Emit is safe
// to call from several goroutines. After Discard, Emit drops e.
func (s *Stream) Emit(e events.Event) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}

	if e.Source == "" {
		e.Source = events.SourceCore
	}

	if s.discard.Load() {
		return
	}

	select {
	case s.queue <- e:
		return
	default:
	}

	// the queue is full, wait for room as part of a blocked stretch that
	// lasts from the first emitter waiting until no emitter is waiting
	s.mu.Lock()
	if s.waiting == 0 {
		s.stretchStart = time.Now()
	}
	s.waiting++
	s.mu.Unlock()

	select {
	case s.queue <- e:
	case <-s.discardCh:
	}

	s.mu.Lock()
	s.waiting--
	announce := s.waiting == 0 && !s.discard.Load()
	if announce {
		s.blocked = &events.Event{
			Time:      time.Now(),
			Source:    events.SourceCore,
			Operation: events.OperationEvents,
			Phase:     events.PhaseBlocked,
			Duration:  time.Since(s.stretchStart),
		}
	}
	s.mu.Unlock()

	if announce {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// Drain calls receiver for every event, one at a time and in emit order, a
// blocked announcement is delivered ahead of the events still queued. It
// returns once done is closed and every event emitted before that has been
// delivered. Drain must be called by one goroutine only, and the receiver
// is never called while the Stream holds a lock.
func (s *Stream) Drain(receiver events.Handler, done <-chan struct{}) {
	for {
		if e, ok := s.takeBlocked(); ok {
			receiver(e)
			continue
		}

		select {
		case e := <-s.queue:
			receiver(e)
		case <-s.wake:
		case <-done:
			s.drainRemaining(receiver)
			return
		}
	}
}

// drainRemaining delivers everything still queued once no more events can
// be emitted
func (s *Stream) drainRemaining(receiver events.Handler) {
	for {
		if e, ok := s.takeBlocked(); ok {
			receiver(e)
			continue
		}

		select {
		case e := <-s.queue:
			receiver(e)
		default:
			return
		}
	}
}

// takeBlocked returns and clears the pending blocked announcement
func (s *Stream) takeBlocked() (events.Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.blocked == nil {
		return events.Event{}, false
	}

	e := *s.blocked
	s.blocked = nil

	return e, true
}

// Discard stops delivery for an emergency shutdown. Emitters waiting for room
// are released and every later event is dropped, so nothing emitting can
// wait on a receiver that is no longer being called.
func (s *Stream) Discard() {
	s.discardOnce.Do(func() {
		s.discard.Store(true)
		close(s.discardCh)
	})
}
