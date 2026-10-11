package coreservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/registry"
)

// A host declares a function by name and signature when it creates a
// configuration. The core turns the signature into a real Go func, made by
// reflection, and offers it through the registry's Functions(), so the
// catalog converts and clash-checks it exactly as a hand-written application
// function: a clash fails NewConfig, arguments are converted the same way,
// and a sensitive argument is unmarked on the way in and marks the result on
// the way out. When configuration calls the function, the made func encodes
// the arguments as a JSON array, posts a FunctionTask under the running
// operation's context, and decodes the result the host completes it with.

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// makeHostFunction builds the registry.Function a DeclaredFunction is
// offered as. A signature that cannot be built is an InvalidFunctionError.
func makeHostFunction(conf *configuration, declared *corepb.DeclaredFunction) (registry.Function, error) {
	name := declared.GetName()
	if name == "" {
		return registry.Function{}, &xclerrors.InvalidFunctionError{Name: name, Registry: registry.LocalName, Err: errors.New("a function must be named")}
	}

	var sig plugins.FunctionSignature
	if err := json.Unmarshal(declared.GetSignature(), &sig); err != nil {
		return registry.Function{}, &xclerrors.InvalidFunctionError{Name: name, Registry: registry.LocalName, Err: fmt.Errorf("signature: %w", err)}
	}

	fn, err := makeHostFunc(conf, name, sig)
	if err != nil {
		return registry.Function{}, &xclerrors.InvalidFunctionError{Name: name, Registry: registry.LocalName, Err: err}
	}

	return registry.Function{Name: name, Func: fn}, nil
}

// makeHostFunc makes the Go func for a signature: one parameter per declared
// parameter, in the Go type its configuration type maps to, returning the
// result type and an error.
func makeHostFunc(conf *configuration, name string, sig plugins.FunctionSignature) (any, error) {
	if sig.Variadic != nil {
		return nil, errors.New("variadic functions are not supported, declare the parameter as a list")
	}

	in := make([]reflect.Type, 0, len(sig.Params))
	for i, p := range sig.Params {
		t, err := goTypeOf(p.Type)
		if err != nil {
			return nil, fmt.Errorf("parameter %d (%s): %w", i+1, p.Name, err)
		}

		in = append(in, t)
	}

	if len(sig.Return) == 0 {
		return nil, errors.New("a function must declare its return type")
	}

	ret, err := goTypeOf(sig.Return)
	if err != nil {
		return nil, fmt.Errorf("return: %w", err)
	}

	call := func(args []reflect.Value) []reflect.Value {
		result, err := callHostFunction(conf, name, args, ret)
		if err != nil {
			failed := reflect.New(errorType).Elem()
			failed.Set(reflect.ValueOf(err))

			return []reflect.Value{reflect.Zero(ret), failed}
		}

		return []reflect.Value{result, reflect.Zero(errorType)}
	}

	return reflect.MakeFunc(reflect.FuncOf(in, []reflect.Type{ret, errorType}, false), call).Interface(), nil
}

// callHostFunction posts one FunctionTask for a call and decodes the result
// the host completes it with into ret.
func callHostFunction(conf *configuration, name string, args []reflect.Value, ret reflect.Type) (reflect.Value, error) {
	values := make([]any, 0, len(args))
	for _, a := range args {
		values = append(values, a.Interface())
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("function %s: arguments: %w", name, err)
	}

	ctx, operationID, err := conf.operationContext()
	if err != nil {
		return reflect.Value{}, fmt.Errorf("function %s cannot be called outside an operation", name)
	}

	completed, err := conf.queue.run(ctx, func(id uint64) *corepb.PollResponse {
		return &corepb.PollResponse{Task: &corepb.PollResponse_Function{Function: &corepb.FunctionTask{
			TaskId:      id,
			OperationId: operationID,
			Name:        name,
			Args:        encoded,
		}}}
	})
	if err != nil {
		return reflect.Value{}, err
	}

	result := completed.GetFunction()
	if result == nil {
		return reflect.Value{}, fmt.Errorf("function %s: the host completed the call without a function result", name)
	}

	if result.GetError() != nil {
		return reflect.Value{}, errors.New(result.GetError().GetMessage())
	}

	out := reflect.New(ret)
	if err := json.Unmarshal(result.GetResult(), out.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("function %s: result is not a %s: %w", name, typeName(ret), err)
	}

	return out.Elem(), nil
}

// goTypeOf maps a configuration type, in its JSON form, to the Go type a
// host function takes or returns: "string", "bool" and "number" (a float64,
// since a configuration number may hold a fraction), ["list", T] and
// ["set", T] as a slice, and ["map", T] as a string-keyed map.
func goTypeOf(raw json.RawMessage) (reflect.Type, error) {
	if len(raw) == 0 {
		return nil, errors.New("no type")
	}

	if raw[0] == '"' {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return nil, err
		}

		switch name {
		case "string":
			return reflect.TypeOf(""), nil
		case "bool":
			return reflect.TypeOf(true), nil
		case "number":
			return reflect.TypeOf(float64(0)), nil
		default:
			return nil, fmt.Errorf("unsupported type %q, a host function takes and returns string, number, bool, and lists, sets and maps of those", name)
		}
	}

	if raw[0] == '[' {
		var parts []json.RawMessage
		if err := json.Unmarshal(raw, &parts); err != nil {
			return nil, err
		}

		if len(parts) != 2 {
			return nil, fmt.Errorf("unsupported type %s", string(raw))
		}

		var kind string
		if err := json.Unmarshal(parts[0], &kind); err != nil {
			return nil, err
		}

		switch kind {
		case "list", "set":
			element, err := goTypeOf(parts[1])
			if err != nil {
				return nil, err
			}

			return reflect.SliceOf(element), nil
		case "map":
			element, err := goTypeOf(parts[1])
			if err != nil {
				return nil, err
			}

			return reflect.MapOf(reflect.TypeOf(""), element), nil
		default:
			return nil, fmt.Errorf("unsupported type %s, a host function takes and returns string, number, bool, and lists, sets and maps of those", string(raw))
		}
	}

	return nil, fmt.Errorf("unsupported type %s", string(raw))
}

// typeName names a Go type as the configuration type it stands for.
func typeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Float64:
		return "number"
	case reflect.Slice:
		return "list of " + typeName(t.Elem())
	case reflect.Map:
		return "map of " + typeName(t.Elem())
	default:
		return t.String()
	}
}
