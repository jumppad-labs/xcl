package coreservice

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

// queueTimeout bounds how long a queue test waits for a goroutine
const queueTimeout = 5 * time.Second

// runResult is what one call to run returned
type runResult struct {
	completed *corepb.CompleteRequest
	err       error
}

// buildStateTask is a build function for run that posts a LOAD state task
// under the id the queue assigns
func buildStateTask(id uint64) *corepb.PollResponse {
	return &corepb.PollResponse{Task: &corepb.PollResponse_State{State: &corepb.StateTask{
		TaskId: id,
		Kind:   corepb.StateTask_LOAD,
	}}}
}

// runOnGoroutine calls run on its own goroutine and returns the channel its
// result arrives on
func runOnGoroutine(ctx context.Context, q *taskQueue) <-chan runResult {
	results := make(chan runResult, 1)

	go func() {
		completed, err := q.run(ctx, buildStateTask)
		results <- runResult{completed: completed, err: err}
	}()

	return results
}

// awaitRun waits for a run to return, failing the test when it does not
func awaitRun(t *testing.T, results <-chan runResult) runResult {
	t.Helper()

	select {
	case result := <-results:
		return result
	case <-time.After(queueTimeout):
		t.Fatal("run did not return")
	}

	return runResult{}
}

// pollState polls the queue and requires the task to be a state task
func pollState(t *testing.T, q *taskQueue) *corepb.StateTask {
	t.Helper()

	task := q.poll(queueTimeout)
	require.NotNil(t, task, "no task arrived")
	require.NotNil(t, task.GetState(), "the task is not a state task: %v", task)

	return task.GetState()
}

// stateAnswer is a Complete for task id carrying an empty state result
func stateAnswer(id uint64) *corepb.CompleteRequest {
	return &corepb.CompleteRequest{
		TaskId: id,
		Result: &corepb.CompleteRequest_State{State: &corepb.StateResult{Exists: true}},
	}
}

func TestQueuePollReturnsPostedTask(t *testing.T) {
	q := newTaskQueue()

	posted := &corepb.PollResponse{Task: &corepb.PollResponse_Event{Event: &corepb.Event{Operation: "apply", Phase: "start"}}}
	q.post(posted)

	got := q.poll(queueTimeout)
	require.Same(t, posted, got)
}

func TestQueuePollTimesOutEmpty(t *testing.T) {
	q := newTaskQueue()

	started := time.Now()

	require.Nil(t, q.poll(10*time.Millisecond))
	require.Less(t, time.Since(started), time.Second)

	// a zero wait does not wait at all
	require.Nil(t, q.poll(0))
}

func TestQueueRunWaitsForComplete(t *testing.T) {
	q := newTaskQueue()

	results := runOnGoroutine(context.Background(), q)

	task := pollState(t, q)

	// run is still waiting for the host
	select {
	case result := <-results:
		t.Fatalf("run returned before the task was completed: %+v", result)
	case <-time.After(50 * time.Millisecond):
	}

	answer := stateAnswer(task.GetTaskId())
	require.NoError(t, q.complete(answer))

	result := awaitRun(t, results)
	require.NoError(t, result.err)
	require.Same(t, answer, result.completed)
}

func TestQueueRunReturnsWhenContextCancelled(t *testing.T) {
	q := newTaskQueue()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := runOnGoroutine(ctx, q)

	task := pollState(t, q)
	require.NotZero(t, task.GetTaskId())

	cancel()

	result := awaitRun(t, results)
	require.ErrorIs(t, result.err, context.Canceled)
	require.Nil(t, result.completed)
}

func TestQueueCompleteUnknownIdIsInvalidArgument(t *testing.T) {
	q := newTaskQueue()

	err := q.complete(stateAnswer(7))
	require.Error(t, err)

	var coded *codedError
	require.True(t, errors.As(err, &coded), "error is %T", err)
	require.Equal(t, CodeInvalidArgument, coded.code)
	require.Equal(t, "7", coded.details["task_id"])
}

func TestQueueCompleteTwiceIsInvalidArgument(t *testing.T) {
	q := newTaskQueue()

	results := runOnGoroutine(context.Background(), q)

	task := pollState(t, q)

	require.NoError(t, q.complete(stateAnswer(task.GetTaskId())))
	require.NoError(t, awaitRun(t, results).err)

	err := q.complete(stateAnswer(task.GetTaskId()))
	require.Error(t, err)

	var coded *codedError
	require.True(t, errors.As(err, &coded), "error is %T", err)
	require.Equal(t, CodeInvalidArgument, coded.code)
	require.Equal(t, task.GetTaskId(), parseUint(t, coded.details["task_id"]))
}

// parseUint reads a decimal id out of an error's details
func parseUint(t *testing.T, s string) uint64 {
	t.Helper()

	id, err := strconv.ParseUint(s, 10, 64)
	require.NoError(t, err)

	return id
}

func TestQueueCloseReleasesWaiters(t *testing.T) {
	q := newTaskQueue()

	results := runOnGoroutine(context.Background(), q)

	// the task is queued and run is waiting on the host
	pollState(t, q)

	// a poll waiting on an empty queue is also released
	polled := make(chan *corepb.PollResponse, 1)
	go func() { polled <- q.poll(queueTimeout) }()

	q.close()

	result := awaitRun(t, results)
	require.Error(t, result.err)
	require.NotErrorIs(t, result.err, context.Canceled)
	require.Nil(t, result.completed)

	select {
	case task := <-polled:
		require.Nil(t, task)
	case <-time.After(queueTimeout):
		t.Fatal("poll was not released by close")
	}

	// closing again is harmless
	q.close()
}

func TestQueueTaskIdsIncrease(t *testing.T) {
	q := newTaskQueue()

	first := runOnGoroutine(context.Background(), q)
	firstTask := pollState(t, q)
	require.NoError(t, q.complete(stateAnswer(firstTask.GetTaskId())))
	require.NoError(t, awaitRun(t, first).err)

	second := runOnGoroutine(context.Background(), q)
	secondTask := pollState(t, q)
	require.NoError(t, q.complete(stateAnswer(secondTask.GetTaskId())))
	require.NoError(t, awaitRun(t, second).err)

	require.Equal(t, uint64(1), firstTask.GetTaskId())
	require.Equal(t, uint64(2), secondTask.GetTaskId())
}
