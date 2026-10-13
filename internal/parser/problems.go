package parser

import (
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
