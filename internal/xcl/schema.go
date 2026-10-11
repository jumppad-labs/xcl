// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package hcl

// BlockHeaderSchema represents the shape of a block header, and is
// used for matching blocks within bodies.
type BlockHeaderSchema struct {
	Type       string
	LabelNames []string

	// LabelsOptional allows a block of this type to carry either no labels
	// or all of LabelNames. A block with some but not all of them is still
	// an error. A block collected into a list or a map by its name uses it,
	// so the name may be left off.
	LabelsOptional bool
}

// AttributeSchema represents the requirements for an attribute, and is used
// for matching attributes within bodies.
type AttributeSchema struct {
	Name     string
	Required bool
}

// BodySchema represents the desired shallow structure of a body.
type BodySchema struct {
	Attributes []AttributeSchema
	Blocks     []BlockHeaderSchema
}
