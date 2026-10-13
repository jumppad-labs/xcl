package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	ctyjson "github.com/jumppad-labs/xcl/internal/cty/json"
	"github.com/jumppad-labs/xcl/internal/functions"
)

// ProviderFunction is a function a provider offers to configuration, which
// calls it by its bare name, as it calls a builtin, while the configuration
// is read. The function runs inside its plugin: the host sends the arguments
// as JSON and receives the result as JSON, so an entity only ever holds the
// result. Build one from a Go func with NewFunction.
type ProviderFunction interface {
	// Signature describes the function's parameters and result, in the
	// configuration types the host builds the callable function from.
	Signature() FunctionSignature

	// Call calls the function with args, a JSON array holding one value per
	// parameter in the configuration form of its type, and returns the
	// result as JSON in the configuration form of the return type. An error
	// is reported to the configuration author with its message.
	Call(ctx context.Context, args []byte) ([]byte, error)
}

// ProviderFunctions is the set of functions a provider offers, returned by
// its Functions method. Build one with NewFunctions, or return nil from
// Functions when the provider offers none.
type ProviderFunctions interface {
	// Get retrieves a provider function by name.
	// It returns an error if the function does not exist.
	Get(name string) (ProviderFunction, error)

	// Find returns the functions whose names match pattern, a regular
	// expression, e.g. "kubernetes.*".
	Find(pattern string) (map[string]ProviderFunction, error)

	// List returns every function, keyed by name.
	List() (map[string]ProviderFunction, error)
}

// FunctionSignature describes a function offered to configuration: its
// parameters and its result, with each type written in the JSON form of a
// configuration type, e.g. "string", "number", "bool", ["list","string"] or
// ["map","number"]. It is language neutral, so a plugin written in another
// language can produce it, and it is what the host builds the callable
// function from.
type FunctionSignature struct {
	// Params are the parameters, in order
	Params []FunctionParam `json:"params"`

	// Variadic is the parameter any further arguments are collected into,
	// or nil when the function takes a fixed number of arguments. Functions
	// built with NewFunction never have one
	Variadic *FunctionParam `json:"variadic,omitempty"`

	// Return is the result's type
	Return json.RawMessage `json:"return"`
}

// FunctionParam is one parameter of a function offered to configuration
type FunctionParam struct {
	// Name is the parameter's name, for messages
	Name string `json:"name"`

	// Type is the parameter's type, in the JSON form of a configuration
	// type
	Type json.RawMessage `json:"type"`
}

// RegisteredFunction is a function a plugin offers, as the host lists it:
// its bare name and its signature
type RegisteredFunction struct {
	Name      string
	Signature FunctionSignature
}

// NewFunction builds a ProviderFunction from fn, a Go func that takes
// string, bool and number parameters, or slices and string-keyed maps of
// those, and returns a result of such a type and an error:
//
//	plugins.NewFunction(func(title string) (string, error) { ... })
//
// It returns an error naming the problem when fn is not such a func: it is
// variadic, it does not return a result and an error, or a parameter or its
// result has a type configuration cannot carry.
func NewFunction(fn any) (ProviderFunction, error) {
	goFunc, err := functions.NewGoFunc(fn)
	if err != nil {
		return nil, err
	}

	sig, err := signatureOf(goFunc.Signature())
	if err != nil {
		return nil, err
	}

	return &goProviderFunction{fn: goFunc, sig: sig}, nil
}

// signatureOf writes a Go func's signature in its language neutral form
func signatureOf(sig functions.Signature) (FunctionSignature, error) {
	out := FunctionSignature{Params: make([]FunctionParam, len(sig.Params))}

	for i, p := range sig.Params {
		ty, err := ctyjson.MarshalType(p.Type)
		if err != nil {
			return FunctionSignature{}, fmt.Errorf("parameter %d: %w", i, err)
		}

		out.Params[i] = FunctionParam{Name: p.Name, Type: ty}
	}

	ret, err := ctyjson.MarshalType(sig.Return)
	if err != nil {
		return FunctionSignature{}, fmt.Errorf("result: %w", err)
	}

	out.Return = ret

	return out, nil
}

// goProviderFunction is a ProviderFunction backed by a Go func
type goProviderFunction struct {
	fn  *functions.GoFunc
	sig FunctionSignature
}

func (f *goProviderFunction) Signature() FunctionSignature {
	return f.sig
}

func (f *goProviderFunction) Call(ctx context.Context, args []byte) ([]byte, error) {
	return f.fn.CallJSON(args)
}

// NewFunctions builds the ProviderFunctions a provider returns from its
// Functions method, from the functions it offers keyed by their bare names:
//
//	func (p *noteProvider) Functions() plugins.ProviderFunctions {
//		return plugins.NewFunctions(map[string]plugins.ProviderFunction{
//			"note_filename": p.noteFilename,
//		})
//	}
func NewFunctions(fns map[string]ProviderFunction) ProviderFunctions {
	set := providerFunctions{}
	for name, fn := range fns {
		set[name] = fn
	}

	return set
}

// providerFunctions is the ProviderFunctions NewFunctions builds
type providerFunctions map[string]ProviderFunction

func (p providerFunctions) Get(name string) (ProviderFunction, error) {
	fn, ok := p[name]
	if !ok {
		return nil, fmt.Errorf("function %q is not offered by this provider", name)
	}

	return fn, nil
}

func (p providerFunctions) Find(pattern string) (map[string]ProviderFunction, error) {
	matcher, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid function pattern %q: %w", pattern, err)
	}

	found := map[string]ProviderFunction{}
	for name, fn := range p {
		if matcher.MatchString(name) {
			found[name] = fn
		}
	}

	return found, nil
}

func (p providerFunctions) List() (map[string]ProviderFunction, error) {
	all := make(map[string]ProviderFunction, len(p))
	for name, fn := range p {
		all[name] = fn
	}

	return all, nil
}

// sortedFunctionNames returns the names in fns, sorted, so a listing is the
// same on every call
func sortedFunctionNames[T any](fns map[string]T) []string {
	names := make([]string, 0, len(fns))
	for name := range fns {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
