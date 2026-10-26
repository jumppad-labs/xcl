package coreservice

import (
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/highlight"
)

// tokensToProto turns labelled text into protocol tokens, in order. Kind
// numbers cross unchanged: TokenKind is numbered as highlight.Kind, so a kind
// added to the Go library crosses with no change here. No tokens give nil.
func tokensToProto(tokens []highlight.Token) []*corepb.Token {
	if len(tokens) == 0 {
		return nil
	}

	out := make([]*corepb.Token, 0, len(tokens))
	for _, token := range tokens {
		out = append(out, &corepb.Token{Kind: corepb.TokenKind(token.Kind), Text: token.Text})
	}

	return out
}
