// Package recorder is an end-to-end fixture that records what xcl tells a
// provider has changed. Every call to Changed and Update appends one JSON line
// to the file named by the resource's log setting, so a test can compare what
// an in-process provider and an external one were given.
//
// The external fixture, ../externalplugin, registers it as
// resource "recorder" "<name>" {}. A test registers it in-process itself with
// Register.
package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// TypeName and SubTypeName are what the recorder block type is registered
// as, written resource "recorder" "<name>" {}
const (
	TypeName    = "resource"
	SubTypeName = "recorder"
)

// Recorder defines the block type `recorder`
type Recorder struct {
	types.ResourceBase `xcl:",remain"`

	// Value is a plain setting, changing it answers update
	Value string `xcl:"value,optional" json:"value,omitempty"`
	// Secret is sensitive, changing it answers update; the record holds the
	// real values
	Secret types.Sensitive[string] `xcl:"secret,optional" json:"secret"`
	// Input is a plain setting, usually set from another recorder's output to
	// make a dependency
	Input string `xcl:"input,optional" json:"input,omitempty"`
	// ReplaceKey is the setting whose change answers replace
	ReplaceKey string `xcl:"replace_key,optional" json:"replace_key,omitempty"`

	// Output is computed by Create as <name>-<replace_key>
	Output string `xcl:"output,optional,computed" json:"output,omitempty"`

	// Log is the path of the file every Changed and Update call appends its
	// record to
	Log string `xcl:"log" json:"log"`
}

// Record is one line of the log file, written by one Changed or Update call
type Record struct {
	// Call is "changed" or "update"
	Call string `json:"call"`
	// ID is the resource's meta id
	ID string `json:"id"`
	// Changes are the property changes the call was given, with real values
	Changes []RecordedChange `json:"changes"`
	// Dependencies are the dependency changes the call was given
	Dependencies []RecordedDependency `json:"dependencies"`
}

// RecordedChange is an entity.PropertyChange copied field by field, so it
// encodes the real values of a sensitive change rather than the mask
// PropertyChange.MarshalJSON writes
type RecordedChange struct {
	Path      string `json:"path"`
	Before    any    `json:"before"`
	After     any    `json:"after"`
	Unknown   bool   `json:"unknown"`
	Sensitive bool   `json:"sensitive"`
}

// RecordedDependency is an entity.DependencyChange with its change written
// as text
type RecordedDependency struct {
	Address string `json:"address"`
	Change  string `json:"change"`
}

// Provider handles the lifecycle of recorder blocks
type Provider struct{}

var _ plugins.ResourceProvider[*Recorder] = (*Provider)(nil)

// Register registers the recorder block type with p, it is used both by the
// external fixture and by tests that run the recorder in-process
func Register(p *plugins.PluginBase, log logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(p, log, state, TypeName, SubTypeName, &Recorder{}, &Provider{})
}

// Init is called once, when the provider is registered
func (p *Provider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

// Create sets the computed output to <name>-<replace_key>
func (p *Provider) Create(ctx context.Context, resource *Recorder) (*Recorder, error) {
	resource.Output = output(resource)

	return resource, nil
}

// Read reports the resource as configured
func (p *Provider) Read(ctx context.Context, old *Recorder, new *Recorder) (*Recorder, error) {
	return new, nil
}

// Update records the call and keeps the computed output current
func (p *Provider) Update(ctx context.Context, resource *Recorder, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Recorder, error) {
	err := appendRecord(resource.Log, "update", resource.Meta.ID, changes, dependencies)
	if err != nil {
		return nil, err
	}

	resource.Output = output(resource)

	return resource, nil
}

// Changed records the call, to the new resource's log, then answers:
//   - entity.Replace when any change is within replace_key;
//   - entity.Update when there is any other change or any dependency change;
//   - entity.NoChange otherwise.
func (p *Provider) Changed(ctx context.Context, old *Recorder, new *Recorder, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
	err := appendRecord(new.Log, "changed", new.Meta.ID, changes, dependencies)
	if err != nil {
		return entity.NoChange, err
	}

	replaceKey := entity.Path{}.Attribute("replace_key")
	for _, change := range changes {
		if change.Within(replaceKey) {
			return entity.Replace, nil
		}
	}

	if len(changes) > 0 || len(dependencies) > 0 {
		return entity.Update, nil
	}

	return entity.NoChange, nil
}

// Destroy does nothing, the recorder creates nothing real
func (p *Provider) Destroy(ctx context.Context, resource *Recorder, force bool) error {
	return nil
}

// Functions returns no functions
func (p *Provider) Functions() plugins.ProviderFunctions {
	return nil
}

// output returns the computed output of resource
func output(resource *Recorder) string {
	name := resource.Meta.Name
	if name == "" {
		name = resource.Meta.ID
	}

	return fmt.Sprintf("%s-%s", name, resource.ReplaceKey)
}

// logMutex serialises appends within one process
var logMutex sync.Mutex

// appendRecord appends one JSON line describing a call to the file at path
func appendRecord(path, call, id string, changes []entity.PropertyChange, dependencies []entity.DependencyChange) error {
	if path == "" {
		return fmt.Errorf("recorder %s has no log path", id)
	}

	record := Record{
		Call:         call,
		ID:           id,
		Changes:      make([]RecordedChange, 0, len(changes)),
		Dependencies: make([]RecordedDependency, 0, len(dependencies)),
	}

	for _, change := range changes {
		record.Changes = append(record.Changes, RecordedChange{
			Path:      change.Path.String(),
			Before:    change.Before,
			After:     change.After,
			Unknown:   change.Unknown,
			Sensitive: change.Sensitive,
		})
	}

	for _, dependency := range dependencies {
		record.Dependencies = append(record.Dependencies, RecordedDependency{
			Address: dependency.Address,
			Change:  dependency.Change.String(),
		})
	}

	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding record for %s: %w", id, err)
	}

	logMutex.Lock()
	defer logMutex.Unlock()

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening recorder log %s: %w", path, err)
	}

	_, err = file.Write(append(line, '\n'))
	if err != nil {
		file.Close()
		return fmt.Errorf("writing recorder log %s: %w", path, err)
	}

	return file.Close()
}
