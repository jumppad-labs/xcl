// Command recording is a fixture plugin for the language bindings' tests: it
// provides resource "recording" "<name>" {}, whose every lifecycle call
// appends one JSON line to the file named by the block's log setting before
// doing its work, then takes delay_ms. A test that cancels an operation reads
// the log back to show that no call started after the cancellation. It is
// built as an external plugin, xcl-plugin-recording, and loaded from a path.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// Recording is the block type resource "recording" "<name>" {}
type Recording struct {
	Meta types.Meta

	// Log is the path of the file every lifecycle call appends its record to
	Log string `xcl:"log" json:"log"`

	// DelayMs is how long each call takes after recording, in milliseconds
	DelayMs int `xcl:"delay_ms,optional" json:"delay_ms,omitempty"`

	// Value is a plain setting, changing it answers update
	Value string `xcl:"value,optional" json:"value,omitempty"`

	// Input is usually set from another recording's output, to make a
	// dependency
	Input string `xcl:"input,optional" json:"input,omitempty"`

	// Output is computed by create as <name>-<value>
	Output string `xcl:"output,optional,computed" json:"output,omitempty"`
}

// Record is one line of the log file: the step that started, the address of
// the entity it started for, and when
type Record struct {
	Step    string    `json:"step"`
	Address string    `json:"address"`
	Time    time.Time `json:"time"`
}

var logMu sync.Mutex

// record appends a line for a call that is starting
func record(path, step, address string) error {
	logMu.Lock()
	defer logMu.Unlock()

	line, err := json.Marshal(Record{Step: step, Address: address, Time: time.Now()})
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("recording: open log %s: %w", path, err)
	}
	defer f.Close()

	_, err = f.Write(append(line, '\n'))

	return err
}

func output(r *Recording) string {
	return r.Meta.Name + "-" + r.Value
}

type recordingProvider struct {
	plugins.DefaultChanged[*Recording]
}

var _ plugins.ResourceProvider[*Recording] = (*recordingProvider)(nil)

func (p *recordingProvider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	return nil
}

func (p *recordingProvider) Create(ctx context.Context, r *Recording) (*Recording, error) {
	if err := record(r.Log, "create", r.Meta.ID); err != nil {
		return nil, err
	}

	wait(ctx, r.DelayMs)
	r.Output = output(r)

	return r, nil
}

func (p *recordingProvider) Read(ctx context.Context, old *Recording, new *Recording) (*Recording, error) {
	if err := record(new.Log, "read", new.Meta.ID); err != nil {
		return nil, err
	}

	new.Output = old.Output

	return new, nil
}

func (p *recordingProvider) Update(ctx context.Context, r *Recording, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Recording, error) {
	if err := record(r.Log, "update", r.Meta.ID); err != nil {
		return nil, err
	}

	wait(ctx, r.DelayMs)
	r.Output = output(r)

	return r, nil
}

func (p *recordingProvider) Destroy(ctx context.Context, r *Recording, force bool) error {
	if err := record(r.Log, "destroy", r.Meta.ID); err != nil {
		return err
	}

	wait(ctx, r.DelayMs)

	return nil
}

func (p *recordingProvider) Functions() plugins.ProviderFunctions {
	return nil
}

// wait sleeps for ms milliseconds, or until ctx ends
func wait(ctx context.Context, ms int) {
	if ms <= 0 {
		return
	}

	select {
	case <-time.After(time.Duration(ms) * time.Millisecond):
	case <-ctx.Done():
	}
}

// RecordingPlugin provides the recording block type
type RecordingPlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*RecordingPlugin)(nil)

func (p *RecordingPlugin) Init(log logger.Logger, state plugins.State) error {
	return plugins.RegisterResourceProvider(&p.PluginBase, log, state, "resource", "recording", &Recording{}, &recordingProvider{})
}

func main() {
	plugins.Serve(&RecordingPlugin{})
}
