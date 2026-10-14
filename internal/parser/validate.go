package parser

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/types"
)

// validate runs the validation stages over a parsed configuration and returns
// every problem found rather than stopping at the first.
//
// Validation runs to completion between parsing and walking. It reads the
// working set of parsed resources and reports what is wrong with them; it never
// resolves values, decodes bodies, or reaches a provider. An empty result means
// the configuration is valid.
//
// The stages run in order - structure, then references, then sensitivity, then
// properties - and each gathers all of its own findings before the next is
// considered. Stopping between stages is deliberate: once references are known
// to be broken, checking properties on them would report confusing
// consequences of a problem that has already been reported.
func (p *Parser) validate() []error {
	problems := []error{}

	// Stage 1: structure. Resources that could not be understood well enough to
	// check anything else about them.
	problems = append(problems, p.validateStructure()...)
	if len(problems) > 0 {
		return problems
	}

	// Stage 2: references. Everything a resource refers to must be defined
	// somewhere in the configuration, and may reach into a module only through
	// one of that module's outputs.
	problems = append(problems, p.validateReferences()...)
	if len(problems) > 0 {
		return problems
	}

	// Stage 3: sensitivity. A sensitive value, or one derived from it, may
	// only be assigned to a field declared sensitive.
	problems = append(problems, p.validateSensitive()...)
	if len(problems) > 0 {
		return problems
	}

	// Stage 4: properties. Every reference that resolves must name properties
	// its target's type actually has.
	problems = append(problems, p.validateProperties()...)

	return problems
}

// validateProperties reports every reference whose trailing property path names
// something its target's type does not have.
//
// It reports at most one problem per reference - the first segment that does
// not exist - because once a segment is unknown the type beyond it is unknown
// too, so later segments cannot be judged. Exhaustiveness is across references
// rather than within a single path.
func (p *Parser) validateProperties() []error {
	problems := []error{}

	for _, id := range p.sortedResourceIDs() {
		resource := p.parsedResources.resources[id]

		meta, err := types.GetMeta(resource)
		if err != nil {
			continue
		}

		for _, link := range meta.Links {
			resolved := p.resolveReference(link, meta.Module)
			if !resolved.found || resolved.attribute == "" {
				continue
			}

			missing := checkPropertyPath(reflect.TypeOf(resolved.target), resolved.attribute)
			if missing == "" {
				continue
			}

			problems = append(problems, p.parsedResources.inEntity(id, p.referenceAt(id, link), problem{
				summary: "Unsupported attribute",
				detail:  propertyProblem(link, missing),
			}))
		}
	}

	return problems
}

// validateReferences reports every reference in the configuration that names
// something defined nowhere, and every reference that crosses a module
// boundary. It reports all of them rather than the first, so an author is not
// made to fix one and rerun to discover the next.
//
// A module's outputs are the only way to reach inside it: a reference to
// anything else inside a module, or to anything inside a module nested in it,
// is a problem whether or not its target exists. The boundary is judged first,
// so such a reference is reported once, as crossing the boundary, and never
// also as undefined. User written depends_on entries are among the links, so
// they are held to the same rule.
//
// Problems are reported in the resource that holds the reference, at the
// reference as written.
func (p *Parser) validateReferences() []error {
	problems := []error{}

	// Sort the resource keys so that problems are reported in a stable order.
	// The working set is a map, so iterating it directly would report the same
	// faults in a different order on every run.
	for _, id := range p.sortedResourceIDs() {
		resource := p.parsedResources.resources[id]

		meta, err := types.GetMeta(resource)
		if err != nil {
			continue
		}

		for _, link := range meta.Links {
			fqrn, parseErr := p.addressParser().Parse(link)
			if parseErr == nil && crossesModuleBoundary(fqrn) {
				problems = append(problems, p.parsedResources.inEntity(id, p.referenceAt(id, link), problem{
					summary: "Reference into a module",
					detail:  fmt.Sprintf("resource '%s' refers to '%s', which is inside module '%s'; only a module's outputs can be referenced from outside it", meta.ID, link, fqrn.Module),
				}))

				continue
			}

			if p.resolveReference(link, meta.Module).found {
				continue
			}

			problems = append(problems, p.parsedResources.inEntity(id, p.referenceAt(id, link), problem{
				summary: "Reference to an undefined entity",
				detail:  fmt.Sprintf("resource '%s' refers to '%s', which is not defined anywhere in the configuration", meta.ID, link),
			}))
		}
	}

	return problems
}

// sortedResourceIDs returns the working set's keys in a stable order.
func (p *Parser) sortedResourceIDs() []string {
	ids := make([]string, 0, len(p.parsedResources.resources))
	for id := range p.parsedResources.resources {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	return ids
}

// validateStructure checks that each parsed resource is sound enough to be
// checked further. Problems found during parsing have already been reported by
// the time validation runs. This stage reports computed fields: a computed field
// that is not optional, which no configuration could satisfy, and every computed
// field set in configuration, since only the provider may set one. It also checks
// each resource body against the schema of its type, reporting attributes and
// blocks the type does not have, before any body is decoded.
func (p *Parser) validateStructure() []error {
	problems := []error{}

	for _, id := range p.sortedResourceIDs() {
		resource := p.parsedResources.resources[id]

		if _, err := types.GetMeta(resource); err != nil {
			continue
		}

		resourceType := dereference(reflect.TypeOf(resource))

		for _, computed := range computedFields(resourceType) {
			if isOptional(computed.field) {
				continue
			}

			problems = append(problems, p.parsedResources.inEntity(id, nil, problem{
				summary: "Invalid type",
				detail:  fmt.Sprintf("resource '%s' field '%s' is computed and must be optional", id, computed.path),
			}))
		}

		body, ok := p.parsedResources.bodies[id]
		if !ok {
			continue
		}

		problems = append(problems, p.parsedResources.configuredComputedFields(id, body, resourceType, "")...)
		problems = append(problems, p.schemaProblems(id, body, resource)...)
	}

	return problems
}

// schemaProblems checks body against the schema implied by the type of
// resource, without evaluating anything, and reports every problem decoding
// would find with its shape, each as the diagnostic HCL gave, in the
// resource id names and at the field it is about
func (p *Parser) schemaProblems(id string, body *hclsyntax.Body, resource any) []error {
	problems := []error{}

	for _, d := range gohcl.CheckBody(body, resource) {
		if d.Severity != hcl.DiagError {
			continue
		}

		problem := errors.NewDiagnostic(d, p.parsedResources.sources)
		problem.Entity = id
		if header, ok := p.parsedResources.headers[id]; ok {
			problem.EntityPos = &header
		}
		if d.Subject != nil {
			problem.Field = fieldAt(body, d.Subject.Start, "")
		}

		problems = append(problems, problem)
	}

	return problems
}

// fieldAt returns the path of the attribute or nested block in body that pos
// falls in, such as image or port[0].host, or empty when it falls in none,
// such as a missing attribute reported at the body's opening brace. prefix is
// the path of body itself, ending in a dot when it is not empty.
func fieldAt(body *hclsyntax.Body, pos hcl.Pos, prefix string) string {
	for name, attribute := range body.Attributes {
		if attribute.SrcRange.ContainsPos(pos) {
			return prefix + name
		}
	}

	seen := map[string]int{}
	for _, block := range body.Blocks {
		index := seen[block.Type]
		seen[block.Type]++

		if !block.Range().ContainsPos(pos) {
			continue
		}

		path := nestedBlockPath(prefix, block, index, nil)
		if inner := fieldAt(block.Body, pos, path+"."); inner != "" {
			return inner
		}

		return path
	}

	return ""
}

// configuredComputedFields reports every computed field of t set in body,
// including inside nested blocks and object values. path is the hcl path of
// body within the resource, ending in a dot when it is not empty.
func (p *parsed) configuredComputedFields(id string, body *hclsyntax.Body, t reflect.Type, path string) []error {
	problems := []error{}

	fields := map[string]structField{}
	for _, f := range structFields(t) {
		fields[f.name] = f
	}

	// report attributes in the order they are written
	attributes := make([]*hclsyntax.Attribute, 0, len(body.Attributes))
	for _, attribute := range body.Attributes {
		attributes = append(attributes, attribute)
	}

	sort.Slice(attributes, func(i, j int) bool {
		return attributes[i].NameRange.Start.Byte < attributes[j].NameRange.Start.Byte
	})

	for _, attribute := range attributes {
		f, ok := fields[attribute.Name]
		if !ok {
			continue
		}

		if isComputed(f.field) {
			problems = append(problems, p.computedFieldProblem(id, path+attribute.Name, attribute.NameRange))
			continue
		}

		problems = append(problems, p.configuredComputedValues(id, attribute.Expr, f.field.Type, path+attribute.Name)...)
	}

	counts := map[string]int{}
	for _, block := range body.Blocks {
		f, ok := fields[block.Type]
		if !ok {
			continue
		}

		index := counts[block.Type]
		counts[block.Type]++

		blockPath := path + block.Type
		if name, named := mapBlockName(block, f.field.Type); named {
			blockPath = fmt.Sprintf("%s[%q]", blockPath, name)
		} else if kind := f.field.Type.Kind(); kind == reflect.Slice || kind == reflect.Array {
			blockPath = fmt.Sprintf("%s[%d]", blockPath, index)
		}

		if isComputed(f.field) {
			problems = append(problems, p.computedFieldProblem(id, blockPath, block.TypeRange))
			continue
		}

		element := blockElement(f.field.Type)
		if element == nil {
			continue
		}

		problems = append(problems, p.configuredComputedFields(id, block.Body, element, blockPath+".")...)
	}

	return problems
}

// configuredComputedValues reports every computed field set inside an object or
// tuple expression assigned to a field of type t
func (p *parsed) configuredComputedValues(id string, expr hclsyntax.Expression, t reflect.Type, path string) []error {
	problems := []error{}

	t = dereference(t)

	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		tuple, ok := expr.(*hclsyntax.TupleConsExpr)
		if !ok {
			return problems
		}

		for i, item := range tuple.Exprs {
			problems = append(problems, p.configuredComputedValues(id, item, t.Elem(), fmt.Sprintf("%s[%d]", path, i))...)
		}

	case reflect.Map:
		object, ok := expr.(*hclsyntax.ObjectConsExpr)
		if !ok {
			return problems
		}

		for _, item := range object.Items {
			key := objectKey(item.KeyExpr)
			problems = append(problems, p.configuredComputedValues(id, item.ValueExpr, t.Elem(), fmt.Sprintf("%s[%s]", path, key))...)
		}

	case reflect.Struct:
		if blockElement(t) == nil {
			return problems
		}

		object, ok := expr.(*hclsyntax.ObjectConsExpr)
		if !ok {
			return problems
		}

		fields := map[string]structField{}
		for _, f := range structFields(t) {
			fields[f.name] = f
		}

		for _, item := range object.Items {
			name := objectKey(item.KeyExpr)

			f, ok := fields[name]
			if !ok {
				continue
			}

			if isComputed(f.field) {
				problems = append(problems, p.computedFieldProblem(id, path+"."+name, item.KeyExpr.Range()))
				continue
			}

			problems = append(problems, p.configuredComputedValues(id, item.ValueExpr, f.field.Type, path+"."+name)...)
		}
	}

	return problems
}

// objectKey returns the name of an object key, written either as a bare
// keyword or as a string
func objectKey(expr hclsyntax.Expression) string {
	if keyword := hcl.ExprAsKeyword(expr); keyword != "" {
		return keyword
	}

	value, diags := expr.Value(nil)
	if diags.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
		return ""
	}

	return value.AsString()
}

// computedFieldProblem reports a computed field set in configuration
func (p *parsed) computedFieldProblem(id string, path string, at hcl.Range) error {
	return p.inEntity(id, &at, problem{
		summary: "Computed field set",
		detail:  fmt.Sprintf("resource '%s' sets computed field '%s', computed fields are set by the provider and cannot be configured", id, path),
	})
}
