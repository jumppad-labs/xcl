package parser

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/types"
)

// sensitivity describes which parts of a value are sensitive. A nil
// sensitivity is a plain value. When whole is set, the value and everything in
// it is sensitive; otherwise parts names the sensitive parts by attribute,
// map key or list index, and the key "*" stands for every element.
type sensitivity struct {
	whole bool
	parts map[string]*sensitivity
}

var wholeSensitivity = &sensitivity{whole: true}

// part returns the sensitivity of the named part of a value.
func (s *sensitivity) part(name string) *sensitivity {
	if s == nil {
		return nil
	}

	if s.whole {
		return s
	}

	if part, ok := s.parts[name]; ok {
		return part
	}

	return s.parts["*"]
}

// any reports whether any part of the value is sensitive.
func (s *sensitivity) any() bool {
	return s != nil && (s.whole || len(s.parts) > 0)
}

// sensitivityChecker predicts, without evaluating anything, which parts of
// each expression are sensitive. It follows references to sensitive fields,
// to outputs, judged on their own value expressions, and to module inputs,
// judged on the expression the parent module passes in.
type sensitivityChecker struct {
	parser *Parser

	// outputs memoises the sensitivity of each output by its key, and
	// inProgress breaks cycles between outputs and module inputs
	outputs    map[string]*sensitivity
	inProgress map[string]bool
}

// validateSensitive reports every sensitive value, or value derived from one,
// assigned to a field that is not declared sensitive. A field typed any or
// cty.Value holds whatever it is given, such as an output's value or a
// variable's default, so it accepts a sensitive value.
//
// It predicts statically rather than evaluating: functions given unknown
// arguments drop marks, so evaluation could miss a sensitive value. A
// reference keeps the exact parts it reaches; an object or tuple is judged
// element by element; anything that combines values, such as a template, a
// function call or an operator, is wholly sensitive when any value it reads
// is sensitive.
func (p *Parser) validateSensitive() []error {
	checker := &sensitivityChecker{
		parser:     p,
		outputs:    map[string]*sensitivity{},
		inProgress: map[string]bool{},
	}

	problems := []error{}

	for _, id := range p.sortedResourceIDs() {
		resource := p.parsedResources.resources[id]

		meta, err := types.GetMeta(resource)
		if err != nil {
			continue
		}

		// a module's inputs are judged where the module uses them
		if meta.Type == resources.TypeModule {
			continue
		}

		body, ok := p.parsedResources.bodies[id]
		if !ok {
			continue
		}

		problems = append(problems, checker.checkBody(id, meta, body, dereference(reflect.TypeOf(resource)), "")...)
	}

	return problems
}

// checkBody checks each attribute of body against the field of t it sets,
// including inside nested blocks. path is the hcl path of body within the
// entity, ending in a dot when it is not empty.
func (c *sensitivityChecker) checkBody(id string, meta *types.Meta, body *hclsyntax.Body, t reflect.Type, path string) []error {
	problems := []error{}

	fields := map[string]structField{}
	for _, f := range structFields(t) {
		fields[f.name] = f
	}

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

		judged := c.expression(attribute.Expr, meta.Module)

		for _, field := range plainFieldsReached(judged, f.field.Type, path+attribute.Name) {
			problems = append(problems, sensitiveFieldProblem(id, field, attribute.Expr.Range()))
		}
	}

	counts := map[string]int{}
	for _, block := range body.Blocks {
		f, ok := fields[block.Type]
		if !ok {
			continue
		}

		index := counts[block.Type]
		counts[block.Type]++

		element := blockElement(f.field.Type)
		if element == nil {
			continue
		}

		blockPath := path + block.Type
		if kind := f.field.Type.Kind(); kind == reflect.Slice || kind == reflect.Array {
			blockPath = fmt.Sprintf("%s[%d]", blockPath, index)
		}

		problems = append(problems, c.checkBody(id, meta, block.Body, element, blockPath+".")...)
	}

	return problems
}

// plainFieldsReached returns the path of each field of type t, at path, that
// a sensitive part of judged would land in although it is not declared
// sensitive.
func plainFieldsReached(judged *sensitivity, t reflect.Type, path string) []string {
	if !judged.any() {
		return nil
	}

	t = dereference(t)
	if acceptsSensitive(t) {
		return nil
	}

	if judged.whole {
		return []string{path}
	}

	reached := []string{}

	switch t.Kind() {
	case reflect.Struct:
		fields := map[string]structField{}
		for _, f := range structFields(t) {
			fields[f.name] = f
		}

		for _, name := range sortedPartNames(judged) {
			if name == "*" {
				for _, f := range structFields(t) {
					reached = append(reached, plainFieldsReached(judged.parts[name], f.field.Type, path+"."+f.name)...)
				}

				continue
			}

			f, ok := fields[name]
			if !ok {
				continue
			}

			reached = append(reached, plainFieldsReached(judged.parts[name], f.field.Type, path+"."+name)...)
		}

	case reflect.Map, reflect.Slice, reflect.Array:
		for _, name := range sortedPartNames(judged) {
			reached = append(reached, plainFieldsReached(judged.parts[name], t.Elem(), fmt.Sprintf("%s[%s]", path, name))...)
		}

	default:
		reached = append(reached, path)
	}

	return reached
}

// acceptsSensitive reports whether a field of type t may hold a sensitive
// value: a sensitive field, or one that holds anything.
func acceptsSensitive(t reflect.Type) bool {
	return t.Kind() == reflect.Interface || t == ctyValueType || isSensitiveType(t)
}

func sortedPartNames(s *sensitivity) []string {
	names := make([]string, 0, len(s.parts))
	for name := range s.parts {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// sensitiveFieldProblem reports a sensitive value assigned to a plain field
func sensitiveFieldProblem(id string, field string, at hcl.Range) error {
	return errors.NewParserError(
		at.Filename,
		at.Start.Line,
		at.Start.Column,
		fmt.Sprintf("field %q of %s is not declared sensitive and cannot be assigned a sensitive value", field, id),
	)
}

// expression predicts the sensitivity of expr, written in module fromModule.
func (c *sensitivityChecker) expression(expr hclsyntax.Expression, fromModule string) *sensitivity {
	switch e := expr.(type) {
	case *hclsyntax.ScopeTraversalExpr:
		return c.traversal(e, fromModule)

	case *hclsyntax.ObjectConsExpr:
		parts := map[string]*sensitivity{}
		for _, item := range e.Items {
			judged := c.expression(item.ValueExpr, fromModule)
			if !judged.any() {
				continue
			}

			key := objectKey(item.KeyExpr)
			if key == "" {
				// a key computed at run time could be any key
				key = "*"
			}

			parts[key] = judged
		}

		if len(parts) == 0 {
			return nil
		}

		return &sensitivity{parts: parts}

	case *hclsyntax.TupleConsExpr:
		parts := map[string]*sensitivity{}
		for i, item := range e.Exprs {
			if judged := c.expression(item, fromModule); judged.any() {
				parts[strconv.Itoa(i)] = judged
			}
		}

		if len(parts) == 0 {
			return nil
		}

		return &sensitivity{parts: parts}
	}

	// anything that combines values is wholly sensitive when any value it
	// reads is
	for _, traversal := range hclsyntax.Variables(expr) {
		if c.traversal(&hclsyntax.ScopeTraversalExpr{Traversal: traversal}, fromModule).any() {
			return wholeSensitivity
		}
	}

	return nil
}

// traversal predicts the sensitivity of the value a reference reaches.
func (c *sensitivityChecker) traversal(expr *hclsyntax.ScopeTraversalExpr, fromModule string) *sensitivity {
	if len(expr.Traversal) == 0 {
		return nil
	}

	reference, err := processScopeTraversal(expr, c.parser.isReferenceRoot)
	if err != nil || reference == "" {
		return nil
	}

	resolved := c.parser.resolveReference(reference, fromModule)
	if !resolved.found {
		return nil
	}

	return descend(c.target(resolved), attributePath(resolved.attribute))
}

// target predicts the sensitivity of a whole referenced entity.
func (c *sensitivityChecker) target(resolved resolvedReference) *sensitivity {
	switch target := resolved.target.(type) {
	case *types.Output:
		return c.output(resolved.key, target)

	case *resources.Variable:
		return c.moduleInput(target)

	case *resources.Module:
		return nil
	}

	return typeSensitivity(reflect.TypeOf(resolved.target), map[reflect.Type]bool{})
}

// output predicts the sensitivity of an output from its value expression.
func (c *sensitivityChecker) output(key string, output *types.Output) *sensitivity {
	if judged, ok := c.outputs[key]; ok {
		return judged
	}

	if c.inProgress[key] {
		return nil
	}

	c.inProgress[key] = true
	defer delete(c.inProgress, key)

	judged := (*sensitivity)(nil)

	if body, ok := c.parser.parsedResources.bodies[key]; ok {
		if attribute, ok := body.Attributes["value"]; ok {
			judged = c.expression(attribute.Expr, output.Meta.Module)
		}
	}

	c.outputs[key] = judged

	return judged
}

// moduleInput predicts the sensitivity of a variable declared in a module
// from the value its module block passes in. A variable outside any module
// holds its plain default or a value the application supplies.
func (c *sensitivityChecker) moduleInput(variable *resources.Variable) *sensitivity {
	moduleName := variable.Meta.Module
	if moduleName == "" {
		return nil
	}

	key := "module." + moduleName
	if c.inProgress[key+".variable."+variable.Meta.Name] {
		return nil
	}

	moduleBlock, ok := c.parser.parsedResources.resources[key]
	if !ok {
		return nil
	}

	moduleMeta, err := types.GetMeta(moduleBlock)
	if err != nil {
		return nil
	}

	body, ok := c.parser.parsedResources.bodies[key]
	if !ok {
		return nil
	}

	attribute, ok := body.Attributes["variables"]
	if !ok {
		return nil
	}

	inputs, ok := attribute.Expr.(*hclsyntax.ObjectConsExpr)
	if !ok {
		// a module's inputs written as one expression are judged whole
		c.inProgress[key+".variable."+variable.Meta.Name] = true
		defer delete(c.inProgress, key+".variable."+variable.Meta.Name)

		return c.expression(attribute.Expr, moduleMeta.Module).part(variable.Meta.Name)
	}

	for _, item := range inputs.Items {
		if objectKey(item.KeyExpr) != variable.Meta.Name {
			continue
		}

		c.inProgress[key+".variable."+variable.Meta.Name] = true
		defer delete(c.inProgress, key+".variable."+variable.Meta.Name)

		return c.expression(item.ValueExpr, moduleMeta.Module)
	}

	return nil
}

// typeSensitivity returns which parts of a value of Go type t are sensitive
// fields.
func typeSensitivity(t reflect.Type, visiting map[reflect.Type]bool) *sensitivity {
	t = dereference(t)
	if t == nil {
		return nil
	}

	if isSensitiveType(t) {
		return wholeSensitivity
	}

	if visiting[t] {
		return nil
	}
	visiting[t] = true
	defer delete(visiting, t)

	switch t.Kind() {
	case reflect.Struct:
		if t == ctyValueType {
			return nil
		}

		parts := map[string]*sensitivity{}
		for _, f := range structFields(t) {
			if judged := typeSensitivity(f.field.Type, visiting); judged.any() {
				parts[f.name] = judged
			}
		}

		if len(parts) == 0 {
			return nil
		}

		return &sensitivity{parts: parts}

	case reflect.Slice, reflect.Array, reflect.Map:
		if judged := typeSensitivity(t.Elem(), visiting); judged.any() {
			return &sensitivity{parts: map[string]*sensitivity{"*": judged}}
		}
	}

	return nil
}

// descend returns the sensitivity of the part of a value at path.
func descend(judged *sensitivity, path []string) *sensitivity {
	for _, name := range path {
		if !judged.any() {
			return nil
		}

		judged = judged.part(name)
	}

	return judged
}

// attributePath splits a reference's property path, such as
// network[0].name or tags["env"], into its attribute names, keys and indices.
func attributePath(attribute string) []string {
	path := []string{}

	current := strings.Builder{}
	flush := func() {
		if current.Len() > 0 {
			path = append(path, current.String())
			current.Reset()
		}
	}

	for i := 0; i < len(attribute); i++ {
		switch attribute[i] {
		case '.':
			flush()

		case '[':
			flush()

			end := strings.IndexByte(attribute[i:], ']')
			if end < 0 {
				current.WriteString(attribute[i+1:])
				i = len(attribute)

				continue
			}

			key := strings.Trim(attribute[i+1:i+end], `"`)
			path = append(path, key)
			i += end

		default:
			current.WriteByte(attribute[i])
		}
	}

	flush()

	return path
}
