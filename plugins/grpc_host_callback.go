package plugins

import (
	"context"
	"encoding/json"
	"github.com/jumppad-labs/xcl/logger"
	"strconv"

	"github.com/jumppad-labs/xcl/plugins/proto"
)

// GRPCHostCallbackServer implements HostCallbackService for the host side
// This consolidates both logger and state functionality into one service
//
// A log message carries the ID of the provider call it was written during,
// and is written to that call's logger, which is bound to the call's resource
// and step and emits to the receiver of the operation making the call. A
// message with no ID, or one for a call that has returned, is written to the
// plugin scoped logger. Detail values arrive as text and are converted back
// to numbers and booleans where they parse as one, so they may not keep their
// original types.
type GRPCHostCallbackServer struct {
	proto.UnimplementedHostCallbackServiceServer
	logger logger.Logger
	state  State

	// calls holds the logger of each provider call in progress, it may be nil
	calls *callLoggers
}

// NewGRPCHostCallbackServer creates a new consolidated host callback server
func NewGRPCHostCallbackServer(logger logger.Logger, state State) *GRPCHostCallbackServer {
	return &GRPCHostCallbackServer{
		logger: logger,
		state:  state,
	}
}

// loggerFor returns the logger a log request is written to, the logger of the
// call it names when that call is in progress, otherwise the plugin scoped one
func (s *GRPCHostCallbackServer) loggerFor(req *proto.LogRequest) logger.Logger {
	if l, ok := s.calls.lookup(req.CallId); ok {
		return l
	}

	return s.logger
}

// Logger methods
func (s *GRPCHostCallbackServer) Info(ctx context.Context, req *proto.LogRequest) (*proto.LogResponse, error) {
	if l := s.loggerFor(req); l != nil {
		l.Info(req.Message, stringArgsToInterfaces(req.Args)...)
	}
	return &proto.LogResponse{}, nil
}

func (s *GRPCHostCallbackServer) Debug(ctx context.Context, req *proto.LogRequest) (*proto.LogResponse, error) {
	if l := s.loggerFor(req); l != nil {
		l.Debug(req.Message, stringArgsToInterfaces(req.Args)...)
	}
	return &proto.LogResponse{}, nil
}

func (s *GRPCHostCallbackServer) Warn(ctx context.Context, req *proto.LogRequest) (*proto.LogResponse, error) {
	if l := s.loggerFor(req); l != nil {
		l.Warn(req.Message, stringArgsToInterfaces(req.Args)...)
	}
	return &proto.LogResponse{}, nil
}

func (s *GRPCHostCallbackServer) Error(ctx context.Context, req *proto.LogRequest) (*proto.LogResponse, error) {
	if l := s.loggerFor(req); l != nil {
		l.Error(req.Message, stringArgsToInterfaces(req.Args)...)
	}
	return &proto.LogResponse{}, nil
}

// State methods
func (s *GRPCHostCallbackServer) Get(ctx context.Context, req *proto.StateGetRequest) (*proto.StateGetResponse, error) {
	if s.state == nil {
		return &proto.StateGetResponse{Error: "state service not available"}, nil
	}

	resource, err := s.state.Get(req.Key)
	if err != nil {
		return &proto.StateGetResponse{Error: err.Error()}, nil
	}

	// Serialize the resource to bytes
	resourceData, err := json.Marshal(resource)
	if err != nil {
		return &proto.StateGetResponse{Error: err.Error()}, nil
	}

	return &proto.StateGetResponse{ResourceData: resourceData}, nil
}

func (s *GRPCHostCallbackServer) Find(ctx context.Context, req *proto.StateFindRequest) (*proto.StateFindResponse, error) {
	if s.state == nil {
		return &proto.StateFindResponse{Error: "state service not available"}, nil
	}

	resources, err := s.state.Find(req.Pattern)
	if err != nil {
		return &proto.StateFindResponse{Error: err.Error()}, nil
	}

	// Serialize all resources to bytes
	resourcesData := make([][]byte, len(resources))
	for i, resource := range resources {
		data, err := json.Marshal(resource)
		if err != nil {
			return &proto.StateFindResponse{Error: err.Error()}, nil
		}
		resourcesData[i] = data
	}

	return &proto.StateFindResponse{ResourcesData: resourcesData}, nil
}

// Helper functions
func stringArgsToInterfaces(args []string) []interface{} {
	result := make([]interface{}, len(args))
	for i, arg := range args {
		// Try to parse as different types, fallback to string
		if val, err := strconv.Atoi(arg); err == nil {
			result[i] = val
		} else if val, err := strconv.ParseFloat(arg, 64); err == nil {
			result[i] = val
		} else if val, err := strconv.ParseBool(arg); err == nil {
			result[i] = val
		} else {
			result[i] = arg
		}
	}
	return result
}
