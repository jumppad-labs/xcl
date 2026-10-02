package plugins

import (
	"context"

	"github.com/jumppad-labs/xcl/logger"
)

// GRPCResourceProviderAdapter implements ProviderAdapter for a specific resource type
// by forwarding calls to the grpcPluginWrapper with the resource type information
type GRPCResourceProviderAdapter struct {
	wrapper         *grpcPluginWrapper
	resourceType    string
	resourceSubType string
}

// NewGRPCResourceProviderAdapter creates a new resource-specific provider adapter
func NewGRPCResourceProviderAdapter(wrapper *grpcPluginWrapper, resourceType, resourceSubType string) *GRPCResourceProviderAdapter {
	return &GRPCResourceProviderAdapter{
		wrapper:         wrapper,
		resourceType:    resourceType,
		resourceSubType: resourceSubType,
	}
}

func (a *GRPCResourceProviderAdapter) Init(state State, functions ProviderFunctions, logger logger.Logger) error {
	// GRPC adapters don't need initialization as they forward to remote plugins
	// The remote plugin handles its own initialization
	return nil
}

func (a *GRPCResourceProviderAdapter) Validate(ctx context.Context, entityData []byte) error {
	return a.wrapper.Validate(ctx, a.resourceType, a.resourceSubType, entityData)
}

func (a *GRPCResourceProviderAdapter) Create(ctx context.Context, entityData []byte) ([]byte, error) {
	return a.wrapper.Create(ctx, a.resourceType, a.resourceSubType, entityData)
}

func (a *GRPCResourceProviderAdapter) Destroy(ctx context.Context, entityData []byte, force bool) error {
	return a.wrapper.Destroy(ctx, a.resourceType, a.resourceSubType, entityData)
}

func (a *GRPCResourceProviderAdapter) Read(ctx context.Context, oldEntityData []byte, newEntityData []byte) ([]byte, error) {
	return a.wrapper.Read(ctx, a.resourceType, a.resourceSubType, oldEntityData, newEntityData)
}

func (a *GRPCResourceProviderAdapter) Update(ctx context.Context, entityData []byte) ([]byte, error) {
	return a.wrapper.Update(ctx, a.resourceType, a.resourceSubType, entityData)
}

func (a *GRPCResourceProviderAdapter) Changed(ctx context.Context, oldEntityData []byte, newEntityData []byte) (bool, error) {
	return a.wrapper.Changed(ctx, a.resourceType, a.resourceSubType, oldEntityData, newEntityData)
}

// Ensure GRPCResourceProviderAdapter implements ProviderAdapter
var _ ProviderAdapter = (*GRPCResourceProviderAdapter)(nil)
