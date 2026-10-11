package coreservice

import (
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

// Operations are added by a later task: each Start* runs the operation on a
// goroutine, the host polls for its tasks and completes them, and Cancel
// cancels its context. Until then every operation method answers that it is
// not available, and no operation is ever in progress.

// lookupAllowed refuses a lookup while an operation runs on the
// configuration, since the entity slice is replaced when an operation ends.
func (c *configuration) lookupAllowed(what string) error {
	if c.running() {
		return operationInProgress(what)
	}

	return nil
}

// running reports whether an operation is in progress.
func (c *configuration) running() bool {
	return false
}

func notAvailable(what string) error {
	return &codedError{code: CodeInternal, message: what + " is not available in this build"}
}

func (s *Service) startValidate(_ *configuration, _ *corepb.StartValidateRequest) (*corepb.StartOperationResponse, error) {
	return nil, notAvailable("StartValidate")
}

func (s *Service) startApply(_ *configuration, _ *corepb.StartApplyRequest) (*corepb.StartOperationResponse, error) {
	return nil, notAvailable("StartApply")
}

func (s *Service) startDiff(_ *configuration, _ *corepb.StartDiffRequest) (*corepb.StartOperationResponse, error) {
	return nil, notAvailable("StartDiff")
}

func (s *Service) startDestroy(_ *configuration, _ *corepb.StartDestroyRequest) (*corepb.StartOperationResponse, error) {
	return nil, notAvailable("StartDestroy")
}

func (s *Service) startLoad(_ *configuration, _ *corepb.StartLoadRequest) (*corepb.StartOperationResponse, error) {
	return nil, notAvailable("StartLoad")
}

func (s *Service) cancel(_ *configuration, _ *corepb.CancelRequest) (*corepb.CancelResponse, error) {
	return nil, notAvailable("Cancel")
}

func (s *Service) poll(_ *configuration, _ *corepb.PollRequest) (*corepb.PollResponse, error) {
	return nil, notAvailable("Poll")
}

func (s *Service) complete(_ *configuration, _ *corepb.CompleteRequest) (*corepb.CompleteResponse, error) {
	return nil, notAvailable("Complete")
}
