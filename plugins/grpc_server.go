package plugins

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/go-plugin"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/proto"
)

// GRPCServer wraps PluginBase and implements the gRPC PluginService
type GRPCServer struct {
	proto.UnimplementedPluginServiceServer
	plugin Plugin
	broker *plugin.GRPCBroker
	state  State

	// mu guards callbackClient, RPCs are served concurrently
	mu             sync.Mutex
	callbackClient proto.HostCallbackServiceClient // the connection to the host, dialled once
}

// NewGRPCServer creates a new gRPC server with provided logger and state
func NewGRPCServer(plugin Plugin, broker *plugin.GRPCBroker) (*GRPCServer, error) {
	server := &GRPCServer{
		plugin: plugin,
		broker: broker,
	}

	return server, nil
}

func (s *GRPCServer) getRegisteredType(entityType, entitySubType string) *RegisteredType {
	types := s.plugin.GetTypes()
	for i := range types {
		if types[i].Type == entityType && types[i].SubType == entitySubType {
			return &types[i]
		}
	}
	return nil
}

func (s *GRPCServer) GetTypes(ctx context.Context, req *proto.GetTypesRequest) (*proto.GetTypesResponse, error) {

	types := s.plugin.GetTypes()
	protoTypes := make([]*proto.RegisteredType, len(types))

	for i, t := range types {
		protoTypes[i] = &proto.RegisteredType{
			Type:    t.Type,
			SubType: t.SubType,
			Schema:  t.Schema,
		}
	}

	return &proto.GetTypesResponse{Types: protoTypes}, nil
}

func (s *GRPCServer) Validate(ctx context.Context, req *proto.ValidateRequest) (*proto.ValidateResponse, error) {
	ctx, err := s.withLogger(ctx)
	if err != nil {
		return nil, err
	}

	err = s.plugin.Validate(ctx, req.EntityType, req.EntitySubType, req.EntityData)
	return &proto.ValidateResponse{Error: errorToString(err)}, nil
}

func (s *GRPCServer) Create(ctx context.Context, req *proto.CreateRequest) (*proto.CreateResponse, error) {
	ctx, err := s.withLogger(ctx)
	if err != nil {
		return nil, err
	}

	// Get the registered type to access its adapter
	rt := s.getRegisteredType(req.EntityType, req.EntitySubType)
	if rt == nil {
		return &proto.CreateResponse{Error: "no registered type found for " + req.EntityType + "." + req.EntitySubType}, nil
	}

	// Call the adapter's Create method which returns mutated data
	mutatedData, err := rt.Adapter.Create(ctx, req.EntityData)
	return &proto.CreateResponse{
		Error:             errorToString(err),
		MutatedEntityData: mutatedData,
	}, nil
}

func (s *GRPCServer) Destroy(ctx context.Context, req *proto.DestroyRequest) (*proto.DestroyResponse, error) {
	ctx, err := s.withLogger(ctx)
	if err != nil {
		return nil, err
	}

	err = s.plugin.Destroy(ctx, req.EntityType, req.EntitySubType, req.EntityData)
	return &proto.DestroyResponse{Error: errorToString(err)}, nil
}

func (s *GRPCServer) Read(ctx context.Context, req *proto.ReadRequest) (*proto.ReadResponse, error) {
	ctx, err := s.withLogger(ctx)
	if err != nil {
		return nil, err
	}

	// Get the registered type to access its adapter
	rt := s.getRegisteredType(req.EntityType, req.EntitySubType)
	if rt == nil {
		return &proto.ReadResponse{Error: "no registered type found for " + req.EntityType + "." + req.EntitySubType}, nil
	}

	// Call the adapter's Read method which returns the read data
	readData, err := rt.Adapter.Read(ctx, req.OldEntityData, req.NewEntityData)
	return &proto.ReadResponse{
		Error:      errorToString(err),
		NotFound:   errors.Is(err, ErrNotFound),
		EntityData: readData,
	}, nil
}

func (s *GRPCServer) Update(ctx context.Context, req *proto.UpdateRequest) (*proto.UpdateResponse, error) {
	ctx, err := s.withLogger(ctx)
	if err != nil {
		return nil, err
	}

	// Get the registered type to access its adapter
	rt := s.getRegisteredType(req.EntityType, req.EntitySubType)
	if rt == nil {
		return &proto.UpdateResponse{Error: "no registered type found for " + req.EntityType + "." + req.EntitySubType}, nil
	}

	// Call the adapter's Update method which returns mutated data
	updatedData, err := rt.Adapter.Update(ctx, req.EntityData)
	return &proto.UpdateResponse{
		Error:             errorToString(err),
		UpdatedEntityData: updatedData,
	}, nil
}

func (s *GRPCServer) Changed(ctx context.Context, req *proto.ChangedRequest) (*proto.ChangedResponse, error) {
	ctx, err := s.withLogger(ctx)
	if err != nil {
		return nil, err
	}

	changed, err := s.plugin.Changed(ctx, req.EntityType, req.EntitySubType, req.OldEntityData, req.NewEntityData)
	return &proto.ChangedResponse{
		Changed: changed,
		Error:   errorToString(err),
	}, nil
}

// withLogger returns ctx carrying the logger that sends the provider's log
// messages back to the host, see Logger. Each message carries the ID of the
// call the host sent in ctx, so the host gives it the call's resource and
// step.
func (s *GRPCServer) withLogger(ctx context.Context) (context.Context, error) {
	client, err := s.hostClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get logger: %w", err)
	}

	return WithLogger(ctx, &GRPCLogger{client: client, callID: callIDFromContext(ctx)}), nil
}

// pluginLogger returns the plugin scoped logger, for messages the plugin
// writes outside a provider call. It connects to the host when a message is
// first written, which may be before the host has connected, so it never
// waits for the host.
func (s *GRPCServer) pluginLogger() logger.Logger {
	return &asyncLogger{client: s.hostClient}
}

// hostClient returns the client of the host callback service, connecting to
// it on first use
func (s *GRPCServer) hostClient() (proto.HostCallbackServiceClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.callbackClient != nil {
		return s.callbackClient, nil
	}

	hostConn, err := s.broker.Dial(HostCallbackServiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to host callback service: %w", err)
	}

	s.callbackClient = proto.NewHostCallbackServiceClient(hostConn)
	return s.callbackClient, nil
}

// Helper function to convert error to string
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
