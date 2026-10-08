// Package template is an in-process xcl plugin that renders Handlebars
// templates to files. It provides the block type template, which has no
// subtype and is written template "<name>" {}.
//
// It renders with the Handlebars library xcl's own template function uses,
// with the same quote and trim helpers.
package template

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/infinytum/raymond/v2"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// Template defines the block type template, a Handlebars template rendered to
// a file
type Template struct {
	types.ResourceBase `xcl:",remain"`

	// Source is the Handlebars template text, use file() to read it from a
	// file
	Source string `xcl:"source" json:"source"`

	// Destination is the path of the file the template is rendered to, its
	// parent directories are created when they do not exist
	Destination string `xcl:"destination" json:"destination"`

	// Variables are the values the template refers to, i.e. {{address}}
	Variables map[string]string `xcl:"variables,optional" json:"variables,omitempty"`
}

// provider renders templates for template blocks and removes the rendered
// files on destroy
type provider struct {
	plugins.DefaultChanged[*Template]
}

var _ plugins.ResourceProvider[*Template] = (*provider)(nil)

// Changed answers replace when the destination changes: Update renders to the
// new path but would leave the file at the old one behind, so the template is
// destroyed, removing its file, and created again. A change to the source or
// variables is left to DefaultChanged, which answers update, and Update
// renders the file again.
func (p *provider) Changed(ctx context.Context, old *Template, new *Template, dependencies []entity.DependencyChange) (entity.Change, error) {
	if old.Destination != new.Destination {
		return entity.Replace, nil
	}

	return p.DefaultChanged.Changed(ctx, old, new, dependencies)
}

// Init logs that the provider is ready, it needs nothing else
func (p *provider) Init(state plugins.State, functions plugins.ProviderFunctions, log logger.Logger) error {
	log.Debug("provider ready")
	return nil
}

// Create renders the template with its variables and writes the result to the
// destination
func (p *provider) Create(ctx context.Context, t *Template) (*Template, error) {
	err := render(t)
	if err != nil {
		return nil, err
	}

	plugins.Logger(ctx).Info("rendered template", "destination", t.Destination)

	return t, nil
}

// Destroy removes the rendered file, a file that is already gone counts as
// removed
func (p *provider) Destroy(ctx context.Context, t *Template, force bool) error {
	err := os.Remove(t.Destination)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("unable to remove %s: %w", t.Destination, err)
	}

	plugins.Logger(ctx).Info("removed template", "destination", t.Destination)

	return nil
}

// Read returns the configured template, rendering is cheap so the example
// does not check the file
func (p *provider) Read(ctx context.Context, old *Template, new *Template) (*Template, error) {
	return new, nil
}

// Update renders the template again
func (p *provider) Update(ctx context.Context, t *Template) (*Template, error) {
	err := render(t)
	if err != nil {
		return nil, err
	}

	plugins.Logger(ctx).Info("rendered template", "destination", t.Destination)

	return t, nil
}

// Functions returns nil, the provider offers no functions
func (p *provider) Functions() plugins.ProviderFunctions {
	return nil
}

// render renders t's source with its variables and writes it to its
// destination, creating the destination's parent directories
func render(t *Template) error {
	tmpl, err := raymond.Parse(t.Source)
	if err != nil {
		return fmt.Errorf("unable to parse template: %w", err)
	}

	// the helpers are the ones xcl's template function offers. They return
	// SafeString because Handlebars HTML-escapes what a helper returns
	// otherwise, which would write &quot; for the quotes quote adds.
	tmpl.RegisterHelpers(map[string]any{
		"quote": func(in string) raymond.SafeString {
			return raymond.SafeString(fmt.Sprintf(`"%s"`, in))
		},
		"trim": func(in string) raymond.SafeString {
			return raymond.SafeString(strings.TrimSpace(in))
		},
	})

	out, err := tmpl.Exec(t.Variables)
	if err != nil {
		return fmt.Errorf("unable to render template: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(t.Destination), 0755)
	if err != nil {
		return fmt.Errorf("unable to create the directory for %s: %w", t.Destination, err)
	}

	err = os.WriteFile(t.Destination, []byte(out), 0644)
	if err != nil {
		return fmt.Errorf("unable to write %s: %w", t.Destination, err)
	}

	return nil
}
