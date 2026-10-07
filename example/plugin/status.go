package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/resources"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/template"
	"github.com/jumppad-labs/xcl/types"
)

// shortIDLength is how much of a Docker ID status shows, as the docker CLI
// does
const shortIDLength = 12

// statusNode is one resource status shows: its address, the type that
// colours it, what it depends on, its saved status and a line of details
type statusNode struct {
	ID      string
	Name    string
	Type    string
	Links   []string
	Status  string
	Details []string
}

// status writes the resources c holds to out as a tree, each under what it
// depends on. Colour is used only when out is a terminal.
func status(out io.Writer, c *xcl.Config) error {
	nodes, err := statusNodes(c)
	if err != nil {
		return err
	}

	fmt.Fprint(out, renderStatus(lipgloss.NewRenderer(out), nodes))

	return nil
}

// statusNodes returns a node for each network, container and template c
// holds. Variables and other builtin entities are left out.
func statusNodes(c *xcl.Config) ([]statusNode, error) {
	nodes := []statusNode{}

	// Plugin types are held as types generated from the plugin's schema, the
	// lookup copies them into the plugin's Go type
	networks, err := xcl.FindByType[resources.Network](c, "docker", "network")
	if err != nil {
		return nil, err
	}

	for _, n := range networks {
		nodes = append(nodes, newStatusNode(n.Meta, n.Subnet, shortID(n.DockerID)))
	}

	containers, err := xcl.FindByType[resources.Container](c, "docker", "container")
	if err != nil {
		return nil, err
	}

	for _, ctr := range containers {
		nodes = append(nodes, newStatusNode(ctr.Meta, ctr.Image, ctr.IPAddress, shortID(ctr.DockerID)))
	}

	// template has no subtype, so its address has one segment before the name
	templates, err := xcl.FindByType[template.Template](c, "template")
	if err != nil {
		return nil, err
	}

	for _, t := range templates {
		nodes = append(nodes, newStatusNode(t.Meta, t.Destination))
	}

	return nodes, nil
}

// newStatusNode returns the node for the resource meta describes, showing
// the details that are set
func newStatusNode(meta types.Meta, details ...string) statusNode {
	shown := []string{}
	for _, d := range details {
		if d != "" {
			shown = append(shown, d)
		}
	}

	return statusNode{
		ID:      meta.ID,
		Name:    meta.Name,
		Type:    meta.Type,
		Links:   meta.Links,
		Status:  meta.Status,
		Details: shown,
	}
}

// shortID returns the first shortIDLength characters of a Docker ID
func shortID(id string) string {
	if len(id) > shortIDLength {
		return id[:shortIDLength]
	}

	return id
}

// statusParents returns the parent of each node that has one, keyed by node
// ID. A node's parent is the node it depends on that is furthest down the
// chain of dependencies, so a template that reads both a network and a
// container on that network sits under the container. Links to anything that
// is not a node, such as a variable, are ignored.
func statusParents(nodes []statusNode) map[string]string {
	byID := map[string]statusNode{}
	for _, n := range nodes {
		byID[n.ID] = n
	}

	depths := map[string]int{}

	var depth func(id string, seen map[string]bool) int
	depth = func(id string, seen map[string]bool) int {
		if d, ok := depths[id]; ok {
			return d
		}

		// a cycle cannot be applied, this only stops a bad state looping
		if seen[id] {
			return 0
		}
		seen[id] = true

		d := 0
		for _, dep := range dependencies(byID[id], byID) {
			d = max(d, depth(dep, seen)+1)
		}

		depths[id] = d
		return d
	}

	parents := map[string]string{}
	for _, n := range nodes {
		deps := dependencies(n, byID)
		sort.Strings(deps)

		parent := ""
		for _, dep := range deps {
			if parent == "" || depth(dep, map[string]bool{}) > depth(parent, map[string]bool{}) {
				parent = dep
			}
		}

		if parent != "" {
			parents[n.ID] = parent
		}
	}

	return parents
}

// dependencies returns the IDs of the nodes n links to. A link names an
// address or a value inside one, i.e. docker.network.app.meta.name, so it
// belongs to the node whose ID it starts with.
func dependencies(n statusNode, byID map[string]statusNode) []string {
	found := map[string]bool{}

	for _, link := range n.Links {
		for id := range byID {
			if id != n.ID && (link == id || strings.HasPrefix(link, id+".")) {
				found[id] = true
			}
		}
	}

	deps := []string{}
	for id := range found {
		deps = append(deps, id)
	}

	return deps
}

// statusStyles are the styles status draws with, made by one renderer so
// they colour only when its output is a terminal
type statusStyles struct {
	ok         lipgloss.Style
	failed     lipgloss.Style
	unknown    lipgloss.Style
	name       lipgloss.Style
	details    lipgloss.Style
	enumerator lipgloss.Style
	types      map[string]lipgloss.Style
}

func newStatusStyles(r *lipgloss.Renderer) statusStyles {
	return statusStyles{
		ok:         r.NewStyle().Foreground(lipgloss.Color("2")),
		failed:     r.NewStyle().Foreground(lipgloss.Color("1")),
		unknown:    r.NewStyle().Foreground(lipgloss.Color("8")),
		name:       r.NewStyle().Bold(true),
		details:    r.NewStyle().Foreground(lipgloss.Color("8")),
		enumerator: r.NewStyle().Foreground(lipgloss.Color("8")).PaddingRight(1),
		types: map[string]lipgloss.Style{
			"docker":   r.NewStyle().Foreground(lipgloss.Color("6")),
			"template": r.NewStyle().Foreground(lipgloss.Color("5")),
		},
	}
}

// renderStatus returns the nodes as trees, one per node that depends on no
// other, or "nothing applied" when there are none
func renderStatus(r *lipgloss.Renderer, nodes []statusNode) string {
	styles := newStatusStyles(r)

	if len(nodes) == 0 {
		return styles.details.Render("nothing applied") + "\n"
	}

	parents := statusParents(nodes)

	children := map[string][]statusNode{}
	roots := []statusNode{}

	for _, n := range nodes {
		if parent, ok := parents[n.ID]; ok {
			children[parent] = append(children[parent], n)
			continue
		}

		roots = append(roots, n)
	}

	var build func(n statusNode) *tree.Tree
	build = func(n statusNode) *tree.Tree {
		t := tree.Root(styles.label(n)).EnumeratorStyle(styles.enumerator)

		kids := children[n.ID]
		sort.Slice(kids, func(i, j int) bool { return kids[i].ID < kids[j].ID })

		for _, kid := range kids {
			t.Child(build(kid))
		}

		return t
	}

	sort.Slice(roots, func(i, j int) bool { return roots[i].ID < roots[j].ID })

	out := &strings.Builder{}
	for _, root := range roots {
		out.WriteString(build(root).String())
		out.WriteString("\n")
	}

	return out.String()
}

// label returns a node's line: a dot coloured by its saved status, its
// address coloured by its type with the name in bold, then its details
func (s statusStyles) label(n statusNode) string {
	dot := s.unknown.Render("●")
	switch n.Status {
	case types.StatusCreated, types.StatusUpdated:
		dot = s.ok.Render("●")
	case types.StatusFailed, types.StatusDestroyFailed:
		dot = s.failed.Render("●")
	}

	typeStyle, ok := s.types[n.Type]
	if !ok {
		typeStyle = s.details
	}

	prefix := strings.TrimSuffix(n.ID, n.Name)
	address := typeStyle.Render(prefix) + typeStyle.Inherit(s.name).Render(n.Name)

	line := dot + " " + address
	if len(n.Details) > 0 {
		line += "  " + s.details.Render(strings.Join(n.Details, " · "))
	}

	return line
}
