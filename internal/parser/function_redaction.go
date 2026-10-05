package parser

import (
	"errors"
	"sort"
	"strings"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/cty/function"
	"github.com/jumppad-labs/xcl/types"
)

// redactingFunctions wraps every function in funcs with redactingFunction.
func redactingFunctions(funcs map[string]function.Function) map[string]function.Function {
	wrapped := make(map[string]function.Function, len(funcs))
	for name, fn := range funcs {
		wrapped[name] = redactingFunction(fn)
	}

	return wrapped
}

// redactingFunction wraps fn so that a sensitive argument never appears in an
// error it returns. Functions build their error messages from their
// arguments, so when a call that received a sensitive value fails, the text of
// each sensitive value in the error is replaced by types.SensitiveMarker.
//
// The wrapper accepts marked arguments, calls fn with them unmarked and marks
// the result with every mark the arguments carried, so a value derived from a
// sensitive value is itself sensitive.
func redactingFunction(fn function.Function) function.Function {
	params := fn.Params()
	for i := range params {
		params[i].AllowMarked = true
	}

	varParam := fn.VarParam()
	if varParam != nil {
		varParam.AllowMarked = true
	}

	return function.New(&function.Spec{
		Description: fn.Description(),
		Params:      params,
		VarParam:    varParam,
		Type: func(args []cty.Value) (cty.Type, error) {
			unmarked, _, secrets := unmarkArguments(args)

			returnType, err := fn.ReturnTypeForValues(unmarked)
			if err != nil {
				return cty.NilType, redactError(err, secrets)
			}

			return returnType, nil
		},
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			unmarked, marks, secrets := unmarkArguments(args)

			result, err := fn.Call(unmarked)
			if err != nil {
				return cty.NilVal, redactError(err, secrets)
			}

			return result.WithMarks(marks...), nil
		},
	})
}

// unmarkArguments returns args with every mark removed, the marks they
// carried, and the text of each sensitive value among them.
func unmarkArguments(args []cty.Value) ([]cty.Value, []cty.ValueMarks, []string) {
	unmarked := make([]cty.Value, len(args))
	marks := []cty.ValueMarks{}
	secrets := []string{}

	for i, arg := range args {
		value, pathMarks := arg.UnmarkDeepWithPaths()
		unmarked[i] = value

		for _, pathMark := range pathMarks {
			marks = append(marks, pathMark.Marks)

			if _, sensitive := pathMark.Marks[types.SensitiveMark]; !sensitive {
				continue
			}

			part, err := pathMark.Path.Apply(value)
			if err != nil {
				continue
			}

			secrets = append(secrets, valueTexts(part)...)
		}
	}

	return unmarked, marks, secrets
}

// valueTexts returns the text forms of every primitive value in value, as an
// error message would show them.
func valueTexts(value cty.Value) []string {
	if !value.IsKnown() || value.IsNull() {
		return nil
	}

	switch {
	case value.Type() == cty.String:
		return []string{value.AsString()}

	case value.Type() == cty.Number:
		return []string{value.AsBigFloat().Text('f', -1)}

	case value.Type() == cty.Bool:
		if value.True() {
			return []string{"true"}
		}

		return []string{"false"}

	case value.CanIterateElements():
		texts := []string{}
		for it := value.ElementIterator(); it.Next(); {
			_, element := it.Element()
			texts = append(texts, valueTexts(element)...)
		}

		return texts
	}

	return nil
}

// redactError replaces each secret in err's text with the marker, keeping the
// argument a function error points at.
func redactError(err error, secrets []string) error {
	// the longest first, so a secret holding another is replaced whole
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })

	redacted := err.Error()
	for _, secret := range secrets {
		if secret == "" {
			continue
		}

		redacted = strings.ReplaceAll(redacted, secret, types.SensitiveMarker)
	}

	if redacted == err.Error() {
		return err
	}

	var argError function.ArgError
	if errors.As(err, &argError) {
		return function.NewArgError(argError.Index, errors.New(redacted))
	}

	return errors.New(redacted)
}
