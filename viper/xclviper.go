// Package xclviper lets Viper read configuration written in xcl.
//
// The data model is the application's. It names its block types, plain
// structs, and gets back a Viper that reads .xcl files into them:
//
//	v := xclviper.New(
//		xclviper.Type[Runner]("runner"),
//		xclviper.Type[Agent]("agent"),
//	)
//	v.SetConfigFile("runner.xcl")
//	err := v.ReadInConfig()
//
// Read does the same for a file or a directory, reading every .xcl file in a
// directory as one configuration.
//
// Every block is laid out in Viper's keys as
//
//	<type>[.<subtype>].<name>.<attribute>
//
// with attributes named as the struct's xcl tags name them, nested blocks as
// nested keys, and repeated blocks keyed by their label when they have one:
// `agent "claude" { models = [...] }` is agent.claude.models. A singleton, a
// type with neither label fields nor a Meta field, is written without labels
// and laid out under its type alone: `runner { server = ... }` is
// runner.server. A value that is a labelled block, such as a
// reference to one, keeps its label among its attributes, under the label
// field's name, so a list of them keeps its order:
// `agents = [agent.claude, agent.local]` is a list whose first element's name
// is claude. Everything else Viper does — defaults, environment variables,
// flags — works as it does for any other format.
package xclviper

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/tags"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/types"
	"github.com/spf13/viper"
)

// Extension is the file extension, and the Viper config type, xcl is read
// under.
const Extension = "xcl"

// Option configures New: a block type, or options for Viper itself.
type Option interface {
	apply(*settings)
}

type settings struct {
	types []func(registry.TypeRegistrar)
	viper []viper.Option
}

type optionFunc func(*settings)

func (f optionFunc) apply(s *settings) { f(s) }

// Type declares the Go type T, a struct, as a block type: name is the block
// type and an optional subtype, as registry.Register takes them. A T with
// neither label fields nor a Meta field is a singleton, written without
// labels.
func Type[T any](name ...string) Option {
	return optionFunc(func(s *settings) {
		s.types = append(s.types, func(r registry.TypeRegistrar) {
			registry.Register[T](r, name...)
		})
	})
}

// Read reads the configuration at path, a file or a directory, into v, which
// New returned, replacing what v read before as ReadInConfig does. A
// directory is read as one configuration, every .xcl file in it, so a block
// in one file can refer to a block in another. Errors name the file and line
// at fault.
func Read(v *viper.Viper, path string) error {
	v.SetConfigFile(path)
	v.SetConfigType(Extension)

	// the codec applies the path Viper was given and never reads the bytes,
	// so none are passed: Viper's own reading would refuse a directory
	return v.ReadConfig(bytes.NewReader(nil))
}

// With passes options to Viper itself, such as viper.KeyDelimiter.
func With(options ...viper.Option) Option {
	return optionFunc(func(s *settings) {
		s.viper = append(s.viper, options...)
	})
}

// New returns a Viper that reads .xcl files into the block types given. A
// type xcl cannot declare panics here, naming it, as registry.Register does.
func New(options ...Option) *viper.Viper {
	s := &settings{}
	for _, option := range options {
		option.apply(s)
	}

	local := registry.NewLocal()
	for _, declare := range s.types {
		declare(local)
	}

	acceptExtension()

	c := &codec{registry: local}

	codecs := viper.NewCodecRegistry()
	if err := codecs.RegisterCodec(Extension, c); err != nil {
		panic(fmt.Sprintf("xclviper: %s", err))
	}

	v := viper.NewWithOptions(append([]viper.Option{viper.WithCodecRegistry(codecs)}, s.viper...)...)
	c.viper = v

	return v
}

var accepted sync.Once

// acceptExtension adds Extension to the extensions Viper accepts. Viper still
// checks this package level list before it asks the codec registry.
func acceptExtension() {
	accepted.Do(func() {
		if !slices.Contains(viper.SupportedExts, Extension) {
			viper.SupportedExts = append(viper.SupportedExts, Extension)
		}
	})
}

// codec reads xcl into Viper's map for the types of one registry.
type codec struct {
	registry *registry.Local
	viper    *viper.Viper
}

// Encode is not supported: an xcl file is written by a person.
func (c *codec) Encode(map[string]any) ([]byte, error) {
	return nil, fmt.Errorf("xclviper: writing xcl is not supported")
}

// Decode applies the configuration and lays every entity out in out.
//
// When Viper read a file, that file is applied, so errors name it and show
// its source. Configuration given to Viper as bytes is written to a file of
// its own for the length of the call.
func (c *codec) Decode(b []byte, out map[string]any) error {
	path := ""
	if c.viper != nil {
		path = c.viper.ConfigFileUsed()
	}

	if path == "" {
		dir, err := os.MkdirTemp("", "xclviper")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)

		path = filepath.Join(dir, "config."+Extension)
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return err
		}
	}

	config, err := xcl.NewConfig(xcl.WithRegistry(c.registry))
	if err != nil {
		return err
	}

	if err := config.Apply(path); err != nil {
		return err
	}

	for _, t := range c.registry.Types() {
		segments := []string{t.Type}
		if t.Subtype != "" {
			segments = append(segments, t.Subtype)
		}

		entities, err := config.FindEntitiesByType(segments...)
		if err != nil {
			return fmt.Errorf("xclviper: %v: %w", segments, err)
		}

		for _, entity := range entities {
			if disabled, _ := types.GetDisabled(entity); disabled {
				continue
			}

			meta, err := types.GetMeta(entity)
			if err != nil {
				return err
			}

			// a singleton has no name, and is laid out under its type
			path := slices.Clone(segments)
			if meta.Name != "" {
				path = append(path, meta.Name)
			}

			put(out, path, attributes(reflect.ValueOf(entity), false))
		}
	}

	return nil
}

// put sets value at the path, making maps along the way.
func put(out map[string]any, path []string, value any) {
	m := out
	for _, key := range path[:len(path)-1] {
		next, ok := m[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[key] = next
		}
		m = next
	}

	m[path[len(path)-1]] = value
}

// attributes lays a struct out by its xcl tags. Fields without a named tag —
// types.ResourceBase, the marker naming a plain type — are left out, and so
// are labels unless withLabels: a block's labels are its keys, while a value
// holding a labelled block, such as a reference to one, has nothing else to
// keep them in.
func attributes(v reflect.Value, withLabels bool) map[string]any {
	v = reflect.Indirect(v)
	out := map[string]any{}

	for i := range v.NumField() {
		field := v.Type().Field(i)
		tag, ok := field.Tag.Lookup(tags.Name)
		if !ok || !field.IsExported() {
			continue
		}

		parsed, err := tags.Parse(tag)
		if err != nil || parsed.Name == "" {
			continue
		}

		if parsed.Kind == tags.KindLabel {
			if withLabels {
				out[parsed.Name] = fmt.Sprint(v.Field(i).Interface())
			}
			continue
		}

		if value, set := plain(v.Field(i), false, parsed.Kind == tags.KindBlock); set {
			out[parsed.Name] = value
		}
	}

	return out
}

// labelOf is the value of a block's label field, when its type has one.
func labelOf(v reflect.Value) (string, bool) {
	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		return "", false
	}

	for i := range v.NumField() {
		tag, ok := v.Type().Field(i).Tag.Lookup(tags.Name)
		if !ok {
			continue
		}

		if parsed, err := tags.Parse(tag); err == nil && parsed.Kind == tags.KindLabel {
			return fmt.Sprint(v.Field(i).Interface()), true
		}
	}

	return "", false
}

var sensitiveType = reflect.TypeFor[types.SensitiveValue]()

// plain turns a field into what Viper keeps: maps, slices and scalars.
//
// What was not written is left out, so a default beneath it still shows. A
// plain field cannot say whether it was written as its zero value or not at
// all, so a zero is left out; a pointer can, so a set pointer is kept whatever
// it points at. A sensitive value is revealed: Viper has no notion of one, and
// the application reading its configuration is who it is for.
//
// block is whether the field holds nested blocks. Repeated labelled blocks are
// keyed by their label; labelled values anywhere else, such as a list of
// references, stay a list in the order written, each keeping its label.
func plain(v reflect.Value, set, block bool) (any, bool) {
	if v.Type().Implements(sensitiveType) && v.CanInterface() {
		return v.Interface().(types.SensitiveValue).RevealAny(), true
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil, false
		}
		return plain(v.Elem(), true, block)

	case reflect.Struct:
		return attributes(v, !block), true

	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return nil, false
		}

		if block && v.Len() > 0 {
			if _, labelled := labelOf(v.Index(0)); labelled {
				out := map[string]any{}
				for i := range v.Len() {
					label, _ := labelOf(v.Index(i))
					out[label], _ = plain(v.Index(i), true, true)
				}
				return out, true
			}
		}

		out := make([]any, 0, v.Len())
		for i := range v.Len() {
			item, _ := plain(v.Index(i), true, block)
			out = append(out, item)
		}
		return out, true

	case reflect.Map:
		if v.IsNil() {
			return nil, false
		}

		out := map[string]any{}
		for _, key := range v.MapKeys() {
			item, _ := plain(v.MapIndex(key), true, block)
			out[fmt.Sprint(key.Interface())] = item
		}
		return out, true

	default:
		if v.IsZero() && !set {
			return nil, false
		}
		return v.Interface(), true
	}
}
