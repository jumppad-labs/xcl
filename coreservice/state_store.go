package coreservice

import (
	"encoding/json"
	"errors"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/state"
)

// hostStateStore is the state.StateStore a configuration created with
// StateStore.host uses: each method is a StateTask the host answers through
// Complete. The engine only calls the store inside an operation, when the
// host is polling, and each call waits under the operation's context, so
// cancelling the operation releases it.
//
// Records carry real sensitive values when no state mask is set, exactly as
// they reach a Go store; a state mask masks them before they cross.
type hostStateStore struct {
	conf *configuration
}

var _ state.StateStore = (*hostStateStore)(nil)

func (s *hostStateStore) Load() ([]any, error) {
	result, err := s.call(corepb.StateTask_LOAD, nil)
	if err != nil {
		return nil, err
	}

	loaded := make([]any, 0, len(result.GetEntities()))
	for _, record := range result.GetEntities() {
		loaded = append(loaded, json.RawMessage(record))
	}

	return loaded, nil
}

func (s *hostStateStore) Save(entities []any) error {
	records := make([][]byte, 0, len(entities))
	for _, e := range entities {
		record, err := encodeRecord(e)
		if err != nil {
			return err
		}

		records = append(records, record)
	}

	_, err := s.call(corepb.StateTask_SAVE, records)

	return err
}

func (s *hostStateStore) Exists() bool {
	result, err := s.call(corepb.StateTask_EXISTS, nil)
	if err != nil {
		return false
	}

	return result.GetExists()
}

func (s *hostStateStore) Clear() error {
	_, err := s.call(corepb.StateTask_CLEAR, nil)

	return err
}

// call posts one StateTask under the operation in progress and waits for its
// StateResult.
func (s *hostStateStore) call(kind corepb.StateTask_Kind, entities [][]byte) (*corepb.StateResult, error) {
	ctx, operationID, err := s.conf.operationContext()
	if err != nil {
		return nil, err
	}

	completed, err := s.conf.queue.run(ctx, func(id uint64) *corepb.PollResponse {
		return &corepb.PollResponse{Task: &corepb.PollResponse_State{State: &corepb.StateTask{
			TaskId:      id,
			OperationId: operationID,
			Kind:        kind,
			Entities:    entities,
		}}}
	})
	if err != nil {
		return nil, err
	}

	result := completed.GetState()
	if result == nil {
		return nil, errors.New("the host completed a state task without a state result")
	}

	if result.GetError() != nil {
		return nil, errors.New(result.GetError().GetMessage())
	}

	return result, nil
}

// encodeRecord renders one saved record as the JSON bytes it crosses as.
func encodeRecord(e any) ([]byte, error) {
	switch record := e.(type) {
	case json.RawMessage:
		return record, nil
	case []byte:
		return record, nil
	default:
		return json.Marshal(e)
	}
}
