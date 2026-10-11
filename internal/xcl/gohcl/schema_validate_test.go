// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"reflect"
	"testing"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/stretchr/testify/require"
)

// ValidateSchemaType reports the first field of a Go type configuration
// cannot be decoded into, so a caller registering the type can return the
// problem where the type is declared rather than panic while a file is read.

type validListener struct {
	Name string `xcl:"name,label"`
	Port int    `xcl:"port"`
}

type validTimeouts struct {
	Connect int `xcl:"connect,optional"`
}

type validServer struct {
	Address   string                   `xcl:"address"`
	Replicas  int                      `xcl:"replicas,optional"`
	Listeners []validListener          `xcl:"listener,block"`
	Volumes   map[string]validTimeouts `xcl:"volume,block"`
	Timeouts  *validTimeouts           `xcl:"timeouts,block"`
	Rest      hcl.Body                 `xcl:",remain"`
}

type stringBlockField struct {
	Listener string `xcl:"listener,block"`
}

type invalidTagField struct {
	Name string `xcl:"name,bogus"`
}

type twoRemainFields struct {
	First  hcl.Body `xcl:",remain"`
	Second hcl.Body `xcl:",remain"`
}

type intLabelElement struct {
	Name int `xcl:"name,label"`
}

type nonStringLabelInBlock struct {
	Listeners []intLabelElement `xcl:"listener,block"`
}

// a map keyed by anything but a string has no configuration form; an any
// field does, it decodes whatever value is written
type attributeWithoutForm struct {
	Mode map[int]string `xcl:"mode"`
}

type anyAttribute struct {
	Mode any `xcl:"mode"`
}

type unexportedTaggedField struct {
	port int `xcl:"port"` //nolint:unused // the tag is what is under test
}

type intKeyedBlocks struct {
	Volumes map[int]validTimeouts `xcl:"volume,block"`
}

type outerWithBadElement struct {
	Address string             `xcl:"address"`
	Inner   []stringBlockField `xcl:"inner,block"`
}

type node struct {
	Children []*node `xcl:"child,block"`
	Name     string  `xcl:"name,label"`
}

func TestValidateSchemaTypeAcceptsDecodableType(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&validServer{}))

	require.NoError(t, err)
}

func TestValidateSchemaTypeRejectsStringBlockField(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&stringBlockField{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot hold blocks")
	require.Contains(t, err.Error(), "Listener")
}

func TestValidateSchemaTypeRejectsInvalidTag(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&invalidTagField{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid xcl field tag option")
	require.Contains(t, err.Error(), "Name")
}

func TestValidateSchemaTypeRejectsTwoRemainFields(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&twoRemainFields{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "only one 'remain' tag")
	require.Contains(t, err.Error(), "Second")
}

func TestValidateSchemaTypeRejectsNonStringLabelField(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&nonStringLabelInBlock{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot hold a label")
	require.Contains(t, err.Error(), "Name")
}

func TestValidateSchemaTypeRejectsAttributeWithoutConfigurationForm(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&attributeWithoutForm{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "no configuration form")
	require.Contains(t, err.Error(), "Mode")
}

func TestValidateSchemaTypeAcceptsAnyAttribute(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&anyAttribute{}))

	require.NoError(t, err)
}

func TestValidateSchemaTypeRejectsUnexportedTaggedField(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&unexportedTaggedField{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "unexported")
	require.Contains(t, err.Error(), "port")
}

func TestValidateSchemaTypeRejectsMapOfBlocksWithNonStringKeys(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&intKeyedBlocks{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "string keys")
	require.Contains(t, err.Error(), "Volumes")
}

func TestValidateSchemaTypeRejectsNonStruct(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(""))

	require.Error(t, err)
	require.Contains(t, err.Error(), "not a struct")
}

func TestValidateSchemaTypeRejectsNilType(t *testing.T) {
	err := ValidateSchemaType(nil)

	require.Error(t, err)
}

func TestValidateSchemaTypeFollowsBlockElements(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&outerWithBadElement{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot hold blocks")
	require.Contains(t, err.Error(), "Listener")
	require.Contains(t, err.Error(), "stringBlockField")
}

func TestValidateSchemaTypeHandlesSelfReferencingBlocks(t *testing.T) {
	err := ValidateSchemaType(reflect.TypeOf(&node{}))

	require.NoError(t, err)
}
