package parser

import (
	"fmt"

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
		return buildDestroyDAG(rp, addresses, rp.GetResources())
	} else {
		// build a create dag
		return buildCreateDAG(rp, addresses)
	}
}

// buildCreateDAG builds the graph that orders creation of every entity rp
// holds. Edges run from each dependency to the entity that depends on it.
func buildCreateDAG(rp ResourceProvider, addresses *resources.AddressParser) (*dagpkg.AcyclicGraph, error) {
	return buildDependencyGraph(rp, addresses, rp.GetResources(), "root", true)
}

// buildDestroyDAG builds the graph for destroying toDestroy. It is the create
// graph, built with the same builder from each entity's Meta.Links resolved
// against everything rp holds, keeping only the edges between entities in
// toDestroy: a dependency that is not being destroyed is ignored, and so is a
// parent module missing from rp. The graph is walked with Reverse so children
// are destroyed before their parents.
func buildDestroyDAG(rp ResourceProvider, addresses *resources.AddressParser, toDestroy []any) (*dagpkg.AcyclicGraph, error) {
	return buildDependencyGraph(rp, addresses, toDestroy, "destroy_root", false)
}

// buildDependencyGraph builds the graph that orders nodes. Each node's
// dependencies are resolved from its Meta.Links, and the module it sits in,
// against every entity rp holds. An edge runs from each dependency that is
// itself one of nodes to the node; a node with none hangs off the root.
// When requireParentModule is false a missing parent module is ignored.
func buildDependencyGraph(rp ResourceProvider, addresses *resources.AddressParser, nodes []any, rootName string, requireParentModule bool) (*dagpkg.AcyclicGraph, error) {
	graph := &dagpkg.AcyclicGraph{}

	if len(nodes) == 0 {
		return graph, nil
	}

	// add a root node for the graph
	root, _ := resources.DefaultResources().CreateResource(resources.TypeRoot, rootName)
	graph.Add(root)

	// add every node to the graph and index them by ID
	nodesByID := make(map[string]any)
	for _, node := range nodes {
		graph.Add(node)

		meta, err := types.GetMeta(node)
		if err != nil {
			continue // Skip resources without ResourceBase
		}
		nodesByID[meta.ID] = node
	}

	for _, node := range nodes {
		meta, err := types.GetMeta(node)
		if err != nil {
			graph.Connect(dagpkg.BasicEdge(root, node))
			continue
		}

		deps, err := getResourceDependencies(rp, addresses, node, meta, requireParentModule)
		if err != nil {
			pe := errors.NewParserErrorFromResource(
				node,
				fmt.Sprintf("unable to get dependencies: %s", err),
			)
			return nil, pe
		}

		// nil entries are references that did not resolve to an entity, and
		// a dependency that is not one of nodes is not ordered
		connected := false
		for d := range deps {
			if d == nil {
				continue
			}

			depMeta, err := types.GetMeta(d)
			if err != nil {
				continue
			}

			dep, ok := nodesByID[depMeta.ID]
			if !ok {
				continue
			}

			graph.Connect(dagpkg.BasicEdge(dep, node))
			connected = true
		}

		// if no deps add to root node
		if !connected {
			graph.Connect(dagpkg.BasicEdge(root, node))
		}
	}

	return graph, nil
}
