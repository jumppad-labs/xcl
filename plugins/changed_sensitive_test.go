package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/types"
)

// sensitiveResource is a resource with a sensitive field, as a provider
// resource that holds a credential would be.
type sensitiveResource struct {
	types.ResourceBase `xcl:",remain"`

	Name     string                  `xcl:"name" json:"name"`
	Password types.Sensitive[string] `xcl:"password" json:"password"`
}

func TestDefaultChangedReportsChangeWhenOnlyASensitiveValueDiffers(t *testing.T) {
	old := &sensitiveResource{Name: "web", Password: types.NewSensitive("first")}
	new := &sensitiveResource{Name: "web", Password: types.NewSensitive("second")}

	change, err := DefaultChanged[*sensitiveResource]{}.Changed(context.Background(), old, new, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestDefaultChangedReportsNoChangeWhenSensitiveValuesAreEqual(t *testing.T) {
	old := &sensitiveResource{Name: "web", Password: types.NewSensitive("same")}
	new := &sensitiveResource{Name: "web", Password: types.NewSensitive("same")}

	change, err := DefaultChanged[*sensitiveResource]{}.Changed(context.Background(), old, new, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}

// The adapter hands the provider the real value and returns what the provider
// produced with the real value intact, it never writes the marker.
func TestTypedProviderAdapterCreateReturnsProviderResultWithRealSensitiveValue(t *testing.T) {
	provider := &sensitiveEchoProvider{}
	adapter := NewTypedProviderAdapter[*sensitiveResource](provider, &sensitiveResource{})

	data, err := adapter.Create(context.Background(), []byte(`{"name":"web","password":"real-secret"}`))
	require.NoError(t, err)

	require.Equal(t, "real-secret", provider.created.Password.Reveal())
	require.Contains(t, string(data), `"password":"real-secret"`)
	require.NotContains(t, string(data), "(sensitive)")
}

func TestTypedProviderAdapterChangedDetectsOnlyASensitiveValueChange(t *testing.T) {
	provider := &sensitiveEchoProvider{}
	adapter := NewTypedProviderAdapter[*sensitiveResource](provider, &sensitiveResource{})

	change, err := adapter.Changed(
		context.Background(),
		[]byte(`{"name":"web","password":"first"}`),
		[]byte(`{"name":"web","password":"second"}`),
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

// sensitiveEchoProvider records what Create received and returns it.
type sensitiveEchoProvider struct {
	DefaultChanged[*sensitiveResource]

	created *sensitiveResource
}

func (p *sensitiveEchoProvider) Init(state State, functions ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *sensitiveEchoProvider) Create(ctx context.Context, resource *sensitiveResource) (*sensitiveResource, error) {
	p.created = resource

	return resource, nil
}

func (p *sensitiveEchoProvider) Destroy(ctx context.Context, resource *sensitiveResource, force bool) error {
	return nil
}

func (p *sensitiveEchoProvider) Read(ctx context.Context, old *sensitiveResource, new *sensitiveResource) (*sensitiveResource, error) {
	return new, nil
}

func (p *sensitiveEchoProvider) Update(ctx context.Context, resource *sensitiveResource) (*sensitiveResource, error) {
	return resource, nil
}

func (p *sensitiveEchoProvider) Functions() ProviderFunctions {
	return nil
}
