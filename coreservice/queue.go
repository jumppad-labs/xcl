package coreservice

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

// pendingTasks bounds the tasks waiting to be polled. Events are posted one
// at a time by the operation's own goroutine and function and state tasks by
// the parser's bounded parallelism, so the bound is only ever reached by a
// host that has stopped polling.
const pendingTasks = 256

// taskQueue carries everything the engine needs the host for, in one ordered
// stream: events, state-store calls, function calls and the end of an
// operation. The engine never calls into the host; it posts a task and, when
// the task has an answer, waits for Complete.
type taskQueue struct {
	pending  chan *pendingTask
	inflight sync.Map // task id -> *pendingTask
	next     atomic.Uint64

	// closed is closed when the configuration is released, so nothing
	// blocks on a host that will never poll again
	closed    chan struct{}
	closeOnce sync.Once
}

// pendingTask is one task on the queue, with the channel its completion is
// delivered on, nil for a task that has no completion.
type pendingTask struct {
	task *corepb.PollResponse
	done chan *corepb.CompleteRequest
}

func newTaskQueue() *taskQueue {
	return &taskQueue{
		pending: make(chan *pendingTask, pendingTasks),
		closed:  make(chan struct{}),
	}
}

// post queues a task that has no completion, an event or the end of an
// operation. It blocks while the queue is full, which is how backpressure
// from a slow host reaches the engine, and returns once the task is queued
// or the queue is closed.
func (q *taskQueue) post(task *corepb.PollResponse) {
	select {
	case q.pending <- &pendingTask{task: task}:
	case <-q.closed:
	}
}

// run queues the task build returns, given its new id, and waits for the
// host to complete it or for ctx to end. An ended ctx returns its error; a
// Complete that arrives afterwards finds no task and is refused.
func (q *taskQueue) run(ctx context.Context, build func(id uint64) *corepb.PollResponse) (*corepb.CompleteRequest, error) {
	id := q.next.Add(1)

	p := &pendingTask{task: build(id), done: make(chan *corepb.CompleteRequest, 1)}

	q.inflight.Store(id, p)
	defer q.inflight.Delete(id)

	select {
	case q.pending <- p:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-q.closed:
		return nil, fmt.Errorf("the configuration was released")
	}

	select {
	case result := <-p.done:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-q.closed:
		return nil, fmt.Errorf("the configuration was released")
	}
}

// poll returns the next task, waiting up to wait for one, or nil when none
// arrived in time.
func (q *taskQueue) poll(wait time.Duration) *corepb.PollResponse {
	if wait <= 0 {
		select {
		case p := <-q.pending:
			return p.task
		default:
			return nil
		}
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case p := <-q.pending:
		return p.task
	case <-timer.C:
		return nil
	case <-q.closed:
		return nil
	}
}

// complete delivers the host's result to the task it names. A task that is
// unknown, already completed or no longer waited for is refused.
func (q *taskQueue) complete(result *corepb.CompleteRequest) error {
	id := result.GetTaskId()

	value, ok := q.inflight.LoadAndDelete(id)
	if !ok {
		return invalidArgument(fmt.Sprintf("no task %d is waiting to be completed", id), "task_id", fmt.Sprint(id))
	}

	p := value.(*pendingTask)

	select {
	case p.done <- result:
	default:
	}

	return nil
}

// close releases everything waiting on the queue.
func (q *taskQueue) close() {
	q.closeOnce.Do(func() { close(q.closed) })
}
