package parser

import (
	"strings"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
)

// processAttribute extracts the resources out of the HCL
// attribute like a function or resource parameter so we can determine
// which attributes are lazy evaluated due to dependency on another resource.
// Attributes can be nested, therefore this function needs to return an array of
// attributes
// examples:
// something = resource.mine.attr
// something = resource.mine.array.0.attr
// something = env(resource.mine.attr)
// something = "${resource.mine.attr}"
// something = "testing/${resource.mine.attr}"
// something = "testing/${env(resource.mine.attr)}"
// something = resource.mine.attr == "abc" ? resource.mine.attr : "abc"
//
// isRoot reports whether a traversal's root names something a reference can
// point at: a builtin keyword, "resource", or the type of a known entity, i.e.
// server in server.big.web.address
func processExpr(expr hclsyntax.Expression, isRoot func(string) bool) ([]string, error) {
	resources := []string{}

	switch ex := expr.(type) {
	// a template is a mix of functions, scope expressions and literals
	// we need to check each part
	case *hclsyntax.TemplateExpr:
		for _, v := range ex.Parts {
			res, err := processExpr(v, isRoot)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
	case *hclsyntax.TemplateWrapExpr:
		res, err := processExpr(ex.Wrapped, isRoot)
		if err != nil {
			return nil, err
		}

		resources = append(resources, res...)

	// function call expressions are user defined functions
	// myfunction(resource.container.base.name)
	case *hclsyntax.FunctionCallExpr:
		for _, v := range ex.Args {
			res, err := processExpr(v, isRoot)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
	// a function can contain args that may also have an expression
	case *hclsyntax.ScopeTraversalExpr:
		ref, err := processScopeTraversal(ex, isRoot)
		if err != nil {
			return nil, err
		}

		// only add if a resource has been returned
		if ref != "" {
			resources = append(resources, ref)
		}

	case *hclsyntax.ObjectConsExpr:
		for _, v := range ex.Items {
			res, err := processExpr(v.ValueExpr, isRoot)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
	case *hclsyntax.TupleConsExpr:
		for _, v := range ex.Exprs {
			res, err := processExpr(v, isRoot)
			if err != nil {
				return nil, err
			}

			resources = append(resources, res...)
		}
	// conditional expressions are like if statements
	// resource.container.base.name == "hello" ? "this" : "that"
	case *hclsyntax.ConditionalExpr:
		conditions, err := processExpr(ex.Condition, isRoot)
		if err != nil {
			return nil, err
		}
		resources = append(resources, conditions...)

		trueResults, err := processExpr(ex.TrueResult, isRoot)
		if err != nil {
			return nil, err
		}
		resources = append(resources, trueResults...)

		falseResults, err := processExpr(ex.FalseResult, isRoot)
		if err != nil {
			return nil, err
		}
		resources = append(resources, falseResults...)
	// binary expressions are two part comparisons
	// resource.container.base.name == "hello"
	// resource.container.base.name != "hello"
	// resource.container.base.name > 3
	case *hclsyntax.BinaryOpExpr:
		lhs, err := processExpr(ex.LHS, isRoot)
		if err != nil {
			return nil, err
		}
		resources = append(resources, lhs...)

		rhs, err := processExpr(ex.RHS, isRoot)
		if err != nil {
			return nil, err
		}
		resources = append(resources, rhs...)
	case *hclsyntax.SplatExpr:
		ref, err := processExpr(ex.Source, isRoot)
		if err != nil {
			return nil, err
		}

		// only add if a resource has been returned
		if len(ref) > 0 {
			resources = append(resources, ref...)
		}
	// unary expressions are single operand operations
	// disabled = !variable.enabled
	case *hclsyntax.UnaryOpExpr:
		val, err := processExpr(ex.Val, isRoot)
		if err != nil {
			return nil, err
		}
		resources = append(resources, val...)
	}

	return resources, nil
}

func processScopeTraversal(expr *hclsyntax.ScopeTraversalExpr, isRoot func(string) bool) (string, error) {
	strExpression := ""
	for i, t := range expr.Traversal {
		if i == 0 {
			strExpression += t.(hcl.TraverseRoot).Name

			// if this is not a reference to an entity quit
			if !isRoot(strExpression) {
				return "", nil
			}
		} else {
			// does this exist in the context
			switch tt := t.(type) {
			case hcl.TraverseAttr:
				strExpression += "." + tt.Name
			case hcl.TraverseIndex:
				// Handle both string and numeric indices
				switch tt.Key.Type() {
				case cty.String:
					strExpression += "[\"" + tt.Key.AsString() + "\"]"
				case cty.Number:
					strExpression += "[" + tt.Key.AsBigFloat().String() + "]"
				default:
					// For other types, use the bigfloat representation as fallback
					strExpression += "[" + tt.Key.AsBigFloat().String() + "]"
				}
			}
		}
	}

	// add to the references collection and replace with a nil value
	// we will resolve these references before processing
	return strExpression, nil
}

// recordWrittenText records in written, under path, the text the author wrote
// after the = of attr, exactly as written: everything from the equals sign to
// the end of the attribute, without the surrounding whitespace. The bytes are
// sliced from src rather than taken from the expression's own range, because
// some expressions' ranges do not cover all that was written, such as
// wrapping parentheses. Nothing is recorded when src does not hold the
// attribute.
func recordWrittenText(written map[string]string, path string, attr *hclsyntax.Attribute, src []byte) {
	start := attr.EqualsRange.End.Byte
	end := attr.SrcRange.End.Byte

	if src == nil || start < 0 || end > len(src) || start >= end {
		return
	}

	text := strings.TrimSpace(string(src[start:end]))
	if text == "" {
		return
	}

	written[path] = text
}
