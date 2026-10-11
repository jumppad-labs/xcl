// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"testing"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/stretchr/testify/require"
)

// mapListener is a block whose single label field carries its name
type mapListener struct {
	Name string `xcl:"name,label"`
	Port int    `xcl:"port"`
}

// mapApp collects listener blocks by name
type mapApp struct {
	Listeners map[string]mapListener `xcl:"listener,block"`
}

// mapPointerApp collects listener blocks by name as pointers
type mapPointerApp struct {
	Listeners map[string]*mapListener `xcl:"listener,block"`
}

// listApp collects listener blocks in order, keeping each one's name
type listApp struct {
	Listeners []mapListener `xcl:"listener,block"`
}

// unnamedListApp collects listener blocks whose element has no label field
type unnamedListApp struct {
	Listeners []struct {
		Port int `xcl:"port"`
	} `xcl:"listener,block"`
}

// twoLabels is a block with two label fields, which keep the strict rules
type twoLabels struct {
	Kind string `xcl:"kind,label"`
	Name string `xcl:"name,label"`
	Port int    `xcl:"port"`
}

type twoLabelsApp struct {
	Listeners []twoLabels `xcl:"listener,block"`
}

// singleApp holds one listener block, which keeps the strict label rules
type singleApp struct {
	Single *mapListener `xcl:"listener,block"`
}

// intKeyApp is a map of blocks with non-string keys, which is not allowed
type intKeyApp struct {
	Listeners map[int]mapListener `xcl:"listener,block"`
}

func parseDecodeBody(t *testing.T, src string) hcl.Body {
	t.Helper()

	f, diags := hclsyntax.ParseConfig([]byte(src), "test.xcl", hcl.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors(), diags.Error())

	return f.Body
}

func TestDecodeNamedBlocksIntoMap(t *testing.T) {
	body := parseDecodeBody(t, `
listener "http" {
  port = 80
}

listener "https" {
  port = 443
}
`)

	var app mapApp
	diags := DecodeBody(body, nil, &app)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, app.Listeners, 2)
	require.Equal(t, 80, app.Listeners["http"].Port)
	require.Equal(t, 443, app.Listeners["https"].Port)
	require.Equal(t, "http", app.Listeners["http"].Name)
	require.Equal(t, "https", app.Listeners["https"].Name)
}

func TestDecodeNamedBlocksIntoMapOfPointers(t *testing.T) {
	body := parseDecodeBody(t, `
listener "http" {
  port = 80
}

listener "https" {
  port = 443
}
`)

	var app mapPointerApp
	diags := DecodeBody(body, nil, &app)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, app.Listeners, 2)
	require.NotNil(t, app.Listeners["http"])
	require.NotNil(t, app.Listeners["https"])
	require.Equal(t, 80, app.Listeners["http"].Port)
	require.Equal(t, 443, app.Listeners["https"].Port)
	require.Equal(t, "http", app.Listeners["http"].Name)
}

func TestDecodeUnnamedBlockIntoMapFails(t *testing.T) {
	body := parseDecodeBody(t, `
listener {
  port = 80
}
`)

	var app mapApp
	diags := DecodeBody(body, nil, &app)
	require.True(t, diags.HasErrors())

	require.Len(t, diags, 1)
	require.Equal(t, "A name is required", diags[0].Summary)
	require.Contains(t, diags[0].Detail, `listener "http" { ... }`)
	require.NotNil(t, diags[0].Subject)
	require.Equal(t, 2, diags[0].Subject.Start.Line)
	require.Equal(t, 1, diags[0].Subject.Start.Column)
}

func TestDecodeDuplicateBlockNameInMapFails(t *testing.T) {
	body := parseDecodeBody(t, `
listener "http" {
  port = 80
}

listener "http" {
  port = 8080
}
`)

	var app mapApp
	diags := DecodeBody(body, nil, &app)
	require.True(t, diags.HasErrors())

	require.Len(t, diags, 1)
	require.Equal(t, "Duplicate listener name", diags[0].Summary)
	require.Contains(t, diags[0].Detail, `"http"`)
	require.NotNil(t, diags[0].Subject)
	require.Equal(t, 6, diags[0].Subject.Start.Line)
	require.Equal(t, 10, diags[0].Subject.Start.Column)
}

func TestDecodeDuplicateBlockNameInListFails(t *testing.T) {
	body := parseDecodeBody(t, `
listener "http" {
  port = 80
}

listener "http" {
  port = 8080
}
`)

	var app listApp
	diags := DecodeBody(body, nil, &app)
	require.True(t, diags.HasErrors())

	require.Len(t, diags, 1)
	require.Equal(t, "Duplicate listener name", diags[0].Summary)
	require.Contains(t, diags[0].Detail, `"http"`)
	require.NotNil(t, diags[0].Subject)
	require.Equal(t, 6, diags[0].Subject.Start.Line)
	require.Equal(t, 10, diags[0].Subject.Start.Column)
}

func TestDecodeNamedAndUnnamedBlocksIntoList(t *testing.T) {
	body := parseDecodeBody(t, `
listener "http" {
  port = 80
}

listener {
  port = 8080
}
`)

	var app listApp
	diags := DecodeBody(body, nil, &app)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, app.Listeners, 2)
	require.Equal(t, "http", app.Listeners[0].Name)
	require.Equal(t, 80, app.Listeners[0].Port)
	require.Equal(t, "", app.Listeners[1].Name)
	require.Equal(t, 8080, app.Listeners[1].Port)
}

func TestDecodeNamedBlockIntoListWithoutLabelFieldDropsTheName(t *testing.T) {
	body := parseDecodeBody(t, `
listener "http" {
  port = 80
}
`)

	var app unnamedListApp
	diags := DecodeBody(body, nil, &app)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, app.Listeners, 1)
	require.Equal(t, 80, app.Listeners[0].Port)
}

func TestDecodeMultiLabelBlocksKeepStrictLabelCount(t *testing.T) {
	body := parseDecodeBody(t, `
listener "a" {
  port = 1
}
`)

	var app twoLabelsApp
	diags := DecodeBody(body, nil, &app)
	require.True(t, diags.HasErrors())
	require.Contains(t, diags.Error(), "Missing name for listener")
}

func TestDecodeMultiLabelBlocksWithAllLabels(t *testing.T) {
	body := parseDecodeBody(t, `
listener "tcp" "a" {
  port = 1
}
`)

	var app twoLabelsApp
	diags := DecodeBody(body, nil, &app)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, app.Listeners, 1)
	require.Equal(t, "tcp", app.Listeners[0].Kind)
	require.Equal(t, "a", app.Listeners[0].Name)
	require.Equal(t, 1, app.Listeners[0].Port)
}

func TestDecodeSingleBlockKeepsStrictLabelCount(t *testing.T) {
	body := parseDecodeBody(t, `
listener {
  port = 1
}
`)

	var app singleApp
	diags := DecodeBody(body, nil, &app)
	require.True(t, diags.HasErrors())
	require.Contains(t, diags.Error(), "Missing name for listener")
}

func TestDecodeSingleBlockWithLabel(t *testing.T) {
	body := parseDecodeBody(t, `
listener "x" {
  port = 1
}
`)

	var app singleApp
	diags := DecodeBody(body, nil, &app)
	require.False(t, diags.HasErrors(), diags.Error())

	require.NotNil(t, app.Single)
	require.Equal(t, "x", app.Single.Name)
	require.Equal(t, 1, app.Single.Port)
}

func TestDecodeEmptyMapBlockFieldLeavesNilMap(t *testing.T) {
	body := parseDecodeBody(t, ``)

	var app mapApp
	diags := DecodeBody(body, nil, &app)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Nil(t, app.Listeners)
}
