// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package hclwrite

import (
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/internal/cty"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/stretchr/testify/require"
)

func TestSetLineCommentWritesCommentAfterValue(t *testing.T) {
	f := NewEmptyFile()

	attr := f.Body().SetAttributeValue("a", cty.StringVal("b"))
	require.NotNil(t, attr)

	attr.SetLineComment("hello")

	out := string(f.Bytes())

	require.Contains(t, out, "# hello")
	require.Regexp(t, `a\s+= "b" # hello`, out)

	// the comment belongs on the same line as the value it marks
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 1)
}

func TestSetLineCommentWithEmptyTextWritesNoComment(t *testing.T) {
	f := NewEmptyFile()

	attr := f.Body().SetAttributeValue("a", cty.StringVal("b"))
	require.NotNil(t, attr)

	attr.SetLineComment("")

	out := string(f.Bytes())

	require.NotContains(t, out, "#")
	require.Regexp(t, `a\s+= "b"`, out)
}

func TestSetLineCommentWithEmptyTextRemovesAnExistingComment(t *testing.T) {
	f := NewEmptyFile()

	attr := f.Body().SetAttributeValue("a", cty.StringVal("b"))
	require.NotNil(t, attr)

	attr.SetLineComment("hello")
	require.Contains(t, string(f.Bytes()), "# hello")

	attr.SetLineComment("")

	out := string(f.Bytes())

	require.NotContains(t, out, "#")
	require.NotContains(t, out, "hello")
}

func TestSetLeadCommentWritesCommentAboveAttribute(t *testing.T) {
	f := NewEmptyFile()
	attr := f.Body().SetAttributeValue("a", cty.StringVal("b"))

	attr.SetLeadComment("hello")

	require.Equal(t, "# hello\na = \"b\"\n", string(f.Bytes()))
}

// a lead comment is the first node of an attribute's children, so setting one
// is what caught node.ReplaceWith leaving the list's first pointer on the node
// it had just detached. The whole attribute used to disappear here
func TestSetLeadCommentKeepsEveryOtherAttribute(t *testing.T) {
	f := NewEmptyFile()
	attr := f.Body().SetAttributeValue("a", cty.StringVal("b"))
	f.Body().SetAttributeValue("c", cty.StringVal("d"))

	attr.SetLeadComment("hello")

	require.Equal(t, "# hello\na = \"b\"\nc = \"d\"\n", string(f.Bytes()))
}

func TestSetLeadCommentWithEmptyTextWritesNoComment(t *testing.T) {
	f := NewEmptyFile()
	attr := f.Body().SetAttributeValue("a", cty.StringVal("b"))

	attr.SetLeadComment("")

	out := string(f.Bytes())

	require.Equal(t, "a = \"b\"\n", out)
	require.NotContains(t, out, "#")
}

func TestSetAttributeValueReturnsTheCreatedAttribute(t *testing.T) {
	f := NewEmptyFile()
	body := f.Body()

	// the body has no attribute of this name, so this call creates one. upstream
	// shadowed the local here and handed back nil for every created attribute
	require.Nil(t, body.GetAttribute("a"))

	attr := body.SetAttributeValue("a", cty.StringVal("b"))

	require.NotNil(t, attr)
}

func TestSetAttributeValueReturnsTheExistingAttribute(t *testing.T) {
	f := NewEmptyFile()
	body := f.Body()

	created := body.SetAttributeValue("a", cty.StringVal("b"))
	require.NotNil(t, created)

	replaced := body.SetAttributeValue("a", cty.StringVal("c"))
	require.NotNil(t, replaced)

	require.Regexp(t, `a\s+= "c"`, string(f.Bytes()))
}

func TestSetAttributeRawReturnsTheCreatedAttribute(t *testing.T) {
	f := NewEmptyFile()
	body := f.Body()

	attr := body.SetAttributeRaw("a", TokensForValue(cty.StringVal("b")))

	require.NotNil(t, attr)
}

func TestSetAttributeTraversalReturnsTheCreatedAttribute(t *testing.T) {
	f := NewEmptyFile()
	body := f.Body()

	attr := body.SetAttributeTraversal("a", hcl.Traversal{
		hcl.TraverseRoot{Name: "var"},
		hcl.TraverseAttr{Name: "region"},
	})

	require.NotNil(t, attr)
}

func TestSetLineCommentOnACreatedAttributeMarksThatAttributeOnly(t *testing.T) {
	f := NewEmptyFile()
	body := f.Body()

	marked := body.SetAttributeValue("a", cty.StringVal("b"))
	require.NotNil(t, marked)

	plain := body.SetAttributeValue("c", cty.StringVal("d"))
	require.NotNil(t, plain)

	marked.SetLineComment("hello")

	out := string(f.Bytes())

	require.Equal(t, 1, strings.Count(out, "#"))
	require.Regexp(t, `a\s+= "b" # hello`, out)
	require.Regexp(t, `c\s+= "d"\n`, out)
}
