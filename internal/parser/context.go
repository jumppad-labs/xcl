package parser

import (
	"fmt"

	"github.com/jumppad-labs/xcl/internal/convert"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/resources"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/types"
)

// buildContextForResource creates a fresh context for a specific resource
// by building variables dynamically from config and module sources
func buildContextForResource(res *parsed, r any, addresses *resources.AddressParser, options *ParserOptions, functions functionsForFile) (*hcl.EvalContext, error) {
	rMeta, err := types.GetMeta(r)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource metadata: %w", err)
	}

	ctx := &hcl.EvalContext{
		Functions: functions(rMeta.File),
		Variables: map[string]cty.Value{},
	}

	// Initialize empty resource namespace
	ctx.Variables["resource"] = cty.ObjectVal(map[string]cty.Value{})

	// Get variables and resources that this resource actually depends on (from its links)
	variableVars := map[string]cty.Value{}
	resourceVars := map[string]cty.Value{}
	// moduleVars holds resources reached via a "module.<name>...." reference,
	// nested as moduleVars[moduleName][resourceType][resourceName] so that an
	// expression like module.consul_1.output.foo resolves against
	// ctx.Variables["module"]["consul_1"]["output"]["foo"].
	moduleVars := map[string]map[string]cty.Value{}
	// typedVars holds entities of a type other than "resource" and the
	// builtins, nested as typedVars[type][subtype][name] or
	// typedVars[type][name], so that server.big.web resolves against
	// ctx.Variables["server"]["big"]["web"] and cache.main against
	// ctx.Variables["cache"]["main"]
	typedVars := map[string]cty.Value{}

	for _, link := range rMeta.Links {
		// Parse the link into an FQDN, against the known types so a subtype
		// is told apart from a name
		fqdn, err := addresses.Parse(link)
		if err != nil {
			continue // Skip invalid links
		}

		// Links are written with no knowledge of their parent module, so a
		// reference from inside a module (e.g. "variable.cpu_resources") must
		// be resolved relative to that module's own scope, matching how
		// getResourceDependencies resolves the same links when building the DAG.
		relFQDN := fqdn.AppendParentModule(rMeta.Module)

		// Find the resource using findResource
		resource, ok := res.resources[relFQDN.StringWithoutAttribute()]
		if !ok {
			continue // Skip if resource not found
		}

		resourceMeta, err := types.GetMeta(resource)
		if err != nil {
			panic(fmt.Sprintf("resource does not have ResourceBase: %v", err))
		}

		if fqdn.Type == resources.TypeVariable && fqdn.Module == "" {
			// Handle variable
			if variable, ok := resource.(*resources.Variable); ok {
				// Default is already a cty.Value
				variableVars[resourceMeta.Name] = variable.Default
			}
		} else {
			// Handle other resource types
			// Skip certain resource types that can't be safely converted to cty values
			if resourceMeta.Type == resources.TypeModule || resourceMeta.Type == resources.TypeRoot {
				continue
			}

			// Convert the resource to cty value
			var ctyRes cty.Value
			switch resourceMeta.Type {
			case resources.TypeOutput:
				out := resource.(*resources.Output)
				ctyRes = out.CtyValue
			case resources.TypeVariable:
				variable := resource.(*resources.Variable)
				ctyRes = variable.Default
			default:
				// For other resource types, convert the entire resource to cty
				ctyRes, err = convert.GoToCtyValue(resource)
				if err != nil {
					// If conversion fails, skip this resource
					continue
				}
			}

			// Skip null values - these resources haven't been processed yet
			if ctyRes.IsNull() {
				continue
			}

			// an entity of a type other than "resource" and the builtins is
			// reached under its own type keyword, by its subtype where it has
			// one and its name, i.e. server.big.web or cache.main
			typedPath := []string{}
			if resourceMeta.Type != types.TypeResource && resourceMeta.Type != resources.TypeOutput {
				typedPath = append([]string{resourceMeta.Type}, resourceMeta.Subtype, resourceMeta.Name)
				if resourceMeta.Subtype == "" {
					typedPath = []string{resourceMeta.Type, resourceMeta.Name}
				}
			}

			// A reference written with a "module." prefix (fqdn.Module != "")
			// is nested under the module namespace instead of the flat
			// resource namespace, keyed by that reference's own module name.
			if fqdn.Module != "" && len(typedPath) > 0 {
				typeMap, ok := moduleVars[fqdn.Module]
				if !ok {
					typeMap = map[string]cty.Value{}
				}

				typeMap[typedPath[0]] = withNested(typeMap[typedPath[0]], typedPath[1:], ctyRes)
				moduleVars[fqdn.Module] = typeMap

				continue
			}

			if len(typedPath) > 0 {
				typedVars[typedPath[0]] = withNested(typedVars[typedPath[0]], typedPath[1:], ctyRes)

				continue
			}

			if fqdn.Module != "" {
				typeMap, ok := moduleVars[fqdn.Module]
				if !ok {
					typeMap = map[string]cty.Value{}
				}

				var innerMap map[string]cty.Value
				if existing, exists := typeMap[resourceMeta.AddressType()]; exists && !existing.IsNull() {
					innerMap = make(map[string]cty.Value)
					for k, v := range existing.AsValueMap() {
						innerMap[k] = v
					}
				} else {
					innerMap = make(map[string]cty.Value)
				}

				innerMap[resourceMeta.Name] = ctyRes
				typeMap[resourceMeta.AddressType()] = cty.ObjectVal(innerMap)
				moduleVars[fqdn.Module] = typeMap

				continue
			}

			// Add to the appropriate nested map structure
			var typeMap map[string]cty.Value
			if existingTypeVal, exists := resourceVars[resourceMeta.AddressType()]; exists && !existingTypeVal.IsNull() {
				typeMap = make(map[string]cty.Value)
				for k, v := range existingTypeVal.AsValueMap() {
					typeMap[k] = v
				}
			} else {
				typeMap = make(map[string]cty.Value)
			}

			typeMap[resourceMeta.Name] = ctyRes
			resourceVars[resourceMeta.AddressType()] = cty.ObjectVal(typeMap)
		}
	}

	// If this resource is in a module, merge in the module's passed variables
	// (module-supplied values override this resource's own variable defaults)
	if rMeta.Module != "" {
		owningModule, ok := res.resources["module."+rMeta.Module]
		if ok {
			if mod, ok := owningModule.(*resources.Module); ok && mod.SubContext != nil {
				if modVars, ok := mod.SubContext.Variables["variable"]; ok && !modVars.IsNull() {
					for k, v := range modVars.AsValueMap() {
						variableVars[k] = v
					}
				}
			}
		}
	}

	ctx.Variables["variable"] = cty.ObjectVal(variableVars)

	if rMeta.Module == "" && options != nil {
		// For root-level resources only, load variables from files and apply precedence
		// Precedence: variable defaults < .vars files < environment variables < direct variables

		// Load variables from .vars files (these override variable defaults)
		for _, vf := range options.VariablesFiles {
			if err := loadVariablesFromFile(ctx, vf); err != nil {
				// Continue processing other files even if one fails
				// This matches the behavior in parser.go
				continue
			}
		}

		// Apply environment variables and direct variables (these override .vars files)
		setVariables(ctx, options.Variables, options.VariableEnvPrefix)
	}

	// Set the resource variables in the context
	ctx.Variables["resource"] = cty.ObjectVal(resourceVars)

	for entityType, value := range typedVars {
		ctx.Variables[entityType] = value
	}

	// Set the module namespace, so references like
	// module.consul_1.output.foo resolve for resources outside that module
	moduleNamespace := map[string]cty.Value{}
	for moduleName, typeMap := range moduleVars {
		moduleNamespace[moduleName] = cty.ObjectVal(typeMap)
	}
	ctx.Variables["module"] = cty.ObjectVal(moduleNamespace)

	return ctx, nil
}

// withNested returns existing with value set at path, creating an object at
// every step that does not have one yet and keeping what is already there
func withNested(existing cty.Value, path []string, value cty.Value) cty.Value {
	if len(path) == 0 {
		return value
	}

	fields := map[string]cty.Value{}
	// a missing entry is the zero value, which reads as null
	if !existing.IsNull() && existing.Type().IsObjectType() {
		for k, v := range existing.AsValueMap() {
			fields[k] = v
		}
	}

	fields[path[0]] = withNested(fields[path[0]], path[1:], value)

	return cty.ObjectVal(fields)
}
