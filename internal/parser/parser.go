package parser

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/errwrap"
	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/cty/function"
	"github.com/jumppad-labs/xcl/internal/dag"
	"github.com/jumppad-labs/xcl/internal/functions"
	"github.com/jumppad-labs/xcl/internal/modules"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclparse"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

type ResourceTypeNotExistError struct {
	Type string
	File string
}

func (r ResourceTypeNotExistError) Error() string {
	return fmt.Sprintf("Resource type %s defined in file %s, is not a registered resource.", r.Type, r.File)
}

// parsed holds resources and their HCL bodies during the parsing phase.
// This is an internal working structure used only within the Parser.
// After parsing completes, resources are transferred to State (without bodies).
type parsed struct {
	resources map[string]any             // FQRN -> resource instance
	bodies    map[string]*hclsyntax.Body // FQRN -> HCL body for later decoding

	// moduleSources records the module source directories already entered,
	// resolved to an absolute path with symlinks evaluated. A module whose
	// source directly or indirectly includes itself would otherwise recurse
	// until the stack is exhausted, which is a crash rather than a reportable
	// problem.
	moduleSources map[string]bool

	// order holds each FQRN in the order it was first parsed. The maps above
	// iterate in no fixed order, so this is what keeps the entities a parse
	// produces in declaration order.
	order []string
}

// store records a parsed resource and its body, keeping the order in which
// resources were first declared.
func (p *parsed) store(id string, body *hclsyntax.Body, resource any) {
	if _, seen := p.resources[id]; !seen {
		p.order = append(p.order, id)
	}

	p.bodies[id] = body
	p.resources[id] = resource
}

type ParserOptions struct {
	// list of default variable values to add to the parser
	Variables map[string]string
	// list of variable files to be read by the parser
	VariablesFiles []string
	// environment variable prefix
	VariableEnvPrefix string
	// location of any downloaded modules
	ModuleCache string
	// default registry to use when fetching modules
	DefaultRegistry string
	// credentials to use with the registries
	RegistryCredentials map[string]string

	// ModuleRegistry is the registry of modules to use for this parser
	// when downloading modules.
	ModuleRegistry *modules.ModuleRegistry

	// PluginRegistry is the registry of plugins to use for this parser.
	// This should be provided by Config when using the Config.Apply/Validate API.
	// For standalone Parser usage, create and configure the PluginRegistry yourself.
	PluginRegistry *registry.PluginRegistry

	// ProviderResolver overrides how provider adapters are looked up during the resource
	// lifecycle walk (Create/Read/Changed/Update/Destroy). Defaults to PluginRegistry.
	// Primarily useful for testing lifecycle/ordering behavior without a real plugin registry.
	ProviderResolver ProviderResolver

	// TypeRegistry overrides how the lifecycle walk recognises plain Go types registered
	// without a plugin, which like builtins are never passed to a provider. Defaults to
	// PluginRegistry. Primarily useful for testing lifecycle behavior without a real registry.
	TypeRegistry TypeRegistry

	// StateStore is the state store to use for loading previous state.
	// and saving new state.
	StateStore state.StateStore

	// StateMask masks every sensitive value saved to the StateStore, and
	// opens them again when state is loaded. Nil saves real values in plain
	// text, and loading state holding a masked value then fails.
	StateMask mask.Masker

	// EventMask masks every sensitive value in the resource data events
	// carry. Nil carries real values. Config always sets it, to mask.Redact()
	// unless event masking was turned off.
	EventMask mask.Masker

	// EventData says what resource data lifecycle events carry. The zero
	// value carries none, so nothing is serialized for an event unless the
	// configuration asked for it.
	EventData events.DataLevel

	// Emit receives every event the parser produces: parse, lifecycle and
	// validation events, their errors, and log messages such as the warning
	// for a configured value a provider changed. It may be called from
	// several goroutines at once. A nil Emit is silent.
	Emit events.Emit

	CustomFunctions map[string]function.Function
}

// ConfigDirectory is the name of the directory, created in the users home
// folder, that holds XCL's configuration and caches
const ConfigDirectory = ".xclconfig"

// DefaultOptions returns a ParserOptions object with the
// ModuleCache set to the default directory of $HOME/.xclconfig/cache
// if the $HOME folder can not be determined, the cache is set to the
// current folder
// VariableEnvPrefix is set to 'HCL_VAR_', should a variable be defined
// called 'foo' setting the environment variable 'HCL_VAR_foo' will override
// any default value
// PluginRegistry is set to a registry containing only the builtin resource
// types, add plugins to it or replace it to use custom resource types
func DefaultOptions() *ParserOptions {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	cacheDir := filepath.Join(homeDir, ConfigDirectory, "cache")

	return &ParserOptions{
		ModuleCache:       cacheDir,
		VariableEnvPrefix: "HCL_VAR_",
		PluginRegistry:    registry.NewPluginRegistry(),
		// event data is redacted by default, as Config does
		EventMask: mask.Redact(),
	}
}

// Parser can parse HCL configuration files
type Parser struct {
	options          ParserOptions
	customFunctions  map[string]function.Function
	stateStore       state.StateStore
	pluginRegistry   *registry.PluginRegistry
	providerResolver ProviderResolver
	typeRegistry     TypeRegistry
	addresses        *resources.AddressParser // resolves addresses against the known types
	parsedResources  *parsed                  // Working storage during parsing
}

// addressParser returns a parser that resolves addresses against the types the
// registry knows. A module relative address cannot be split without them:
// in module.a.b.c, "b" is a module name unless something is registered under
// it. The set does not change during a parse, so it is built once.
func (p *Parser) addressParser() *resources.AddressParser {
	if p.addresses == nil {
		var known []types.TypeInfo
		if p.pluginRegistry != nil {
			known = p.pluginRegistry.Types()
		}

		p.addresses = resources.NewAddressParser(known)
	}

	return p.addresses
}

// NewParser creates a new parser with the given options
// if options are nil, default options are used
func NewParser(options *ParserOptions) *Parser {
	o := options
	if o == nil {
		o = DefaultOptions()
	}

	p := &Parser{
		options:         *o,
		customFunctions: map[string]function.Function{},
	}

	// Parser should never create plugin registry or state store - these are owned by Config
	// and passed in via options. If not provided, they will be nil and operations will skip
	// plugin/state functionality.
	p.pluginRegistry = o.PluginRegistry
	p.stateStore = o.StateStore

	p.providerResolver = o.ProviderResolver
	if p.providerResolver == nil {
		p.providerResolver = p.pluginRegistry
	}

	p.typeRegistry = o.TypeRegistry
	if p.typeRegistry == nil && p.pluginRegistry != nil {
		p.typeRegistry = p.pluginRegistry
	}

	if o.CustomFunctions != nil {
		p.customFunctions = o.CustomFunctions
	}

	return p
}

// Apply parses HCL configuration from multiple paths, calls the provider
// lifecycle for every resource and returns State with decoded resources.
// This is the main entry point for applying configuration.
//
// Parameters:
//   - paths: one or more file or directory paths to parse
//
// The parsing process:
//  1. Load previous state from StateStore (if exists)
//  2. Parse all HCL files from the given paths
//  3. Reject a configuration that declares no blocks with ErrEmptyConfiguration
//  4. Destroy the resources in previousState that are no longer in the
//     configuration, children first, saving the state after each one
//  5. Build a DAG based on resource dependencies
//  6. Walk the DAG in dependency order
//  7. Decode each resource body (HCL → Go structs)
//  8. Call the provider lifecycle: resources not in previousState are
//     created, resources in it are read, then updated if they changed
//
// Returns the new State containing all parsed resources.
//
// When destroying a removed resource fails, nothing is created or changed:
// Apply returns the previous state minus the resources that were destroyed,
// with the failed ones marked destroy_failed, together with the error. The
// next Apply retries them first.
//
// When a provider call fails the walk stops processing the resources that
// depend on the failed one, and Apply returns a partial State together with
// the error. The partial State holds the resources that were reached, the
// failed resource with status "failed", and the previous entry of resources
// that existed before but were not reached. Parse and validation failures
// return a nil State.
//
// ctx is the operation's context. Once it is cancelled no new provider call
// starts, calls already running are left to finish, and Apply returns the
// partial State of what was reached together with ctx's error. Providers are
// given a copy of ctx that is never cancelled.
func (p *Parser) Apply(ctx context.Context, paths ...string) (*State, error) {
	currentState, previousState, err := p.parseAndValidate(paths...)
	if err != nil {
		return nil, err
	}

	// an empty configuration would remove everything, Destroy does that
	if currentState.ResourceCount() == 0 {
		return nil, ErrEmptyConfiguration
	}

	ce := errors.NewConfigError()

	// destroy the resources that are no longer in the configuration before
	// anything is created or changed, freeing what they held for their
	// replacements
	removed := removedResources(currentState, previousState)
	if len(removed) > 0 {
		working := NewState()
		for _, r := range previousState.GetResources() {
			if err := working.AppendResource(r); err != nil {
				ce.AppendError(err)
				return nil, ce
			}
		}

		d := &destroyer{
			ctx:       ctx,
			working:   working,
			store:     p.stateStore,
			resolver:  p.providerResolver,
			types:     p.typeRegistry,
			options:   &p.options,
			addresses: p.addressParser(),
		}

		// a failed removal stops the apply, the working state holds the
		// previous state minus what was destroyed, with the failures marked
		// destroy_failed so the next apply retries them first
		if err := d.destroy(removed); err != nil {
			ce.AppendError(err)
			return working, ce
		}

		previousState = working
	}

	// Get functions for HCL context
	functions := p.getFunctions

	// Always walk the DAG to decode resources (fills in their fields from HCL)
	// This decodes interpolations and resolves dependencies regardless of plugin execution
	progress, errs := p.walk(ctx, currentState, previousState, functions)

	// a cancelled walk skipped the resources it had not reached, keep only
	// the progress it made
	if len(errs) == 0 && ctx.Err() != nil {
		errs = append(errs, fmt.Errorf("apply stopped before every resource was reached: %w", ctx.Err()))
	}

	if len(errs) > 0 {
		for _, e := range errs {
			ce.AppendError(e)
		}

		// keep the progress the walk made so that the next apply picks up
		// where this one stopped
		if progress == nil {
			return nil, ce
		}

		partial, err := progress.buildState(currentState, previousState)
		if err != nil {
			ce.AppendError(err)
			return nil, ce
		}

		return partial, ce
	}

	return currentState, nil
}

// Destroy destroys every resource in saved, working only from the saved state:
// it needs no configuration. Resources are destroyed children first, in the
// reverse of their create order, built from the links each resource saved. Builtin, registered and disabled resources never reach
// a provider.
//
// The state is saved through the configured StateStore after every resource,
// so an interrupted destroy resumes from where it stopped. A resource whose
// destroy fails stays in the state as destroy_failed, together with every
// resource it depends on, while unrelated resources are still destroyed.
//
// Destroy returns what is left, which is empty when everything was destroyed,
// together with an error naming every resource that failed. The returned state
// is never nil.
// Destroy removes the entities given, children before their parents. It takes
// plain entities rather than a container, because that is what a state store
// hands back and what a caller holds.
//
// ctx is the operation's context. Once it is cancelled no new provider call
// starts and calls already running are left to finish. The resources not yet
// reached stay in the state and Destroy returns ctx's error.
func (p *Parser) Destroy(ctx context.Context, saved []any) (*State, error) {
	working := NewState()

	if err := p.loadPlugins(); err != nil {
		return working, err
	}

	// saved is what a store loaded, the records it holds are typed here
	saved, err := savedentity.DecodeAll(p.pluginRegistry, saved, savedentity.ReadOptions{Mask: p.options.StateMask})
	if err != nil {
		return working, err
	}

	for _, r := range saved {
		if err := working.AppendResource(r); err != nil {
			return working, err
		}
	}

	d := &destroyer{
		ctx:       ctx,
		working:   working,
		store:     p.stateStore,
		resolver:  p.providerResolver,
		types:     p.typeRegistry,
		options:   &p.options,
		addresses: p.addressParser(),
	}

	// copy the targets, destroying a resource removes it from the working
	// state's backing slice
	targets := append([]any{}, working.GetResources()...)

	err = d.destroy(targets)

	if d.unmaskedSensitive && emitting(&p.options) {
		logger.New(p.options.Emit, events.Event{
			Source:    events.SourceCore,
			Operation: events.OperationDestroy,
		}).Warn(PlaintextStateWarning)
	}

	return working, err
}

// removedResources returns the resources in previous that are no longer in
// current
func removedResources(current, previous *State) []any {
	if previous == nil {
		return nil
	}

	removed := []any{}
	for _, r := range previous.GetResources() {
		meta, err := types.GetMeta(r)
		if err != nil {
			continue
		}

		if _, err := findByID(current.GetResources(), meta.ID); err != nil {
			removed = append(removed, r)
		}
	}

	return removed
}

// Validate parses and validates the configuration discovered from paths without
// resolving it: no body is decoded, no dependency graph is walked and no provider
// is reached. A nil error means the configuration is valid.
//
// ctx is the operation's context, validation reaches no provider so it is
// accepted for symmetry with Apply and Destroy.
func (p *Parser) Validate(ctx context.Context, paths ...string) error {
	_, _, err := p.parseAndValidate(paths...)
	return err
}

// parseAndValidate reads every file discovered from paths, then validates the
// result as a whole before any of it is acted upon. It performs no resolution:
// no body is decoded, no dependency graph is walked and no provider is reached.
//
// It returns the state holding the parsed resources along with the previously
// stored state, or every problem found. A configuration that does not parse is
// never validated, and a configuration that does not validate is never returned,
// so a caller that receives no error holds a configuration worth acting on.
func (p *Parser) parseAndValidate(paths ...string) (*State, *State, error) {
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("at least one path is required")
	}

	// plugin types are needed to read both the saved state and the files
	if err := p.loadPlugins(); err != nil {
		return nil, nil, err
	}

	// Load previous state from store (for comparison during Create/Update)
	var previousState *State
	if p.stateStore != nil && p.stateStore.Exists() {
		// the store hands back plain entities; the container is this
		// package's own working shape and is built from them here
		saved, err := p.stateStore.Load()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load previous state: %w", err)
		}

		// the store only reads and writes records, typing them needs the
		// registry
		saved, err = savedentity.DecodeAll(p.pluginRegistry, saved, savedentity.ReadOptions{Mask: p.options.StateMask})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load previous state: %w", err)
		}

		previousState = NewState()
		for _, e := range saved {
			if appendErr := previousState.AppendResource(e); appendErr != nil {
				return nil, nil, fmt.Errorf("failed to load previous state: %w", appendErr)
			}
		}
	}
	if previousState == nil {
		previousState = NewState()
	}

	// Create new state for current parse
	currentState := NewState()

	// Initialize parsed resources container
	p.parsedResources = &parsed{
		resources:     map[string]any{},
		bodies:        map[string]*hclsyntax.Body{},
		moduleSources: map[string]bool{},
	}

	ce := errors.NewConfigError()

	// Auto-discover .vars files from the root directories of all paths
	// These have lower precedence than manually specified VariablesFiles
	discoveredVarsFiles, err := findVarsFiles(paths...)
	if err != nil {
		ce.AppendError(errors.NewParserError("", 0, 0,
			fmt.Sprintf("error finding .vars files: %s", err)))
		return nil, nil, ce
	}

	// Merge discovered vars files with manually specified ones
	// Discovered files come first (lower precedence), then manually specified (higher precedence)
	mergedVarsFiles := []string{}

	// Add discovered files only if not already in manually specified list
	for _, discovered := range discoveredVarsFiles {
		found := false
		for _, manual := range p.options.VariablesFiles {
			if discovered == manual {
				found = true
				break
			}
		}
		if !found {
			mergedVarsFiles = append(mergedVarsFiles, discovered)
		}
	}

	// Add all manually specified files (these have higher precedence)
	mergedVarsFiles = append(mergedVarsFiles, p.options.VariablesFiles...)

	// Update options with merged list
	p.options.VariablesFiles = mergedVarsFiles

	// Get all the xcl files from the paths
	files, err := findXclFiles(paths...)
	if err != nil {
		ce.AppendError(errors.NewParserError("", 0, 0,
			fmt.Sprintf("error finding .xcl files: %s", err)))
		return nil, nil, ce
	}

	// Parse all files
	for _, file := range files {
		errs := p.parseResourcesInFile(file, "")
		for _, e := range errs {
			ce.AppendError(e)
		}
	}

	if len(ce.Errors) > 0 {
		return nil, nil, ce
	}

	// Validate the configuration as a whole before any of it is acted upon.
	// Every resource from every file is parsed by this point and no body has
	// been decoded, which is what lets validation judge the configuration
	// without resolving it.
	for _, e := range p.validate() {
		p.emitValidateError(e)
		ce.AppendError(e)
	}

	if len(ce.Errors) > 0 {
		return nil, nil, ce
	}

	// Move parsed resources into currentState, in declaration order
	for _, id := range p.parsedResources.order {
		if err := currentState.AppendResource(p.parsedResources.resources[id]); err != nil {
			return nil, nil, fmt.Errorf("failed to add resource to state: %w", err)
		}
	}

	return currentState, previousState, nil
}

// parseResourcesInFile parses a hcl file and adds any found resources to the config
func (p *Parser) parseResourcesInFile(file string, module string) []error {
	parser := hclparse.NewParser()

	f, diag := parser.ParseXCLFile(file)
	if diag.HasErrors() {
		// a single malformed input can produce several diagnostics, report every
		// one of them rather than only the first
		errs := []error{}
		for _, d := range diag {
			pe := errors.NewParserErrorFromHCLDiag(d, file)
			errs = append(errs, pe)

			// the file is not valid syntax, so no problem is in a resource
			emitParse(&p.options, "", "", file, pe)
		}

		return errs
	}

	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		// this should never happen, body should always be a hclsyntax.Body
		panic("Error getting body")
	}

	// gather every problem in the file rather than returning at the first, so an
	// author sees all of them at once
	blockErrors := []error{}

	for _, b := range body.Blocks {

		// check the resource has a name
		if len(b.Labels) == 0 {
			de := errors.NewParserError(
				file,
				b.TypeRange.Start.Line,
				b.TypeRange.Start.Column,
				fmt.Sprintf("resource '%s' has no name, please specify resources using the syntax 'resource_type \"name\" {}'", b.Type),
			)

			emitParse(&p.options, "", "", file, de)
			blockErrors = append(blockErrors, de)
			continue
		}

		// create the registered type if not a variable or output
		// variables and outputs are processed in a separate run
		switch b.Type {
		case resources.TypeModule:
			// parseModule fires the module's own parse event, the resources in
			// its source fire theirs as they are parsed
			errs := p.parseModule(file, b, module, f.Bytes)
			blockErrors = append(blockErrors, errs...)
		case resources.TypeVariable:
			fallthrough
		case resources.TypeOutput:
			err := p.parseResource(file, b, module, f.Bytes)
			if err != nil {
				blockErrors = append(blockErrors, err)
			}

			resourceType, resourceID := p.blockResource(b, module)
			emitParse(&p.options, resourceType, resourceID, file, err)
		default:
			// not a builtin keyword, so it must be the type of an entity.
			// "resource" always is, any other keyword only once something
			// is registered under it
			if _, known := p.takesSubtype(b.Type); !known {
				de := errors.NewParserError(
					file,
					b.TypeRange.Start.Line,
					b.TypeRange.Start.Column,
					fmt.Sprintf("unable to process stanza '%s' in file %s at %d,%d , '%s' is not a known type", b.Type, file, b.Range().Start.Line, b.Range().Start.Column, b.Type),
				)

				emitParse(&p.options, "", "", file, de)
				blockErrors = append(blockErrors, de)

				continue
			}

			err := p.parseResource(file, b, module, f.Bytes)
			if err != nil {
				blockErrors = append(blockErrors, err)
			}

			resourceType, resourceID := p.blockResource(b, module)
			emitParse(&p.options, resourceType, resourceID, file, err)
		}
	}

	if len(blockErrors) > 0 {
		return blockErrors
	}

	return nil
}

// takesSubtype reports whether declarations of the type keyword entityType
// carry a subtype as their first label, and whether the keyword is known.
// "resource" always takes one, even with no registry
func (p *Parser) takesSubtype(entityType string) (bool, bool) {
	if p.pluginRegistry == nil {
		return entityType == types.TypeResource, entityType == types.TypeResource
	}

	return p.pluginRegistry.TakesSubtype(entityType)
}

// isReferenceRoot reports whether name, the first segment of a traversal in an
// expression, names something a reference can point at: one of the builtin
// keywords, "resource", or the type of any known entity
func (p *Parser) isReferenceRoot(name string) bool {
	switch name {
	case types.TypeResource, resources.TypeModule, resources.TypeVariable, resources.TypeOutput:
		return true
	}

	_, known := p.takesSubtype(name)
	return known
}

// blockResource returns the "<type>.<name>" and the ID of the entity a block
// declares in module, i.e. "postgres.main" and "resource.postgres.main". They
// are the same the parsed entity gets, and are worked out from the block's
// labels so a block that fails to parse can still be reported against its
// entity. They are empty when the labels do not name an entity.
func (p *Parser) blockResource(b *hclsyntax.Block, module string) (string, string) {
	fqrn := resources.FQRN{Module: module, Type: b.Type}

	takes, _ := p.takesSubtype(b.Type)

	switch {
	case takes && len(b.Labels) == 2:
		fqrn.Subtype = b.Labels[0]
		fqrn.Resource = b.Labels[1]
	case !takes && len(b.Labels) == 1:
		// covers the single label builtins and every type declared without
		// a subtype alike: the keyword is the type and there is no subtype
		fqrn.Resource = b.Labels[0]
	default:
		return "", ""
	}

	return fqrn.AddressType() + "." + fqrn.Resource, fqrn.String()
}

// src is the text of the file b was parsed from, it is where the text the
// author wrote for each reference is read from
func (p *Parser) parseResource(file string, b *hclsyntax.Block, moduleName string, src []byte) error {
	var rt any
	var err error

	switch b.Type {
	case resources.TypeOutput:
		// If the type is output check there is one label
		if len(b.Labels) != 1 {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = `invalid formatting for 'output' stanza, resources should have a name and a type, i.e. 'output "name" {}'`

			return de
		}

		name := b.Labels[0]
		if err := validateResourceName(name); err != nil {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = err.Error()

			return de
		}

		rt, err = p.createBuiltinResource(resources.TypeOutput, name)
		if err != nil {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`unable to create output, this error should never happen %s`, err)
			de.Cause = err

			return de
		}
	case resources.TypeVariable:
		// If the type is variable check there is one label
		if len(b.Labels) != 1 {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = `invalid formatting for 'variable' stanza, resources should have a name and a type, i.e. 'variable "name" {}'`

			return de
		}

		name := b.Labels[0]
		if err := validateResourceName(name); err != nil {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = err.Error()

			return de
		}

		rt, err = p.createBuiltinResource(resources.TypeVariable, name)
		if err != nil {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`unable to create variable, this error should never happen %s`, err)
			de.Cause = err

			return de
		}

	default:
		// an entity declared by its type keyword. A type that takes a
		// subtype carries it as the first label and the name as the second,
		// i.e. resource "container" "nics" or server "big" "web", and one
		// that does not carries only the name, i.e. cache "main"
		takes, _ := p.takesSubtype(b.Type)

		subtype := ""
		name := ""

		switch {
		case takes && len(b.Labels) == 2:
			subtype = b.Labels[0]
			name = b.Labels[1]
		case takes:
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`invalid format for '%s', it is declared with a subtype and a name, i.e. '%s "subtype" "name" {}'`, b.Type, b.Type)

			return de
		case len(b.Labels) == 1:
			name = b.Labels[0]
		default:
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = fmt.Sprintf(`invalid format for '%s', it is declared with only a name, i.e. '%s "name" {}'`, b.Type, b.Type)

			return de
		}

		if err := validateResourceName(name); err != nil {
			de := &errors.ParserError{}
			de.Line = b.TypeRange.Start.Line
			de.Column = b.TypeRange.Start.Column
			de.Filename = file
			de.Message = err.Error()

			return de
		}

		rt, err = p.pluginRegistry.CreateEntity(b.Type, subtype, name)
		if err != nil {
			typeName := b.Type
			if subtype != "" {
				typeName = subtype
			}

			de := errors.NewParserErrorWrapping(
				file,
				b.TypeRange.Start.Line,
				b.TypeRange.Start.Column,
				err,
				fmt.Sprintf("unable to create resource '%s' of type '%s': %s", name, typeName, err),
			)

			return de
		}
	}

	// We now have an entity, get the meta
	rtMeta, err := types.GetMeta(rt)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf("unable to get resource meta for resource %s: %s", b.Labels[0], err)
		de.Cause = err
		return de
	}

	rtMeta.Module = moduleName
	rtMeta.File = file
	rtMeta.Line = b.TypeRange.Start.Line
	rtMeta.Column = b.TypeRange.Start.Column

	// Set the ID from the FQRN
	fqrn := resources.FQRNFromResource(rt)
	rtMeta.ID = fqrn.String()

	// We now need to get all the dependent resources for this resource
	// so that we can build the dependency graph
	err = p.getUniqueResourceLinks(rt, b, src)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf("error creating resource '%s' in file %s: %s", b.Labels[0], file, err)
		de.Cause = err
		return de
	}

	// if we have an output, get the description
	// this is needed during parsing as the value may not be set during walk
	if err == nil && rtMeta.Type == resources.TypeOutput && b.Body.Attributes["description"] != nil {
		desc, diags := b.Body.Attributes["description"].Expr.Value(nil)
		if !diags.HasErrors() {
			rt.(*types.Output).Description = desc.AsString()
		}
	}

	// add the resource to the cache
	p.parsedResources.store(rtMeta.ID, b.Body, rt)

	return nil
}

// parseModule creates a shell for a module block, mirroring the non-eager
// shell-creation path parseResource uses for other block types. It does not
// decode the module's body at parse time; Variables and Disabled remain
// zero-valued on the shell until the Phase-2 DAG walk decodes them. The
// module's source directory is resolved and recursed into here (Phase 1.2)
// so that the module's child resources are discovered and scoped under the
// module's own instance name.
func (p *Parser) parseModule(file string, b *hclsyntax.Block, parentModule string, src []byte) []error {
	// fail fires the module's parse error and returns it, every problem with
	// the module block itself goes through it
	resourceType, resourceID := p.blockResource(b, parentModule)
	fail := func(err error) []error {
		emitParse(&p.options, resourceType, resourceID, file, err)
		return []error{err}
	}

	// If the type is module there should be one label for the instance name
	if len(b.Labels) != 1 {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = `invalid formatting for 'module' stanza, resources should have a name and a type, i.e. 'module "name" {}'`

		return fail(de)
	}

	name := b.Labels[0]
	if err := validateResourceName(name); err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = err.Error()

		return fail(de)
	}

	rt, err := p.createBuiltinResource(resources.TypeModule, name)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`unable to create module, this error should never happen %s`, err)
		de.Cause = err

		return fail(de)
	}

	// We now have an entity, get the meta
	rtMeta, err := types.GetMeta(rt)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf("unable to get resource meta for resource %s: %s", b.Labels[0], err)
		de.Cause = err
		return fail(de)
	}

	rtMeta.Module = parentModule
	rtMeta.File = file
	rtMeta.Line = b.TypeRange.Start.Line
	rtMeta.Column = b.TypeRange.Start.Column

	// Set the ID from the FQRN
	fqrn := resources.FQRNFromResource(rt)
	rtMeta.ID = fqrn.String()

	// We now need to get all the dependent resources for this module so that
	// we can build the dependency graph; this walks the module's source,
	// variables, and disabled attributes the same way it does for any other
	// resource type
	err = p.getUniqueResourceLinks(rt, b, src)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf("error creating resource '%s' in file %s: %s", b.Labels[0], file, err)
		de.Cause = err
		return fail(de)
	}

	// add the module to the cache
	p.parsedResources.store(rtMeta.ID, b.Body, rt)

	// Resolve the module's source as a local directory relative to the file
	// that declared it, and recurse into that directory's .xcl files, scoping
	// every resource discovered there under this module's own instance name.
	sourceAttr, ok := b.Body.Attributes["source"]
	if !ok {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`module '%s' has no 'source' attribute`, name)
		return fail(de)
	}

	sourceVal, diags := sourceAttr.Expr.Value(nil)
	if diags.HasErrors() {
		de := &errors.ParserError{}
		de.Line = sourceAttr.SrcRange.Start.Line
		de.Column = sourceAttr.SrcRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`unable to resolve 'source' for module '%s': %s`, name, diags.Error())
		return fail(de)
	}

	sourceDir := filepath.Join(filepath.Dir(file), sourceVal.AsString())

	moduleInstanceName := name
	if parentModule != "" {
		moduleInstanceName = parentModule + "." + name
	}

	// Resolve the source to a canonical path so that a module reached by two
	// different spellings of the same directory is recognised as the same
	// source. A source that cannot be resolved is reported against the module,
	// like any other module whose contents cannot be obtained.
	canonicalSource, err := canonicalPath(sourceDir)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`unable to obtain contents for module '%s' source '%s': %s`, name, sourceDir, err)
		de.Cause = err
		return fail(de)
	}

	// A module whose source transitively includes itself would recurse until
	// the stack is exhausted. Report it against the module instead.
	if p.parsedResources.moduleSources[canonicalSource] {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`module '%s' source '%s' includes itself`, name, sourceDir)
		return fail(de)
	}

	p.parsedResources.moduleSources[canonicalSource] = true
	defer delete(p.parsedResources.moduleSources, canonicalSource)

	childFiles, err := findXclFiles(sourceDir)
	if err != nil {
		de := &errors.ParserError{}
		de.Line = b.TypeRange.Start.Line
		de.Column = b.TypeRange.Start.Column
		de.Filename = file
		de.Message = fmt.Sprintf(`unable to discover files for module '%s' source '%s': %s`, name, sourceDir, err)
		de.Cause = err
		return fail(de)
	}

	// the module block itself has parsed, the resources in its source fire
	// their own parse events as they are parsed
	emitParse(&p.options, resourceType, resourceID, file, nil)

	moduleErrors := []error{}
	for _, childFile := range childFiles {
		moduleErrors = append(moduleErrors, p.parseResourcesInFile(childFile, moduleInstanceName)...)
	}

	if len(moduleErrors) > 0 {
		return moduleErrors
	}

	return nil
}

// getUniqueResourceLinks gets all the dependent resources for a resource
// these are either manaully set using the depends_on attribute or automatically
// inferred from the interpolations in the resource. src is the text of the
// file b was parsed from
func (p *Parser) getUniqueResourceLinks(resource any, b *hclsyntax.Block, src []byte) error {
	dr, err := p.getDependentResources(resource, b, src)
	if err != nil {
		return err
	}

	for _, d := range dr {
		if err := types.AppendUniqueLink(resource, d); err != nil {
			return fmt.Errorf("failed to add dependency %s: %w", d, err)
		}
	}

	body := b.Body
	if body == nil {
		// no body, nothing to do
		return nil
	}

	// now add the manually defined depends_on dependencies
	// depends_on does not support interpolation so use a nil context
	if attr, ok := body.Attributes["depends_on"]; ok {
		dependsOnVal, diags := attr.Expr.Value(&hcl.EvalContext{})
		if diags.HasErrors() {
			return fmt.Errorf("unable to read depends_on attribute: %s", diags.Error())
		}

		// depends on is a slice of string, each entry is validated as an
		// address and added to the links in its canonical form, while the
		// dependency list keeps the strings exactly as written
		dependsOnSlice := dependsOnVal.AsValueSlice()
		written := make([]string, 0, len(dependsOnSlice))
		for _, d := range dependsOnSlice {
			fqdn, err := p.addressParser().Parse(d.AsString())
			if err != nil {
				return fmt.Errorf("invalid dependency %s, %s", d.AsString(), err)
			}

			if err := types.AppendUniqueLink(resource, fqdn.String()); err != nil {
				return fmt.Errorf("failed to add dependency %s: %w", d, err)
			}

			written = append(written, d.AsString())
		}

		if err := types.SetDependencies(resource, written); err != nil {
			return fmt.Errorf("unable to set depends_on: %w", err)
		}
	}

	return nil
}

// getDependentResources recursively checks the fields and blocks on the resource to identify links to other resources
// i.e. resource.container.foo.network[0].name
// This enables automatic building of the dependency graph
//
// The same walk records, in the resource's Meta.References, the text the
// author wrote after the = for every attribute holding a reference, keyed by
// the attribute's path, i.e. "location" or "network[1].name". A reference is
// decided by the same rule as the links, so the two always agree on which
// attributes hold one
func (p *Parser) getDependentResources(resource any, b *hclsyntax.Block, src []byte) ([]string, error) {
	references := []string{}
	written := map[string]string{}

	// Process all attributes in the block
	for _, a := range b.Body.Attributes {
		refs, err := processExpr(a.Expr, p.isReferenceRoot)
		if err != nil {
			return nil, errors.NewParserError(
				b.Body.SrcRange.Filename,
				b.Body.SrcRange.Start.Line,
				b.Body.SrcRange.Start.Column,
				fmt.Sprintf("unable to process attribute %s: %s", a.Name, err),
			)
		}

		references = append(references, refs...)

		// depends_on holds addresses as strings, never references, it is
		// skipped so it is never recorded as one
		if len(refs) > 0 && a.Name != "depends_on" {
			recordWrittenText(written, a.Name, a, src)
		}
	}

	// Process nested blocks recursively
	blockIndex := map[string]int{}
	for _, block := range b.Body.Blocks {
		if _, ok := blockIndex[block.Type]; ok {
			blockIndex[block.Type]++
		} else {
			blockIndex[block.Type] = 0
		}

		// Recursively get dependencies from nested blocks
		prefix := fmt.Sprintf("%s[%d].", block.Type, blockIndex[block.Type])
		cr, err := p.getDependentResourcesFromBlock(block, prefix, src, written)
		if err != nil {
			return nil, err
		}

		references = append(references, cr...)
	}

	// Check for cyclical dependencies
	rMeta, err := types.GetMeta(resource)
	if err != nil {
		return references, nil // Skip cycle check if resource doesn't have metadata
	}

	if len(written) > 0 {
		rMeta.References = written
	}

	for _, dep := range references {
		// Check if this dependency would create a cycle
		// Look up the dependency in parsedResources
		if p.parsedResources != nil {
			if depResource, ok := p.parsedResources.resources[dep]; ok {
				depMeta, err := types.GetMeta(depResource)
				if err != nil {
					continue // Skip if dependency doesn't have metadata
				}

				// Check if the dependency's links contain a reference back to us
				for _, cdep := range depMeta.Links {
					fqrn, err := p.addressParser().Parse(cdep)
					if err != nil {
						continue
					}
					fqrn.Attribute = ""

					// Check for direct cycle
					if rMeta.Name == fqrn.Resource &&
						rMeta.Type == fqrn.Type &&
						rMeta.Subtype == fqrn.Subtype &&
						rMeta.Module == fqrn.Module {
						return nil, errors.NewParserError(
							b.Body.SrcRange.Filename,
							b.Body.SrcRange.Start.Line,
							b.Body.SrcRange.Start.Column,
							fmt.Sprintf("'%s' depends on '%s' which creates a cyclical dependency", rMeta.ID, depMeta.ID),
						)
					}
				}
			}
		}
	}

	return references, nil
}

// getDependentResourcesFromBlock extracts dependencies from a nested block.
// prefix is the block's path, i.e. "network[1].", and the text written for
// each attribute holding a reference is recorded in written under prefix and
// the attribute's name
func (p *Parser) getDependentResourcesFromBlock(b *hclsyntax.Block, prefix string, src []byte, written map[string]string) ([]string, error) {
	references := []string{}

	// Process attributes in the block
	for _, a := range b.Body.Attributes {
		refs, err := processExpr(a.Expr, p.isReferenceRoot)
		if err != nil {
			return nil, fmt.Errorf("unable to process attribute %s: %w", a.Name, err)
		}
		references = append(references, refs...)

		if len(refs) > 0 {
			recordWrittenText(written, prefix+a.Name, a, src)
		}
	}

	// Process nested blocks recursively, numbering each by its position
	// among the blocks of the same type
	blockIndex := map[string]int{}
	for _, block := range b.Body.Blocks {
		index := blockIndex[block.Type]
		blockIndex[block.Type] = index + 1

		blockPrefix := fmt.Sprintf("%s%s[%d].", prefix, block.Type, index)
		cr, err := p.getDependentResourcesFromBlock(block, blockPrefix, src, written)
		if err != nil {
			return nil, err
		}
		references = append(references, cr...)
	}

	return references, nil
}

// processDisabled processes any expression for the disabled attribute
// and sets the disabled state on the resource
func processDisabled(bdy *hclsyntax.Body, ctx *hcl.EvalContext, r dag.Vertex) (bool, error) {
	var isDisabled bool

	// This expression could be a reference to another resource or it could be a
	// function or a conditional statement. We need to evaluate the expression
	// to determine if the resource should be disabled
	if attr, ok := bdy.Attributes["disabled"]; ok {
		// now we need to evaluate the expression
		expdiags := gohcl.DecodeExpression(attr.Expr, ctx, &isDisabled)
		if expdiags.HasErrors() {
			return isDisabled,
				errors.NewParserErrorFromResource(
					r,
					fmt.Sprintf("unable to decode disabled expression: %s", expdiags.Error()),
				)
		}

		err := types.SetDisabled(r, isDisabled)
		if err != nil {
			return isDisabled, errors.NewParserErrorFromResource(
				r,
				fmt.Sprintf("failed to set disabled state: %s", err),
			)
		}
	}

	return isDisabled, nil
}

// walk builds a DAG from the state and walks it with the given callback
// This is the core parsing logic that processes resources in dependency order
// and calls the provider lifecycle for each resource
//
// It returns the progress of the walk, which is nil when the walk did not start.
func (p *Parser) walk(ctx context.Context, currentState, previousState *State, functions functionsForFile) (*applyProgress, []error) {
	// Build the DAG using currentState (implements ResourceProvider)
	d, err := DoYouLikeDags(currentState, p.addressParser(), false)
	if err != nil {
		p.emitOperationError(events.OperationApply, err)
		return nil, []error{err}
	}

	// Reduce the graph nodes to unique instances
	d.TransitiveReduction()

	// Validate the dependency graph is ok
	err = d.Validate()
	if err != nil {
		err = fmt.Errorf("unable to validate dependency graph: %w", err)
		p.emitOperationError(events.OperationApply, err)
		return nil, []error{err}
	}

	// Define the walker callback that will be called for every node in the graph
	w := dag.Walker{}

	// The lifecycle decides the provider calls for each resource from the
	// state saved by the last apply
	lifecycle := &resourceLifecycle{
		ctx:      ctx,
		previous: previousState,
		resolver: p.providerResolver,
		options:  &p.options,
		types:    p.typeRegistry,
		bodies:   p.parsedResources.bodies,
		progress: newApplyProgress(),
	}

	w.Callback = walkCallback(p.parsedResources, currentState, p.addressParser(), lifecycle, &p.options, functions)
	w.Reverse = false

	// Update the dag and process the nodes
	errs := []error{}
	w.Update(d)
	diags := w.Wait()
	if diags.HasErrors() {
		errs = append(errs, diags.Err().(errwrap.Wrapper).WrappedErrors()...)
		return lifecycle.progress, errs
	}

	return lifecycle.progress, nil
}

// createBuiltinResource creates built-in resource types (local, output, variable, module)
func (p *Parser) createBuiltinResource(resourceType, resourceName string) (any, error) {
	builtinTypes := resources.DefaultResources()
	return builtinTypes.CreateResource(resourceType, resourceName)
}

// functionsForFile returns the HCL functions available to a resource defined
// in the given file
type functionsForFile func(file string) map[string]function.Function

// getFunctions returns all HCL functions (custom + builtins) for a resource
// defined in file, builtins such as file() and dir() resolve relative paths
// against the directory containing file
func (p *Parser) getFunctions(file string) map[string]function.Function {
	// Start with default built-in functions
	funcs := functions.GetDefaultFunctions(file)

	// Override with custom functions from parser options
	for name, fn := range p.customFunctions {
		funcs[name] = fn
	}

	// every function, built-in or custom, keeps sensitive arguments out of
	// its errors and marks what it derives from them
	return redactingFunctions(funcs)
}

// emitValidateError emits a validate error event for a problem found by
// validation, naming the resource declared at the problem's position when
// there is one, otherwise only its file
func (p *Parser) emitValidateError(problem error) {
	if !emitting(&p.options) {
		return
	}

	e := events.Event{
		Source:    events.SourceCore,
		Operation: events.OperationValidate,
		Phase:     events.PhaseError,
		Error:     problem,
	}

	var pe *errors.ParserError
	if stderrors.As(problem, &pe) {
		e.File = pe.Filename

		for _, r := range p.parsedResources.resources {
			meta, err := types.GetMeta(r)
			if err != nil || meta.File != pe.Filename || meta.Line != pe.Line || meta.Column != pe.Column {
				continue
			}

			e.ResourceID = meta.ID
			e.ResourceType = resourceType(meta)
			break
		}
	}

	emit(&p.options, e)
}

// emitOperationError emits an error event for a failure of the operation as a
// whole, one that belongs to no single resource
func (p *Parser) emitOperationError(operation string, err error) {
	emitOperationError(&p.options, operation, err)
}

// loadPlugins loads the plugin registry's plugins, which happens once per
// registry, so it is cheap when a Config has already loaded them
func (p *Parser) loadPlugins() error {
	if p.pluginRegistry == nil {
		return nil
	}

	return p.pluginRegistry.Load(p.options.Emit)
}
