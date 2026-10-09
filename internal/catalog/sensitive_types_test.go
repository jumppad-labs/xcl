package catalog

import (
	"context"
	"testing"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// Vault is a plugin resource type with a supported sensitive field
type Vault struct {
	types.ResourceBase `xcl:",remain"`

	Token types.Sensitive[string] `xcl:"token" json:"token"`
}

// BadVault is a plugin resource type whose sensitive field uses an
// instantiation the host cannot rebuild
type BadVault struct {
	types.ResourceBase `xcl:",remain"`

	Secret types.Sensitive[uint8] `xcl:"secret" json:"secret"`
}

type vaultProvider struct {
	plugins.DefaultChanged[*Vault]
}

func (p *vaultProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

func (p *vaultProvider) Create(ctx context.Context, resource *Vault) (*Vault, error) {
	return resource, nil
}

func (p *vaultProvider) Destroy(ctx context.Context, resource *Vault, force bool) error {
	return nil
}

func (p *vaultProvider) Read(ctx context.Context, old *Vault, new *Vault) (*Vault, error) {
	return new, nil
}

func (p *vaultProvider) Update(ctx context.Context, resource *Vault, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Vault, error) {
	return resource, nil
}

func (p *vaultProvider) Functions() plugins.ProviderFunctions {
	return nil
}

type badVaultProvider struct {
	plugins.DefaultChanged[*BadVault]
}

func (p *badVaultProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

func (p *badVaultProvider) Create(ctx context.Context, resource *BadVault) (*BadVault, error) {
	return resource, nil
}

func (p *badVaultProvider) Destroy(ctx context.Context, resource *BadVault, force bool) error {
	return nil
}

func (p *badVaultProvider) Read(ctx context.Context, old *BadVault, new *BadVault) (*BadVault, error) {
	return new, nil
}

func (p *badVaultProvider) Update(ctx context.Context, resource *BadVault, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*BadVault, error) {
	return resource, nil
}

func (p *badVaultProvider) Functions() plugins.ProviderFunctions {
	return nil
}

type vaultPlugin struct {
	plugins.PluginBase
}

func (p *vaultPlugin) Init(logger logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "vault", &Vault{}, &vaultProvider{})
}

type badVaultPlugin struct {
	plugins.PluginBase
}

func (p *badVaultPlugin) Init(logger logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "resource", "badvault", &BadVault{}, &badVaultProvider{})
}

func TestLoadAcceptsPluginTypeWithSupportedSensitiveField(t *testing.T) {
	c := New()

	c.AddRegistry(localWith(&vaultPlugin{}))

	err := c.Load(nil)
	require.NoError(t, err)

	require.Len(t, c.GetPluginHosts(), 1)
}

func TestLoadRejectsPluginTypeWithUnsupportedSensitiveField(t *testing.T) {
	c := New()

	c.AddRegistry(localWith(&badVaultPlugin{}))

	err := c.Load(nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "resource.badvault")
	require.ErrorContains(t, err, "Secret")
	require.ErrorContains(t, err, "unsupported sensitive type")

	require.Empty(t, c.GetPluginHosts())
}
