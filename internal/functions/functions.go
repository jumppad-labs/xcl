package functions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/infinytum/raymond/v2"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/cty/function"
	"github.com/jumppad-labs/xcl/internal/cty/function/stdlib"
	"github.com/jumppad-labs/xcl/internal/cty/gocty"
	ctyjson "github.com/jumppad-labs/xcl/internal/cty/json"
	"github.com/jumppad-labs/xcl/internal/utils"
)

// Parameter is one parameter of a Go func offered to configuration: its
// configuration type and the Go type it is decoded into.
type Parameter struct {
	Name   string
	Type   cty.Type
	GoType reflect.Type
}

// Signature describes a Go func offered to configuration: the configuration
// type of each parameter and of the result. It is the one description of a
// func both the application and the plugin sides of functions are built
// from, so a func behaves the same in process and over a plugin boundary.
type Signature struct {
	Params       []Parameter
	Return       cty.Type
	ReturnGoType reflect.Type
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

// GoFuncSignature returns the signature of fn, a Go func that takes string,
// bool and number parameters, or slices and string-keyed maps of those, and
// returns a result of such a type and an error, i.e. func(a, b int) (int,
// error). Anything else is returned as an error naming the problem.
func GoFuncSignature(fn any) (Signature, error) {
	rt := reflect.TypeOf(fn)
	if rt == nil || rt.Kind() != reflect.Func {
		return Signature{}, fmt.Errorf("a Go func is required, not %T", fn)
	}

	if rt.IsVariadic() {
		return Signature{}, fmt.Errorf("variadic functions are not supported, declare the parameter as a slice")
	}

	if rt.NumOut() != 2 || !rt.Out(1).Implements(errorInterface) {
		return Signature{}, fmt.Errorf("functions must return a result and an error, i.e. func(a, b int) (int, error)")
	}

	sig := Signature{}

	for i := 0; i < rt.NumIn(); i++ {
		in := rt.In(i)

		ty, err := configurationType(in)
		if err != nil {
			return Signature{}, fmt.Errorf("parameter %d: %w", i, err)
		}

		sig.Params = append(sig.Params, Parameter{
			Name:   fmt.Sprintf("arg%d", i),
			Type:   ty,
			GoType: in,
		})
	}

	ret, err := configurationType(rt.Out(0))
	if err != nil {
		return Signature{}, fmt.Errorf("result: %w", err)
	}

	sig.Return = ret
	sig.ReturnGoType = rt.Out(0)

	return sig, nil
}

// configurationType returns the configuration type a Go type is carried as:
// a string, a bool, a number for every int, uint and float kind, a list for a
// slice and a map for a string-keyed map, of those recursively.
func configurationType(t reflect.Type) (cty.Type, error) {
	switch t.Kind() {
	case reflect.String:
		return cty.String, nil
	case reflect.Bool:
		return cty.Bool, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return cty.Number, nil
	case reflect.Slice:
		elem, err := configurationType(t.Elem())
		if err != nil {
			return cty.NilType, err
		}

		return cty.List(elem), nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return cty.NilType, fmt.Errorf("type %s is not supported, a map must have string keys", t)
		}

		elem, err := configurationType(t.Elem())
		if err != nil {
			return cty.NilType, err
		}

		return cty.Map(elem), nil
	default:
		return cty.NilType, fmt.Errorf("type %s is not supported, use string, bool, a number type, or a slice or string-keyed map of those", t)
	}
}

// GoFunc is a Go func offered to configuration, with its signature. It is
// called with configuration values in process, and with JSON arguments
// across a plugin boundary, so the same func behaves the same either way.
type GoFunc struct {
	callable reflect.Value
	sig      Signature
}

// NewGoFunc wraps fn, a Go func with a signature GoFuncSignature accepts.
func NewGoFunc(fn any) (*GoFunc, error) {
	sig, err := GoFuncSignature(fn)
	if err != nil {
		return nil, err
	}

	return &GoFunc{callable: reflect.ValueOf(fn), sig: sig}, nil
}

// Signature returns the func's signature.
func (g *GoFunc) Signature() Signature {
	return g.sig
}

// Call calls the func with args, one configuration value per parameter,
// converting each to the parameter's Go type and the result back to its
// configuration type. An error the func returns is returned as it is.
func (g *GoFunc) Call(args []cty.Value) (cty.Value, error) {
	if len(args) != len(g.sig.Params) {
		return cty.NilVal, fmt.Errorf("%d arguments given, %d expected", len(args), len(g.sig.Params))
	}

	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		target := reflect.New(g.sig.Params[i].GoType)
		if err := gocty.FromCtyValue(arg, target.Interface()); err != nil {
			return cty.NilVal, fmt.Errorf("argument %d: %w", i, err)
		}

		in[i] = target.Elem()
	}

	out := g.callable.Call(in)

	if callErr, _ := out[1].Interface().(error); callErr != nil {
		return cty.NullVal(g.sig.Return), callErr
	}

	result, err := gocty.ToCtyValue(out[0].Interface(), g.sig.Return)
	if err != nil {
		return cty.NilVal, fmt.Errorf("result: %w", err)
	}

	return result, nil
}

// CallJSON calls the func with args, a JSON array holding one value per
// parameter in the configuration form of its type, and returns the result
// as JSON in the configuration form of its type. It is how a function is
// called across a plugin boundary.
func (g *GoFunc) CallJSON(args []byte) ([]byte, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return nil, fmt.Errorf("arguments must be a JSON array: %w", err)
	}

	if len(raw) != len(g.sig.Params) {
		return nil, fmt.Errorf("%d arguments given, %d expected", len(raw), len(g.sig.Params))
	}

	values := make([]cty.Value, len(raw))
	for i, arg := range raw {
		value, err := ctyjson.Unmarshal(arg, g.sig.Params[i].Type)
		if err != nil {
			return nil, fmt.Errorf("argument %d: %w", i, err)
		}

		values[i] = value
	}

	result, err := g.Call(values)
	if err != nil {
		return nil, err
	}

	encoded, err := ctyjson.Marshal(result, g.sig.Return)
	if err != nil {
		return nil, fmt.Errorf("result: %w", err)
	}

	return encoded, nil
}

// CreateCtyFunctionFromGoFunc turns fn, a Go func with a signature
// GoFuncSignature accepts, into a function configuration can call. Each
// argument is converted to the parameter's Go type and the result back to
// its configuration type; an error fn returns is returned from the call.
func CreateCtyFunctionFromGoFunc(fn any) (function.Function, error) {
	goFunc, err := NewGoFunc(fn)
	if err != nil {
		return function.Function{}, err
	}

	return CtyFunctionFromSignature(goFunc.Signature(), goFunc.Call), nil
}

// CtyFunctionFromSignature builds a function configuration can call from a
// signature and the implementation to call with the converted arguments. It
// is shared by application functions, which call a Go func in process, and
// plugin functions, which call across the plugin boundary.
func CtyFunctionFromSignature(sig Signature, impl func(args []cty.Value) (cty.Value, error)) function.Function {
	params := make([]function.Parameter, len(sig.Params))
	for i, p := range sig.Params {
		params[i] = function.Parameter{
			Name:             p.Name,
			Type:             p.Type,
			AllowDynamicType: true,
		}
	}

	return function.New(&function.Spec{
		Params: params,
		Type:   function.StaticReturnType(sig.Return),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return impl(args)
		},
	})
}

// builtinNames holds the names of the built-in functions, read once
var (
	builtinNamesOnce sync.Once
	builtinNames     []string
)

// BuiltinNames returns the names of the built-in functions, sorted, so a
// caller can refuse a function that would shadow one.
func BuiltinNames() []string {
	builtinNamesOnce.Do(func() {
		for name := range GetDefaultFunctions("") {
			builtinNames = append(builtinNames, name)
		}

		sort.Strings(builtinNames)
	})

	return append([]string{}, builtinNames...)
}

func GetDefaultFunctions(filePath string) map[string]function.Function {
	var EnvFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "env",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(os.Getenv(args[0].AsString())), nil
		},
	})

	var HomeFunc = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			h, _ := os.UserHomeDir()
			return cty.StringVal(h), nil
		},
	})

	var ReadFileFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "path",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			// convert the file path to an absolute
			fp := utils.EnsureAbsolute(args[0].AsString(), filePath)

			// read the contents of the file
			d, err := os.ReadFile(fp)
			if err != nil {
				return cty.StringVal(""), err
			}

			return cty.StringVal(string(d)), nil
		},
	})

	var ReadTemplateFileFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "path",
				Type:             cty.String,
				AllowDynamicType: true,
			},
			{
				Name:             "variables",
				Type:             cty.DynamicPseudoType,
				AllowUnknown:     true,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			// convert the file path to an absolute
			fp := utils.EnsureAbsolute(args[0].AsString(), filePath)

			// read the contents of the file
			d, err := os.ReadFile(fp)
			if err != nil {
				return cty.StringVal(""), err
			}

			vars := args[1]
			if vars.IsNull() || !vars.Type().IsObjectType() {
				return cty.StringVal(""), fmt.Errorf(`variables is either empty or not correctly formatted, e.g. { foo = "bar" list = ["a", "b"] number = 3 }`)
			}

			variables := utils.ParseVars(vars.AsValueMap())

			tmpl, err := raymond.Parse(string(d))
			if err != nil {
				return cty.StringVal(""), fmt.Errorf("error parsing template: %s", err)
			}

			tmpl.RegisterHelpers(map[string]any{
				"quote": func(in string) string {
					return fmt.Sprintf(`"%s"`, in)
				},
				"trim": func(in string) string {
					return strings.TrimSpace(in)
				},
			})

			result, err := tmpl.Exec(variables)
			if err != nil {
				return cty.StringVal(""), fmt.Errorf("error processing template: %s", err)
			}

			return cty.StringVal(result), nil
		},
	})

	var LenFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "var",
				Type:             cty.DynamicPseudoType,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.Number),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if len(args) == 1 && args[0].Type().IsCollectionType() || args[0].Type().IsTupleType() {
				i := args[0].ElementIterator()
				if i.Next() {
					return args[0].Length(), nil
				}
			}

			if len(args) == 1 && args[0].Type() == cty.String {
				return cty.NumberIntVal(int64(len(args[0].AsString()))), nil
			}

			return cty.NumberIntVal(0), nil
		},
	})

	var DirFunc = function.New(&function.Spec{
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			s, err := filepath.Abs(filePath)

			// check if filepath is already a directory
			if stat, err := os.Stat(s); err == nil && stat.IsDir() {
				return cty.StringVal(s), err
			}

			return cty.StringVal(filepath.Dir(s)), err
		},
	})

	var TrimFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "string",
				Type:             cty.String,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(strings.TrimSpace(args[0].AsString())), nil
		},
	})

	var ElementFunc = function.New(&function.Spec{
		Params: []function.Parameter{
			{
				Name:             "value",
				Type:             cty.DynamicPseudoType,
				AllowDynamicType: true,
			},
			{
				Name:             "index",
				Type:             cty.DynamicPseudoType,
				AllowDynamicType: true,
			},
		},
		Type: function.StaticReturnType(cty.DynamicPseudoType),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			if args[0].Type().IsTupleType() || args[0].Type().IsListType() {
				i := args[0].ElementIterator()

				for {
					if !i.Next() {
						break
					}

					index, e := i.Element()
					if index.Equals(args[1]).True() {
						return e, nil
					}
				}

				return cty.NullVal(retType), nil
			} else if args[1].Type() == cty.String && (args[0].Type().IsObjectType() || args[0].Type().IsMapType()) {
				index := args[1].AsString()
				m := args[0].AsValueMap()

				return m[index], nil
			}

			return cty.NullVal(retType), nil
		},
	})

	//var ToString = function.New(&function.Spec{
	//	Params: []function.Parameter{
	//		{
	//			Name:             "value",
	//			Type:             cty.DynamicPseudoType,
	//			AllowDynamicType: true,
	//		},
	//	},
	//	Type: function.StaticReturnType(cty.DynamicPseudoType),
	//	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
	//		if args[0].Type().IsTupleType() || args[0].Type().IsListType() {
	//			vals := []cty.Value{}
	//			i := args[0].ElementIterator()

	//			for {
	//				if !i.Next() {
	//					break
	//				}

	//				, e := i.Element()
	//			}

	//		return cty.NullVal(retType), nil
	//	},
	//})

	funcs := map[string]function.Function{
		"abs":             stdlib.AbsoluteFunc,
		"ceil":            stdlib.CeilFunc,
		"chomp":           stdlib.ChompFunc,
		"coalescelist":    stdlib.CoalesceListFunc,
		"compact":         stdlib.CompactFunc,
		"concat":          stdlib.ConcatFunc,
		"contains":        stdlib.ContainsFunc,
		"csvdecode":       stdlib.CSVDecodeFunc,
		"dir":             DirFunc,
		"distinct":        stdlib.DistinctFunc,
		"element":         ElementFunc,
		"env":             EnvFunc,
		"chunklist":       stdlib.ChunklistFunc,
		"file":            ReadFileFunc,
		"flatten":         stdlib.FlattenFunc,
		"floor":           stdlib.FloorFunc,
		"format":          stdlib.FormatFunc,
		"formatdate":      stdlib.FormatDateFunc,
		"formatlist":      stdlib.FormatListFunc,
		"home":            HomeFunc,
		"indent":          stdlib.IndentFunc,
		"join":            stdlib.JoinFunc,
		"jsondecode":      stdlib.JSONDecodeFunc,
		"jsonencode":      stdlib.JSONEncodeFunc,
		"keys":            stdlib.KeysFunc,
		"len":             LenFunc,
		"log":             stdlib.LogFunc,
		"lower":           stdlib.LowerFunc,
		"max":             stdlib.MaxFunc,
		"merge":           stdlib.MergeFunc,
		"min":             stdlib.MinFunc,
		"parseint":        stdlib.ParseIntFunc,
		"pow":             stdlib.PowFunc,
		"range":           stdlib.RangeFunc,
		"regex":           stdlib.RegexFunc,
		"regexall":        stdlib.RegexAllFunc,
		"reverse":         stdlib.ReverseListFunc,
		"setintersection": stdlib.SetIntersectionFunc,
		"setproduct":      stdlib.SetProductFunc,
		"setsubtract":     stdlib.SetSubtractFunc,
		"setunion":        stdlib.SetUnionFunc,
		"signum":          stdlib.SignumFunc,
		"slice":           stdlib.SliceFunc,
		"sort":            stdlib.SortFunc,
		"split":           stdlib.SplitFunc,
		"strrev":          stdlib.ReverseFunc,
		"substr":          stdlib.SubstrFunc,
		"template_file":   ReadTemplateFileFunc,
		"timeadd":         stdlib.TimeAddFunc,
		"title":           stdlib.TitleFunc,
		"trim":            TrimFunc,
		"trimprefix":      stdlib.TrimPrefixFunc,
		"trimspace":       stdlib.TrimSpaceFunc,
		"trimsuffix":      stdlib.TrimSuffixFunc,
		"upper":           stdlib.UpperFunc,
		"values":          stdlib.ValuesFunc,
		"zipmap":          stdlib.ZipmapFunc,
	}

	return funcs
}
