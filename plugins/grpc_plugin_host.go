package plugins

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-plugin"
	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins/proto"
	"google.golang.org/grpc/metadata"
)

// GRPCPluginHost manages external plugin processes via gRPC and provides host services
type GRPCPluginHost struct {
	emit        events.Emit
	state       State
	client      *plugin.Client
	plugin      PluginEntityProvider
	cachedTypes []RegisteredType // cached types with adapters
	typesCached bool             // flag to track if types have been cached

	// path is the plugin binary Start was given, Restart starts it again
	path string

	// calls maps the ID of each provider call in progress to its logger,
	// so the plugin's log messages reach the right resource and receiver
	calls *callLoggers
}

// NewGRPCPluginHost creates a new gRPC plugin host for external plugin
// binaries. The plugin's log messages, and go-plugin's own, are emitted to
// emit naming the plugin binary as their Source. A nil emit is silent.
func NewGRPCPluginHost(emit events.Emit, state State) *GRPCPluginHost {
	return &GRPCPluginHost{
		emit:  emit,
		state: state,
		calls: newCallLoggers(),
	}
}

// Start initializes and starts the external plugin process. The plugin's logs
// are emitted as events naming the binary's file name as their Source, so
// they can be told apart from the host's. A message written during a provider
// call carries that call's resource and step, one written outside a call,
// and go-plugin's own messages, go to emit.
func (h *GRPCPluginHost) Start(pluginPath string) error {
	h.path = pluginPath

	return h.connect()
}

// Restart starts the plugin process again when it is not running, after Stop
// or because it exited, and does nothing while it runs. The types the plugin
// provides and their adapters are kept, the adapters reach the new process.
// It must not be called while a provider call is in progress.
func (h *GRPCPluginHost) Restart() error {
	if h.Running() {
		return nil
	}

	return h.connect()
}

// Running reports whether the plugin process is running
func (h *GRPCPluginHost) Running() bool {
	return h.client != nil && !h.client.Exited()
}

// Path returns the path of the plugin binary the host starts
func (h *GRPCPluginHost) Path() string {
	return h.path
}

// connect starts the plugin binary at h.path and connects to it. A host that
// was connected before keeps its wrapper, which the cached adapters hold, and
// points it at the new process.
func (h *GRPCPluginHost) connect() error {
	pluginPath := h.path
	name := PluginBinaryName(pluginPath)
	pluginLogger := logger.New(h.emit, events.Event{Source: name, Operation: events.OperationLoad})

	var PluginMap = map[string]plugin.Plugin{
		pluginName: &GRPCPlugin{logger: pluginLogger, calls: h.calls},
	}

	// Create the plugin client
	h.client = plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  HandshakeConfig,
		Plugins:          PluginMap,
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		// go-plugin logs how it starts and talks to the plugin process, and
		// passes on what the process writes to stderr. Emit it all as events
		// sourced to the plugin, rather than through go-plugin's default
		// logger which writes straight to stderr.
		Logger: newHCLogAdapter(pluginLogger),
	})

	// Connect to the plugin
	rpcClient, err := h.client.Client()
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}

	// Get the plugin interface
	raw, err := rpcClient.Dispense(pluginName)
	if err != nil {
		return fmt.Errorf("failed to dispense plugin: %w", err)
	}

	// Cast to gRPC client and wrap it
	grpcClient := raw.(proto.PluginServiceClient)

	if wrapper, ok := h.plugin.(*grpcPluginWrapper); ok {
		wrapper.client = grpcClient
		return nil
	}

	h.plugin = &grpcPluginWrapper{client: grpcClient, name: name, calls: h.calls}

	return nil
}

// PluginBinaryName returns the name an external plugin is known by, the file
// name of its binary without a Windows .exe extension, i.e. xcl-plugin-person
// for /plugins/xcl-plugin-person. It is the Source of the plugin's log events.
func PluginBinaryName(pluginPath string) string {
	return strings.TrimSuffix(filepath.Base(pluginPath), ".exe")
}

// Stop stops the plugin process, Restart starts it again
func (h *GRPCPluginHost) Stop() {
	if h.client != nil {
		h.client.Kill()
	}
}

// grpcPluginWrapper wraps a gRPC client to implement PluginEntityProvider
type grpcPluginWrapper struct {
	client proto.PluginServiceClient

	// name is the plugin's name, the Source of its log messages
	name string

	// calls records the logger of each call in progress
	calls *callLoggers
}

// callContext registers the logger in ctx, sourced to the plugin, under a new
// call ID and returns ctx carrying the ID as outgoing gRPC metadata, with a
// function to call once the call returns
func (w *grpcPluginWrapper) callContext(ctx context.Context) (context.Context, func()) {
	if w.calls == nil {
		return ctx, func() {}
	}

	id, done := w.calls.register(logger.WithSource(Logger(ctx), w.name))
	return metadata.AppendToOutgoingContext(ctx, callIDKey, id), done
}

func (w *grpcPluginWrapper) GetTypes() []RegisteredType {
	resp, err := w.client.GetTypes(context.Background(), &proto.GetTypesRequest{})
	if err != nil {
		return nil
	}

	types := make([]RegisteredType, len(resp.Types))
	for i, t := range resp.Types {
		types[i] = RegisteredType{
			Type:    t.Type,
			SubType: t.SubType,
			Schema:  t.Schema,
			// Note: Adapter is set to nil for remote plugins as it's handled by the gRPC wrapper
		}
	}

	return types
}

func (w *grpcPluginWrapper) Validate(ctx context.Context, entityType, entitySubType string, entityData []byte) error {
	ctx, done := w.callContext(ctx)
	defer done()

	resp, err := w.client.Validate(ctx, &proto.ValidateRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return errors.New(resp.Error)
	}

	return nil
}

func (w *grpcPluginWrapper) Create(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	ctx, done := w.callContext(ctx)
	defer done()

	resp, err := w.client.Create(ctx, &proto.CreateRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	return resp.MutatedEntityData, nil
}

func (w *grpcPluginWrapper) Destroy(ctx context.Context, entityType, entitySubType string, entityData []byte) error {
	ctx, done := w.callContext(ctx)
	defer done()

	resp, err := w.client.Destroy(ctx, &proto.DestroyRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return errors.New(resp.Error)
	}

	return nil
}

func (w *grpcPluginWrapper) Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error) {
	ctx, done := w.callContext(ctx)
	defer done()

	resp, err := w.client.Read(ctx, &proto.ReadRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		OldEntityData: oldEntityData,
		NewEntityData: newEntityData,
	})
	if err != nil {
		return nil, err
	}

	// the not found signal is carried in its own field, restore the sentinel
	// so that callers can check it with errors.Is
	if resp.NotFound {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, resp.Error)
	}

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	return resp.EntityData, nil
}

func (w *grpcPluginWrapper) Update(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	ctx, done := w.callContext(ctx)
	defer done()

	resp, err := w.client.Update(ctx, &proto.UpdateRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	return resp.UpdatedEntityData, nil
}

func (w *grpcPluginWrapper) Changed(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte, dependencies []entity.DependencyChange) (entity.Change, error) {
	ctx, done := w.callContext(ctx)
	defer done()

	protoDependencies, err := toProtoDependencies(dependencies)
	if err != nil {
		return entity.NoChange, err
	}

	resp, err := w.client.Changed(ctx, &proto.ChangedRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		OldEntityData: oldEntityData,
		NewEntityData: newEntityData,
		Dependencies:  protoDependencies,
	})
	if err != nil {
		return entity.NoChange, err
	}

	if resp.Error != "" {
		return entity.NoChange, errors.New(resp.Error)
	}

	return fromProtoChange(resp.Change)
}

// Ensure GRPCPluginHost implements PluginHost interface
var _ PluginHost = (*GRPCPluginHost)(nil)

// GetTypes returns the types handled by the plugin, creating and caching adapters on first call
func (h *GRPCPluginHost) GetTypes() []RegisteredType {
	if h.plugin == nil {
		return nil
	}

	// Return cached types if already created
	if h.typesCached {
		return h.cachedTypes
	}

	// Get types from remote plugin (this calls grpcPluginWrapper.GetTypes())
	remoteTypes := h.plugin.GetTypes()
	if remoteTypes == nil {
		return nil
	}

	// Create resource-specific adapters for each type
	h.cachedTypes = make([]RegisteredType, len(remoteTypes))
	wrapper := h.plugin.(*grpcPluginWrapper) // cast to access wrapper

	for i, t := range remoteTypes {
		// Create a resource-specific adapter
		adapter := NewGRPCResourceProviderAdapter(wrapper, t.Type, t.SubType)

		h.cachedTypes[i] = RegisteredType{
			Type:    t.Type,
			SubType: t.SubType,
			Schema:  t.Schema,
			Adapter: adapter,
		}
	}

	h.typesCached = true
	return h.cachedTypes
}

// Validate validates the given entity data
func (h *GRPCPluginHost) Validate(ctx context.Context, entityType, entitySubType string, entityData []byte) error {
	if h.plugin == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Validate(ctx, entityType, entitySubType, entityData)
}

// Create creates a new entity
func (h *GRPCPluginHost) Create(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	if h.plugin == nil {
		return nil, fmt.Errorf("plugin not initialized")
	}
	return h.plugin.(*grpcPluginWrapper).Create(ctx, entityType, entitySubType, entityData)
}

// Destroy deletes an existing entity
func (h *GRPCPluginHost) Destroy(ctx context.Context, entityType, entitySubType string, entityData []byte) error {
	if h.plugin == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Destroy(ctx, entityType, entitySubType, entityData)
}

// Read reports the real entity, given its saved and configured copies
func (h *GRPCPluginHost) Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error) {
	if h.plugin == nil {
		return nil, fmt.Errorf("plugin not initialized")
	}
	return h.plugin.(*grpcPluginWrapper).Read(ctx, entityType, entitySubType, oldEntityData, newEntityData)
}

// Update updates an existing entity
func (h *GRPCPluginHost) Update(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	if h.plugin == nil {
		return nil, fmt.Errorf("plugin not initialized")
	}
	return h.plugin.(*grpcPluginWrapper).Update(ctx, entityType, entitySubType, entityData)
}

// Changed decides what applying new needs for the entity saved as old,
// given the dependencies the same apply will update or replace
func (h *GRPCPluginHost) Changed(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte, dependencies []entity.DependencyChange) (entity.Change, error) {
	if h.plugin == nil {
		return entity.NoChange, fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Changed(ctx, entityType, entitySubType, oldEntityData, newEntityData, dependencies)
}
