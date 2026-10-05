package parser

import (
	"strings"

	"github.com/jumppad-labs/xcl/internal/resources"
)

// resolvedReference is the outcome of matching a reference string against
// everything the configuration defines.
type resolvedReference struct {
	// target is the resource the reference names, when it was found.
	target any

	// key is the working-set key the reference resolved to, which carries the
	// module scope the reference was resolved in.
	key string

	// attribute is the property path left over once the part naming the
	// resource has been taken off. It is empty when the reference names only a
	// resource. Stage 3 checks this against the target's type.
	attribute string

	// found reports whether the named resource exists anywhere in the
	// configuration.
	found bool
}

// resolveReference answers whether the thing a reference names exists, and
// hands back the property path left over.
//
// References carry a trailing property path while the working-set keys never
// do, so the attribute is stripped *before* the lookup rather than after -
// doing it the other way round is why the existing dependency lookup misses.
//
// A reference made from inside a module is written as though it were at the top
// level, so it is resolved against the referring resource's module scope first
// and against the global scope second. Resolving in that order is what lets a
// module refer to its own resources while still being able to name anything
// declared outside it.
//
// The module-scoped key is composed with FQRN.AppendParentModule, exactly as
// the dependency graph and the evaluation context compose theirs, so a
// reference written inside module a to its child's output, module.b.output.x,
// resolves to module.a.b.output.x rather than to the string-joined
// module.a.module.b.output.x that no entity is ever keyed by.
func (p *Parser) resolveReference(reference string, fromModule string) resolvedReference {
	fqrn, err := p.addressParser().Parse(reference)
	if err != nil {
		// A reference that cannot be parsed names nothing that exists.
		return resolvedReference{found: false}
	}

	base := fqrn.StringWithoutAttribute()

	// A reference from inside a module names things in that module first.
	if fromModule != "" {
		scopedFQRN := fqrn.AppendParentModule(fromModule)
		scoped := scopedFQRN.StringWithoutAttribute()
		if target, ok := p.parsedResources.resources[scoped]; ok {
			return resolvedReference{
				target:    target,
				key:       scoped,
				attribute: fqrn.Attribute,
				found:     true,
			}
		}
	}

	if target, ok := p.parsedResources.resources[base]; ok {
		return resolvedReference{
			target:    target,
			key:       base,
			attribute: fqrn.Attribute,
			found:     true,
		}
	}

	return resolvedReference{attribute: fqrn.Attribute, found: false}
}

// crossesModuleBoundary reports whether a reference, as written, reaches
// inside a module further than the module's outputs allow.
//
// A module's outputs are the only way to reach inside it from configuration.
// The module part of a parsed reference is the path into child modules,
// relative to the scope the reference is written in, so the rule needs nothing
// but the reference itself:
//
//   - no module part, as in resource.container.c or module.a, stays within the
//     scope it is written in and is always allowed;
//   - a single child module whose output is the target, as in
//     module.a.output.x, is allowed;
//   - anything else inside a child module, such as module.a.resource.container.c,
//     module.a.variable.v or the nested module module.a.b, crosses the boundary;
//   - anything inside a grandchild, such as module.a.b.output.x, crosses it too.
//     A value that deep is reached only when each module in between re-exports
//     it as one of its own outputs.
func crossesModuleBoundary(fqrn *resources.FQRN) bool {
	if fqrn.Module == "" {
		return false
	}

	if strings.Contains(fqrn.Module, ".") {
		return true
	}

	return fqrn.Type != resources.TypeOutput
}
