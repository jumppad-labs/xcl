package parser

import (
	"fmt"
	"sort"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/types"
)

// problem is what a diagnostic says, and the error it was found through, if
// any
type problem struct {
	summary string
	detail  string
	cause   error
}

// diagnostic makes the diagnostic for problem in the entity whose block
// starts at header and holds body: at subject, or at the header when subject
// is nil, with the lines of source around it from sources. entity, header and
// body are empty when the problem is in no entity.
func (pr problem) diagnostic(sources map[string][]byte, entity string, header *hcl.Range, body *hclsyntax.Body, subject *hcl.Range) *errors.Diagnostic {
	atField := subject != nil
	if subject == nil {
		subject = header
	}

	d := errors.NewDiagnostic(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  pr.summary,
		Detail:   pr.detail,
		Subject:  subject,
	}, sources)

	d.Entity = entity
	d.EntityPos = header
	d.Cause = pr.cause

	if atField && body != nil {
		d.Field = fieldAt(body, subject.Start, "")
	}

	return d
}

// inBlock makes the diagnostic for problem in the block b of module, which
// may not have become an entity yet: at subject, or at the block's header
// when subject is nil
func (p *Parser) inBlock(b *hclsyntax.Block, module string, subject *hcl.Range, pr problem) *errors.Diagnostic {
	header := b.DefRange()
	_, entity := p.blockResource(b, module)

	return pr.diagnostic(p.parsedResources.sources, entity, &header, b.Body, subject)
}

// placer makes the diagnostic for a problem at subject within one block, or
// at the block's header when subject is nil
type placer func(subject *hcl.Range, pr problem) *errors.Diagnostic

// inDeclaration returns the placer for the block b, which declares resource
func (p *Parser) inDeclaration(resource any, b *hclsyntax.Block) placer {
	id := ""
	if meta, err := types.GetMeta(resource); err == nil {
		id = meta.ID
	}

	header := b.DefRange()

	return func(subject *hcl.Range, pr problem) *errors.Diagnostic {
		return pr.diagnostic(p.parsedResources.sources, id, &header, b.Body, subject)
	}
}

// unreadable is the diagnostic for files of the extension that could not be
// listed, which is in no file and no entity
func unreadable(extension string, err error) *errors.Diagnostic {
	return problem{
		summary: "Unable to read the configuration",
		detail:  fmt.Sprintf("error finding %s files: %s", extension, err),
		cause:   err,
	}.diagnostic(nil, "", nil, nil, nil)
}

// invalidName is the problem with a block whose name is not valid
func invalidName(err error) problem {
	return problem{summary: "Invalid name", detail: err.Error(), cause: err}
}

// missingMeta is the problem with an entity xcl made but can not read the
// meta of, which never happens to a type it accepted
func missingMeta(err error) problem {
	return problem{
		summary: "Internal error",
		detail:  fmt.Sprintf("unable to get resource meta: %s", err),
		cause:   err,
	}
}

// invalidReferences is the problem with the attribute a, whose references
// could not be read, placed at its value
func invalidReferences(at placer, a *hclsyntax.Attribute, err error) *errors.Diagnostic {
	value := a.Expr.Range()

	return at(&value, problem{
		summary: "Invalid reference",
		detail:  fmt.Sprintf("The references in %s could not be read: %s", a.Name, err),
		cause:   err,
	})
}

// referenceAt returns where the entity id writes link: the reference in its
// body, or its depends_on when no reference is, or nil when it has neither,
// so a problem with the link is placed at the block's header
func (p *Parser) referenceAt(id, link string) *hcl.Range {
	body := p.parsedResources.bodies[id]
	if body == nil {
		return nil
	}

	if at := p.traversalOf(body, link); at != nil {
		return at
	}

	if dependsOn, ok := body.Attributes["depends_on"]; ok {
		at := dependsOn.Expr.Range()
		return &at
	}

	return nil
}

// traversalOf returns the first traversal in body, in the order it was
// written, that refers to link, as referencesIn reads one
func (p *Parser) traversalOf(body *hclsyntax.Body, link string) *hcl.Range {
	attributes := make([]*hclsyntax.Attribute, 0, len(body.Attributes))
	for _, a := range body.Attributes {
		attributes = append(attributes, a)
	}

	sort.Slice(attributes, func(i, j int) bool {
		return attributes[i].SrcRange.Start.Byte < attributes[j].SrcRange.Start.Byte
	})

	for _, a := range attributes {
		for _, traversal := range hclsyntax.Variables(a.Expr) {
			at := traversal.SourceRange()

			reference, err := processScopeTraversal(&hclsyntax.ScopeTraversalExpr{Traversal: traversal, SrcRange: at}, p.isReferenceRoot)
			if err == nil && reference == link {
				return &at
			}
		}
	}

	for _, block := range body.Blocks {
		if at := p.traversalOf(block.Body, link); at != nil {
			return at
		}
	}

	return nil
}

// inEntity makes the diagnostic for problem in the parsed entity id: at
// subject, or at its block's header when subject is nil
func (p *parsed) inEntity(id string, subject *hcl.Range, pr problem) *errors.Diagnostic {
	var header *hcl.Range
	if h, ok := p.headers[id]; ok {
		header = &h
	}

	return pr.diagnostic(p.sources, id, header, p.bodies[id], subject)
}

// inResource makes the diagnostic for problem in the entity r, placed where
// its meta says its block starts. It is for an entity that was not parsed by
// this operation, such as one destroyed from saved state, so it has no
// source.
func inResource(r any, pr problem) *errors.Diagnostic {
	meta, err := types.GetMeta(r)
	if err != nil {
		return pr.diagnostic(nil, "", nil, nil, nil)
	}

	var header *hcl.Range
	if meta.File != "" {
		at := hcl.Pos{Line: meta.Line, Column: meta.Column}
		header = &hcl.Range{Filename: meta.File, Start: at, End: at}
	}

	return pr.diagnostic(nil, meta.ID, header, nil, nil)
}
