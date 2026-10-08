package template

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
)

func TestTemplateCreateWritesTheRenderedFile(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "out.txt")
	p := &provider{}

	_, err := p.Create(context.Background(), &Template{
		Source:      "hello world",
		Destination: destination,
	})
	require.NoError(t, err)

	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "hello world", string(content))
}

func TestTemplateCreateSubstitutesVariables(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "out.txt")
	p := &provider{}

	_, err := p.Create(context.Background(), &Template{
		Source:      "listen {{address}}:{{port}}",
		Destination: destination,
		Variables:   map[string]string{"address": "0.0.0.0", "port": "8080"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "listen 0.0.0.0:8080", string(content))
}

func TestTemplateCreateCreatesParentDirectories(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "a", "b", "c", "out.txt")
	p := &provider{}

	_, err := p.Create(context.Background(), &Template{
		Source:      "nested",
		Destination: destination,
	})
	require.NoError(t, err)

	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "nested", string(content))
}

func TestTemplateCreateSupportsTheQuoteHelper(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "out.txt")
	p := &provider{}

	_, err := p.Create(context.Background(), &Template{
		Source:      "{{quote name}}",
		Destination: destination,
		Variables:   map[string]string{"name": "web"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "\"web\"", string(content))
}

func TestTemplateCreateSupportsTheTrimHelper(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "out.txt")
	p := &provider{}

	_, err := p.Create(context.Background(), &Template{
		Source:      "[{{trim name}}]",
		Destination: destination,
		Variables:   map[string]string{"name": "  web \n"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "[web]", string(content))
}

func TestTemplateCreateFailsForAnInvalidTemplate(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "out.txt")
	p := &provider{}

	_, err := p.Create(context.Background(), &Template{
		Source:      "{{#if}}",
		Destination: destination,
	})
	require.Error(t, err)

	_, statErr := os.Stat(destination)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestTemplateDestroyRemovesTheFile(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "out.txt")
	require.NoError(t, os.WriteFile(destination, []byte("content"), 0644))
	p := &provider{}

	err := p.Destroy(context.Background(), &Template{Destination: destination}, false)
	require.NoError(t, err)

	_, statErr := os.Stat(destination)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestTemplateDestroySucceedsWhenTheFileIsGone(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "missing.txt")
	p := &provider{}

	err := p.Destroy(context.Background(), &Template{Destination: destination}, false)
	require.NoError(t, err)
}

func TestTemplateUpdateRendersTheFileAgain(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "out.txt")
	require.NoError(t, os.WriteFile(destination, []byte("old content"), 0644))
	p := &provider{}

	_, err := p.Update(context.Background(), &Template{
		Source:      "new {{value}}",
		Destination: destination,
		Variables:   map[string]string{"value": "content"},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "new content", string(content))
}

func TestTemplateChangedReplacesOnDestinationChange(t *testing.T) {
	p := &provider{}

	old := &Template{Source: "hello", Destination: "/tmp/old.txt"}
	new := &Template{Source: "hello", Destination: "/tmp/new.txt"}

	change, err := p.Changed(context.Background(), old, new, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Replace, change)
}

func TestTemplateChangedUpdatesOnSourceChange(t *testing.T) {
	p := &provider{}

	old := &Template{Source: "hello", Destination: "/tmp/out.txt"}
	new := &Template{Source: "goodbye", Destination: "/tmp/out.txt"}

	change, err := p.Changed(context.Background(), old, new, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestTemplateChangedUpdatesOnVariablesChange(t *testing.T) {
	p := &provider{}

	old := &Template{
		Source:      "hello {{name}}",
		Destination: "/tmp/out.txt",
		Variables:   map[string]string{"name": "world"},
	}
	new := &Template{
		Source:      "hello {{name}}",
		Destination: "/tmp/out.txt",
		Variables:   map[string]string{"name": "xcl"},
	}

	change, err := p.Changed(context.Background(), old, new, nil)
	require.NoError(t, err)
	require.Equal(t, entity.Update, change)
}

func TestTemplateChangedReportsNoChangeForIdenticalTemplate(t *testing.T) {
	p := &provider{}

	old := &Template{
		Source:      "hello {{name}}",
		Destination: "/tmp/out.txt",
		Variables:   map[string]string{"name": "world"},
	}
	new := &Template{
		Source:      "hello {{name}}",
		Destination: "/tmp/out.txt",
		Variables:   map[string]string{"name": "world"},
	}

	change, err := p.Changed(context.Background(), old, new, nil)
	require.NoError(t, err)
	require.Equal(t, entity.NoChange, change)
}
