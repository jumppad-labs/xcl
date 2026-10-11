package parser

import (
	"strings"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
)

// referencesIn returns the entity references an expression holds, in the
// order they are written, so the resource can be linked to each one before it
// is evaluated. Every node of the expression is walked, so a reference is
// found wherever it sits: inside a template, a function call, an object or
// tuple, a conditional, an operator, a splat, an index expression, a
// parenthesised expression, a relative traversal or a for expression. The key
// and value names a for expression declares are local to it and are never
// references, so they are ignored.
//
// examples:
// something = resource.mine.attr
// something = resource.mine.array.0.attr
// something = env(resource.mine.attr)
// something = "${resource.mine.attr}"
// something = "testing/${resource.mine.attr}"
// something = resource.mine.attr == "abc" ? resource.mine.attr : "abc"
// something = { for name, l in app.web.listener : name => l.port }
//
// isRoot reports whether a traversal's root names something a reference can
// point at: a builtin keyword, "resource", or the type of a known entity, i.e.
// server in server.big.web.address
func referencesIn(expr hclsyntax.Expression, isRoot func(string) bool) ([]string, error) {
	references := []string{}

	for _, traversal := range hclsyntax.Variables(expr) {
		scope := &hclsyntax.ScopeTraversalExpr{
			Traversal: traversal,
			SrcRange:  traversal.SourceRange(),
		}

		reference, err := processScopeTraversal(scope, isRoot)
		if err != nil {
			return nil, err
		}

		// only a traversal rooted at an entity is a reference
		if reference != "" {
			references = append(references, reference)
		}
	}

	return references, nil
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
