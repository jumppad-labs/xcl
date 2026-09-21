package resources

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jumppad-labs/xcl/types"
)

// FQRN is the fully qualified resource name
type FQRN struct {
	// Name of the module
	Module string
	// Type is the kind of stanza the resource was declared by, i.e. the
	// "resource" in resource.container.mine
	Type string
	// Subtype is the variety of a resource kind address, i.e. the "container"
	// in resource.container.mine. It is empty for the single label kinds
	Subtype string
	// Resource name
	Resource string
	// Attribute for the resource
	Attribute string
}

// AddressType returns the segment an address carries between its kind and its
// name: the variety where there is one, and the kind itself otherwise. A bare
// "local" has no Go type and no variety, so it formats through the kind, which
// is what keeps local.x rendering as it always has
func (f FQRN) AddressType() string {
	if f.Subtype != "" {
		return f.Subtype
	}

	return f.Type
}

// ParseFQRN parses a "resource" fqrn and returns the individual components
// e.g:
//
// get the "resource" container called mine that is in the root "module"
// which also has "attributes"
// // resource.container.mine.property.value
//
// get the "resource" container called mine that is in the root "module"
// // resource.container.mine
//
// get the "output" called mine that is in the root "module"
// // output.mine
//
// get the "local" called mine that is in the root "module"
// // local.mine
//
// get the container "resource" called mine in the "module" module2 that
// is in the "module" module1
// // module1.module2.resource.container.mine
//
// get the "output" called mine in the "module" module2 that is in the
// "module" module1
// // module1.module2.output.mine
//
// get the "module" resource called module2 in the "module" module1
// // module1.module2
//
// get the "module" resource called module1 in the root "module"
// // module1
func ParseFQRN(fqrn string) (*FQRN, error) {
	return parseAddress(fqrn, structuralKeywords)
}

// structuralKeywords are the leading keywords that always begin an address
// body, whatever types are registered. They are what ParseFQRN resolves with
// when no registry is at hand.
var structuralKeywords = map[string]bool{
	types.TypeResource: true,
	TypeOutput:         true,
	TypeVariable:       true,
	TypeModule:         true,
	"local":            true,
}

// AddressParser parses addresses against a known set of types. It exists
// because a module relative address cannot be split by position alone: in
// module.a.b.c, "b" is a module name when nothing is registered under it and
// the start of the body when something is. Only the set of known types
// settles it, so the parser is constructed with them rather than reaching for
// a registry it must not depend on.
type AddressParser struct {
	keywords map[string]bool
}

// NewAddressParser returns a parser that recognises the given types in
// addition to the structural keywords.
func NewAddressParser(known []types.TypeInfo) *AddressParser {
	keywords := map[string]bool{}
	for k := range structuralKeywords {
		keywords[k] = true
	}

	for _, info := range known {
		keywords[info.Name] = true
	}

	return &AddressParser{keywords: keywords}
}

// Parse returns the individual components of an address.
func (p *AddressParser) Parse(address string) (*FQRN, error) {
	return parseAddress(address, p.keywords)
}

// parseAddress splits an address into its parts. keywords is consulted only to
// decide where a module prefix ends and the body begins; the body itself is
// read positionally, so a type the caller has not declared still parses.
func parseAddress(fqrn string, keywords map[string]bool) (*FQRN, error) {
	if fqrn == "" {
		return nil, errors.New(formatErrorString(fqrn))
	}

	segments := strings.Split(fqrn, ".")
	moduleName := ""
	body := segments

	if segments[0] == TypeModule {
		rest := segments[1:]
		if len(rest) == 0 {
			return nil, errors.New(formatErrorString(fqrn))
		}

		// the first segment after "module." is always a module name; the body
		// begins at the first segment after it that names a type
		bodyStart := -1
		for i := 1; i < len(rest); i++ {
			if keywords[rest[i]] {
				bodyStart = i
				break
			}
		}

		if bodyStart == -1 {
			// nothing names a type, so the whole thing addresses a module
			return &FQRN{
				Module:   strings.Join(rest[:len(rest)-1], "."),
				Type:     TypeModule,
				Resource: rest[len(rest)-1],
			}, nil
		}

		moduleName = strings.Join(rest[:bodyStart], ".")
		body = rest[bodyStart:]
	}

	if len(body) < 2 {
		return nil, errors.New(formatErrorString(fqrn))
	}

	switch body[0] {
	case types.TypeResource:
		// resource.<variety>.<name>[.attribute]
		if len(body) < 3 {
			return nil, errors.New(formatErrorString(fqrn))
		}

		return &FQRN{
			Module:    moduleName,
			Type:      types.TypeResource,
			Subtype:   body[1],
			Resource:  body[2],
			Attribute: strings.Join(body[3:], "."),
		}, nil

	case TypeVariable:
		// a variable may not carry a trailing attribute
		if len(body) != 2 {
			return nil, errors.New(formatErrorString(fqrn))
		}

		return &FQRN{Module: moduleName, Type: TypeVariable, Resource: body[1]}, nil

	case TypeModule:
		// module.<name> reached through a parent module prefix
		return &FQRN{Module: moduleName, Type: TypeModule, Resource: body[1]}, nil

	default:
		// every single label form: output, local, and any type declared by its
		// own keyword. Segment one is the kind, segment two the name
		resourceName := body[1]
		attribute := strings.Join(body[2:], ".")

		// an index selector, i.e. output.mine[0], moves the index into the
		// attribute path
		indexR := regexp.MustCompile(`(?P<name>.*)\[(?P<index>\d+)\]`)
		indexMatch := indexR.FindStringSubmatch(resourceName)
		indexResults := map[string]string{}
		for i, name := range indexMatch {
			indexResults[indexR.SubexpNames()[i]] = name
		}

		if i := indexResults["index"]; i != "" {
			attribute = strings.Trim(fmt.Sprintf("%s.%s", i, attribute), ".")
			resourceName = indexResults["name"]
		}

		return &FQRN{
			Module:    moduleName,
			Type:      body[0],
			Resource:  resourceName,
			Attribute: attribute,
		}, nil
	}
}

func formatErrorString(address string) string {
	if address == "" {
		return "an address is required: it needs at least a kind and a name, like resource.container.mine, variable.region or module.db.output.url"
	}

	if !strings.Contains(address, ".") {
		return fmt.Sprintf("%q is not an address: an address has at least two parts, a kind and a name, like variable.region or resource.container.mine", address)
	}

	if strings.HasPrefix(address, types.TypeResource+".") {
		return fmt.Sprintf("%q is not a complete address: a resource is addressed by its kind, its variety and its name, like resource.container.mine", address)
	}

	return fmt.Sprintf("%q is not a valid address, like resource.container.mine, variable.region, output.url, module.db or module.db.output.url", address)
}

// AppendParentModule creates a new FQRN by adding the parent module
// to the reference.
func (f *FQRN) AppendParentModule(parent string) FQRN {
	newFQRN := FQRN{}

	newFQRN.Module = f.Module
	if parent != "" {
		newFQRN.Module = fmt.Sprintf("%s.%s", parent, f.Module)
		newFQRN.Module = strings.TrimSuffix(newFQRN.Module, ".")
	}

	newFQRN.Resource = f.Resource
	newFQRN.Type = f.Type
	newFQRN.Subtype = f.Subtype
	newFQRN.Attribute = f.Attribute

	return newFQRN
}

// FQRNFromResource returns the ResourceFQDN for the given Resource
func FQRNFromResource(r any) *FQRN {
	meta, err := types.GetMeta(r)
	if err != nil {
		return nil
	}
	return &FQRN{
		Module:   meta.Module,
		Resource: meta.Name,
		Type:     meta.Type,
		Subtype:  meta.Subtype,
	}
}

func (f FQRN) String() string {
	modulePart := ""
	if f.Module != "" {
		modulePart = fmt.Sprintf("module.%s.", f.Module)
	}

	attrPart := ""
	if f.Attribute != "" {
		attrPart = fmt.Sprintf(".%s", f.Attribute)
	}

	if f.Type == TypeOutput || f.Type == TypeVariable {
		return fmt.Sprintf("%s%s.%s%s", modulePart, f.Type, f.Resource, attrPart)
	}

	if f.Type == TypeModule {
		if f.Module == "" {
			return fmt.Sprintf("module.%s", f.Resource)
		}

		return fmt.Sprintf("%s%s", modulePart, f.Resource)
	}

	if f.Type == types.TypeResource {
		return fmt.Sprintf("%sresource.%s.%s%s", modulePart, f.Subtype, f.Resource, attrPart)
	}

	return fmt.Sprintf("%s%s.%s%s", modulePart, f.Type, f.Resource, attrPart)
}

func (f FQRN) StringWithoutAttribute() string {
	modulePart := ""
	if f.Module != "" {
		modulePart = fmt.Sprintf("module.%s.", f.Module)
	}

	if f.Type == TypeOutput || f.Type == TypeVariable {
		return fmt.Sprintf("%s%s.%s", modulePart, f.Type, f.Resource)
	}

	if f.Type == TypeModule {
		if f.Module == "" {
			return fmt.Sprintf("module.%s", f.Resource)
		}

		return fmt.Sprintf("%s%s", modulePart, f.Resource)
	}

	if f.Type == types.TypeResource {
		return fmt.Sprintf("%sresource.%s.%s", modulePart, f.Subtype, f.Resource)
	}

	return fmt.Sprintf("%s%s.%s", modulePart, f.Type, f.Resource)
}

// Match returns the entity in entities addressed by fqrn.
//
// It takes a plain slice rather than any container, so the public lookup
// surface and the parser can both use it without either depending on the
// other, and the layer that stores entities needs no knowledge of addresses
// at all.
//
// The attribute is deliberately ignored: an address carrying a trailing
// attribute, such as one taken verbatim from a stored reference between
// entities, names the same entity as one without it.
func Match(entities []any, fqrn *FQRN) (any, bool) {
	if fqrn == nil {
		return nil, false
	}

	for _, e := range entities {
		meta, err := types.GetMeta(e)
		if err != nil {
			// not an entity, so it cannot be the one addressed
			continue
		}

		if meta.Module == fqrn.Module &&
			meta.Type == fqrn.Type &&
			meta.Subtype == fqrn.Subtype &&
			meta.Name == fqrn.Resource {
			return e, true
		}
	}

	return nil, false
}

// MatchModule returns the entities declared inside the module addressed by
// fqrn. With includeSubModules it also returns those declared in modules
// nested within it.
//
// Like Match it takes a plain slice, so the layer that stores entities needs
// no knowledge of addresses and the parser and the public surface can both use
// it without depending on each other.
func MatchModule(entities []any, fqrn *FQRN, includeSubModules bool) ([]any, error) {
	if fqrn == nil || fqrn.Type != TypeModule {
		return nil, fmt.Errorf("address does not name a module")
	}

	module := strings.TrimPrefix(fmt.Sprintf("%s.%s", fqrn.Module, fqrn.Resource), ".")

	found := []any{}

	for _, e := range entities {
		meta, err := types.GetMeta(e)
		if err != nil {
			continue
		}

		// a nested module is one whose path continues past a separator, so
		// the prefix is anchored: module "shared" contains "shared.nested"
		// but has nothing to do with a sibling named "sharedother"
		if includeSubModules && (meta.Module == module || strings.HasPrefix(meta.Module, module+".")) {
			found = append(found, e)
			continue
		}

		if !includeSubModules && meta.Module == module {
			found = append(found, e)
		}
	}

	return found, nil
}
