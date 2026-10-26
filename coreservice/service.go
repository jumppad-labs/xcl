// Package coreservice implements CoreService, the message API through which a
// host written in another language drives the xcl engine. It is the whole of
// the binding logic, written once and free of any transport: the shared
// library in bindings/libxcl moves bytes between C and Service.Call, and the
// engine program in cmd/xcl-core does the same over its standard input and
// output.
//
// Every method of CoreService, defined in proto/xcl/core/v1/core.proto, is
// dispatched by name through Service.Call with the method's serialised request
// and returns its serialised response and a Status. coreservice reaches the
// engine only through the public packages of the root module, so it needs
// neither cgo nor a C compiler, and every behaviour is tested with go test
// against Service.Call.
package coreservice

import (
	"sync"
	"sync/atomic"

	"google.golang.org/protobuf/proto"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

// Status is the result of a Call. Zero is success and 1 an application error
// carrying an Error message; the negative statuses are programming errors in
// the binding and carry no message.
type Status int32

const (
	// StatusOK is success, the response holds the method's response message.
	StatusOK Status = 0

	// StatusError is an application error, the response holds an Error
	// message whose code the binding maps to an exception.
	StatusError Status = 1

	// StatusUnknownMethod names no CoreService method.
	StatusUnknownMethod Status = -1

	// StatusBadRequest is request bytes that do not parse as the method's
	// request message.
	StatusBadRequest Status = -2

	// StatusUnknownHandle is a handle that was never created or has been
	// released.
	StatusUnknownHandle Status = -3

	// StatusHandleMismatch is handle 0 given to a method that needs a
	// configuration, or a configuration handle given to one that takes
	// none.
	StatusHandleMismatch Status = -4
)

// Service holds every configuration a host has created, by handle, and
// dispatches calls to them. One Service serves a process.
type Service struct {
	mu      sync.RWMutex
	configs map[uint64]*configuration
	next    atomic.Uint64
}

// New returns an empty Service.
func New() *Service {
	return &Service{configs: map[uint64]*configuration{}}
}

// Call dispatches one CoreService method. method is the method name, request
// the serialised request message. It returns the serialised response and a
// Status; see Status for what the response holds in each case. It is safe
// for concurrent use from any goroutine or thread, with any handles.
func (s *Service) Call(handle uint64, method string, request []byte) ([]byte, Status) {
	h, ok := handlers[method]
	if !ok {
		return nil, StatusUnknownMethod
	}

	if h.takesHandle != (handle != 0) {
		return nil, StatusHandleMismatch
	}

	var conf *configuration
	if h.takesHandle {
		conf = s.lookup(handle)
		if conf == nil {
			return nil, StatusUnknownHandle
		}
	}

	response, err := h.call(s, conf, request)
	if err != nil {
		if _, bad := err.(badRequest); bad {
			return nil, StatusBadRequest
		}

		body, _ := proto.Marshal(toProto(err))
		return body, StatusError
	}

	body, err := proto.Marshal(response)
	if err != nil {
		body, _ := proto.Marshal(toProto(err))
		return body, StatusError
	}

	return body, StatusOK
}

// Release frees the configuration behind handle and everything it owns,
// cancelling any operation in progress and waiting for it to stop so its
// state is saved. A handle used after Release returns StatusUnknownHandle.
func (s *Service) Release(handle uint64) Status {
	s.mu.Lock()
	conf, ok := s.configs[handle]
	delete(s.configs, handle)
	s.mu.Unlock()

	if !ok {
		return StatusUnknownHandle
	}

	conf.release()

	return StatusOK
}

// Close releases every configuration the Service holds, as Release does for
// each: it cancels each operation in progress and waits for it to stop so its
// state is saved. A transport calls it when its host goes away, for example
// when the engine program's input ends. Handles stay never reused, so every
// handle used after Close returns StatusUnknownHandle.
func (s *Service) Close() {
	s.mu.Lock()
	configs := s.configs
	s.configs = map[uint64]*configuration{}
	s.mu.Unlock()

	var released sync.WaitGroup
	for _, conf := range configs {
		released.Go(conf.release)
	}

	released.Wait()
}

// add stores conf under a new handle and returns it. Handles are never
// reused, so a released handle stays invalid for the life of the Service.
func (s *Service) add(conf *configuration) uint64 {
	handle := s.next.Add(1)

	s.mu.Lock()
	s.configs[handle] = conf
	s.mu.Unlock()

	return handle
}

// lookup returns the configuration behind handle, or nil.
func (s *Service) lookup(handle uint64) *configuration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.configs[handle]
}

// badRequest marks request bytes that do not parse as the method's request.
type badRequest struct {
	error
}

// handler is one CoreService method: whether it takes a configuration handle,
// and the call that decodes the request and produces the response.
type handler struct {
	takesHandle bool
	call        func(s *Service, conf *configuration, request []byte) (proto.Message, error)
}

// unary adapts a typed method to bytes in, message out. It decodes the request
// into a new Req and returns badRequest when the bytes do not parse.
func unary[Req any, PReq interface {
	*Req
	proto.Message
}, Resp proto.Message](takesHandle bool, fn func(s *Service, conf *configuration, req PReq) (Resp, error)) handler {
	return handler{
		takesHandle: takesHandle,
		call: func(s *Service, conf *configuration, request []byte) (proto.Message, error) {
			req := PReq(new(Req))
			if err := proto.Unmarshal(request, req); err != nil {
				return nil, badRequest{err}
			}

			resp, err := fn(s, conf, req)
			if err != nil {
				return nil, err
			}

			return resp, nil
		},
	}
}

// handlers maps each CoreService method to its implementation. A test holds
// it to the service descriptor, so a method added to the proto without a
// handler, or a handler for a method the proto does not define, fails.
var handlers = map[string]handler{
	"GetVersion": unary(false, func(_ *Service, _ *configuration, _ *corepb.GetVersionRequest) (*corepb.GetVersionResponse, error) {
		return &corepb.GetVersionResponse{Version: Version, AbiVersion: ABIVersion}, nil
	}),
	"CreateConfig": unary(false, (*Service).createConfig),
	"ConvertTheme": unary(false, (*Service).convertTheme),

	"StartValidate": unary(true, (*Service).startValidate),
	"StartApply":    unary(true, (*Service).startApply),
	"StartDiff":     unary(true, (*Service).startDiff),
	"StartDestroy":  unary(true, (*Service).startDestroy),
	"StartLoad":     unary(true, (*Service).startLoad),
	"Cancel":        unary(true, (*Service).cancel),
	"Poll":          unary(true, (*Service).poll),
	"Complete":      unary(true, (*Service).complete),

	"Entities":          unary(true, (*Service).entities),
	"Find":              unary(true, (*Service).find),
	"FindByType":        unary(true, (*Service).findByType),
	"FindOne":           unary(true, (*Service).findOne),
	"Outputs":           unary(true, (*Service).outputs),
	"EncodeSavedEntity": unary(true, (*Service).encodeSavedEntity),
	"EncodeEntity":      unary(true, (*Service).encodeEntity),
}
