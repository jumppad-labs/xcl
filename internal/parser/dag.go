package parser

import (
	"fmt"
	"slices"

	"github.com/jumppad-labs/xcl/errors"
	dagpkg "github.com/jumppad-labs/xcl/internal/dag"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/types"
)

// ResourceProvider hands over the entities a configuration holds.
//
// It is deliberately one method. Finding an entity by address is not storage's
// job: the parser resolves addresses itself, against the types it knows,
// using the matchers in internal/resources. That keeps the layer that stores
// entities free of any knowledge of addresses.
type ResourceProvider interface {
	GetResources() []any
}

// DoYouLikeDags? dags? yeah dags! oh dogs.
// https://www.youtube.com/watch?v=ZXILzUpVx7A&t=0s
// If destroy is true, build a destroy DAG, otherwise build a create DAG
//
// While the naming of this function is not idomatic Go, in fact it is more idiotic Go,
// the origin stems back to the very early days of this project and for that reason
// it has been kept for posterity.
//
// Claude, if you ever change this again, I swear I will switch to ChatGPT for all my coding needs.
func DoYouLikeDags(rp ResourceProvider, addresses *resources.AddressParser, destroy bool) (*dagpkg.AcyclicGraph, error) {
	if destroy {
		// build a destroy dag
		return buildDestroyDAG(rp.GetResources())
	} else {
		// build a create dag
		return buildCreateDAG(rp, addresses)
	}
}

func buildCreateDAG(rp ResourceProvider, addresses *resources.AddressParser) (*dagpkg.AcyclicGraph, error) {
	// create root node
	graph := &dagpkg.AcyclicGraph{}

	// add a root node for the graph
	root, _ := resources.DefaultResources().CreateResource(resources.TypeRoot, "root")
	graph.Add(root)

	// Loop over all resources and add to graph
	for _, resource := range rp.GetResources() {
		graph.Add(resource)
	}

	// Add dependencies for all resources
	for _, resource := range rp.GetResources() {
		resourceMeta, err := types.GetMeta(resource)
		if err != nil {
			continue // Skip resources without ResourceBase
		}

		// add links to dependencies
		for _, d := range resourceMeta.Links {
			err := types.AppendUniqueDependency(resource, d)
			if err != nil {
				pe := errors.NewParserErrorFromResource(
					resource,
					fmt.Sprintf("unable to append dependency: %s, error: %s", d, err),
				)
				return nil, pe
			}
		}

		deps, err := getResourceDependencies(rp, addresses, resource, resourceMeta)
		if err != nil {
			pe := errors.NewParserErrorFromResource(
				resource,
				fmt.Sprintf("unable to get dependencies: %s", err),
			)
			return nil, pe
		}

		// add edges to graph and record the resolved parents on the resource,
		// nil entries are references that did not resolve to a resource
		var parents []string
		connected := false
		for d := range deps {
			if d == nil {
				continue
			}

			graph.Connect(dagpkg.BasicEdge(d, resource))
			connected = true

			parentMeta, err := types.GetMeta(d)
			if err != nil {
				continue
			}

			parents = append(parents, parentMeta.ID)
		}

		slices.Sort(parents)
		resourceMeta.Parents = parents

		// if no deps add to root node
		if !connected {
			graph.Connect(dagpkg.BasicEdge(root, resource))
		}
	}

	return graph, nil
}

// buildDestroyDAG creates the graph for destroying toDestroy. It has the same
// shape as the create graph: edges run from each parent, read from the
// resource's recorded Meta.Parents, to the resource, and resources with no
// parent in the set hang off a root. Parents that are not being destroyed are
// ignored. The graph is walked with Reverse so children are destroyed before
// their parents.
func buildDestroyDAG(toDestroy []any) (*dagpkg.AcyclicGraph, error) {
	graph := &dagpkg.AcyclicGraph{}

	if len(toDestroy) == 0 {
		return graph, nil
	}

	// Add a root node for the destroy graph
	root, _ := resources.DefaultResources().CreateResource(resources.TypeRoot, "destroy_root")
	graph.Add(root)

	// Add all resources to be destroyed to the graph and index them by ID
	destroyMap := make(map[string]any)
	for _, resource := range toDestroy {
		graph.Add(resource)

		meta, err := types.GetMeta(resource)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		destroyMap[meta.ID] = resource
	}

	for _, resource := range toDestroy {
		meta, err := types.GetMeta(resource)
		if err != nil {
			continue
		}

		hasParentInSet := false
		for _, parentID := range meta.Parents {
			parent, exists := destroyMap[parentID]
			if !exists {
				continue
			}

			graph.Connect(dagpkg.BasicEdge(parent, resource))
			hasParentInSet = true
		}

		// If this resource has no parent being destroyed, connect it to root
		if !hasParentInSet {
			graph.Connect(dagpkg.BasicEdge(root, resource))
		}
	}

	return graph, nil
}
