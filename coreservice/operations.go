package coreservice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/diff"
)

// operation is one Validate, Apply, Diff, Destroy or Load running on a
// configuration.
type operation struct {
	id     uint64
	ctx    context.Context
	cancel context.CancelFunc

	// done is closed once the operation has returned and its finished task
	// is queued
	done chan struct{}
}

// start runs work as a new operation on its own goroutine, under a context
// Cancel cancels, and returns its id. Its end is posted as an
// OperationFinished task after every task the work posted, so the host sees
// it last. At most one operation runs at a time; a second is refused.
func (c *configuration) start(work func(ctx context.Context) (diffJSON []byte, err error)) (uint64, error) {
	c.mu.Lock()

	if c.inProgress != nil {
		c.mu.Unlock()
		return 0, operationInProgress("another operation")
	}

	c.nextOperation++

	ctx, cancel := context.WithCancel(context.Background())
	op := &operation{id: c.nextOperation, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	c.inProgress = op

	c.mu.Unlock()

	go func() {
		defer close(op.done)

		diffJSON, err := work(op.ctx)
		op.cancel()

		// the operation is over before the host can see it finish, so a call
		// the host makes as soon as it polls the finished task is not refused
		c.mu.Lock()
		if c.inProgress == op {
			c.inProgress = nil
		}
		c.mu.Unlock()

		c.queue.post(&corepb.PollResponse{Task: &corepb.PollResponse_Finished{Finished: &corepb.OperationFinished{
			OperationId: op.id,
			Error:       toProto(err),
			DiffJson:    diffJSON,
		}}})
	}()

	return op.id, nil
}

// current returns the operation in progress, or nil.
func (c *configuration) current() *operation {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.inProgress
}

// running reports whether an operation is in progress.
func (c *configuration) running() bool {
	return c.current() != nil
}

// lookupAllowed refuses a lookup while an operation runs on the
// configuration, since the entity slice is replaced when an operation ends.
func (c *configuration) lookupAllowed(what string) error {
	if c.running() {
		return operationInProgress(what)
	}

	return nil
}

// operationContext returns the context and id of the operation in progress,
// which a task waits under, or an error when none is running.
func (c *configuration) operationContext() (context.Context, uint64, error) {
	op := c.current()
	if op == nil {
		return nil, 0, fmt.Errorf("no operation is in progress")
	}

	return op.ctx, op.id, nil
}

// release cancels the operation in progress, waits for it to return so its
// state is saved, and releases everything waiting on the queue.
func (c *configuration) release() {
	if op := c.current(); op != nil {
		op.cancel()
		<-op.done
	}

	c.queue.close()
}

func (s *Service) startValidate(conf *configuration, req *corepb.StartValidateRequest) (*corepb.StartOperationResponse, error) {
	if len(req.GetPaths()) == 0 {
		return nil, invalidArgument("at least one path is required", "argument", "paths")
	}

	return startResponse(conf.start(func(ctx context.Context) ([]byte, error) {
		return nil, conf.cfg.ValidateContext(ctx, req.GetPaths()...)
	}))
}

func (s *Service) startApply(conf *configuration, req *corepb.StartApplyRequest) (*corepb.StartOperationResponse, error) {
	if len(req.GetPaths()) == 0 {
		return nil, invalidArgument("at least one path is required", "argument", "paths")
	}

	return startResponse(conf.start(func(ctx context.Context) ([]byte, error) {
		return nil, conf.cfg.ApplyContext(ctx, req.GetPaths()...)
	}))
}

func (s *Service) startDiff(conf *configuration, req *corepb.StartDiffRequest) (*corepb.StartOperationResponse, error) {
	if len(req.GetPaths()) == 0 {
		return nil, invalidArgument("at least one path is required", "argument", "paths")
	}

	options := []diff.Option{}
	if req.GetRevealSensitive() {
		options = append(options, diff.RevealSensitive())
	}

	return startResponse(conf.start(func(ctx context.Context) ([]byte, error) {
		found, err := conf.cfg.DiffContext(ctx, req.GetPaths(), options...)
		if err != nil {
			return nil, err
		}

		return json.Marshal(found)
	}))
}

func (s *Service) startDestroy(conf *configuration, _ *corepb.StartDestroyRequest) (*corepb.StartOperationResponse, error) {
	return startResponse(conf.start(func(ctx context.Context) ([]byte, error) {
		return nil, conf.cfg.DestroyContext(ctx)
	}))
}

func (s *Service) startLoad(conf *configuration, _ *corepb.StartLoadRequest) (*corepb.StartOperationResponse, error) {
	return startResponse(conf.start(func(ctx context.Context) ([]byte, error) {
		return nil, conf.cfg.LoadContext(ctx)
	}))
}

// startResponse turns the result of start into the response.
func startResponse(id uint64, err error) (*corepb.StartOperationResponse, error) {
	if err != nil {
		return nil, err
	}

	return &corepb.StartOperationResponse{OperationId: id}, nil
}

// cancel is Cancel: it cancels the named operation's context, so every task
// waiting under it returns at once and the engine starts no further provider
// call. The operation ends with the cancelled code, after saving the progress
// made. Cancelling an operation that has already ended does nothing.
func (s *Service) cancel(conf *configuration, req *corepb.CancelRequest) (*corepb.CancelResponse, error) {
	id := req.GetOperationId()

	conf.mu.Lock()
	op := conf.inProgress
	known := id != 0 && id <= conf.nextOperation
	conf.mu.Unlock()

	if !known {
		return nil, invalidArgument(fmt.Sprintf("no operation %d was started", id), "operation_id", fmt.Sprint(id))
	}

	if op != nil && op.id == id {
		op.cancel()
	}

	return &corepb.CancelResponse{}, nil
}

// poll is Poll: the next task, waited for up to wait_ms.
func (s *Service) poll(conf *configuration, req *corepb.PollRequest) (*corepb.PollResponse, error) {
	task := conf.queue.poll(time.Duration(req.GetWaitMs()) * time.Millisecond)
	if task == nil {
		return &corepb.PollResponse{}, nil
	}

	return task, nil
}

// complete is Complete: the host's answer to a task.
func (s *Service) complete(conf *configuration, req *corepb.CompleteRequest) (*corepb.CompleteResponse, error) {
	if err := conf.queue.complete(req); err != nil {
		return nil, err
	}

	return &corepb.CompleteResponse{}, nil
}
