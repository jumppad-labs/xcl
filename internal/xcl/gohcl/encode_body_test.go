// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"strings"
	"testing"
	"time"

	"github.com/jumppad-labs/xcl/internal/xcl/hclwrite"
	"github.com/stretchr/testify/require"
)

// encodeResourceBase is the innermost base, standing in for types.ResourceBase
type encodeResourceBase struct {
	DependsOn []string `xcl:"depends_on,optional"`
}

// encodeContainerBase embeds the resource base, standing in for
// structs.ContainerBase, so the fixture nests remain structs two levels deep
type encodeContainerBase struct {
	encodeResourceBase `xcl:",remain"`

	Image string `xcl:"image"`
}

// encodeContainer embeds the container base, standing in for structs.Container
type encodeContainer struct {
	encodeContainerBase `xcl:",remain"`

	Command []string `xcl:"command,optional"`
}

type encodeInterfaceHolder struct {
	Mode    any `xcl:"mode,optional"`
	Retries any `xcl:"retries,optional"`
}

type encodeNillable struct {
	Name    string            `xcl:"name"`
	Timeout *int              `xcl:"timeout,optional"`
	Tags    []string          `xcl:"tags,optional"`
	Labels  map[string]string `xcl:"labels,optional"`
}

type encodeZeroScalars struct {
	Port     int    `xcl:"port,optional"`
	Disabled bool   `xcl:"disabled,optional"`
	Name     string `xcl:"name,optional"`
}

type encodeComputedField struct {
	Name       string `xcl:"name"`
	ProviderID string `xcl:"provider_id,optional,computed"`
}

type encodeNetwork struct {
	ID string `xcl:"id"`
}

type encodeNetworked struct {
	Name     string          `xcl:"name"`
	Networks []encodeNetwork `xcl:"network,block"`
}

type encodeUntypedMap struct {
	M map[string]any `xcl:"m,optional"`
}

type encodeTimestamped struct {
	CreatedAt time.Time `xcl:"created_at,optional"`
}

// encodeNetworkObject stands in for a type held as an object valued attribute,
// carrying the bookkeeping base every entity has as well as its own fields
type encodeNetworkObject struct {
	encodeResourceBase `xcl:",remain"`

	Subnet     string `xcl:"subnet"`
	ProviderID string `xcl:"provider_id,optional,computed"`
}

// encodeObjectHolder holds a tagged struct as an attribute, which is the shape
// the encoder builds itself rather than handing whole to gocty
type encodeObjectHolder struct {
	Name    string              `xcl:"name"`
	Network encodeNetworkObject `xcl:"networkobj,optional"`
}

func TestEncodeBodyWritesEmbeddedRemainFields(t *testing.T) {
	container := encodeContainer{
		encodeContainerBase: encodeContainerBase{
			encodeResourceBase: encodeResourceBase{
				DependsOn: []string{"resource.container.other"},
			},
			Image: "nginx:latest",
		},
		Command: []string{"nginx", "-g", "daemon off;"},
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&container, f.Body(), EncodeOptions{})
	require.NoError(t, err)

	expected := `depends_on = ["resource.container.other"]
image      = "nginx:latest"
command    = ["nginx", "-g", "daemon off;"]
`

	require.Equal(t, expected, string(f.Bytes()))
}

func TestEncodeBodyWritesInterfaceHeldValue(t *testing.T) {
	holder := encodeInterfaceHolder{
		Mode:    "fast",
		Retries: 3,
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&holder, f.Body(), EncodeOptions{})
	require.NoError(t, err)

	out := string(f.Bytes())
	require.Contains(t, out, `mode    = "fast"`)
	require.Contains(t, out, `retries = 3`)
}

func TestEncodeBodySkipsNilValues(t *testing.T) {
	nillable := encodeNillable{
		Name:    "web",
		Timeout: nil,
		Tags:    nil,
		Labels:  nil,
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&nillable, f.Body(), EncodeOptions{})
	require.NoError(t, err)

	out := string(f.Bytes())
	require.Contains(t, out, `name = "web"`)
	require.NotContains(t, out, "timeout")
	require.NotContains(t, out, "tags")
	require.NotContains(t, out, "labels")
	require.NotContains(t, out, "null")
}

func TestEncodeBodyWritesZeroScalars(t *testing.T) {
	zeros := encodeZeroScalars{
		Port:     0,
		Disabled: false,
		Name:     "",
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&zeros, f.Body(), EncodeOptions{})
	require.NoError(t, err)

	out := string(f.Bytes())
	require.Contains(t, out, `port     = 0`)
	require.Contains(t, out, `disabled = false`)
	require.Contains(t, out, `name     = ""`)
}

func TestEncodeBodyOmitsComputedByDefault(t *testing.T) {
	computed := encodeComputedField{
		Name:       "web",
		ProviderID: "i-0123456789",
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&computed, f.Body(), EncodeOptions{})
	require.NoError(t, err)

	out := string(f.Bytes())
	require.Contains(t, out, `name = "web"`)
	require.NotContains(t, out, "provider_id")
	require.NotContains(t, out, "i-0123456789")
}

func TestEncodeBodyIncludesComputedWhenAsked(t *testing.T) {
	computed := encodeComputedField{
		Name:       "web",
		ProviderID: "i-0123456789",
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&computed, f.Body(), EncodeOptions{IncludeComputed: true})
	require.NoError(t, err)

	out := string(f.Bytes())
	require.Contains(t, out, `name        = "web"`)
	require.Contains(t, out, `provider_id = "i-0123456789"`)
}

func TestEncodeBodyWritesRepeatedBlocks(t *testing.T) {
	networked := encodeNetworked{
		Name: "web",
		Networks: []encodeNetwork{
			{ID: "network.cloud"},
			{ID: "network.onprem"},
		},
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&networked, f.Body(), EncodeOptions{})
	require.NoError(t, err)

	expected := `name = "web"

network {
  id = "network.cloud"
}
network {
  id = "network.onprem"
}
`

	require.Equal(t, expected, string(f.Bytes()))
}

func TestEncodeBodyReturnsErrorForUntypedMap(t *testing.T) {
	untyped := encodeUntypedMap{
		M: map[string]any{"a": 1},
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&untyped, f.Body(), EncodeOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "m: cannot encode")
}

func TestEncodeBodyReturnsErrorForTime(t *testing.T) {
	timestamped := encodeTimestamped{
		CreatedAt: time.Date(2026, time.September, 23, 10, 30, 0, 0, time.UTC),
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&timestamped, f.Body(), EncodeOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "created_at: cannot encode")
}

func TestEncodeBodyReturnsErrorForNonStruct(t *testing.T) {
	f := hclwrite.NewEmptyFile()
	err := EncodeBody("not a struct", f.Body(), EncodeOptions{})
	require.Error(t, err)
	require.Equal(t, "value is string, not struct", err.Error())
}

func TestEncodeBodyOmitsEmbeddedBaseFromObjectAttribute(t *testing.T) {
	holder := encodeObjectHolder{
		Name: "web",
		Network: encodeNetworkObject{
			encodeResourceBase: encodeResourceBase{
				DependsOn: []string{"resource.network.other"},
			},
			Subnet: "10.0.0.0/16",
		},
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&holder, f.Body(), EncodeOptions{})
	require.NoError(t, err)

	out := string(f.Bytes())

	// the struct's own field is written
	require.Contains(t, out, "subnet")
	require.Contains(t, out, `"10.0.0.0/16"`)

	// the embedded base is bookkeeping nobody wrote inside the object, so it
	// is left out
	require.NotContains(t, out, "depends_on")
	require.NotContains(t, out, "resource.network.other")
}

func TestEncodeBodyOmitsComputedInsideObjectAttribute(t *testing.T) {
	holder := encodeObjectHolder{
		Name: "web",
		Network: encodeNetworkObject{
			Subnet:     "10.0.0.0/16",
			ProviderID: "net-0123456789",
		},
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&holder, f.Body(), EncodeOptions{})
	require.NoError(t, err)

	out := string(f.Bytes())

	require.Contains(t, out, "subnet")
	require.NotContains(t, out, "provider_id")
	require.NotContains(t, out, "net-0123456789")
}

func TestEncodeBodyIncludesComputedInsideObjectAttributeWhenAsked(t *testing.T) {
	holder := encodeObjectHolder{
		Name: "web",
		Network: encodeNetworkObject{
			Subnet:     "10.0.0.0/16",
			ProviderID: "net-0123456789",
		},
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&holder, f.Body(), EncodeOptions{IncludeComputed: true})
	require.NoError(t, err)

	out := string(f.Bytes())

	require.Contains(t, out, "subnet")
	require.Contains(t, out, "provider_id")
	require.Contains(t, out, `"net-0123456789"`)
}

func TestEncodeBodyWritesComputedComment(t *testing.T) {
	computed := encodeComputedField{
		Name:       "web",
		ProviderID: "i-0123456789",
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&computed, f.Body(), EncodeOptions{
		IncludeComputed: true,
		ComputedComment: "set by the provider",
	})
	require.NoError(t, err)

	out := string(f.Bytes())

	require.Regexp(t, `provider_id\s+= "i-0123456789" # set by the provider`, out)

	// only the computed field is marked, the configured one is left alone
	require.Regexp(t, `name\s+= "web"\n`, out)
	require.Equal(t, 1, strings.Count(out, "#"))
}

func TestEncodeBodyWritesNoComputedCommentByDefault(t *testing.T) {
	computed := encodeComputedField{
		Name:       "web",
		ProviderID: "i-0123456789",
	}

	f := hclwrite.NewEmptyFile()
	err := EncodeBody(&computed, f.Body(), EncodeOptions{IncludeComputed: true})
	require.NoError(t, err)

	out := string(f.Bytes())

	require.Contains(t, out, "provider_id")
	require.NotContains(t, out, "#")
}
