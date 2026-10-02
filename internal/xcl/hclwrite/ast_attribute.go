// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package hclwrite

import (
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
)

type Attribute struct {
	inTree

	leadComments *node
	name         *node
	expr         *node
	lineComments *node
}

func newAttribute() *Attribute {
	return &Attribute{
		inTree: newInTree(),
	}
}

func (a *Attribute) init(name string, expr *Expression) {
	expr.assertUnattached()

	nameTok := newIdentToken(name)
	nameObj := newIdentifier(nameTok)
	a.leadComments = a.children.Append(newComments(nil))
	a.name = a.children.Append(nameObj)
	a.children.AppendUnstructuredTokens(Tokens{
		{
			Type:  hclsyntax.TokenEqual,
			Bytes: []byte{'='},
		},
	})
	a.expr = a.children.Append(expr)
	a.expr.list = a.children
	a.lineComments = a.children.Append(newComments(nil))
	a.children.AppendUnstructuredTokens(Tokens{
		{
			Type:  hclsyntax.TokenNewline,
			Bytes: []byte{'\n'},
		},
	})
}

func (a *Attribute) Expr() *Expression {
	return a.expr.content.(*Expression)
}

// SetLineComment replaces the comment that follows the attribute's value on
// the same line, writing text after a "# " marker. An empty text removes the
// comment. text is one line, it must not contain a newline, the attribute
// writes its own line ending after the comment.
func (a *Attribute) SetLineComment(text string) {
	a.lineComments = a.lineComments.ReplaceWith(newComments(commentTokens(text)))
}

// SetLeadComment replaces the comment written on its own line above the
// attribute, writing text after a "# " marker. An empty text removes the
// comment. text is one line and must not contain a newline.
func (a *Attribute) SetLeadComment(text string) {
	tokens := commentTokens(text)
	if len(tokens) > 0 {
		// a lead comment stands on its own line, so it ends with one
		tokens = append(tokens, &Token{
			Type:  hclsyntax.TokenNewline,
			Bytes: []byte{'\n'},
		})
	}

	a.leadComments = a.leadComments.ReplaceWith(newComments(tokens))
}

// commentTokens builds the tokens for a single line comment, or none at all
// when there is nothing to say
func commentTokens(text string) Tokens {
	if text == "" {
		return nil
	}

	return Tokens{
		{
			Type:         hclsyntax.TokenComment,
			Bytes:        []byte("# " + text),
			SpacesBefore: 1,
		},
	}
}
