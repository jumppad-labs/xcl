package parser

import (
	"sort"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/types"
)

// dependencyChanges returns what the provider of r is told about its
// dependencies: the provider-backed resources r depends on that the record
// says the same apply will update or replace, sorted by address.
//
// Dependencies come from r's Meta.Links. A dependency handled without a
// provider (an output, variable, module or registered config-only type) is
// looked through to the provider-backed resources behind it, so a module
// boundary or a variable never hides a replaced resource. A provider-backed
// dependency is never looked through: r is told about what it depends on,
// not about what that depends on.
//
// A dependency decided create is not listed: the vocabulary is Update and
// Replace only. A newly referenced resource already changes r's configuration
// and leaves its computed values unknown, so r is decided at least an update
// anyway.
//
// The DAG decides a parent before its dependents, so every provider-backed
// dependency's decision is recorded by the time r is decided.
func dependencyChanges(r any, current ResourceProvider, addresses *resources.AddressParser, typeRegistry TypeRegistry, record *diffRecorder) ([]entity.DependencyChange, error) {
	meta, err := types.GetMeta(r)
	if err != nil {
		return nil, err
	}

	changes := map[string]entity.Change{}
	visited := map[string]bool{meta.ID: true}

	if err := collectDependencyChanges(r, meta, current, addresses, typeRegistry, record, visited, changes); err != nil {
		return nil, err
	}

	if len(changes) == 0 {
		return nil, nil
	}

	dependencies := make([]entity.DependencyChange, 0, len(changes))
	for address, change := range changes {
		dependencies = append(dependencies, entity.DependencyChange{Address: address, Change: change})
	}

	sort.Slice(dependencies, func(i, j int) bool {
		return dependencies[i].Address < dependencies[j].Address
	})

	return dependencies, nil
}

// collectDependencyChanges adds the changing provider-backed dependencies of
// the entity to changes, looking through provider-less dependencies. visited
// holds the IDs already seen, so a dependency reached twice is handled once.
func collectDependencyChanges(r any, meta *types.Meta, current ResourceProvider, addresses *resources.AddressParser, typeRegistry TypeRegistry, record *diffRecorder, visited map[string]bool, changes map[string]entity.Change) error {
	deps, err := getResourceDependencies(current, addresses, r, meta, false)
	if err != nil {
		return err
	}

	for dep := range deps {
		// nil entries are references that did not resolve to an entity
		if dep == nil {
			continue
		}

		depMeta, err := types.GetMeta(dep)
		if err != nil {
			continue
		}

		if visited[depMeta.ID] {
			continue
		}
		visited[depMeta.ID] = true

		if handledWithoutProvider(typeRegistry, depMeta) {
			if err := collectDependencyChanges(dep, depMeta, current, addresses, typeRegistry, record, visited, changes); err != nil {
				return err
			}

			continue
		}

		dec, ok := record.lookup(depMeta.ID)
		if !ok {
			continue
		}

		switch dec.action {
		case diff.ActionUpdate:
			changes[depMeta.ID] = entity.Update
		case diff.ActionReplace:
			changes[depMeta.ID] = entity.Replace
		}
	}

	return nil
}
