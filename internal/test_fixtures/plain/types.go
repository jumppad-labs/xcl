// Package plain holds block types written as plain structs, with no
// types.ResourceBase, for the tests of plain type support.
package plain

import "github.com/jumppad-labs/xcl/types"

// Agent is a plain block type named by its one label.
type Agent struct {
	Name    string   `xcl:"name,label"`
	Enabled *bool    `xcl:"enabled,optional"`
	Models  []string `xcl:"models,optional"`
}

// Database is declared with a subtype, so it has two labels.
type Database struct {
	Engine string `xcl:"engine,label"`
	Name   string `xcl:"name,label"`
	Port   int    `xcl:"port,optional"`
	Meta   types.Meta
}

// Override is a nested block of Organization, never registered itself.
type Override struct {
	Name   string   `xcl:"name,label"`
	Models []string `xcl:"models,optional"`
}

// Organization nests labelled blocks.
type Organization struct {
	Name      string     `xcl:"name,label"`
	Capacity  int        `xcl:"capacity,optional"`
	Overrides []Override `xcl:"agent,block"`
}

// Job sees the dependencies, disabled flag and metadata every block has, in
// its Meta.
type Job struct {
	Name    string `xcl:"name,label"`
	Command string `xcl:"command,optional"`
	Meta    types.Meta
}

// Service reads databases by reference, as plain values of their type.
type Service struct {
	Name      string     `xcl:"name,label"`
	Database  Database   `xcl:"database"`
	Databases []Database `xcl:"databases,optional"`
}

// Runner is a singleton: it has neither label fields nor a Meta field, so its
// block has no labels, and there is one of it.
type Runner struct {
	Server string `xcl:"server"`
	Listen string `xcl:"listen,optional"`
}

// Client reads the runner by reference, an attribute and the whole of it.
type Client struct {
	Name   string `xcl:"name,label"`
	Server string `xcl:"server"`
	Runner Runner `xcl:"runner,optional"`
}

// Unlabelled has neither label fields nor a Meta field, so it is a singleton.
type Unlabelled struct {
	Value string `xcl:"value,optional"`
}
