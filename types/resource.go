package types

var TypeResource = "resource"

type Meta struct {
	// ID is the unique id for the resource
	// this follows the convention module_name.resource_name
	// i.e module.module1.module2.resource.container.mine
	ID string `xcl:"id,optional" json:"id"`

	// Name is the name of the resource
	// this is an internal property that is set from the stanza label
	Name string `xcl:"name,optional" json:"name"`

	// Type is the keyword that declared this entity: "resource", "variable",
	// "output", "module", "root", or the type of a registered or plugin
	// provided entity, i.e. "server". It is the same for every entity declared
	// by the same keyword, whatever its subtype
	// this is an internal property that can not be set with hcl
	Type string `xcl:"type,optional" json:"type"`

	// Subtype is the optional first label of an entity whose type takes one,
	// i.e. the "container" in resource "container" "mine" or the "big" in
	// server "big" "web". It is empty for an entity declared with only a name
	// this is an internal property that can not be set with hcl
	Subtype string `xcl:"subtype,optional" json:"subtype,omitempty"`

	// Module is the name of the module if a resource has been loaded from a module
	// this is an internal property that can not be set with hcl
	Module string `xcl:"module,optional" json:"module,omitempty"`

	// File is the absolute path of the file where the resource is defined
	// this is an internal property that can not be set with hcl
	File string `xcl:"file,optional" json:"file"`

	// Line is the starting line number where the resource is located in the
	// file from where it was originally parsed
	Line int `xcl:"line,optional" json:"line"`

	// Column is the starting column number where the resource is located in the
	// file from where it was originally parsed
	Column int `xcl:"column,optional" json:"column"`

	// Properties holds a collection that can be used to store adhoc data
	Properties map[string]any `json:"properties,omitempty"`

	// Linked resources which must be set before this config can be processed
	// this is an internal property that can not be set with hcl
	Links []string `json:"links,omitempty"`

	// References holds, for each attribute whose value refers to another
	// entity, the expression exactly as the user wrote it, keyed by the
	// attribute's path, i.e. "location" or "network[1].name". Configuration
	// text shows it in place of the resolved value when asked to
	// this is an internal property that can not be set with hcl
	References map[string]string `json:"references,omitempty"`

	// Parents holds the IDs of the resources this resource depends on, resolved
	// when the create graph is built. It covers explicit depends_on, references,
	// module-wide references and the module the resource sits in, and is what
	// orders a destroy from the saved state alone
	// this is an internal property that can not be set with hcl
	Parents []string `json:"parents,omitempty"`

	// Status tracks the operational state of the resource
	// Possible values: "created", "updated", "failed", "destroyed", "destroy_failed"
	// (see StatusCreated, StatusUpdated, StatusFailed, StatusDestroyed, StatusDestroyFailed)
	// this is an internal property that can not be set with hcl
	Status string `json:"status,omitempty"`
}

// AddressType returns the subtype of an entity that has one, and its type
// otherwise: the "container" in resource.container.mine and the "cache" in
// cache.main. It is the short "<type>.<name>" form events name an entity by.
func (m *Meta) AddressType() string {
	if m.Subtype != "" {
		return m.Subtype
	}

	return m.Type
}

// ResourceBase is the embedded type for any config resources
// it defines common meta data that all resources share
type ResourceBase struct {
	// DependsOn is a user configurable list of dependencies for this resource
	DependsOn []string `xcl:"depends_on,optional" json:"depends_on,omitempty"`

	// Enabled determines if a resource is enabled and should be processed
	Disabled bool `xcl:"disabled,optional" json:"disabled,omitempty"`

	Meta Meta `xcl:"meta,optional" json:"meta,omitempty"`
}
