// Command externalplugin is the end-to-end suite's external plugin. It is
// compiled to its own binary, which xcl starts as a separate process and
// talks to over gRPC. It provides the app and ingress block types of
// testdata/plugin. The suite's TestMain builds it once per run.
package main

import (
	"context"
	"fmt"

	"github.com/jumppad-labs/xcl/e2e/fixtures/services"
	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// ExternalPlugin provides the app and ingress block types
type ExternalPlugin struct {
	plugins.PluginBase
}

// Ensure ExternalPlugin implements the Plugin interface
var _ plugins.Plugin = (*ExternalPlugin)(nil)

// Init registers the block types the plugin provides, with their providers.
// An external plugin registers several block types exactly as an in-process
// one does, calling RegisterResourceProvider once per type.
//
// logger is plugin scoped, for messages written outside a provider call. It
// sends to the host once the host has connected. During a call the providers
// log through plugins.Logger(ctx), which the host binds to the resource and
// step being worked on.
func (p *ExternalPlugin) Init(logger logger.Logger, state plugins.State) error {
	err := plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"app",
		&services.App{},
		&appProvider{},
	)
	if err != nil {
		return err
	}

	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",
		"ingress",
		&services.Ingress{},
		&ingressProvider{},
	)
}

// appProvider handles the lifecycle of app blocks, the only thing it creates
// is the computed url, which the ingress block reads
//
// Change detection comes from the embedded DefaultChanged.
type appProvider struct {
	plugins.DefaultChanged[*services.App]
}

var _ plugins.ResourceProvider[*services.App] = (*appProvider)(nil)

// Init is called once, when the plugin process starts. The provider logs
// during a call through plugins.Logger(ctx), which the host binds to the
// resource and step, exactly as an in-process provider does.
func (p *appProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

// Create receives the app with every reference resolved, including the
// connection strings the in-process postgres and redis providers filled in,
// and sets the computed url that the ingress block reads
func (p *appProvider) Create(ctx context.Context, app *services.App) (*services.App, error) {
	app.URL = appURL(app)
	plugins.Logger(ctx).Info("created app",
		"connection_string", app.ConnectionString, "cache_connection_string", app.CacheConnectionString, "url", app.URL)

	return app, nil
}

func (p *appProvider) Read(ctx context.Context, old *services.App, new *services.App) (*services.App, error) {
	plugins.Logger(ctx).Info("read app")

	return new, nil
}

func (p *appProvider) Update(ctx context.Context, app *services.App) (*services.App, error) {
	app.URL = appURL(app)
	plugins.Logger(ctx).Info("updated app", "url", app.URL)

	return app, nil
}

func (p *appProvider) Destroy(ctx context.Context, app *services.App, force bool) error {
	plugins.Logger(ctx).Info("destroyed app", "force", force)

	return nil
}

func (p *appProvider) Functions() plugins.ProviderFunctions {
	return nil
}

func appURL(app *services.App) string {
	return fmt.Sprintf("http://%s", app.Meta.Name)
}

// ingressProvider handles the lifecycle of ingress blocks, the second block
// type this plugin provides. It is a separate provider, registered in Init
// alongside the app one.
//
// Change detection comes from the embedded DefaultChanged, except that a
// changed hostname answers replace, see Changed.
type ingressProvider struct {
	plugins.DefaultChanged[*services.Ingress]
}

var _ plugins.ResourceProvider[*services.Ingress] = (*ingressProvider)(nil)

// Init is called once, when the plugin process starts, see appProvider.Init
func (p *ingressProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
	return nil
}

// Changed answers replace when the hostname changes, the route is identified by its hostname, so the
// e2e scenarios have a provider-decided replacement. Every other change is
// left to DefaultChanged.
func (p *ingressProvider) Changed(ctx context.Context, old *services.Ingress, new *services.Ingress, dependencies []entity.DependencyChange) (entity.Change, error) {
	if old.Hostname != new.Hostname {
		return entity.Replace, nil
	}

	return p.DefaultChanged.Changed(ctx, old, new, dependencies)
}

// Create receives the ingress with the computed url of the app it routes to,
// filled in by the app provider in this same plugin
func (p *ingressProvider) Create(ctx context.Context, ingress *services.Ingress) (*services.Ingress, error) {
	plugins.Logger(ctx).Info("created ingress", "hostname", ingress.Hostname, "app_url", ingress.AppURL)

	return ingress, nil
}

func (p *ingressProvider) Read(ctx context.Context, old *services.Ingress, new *services.Ingress) (*services.Ingress, error) {
	plugins.Logger(ctx).Info("read ingress")

	return new, nil
}

func (p *ingressProvider) Update(ctx context.Context, ingress *services.Ingress) (*services.Ingress, error) {
	plugins.Logger(ctx).Info("updated ingress")

	return ingress, nil
}

func (p *ingressProvider) Destroy(ctx context.Context, ingress *services.Ingress, force bool) error {
	plugins.Logger(ctx).Info("destroyed ingress", "force", force)

	return nil
}

func (p *ingressProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// main serves the plugin to the host that started this process
func main() {
	plugins.Serve(&ExternalPlugin{})
}
