package coreservice

import (
	"bytes"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/highlight"
)

// convertTheme is ConvertTheme: a VS Code colour theme turned into a style
// string per kind number, by highlight.LoadVSCodeTheme, which holds the
// TextMate selector rules. It takes handle 0, so it needs no configuration.
// A theme LoadVSCodeTheme cannot use fails with its InvalidThemeError, which
// crosses as invalid_theme.
func (s *Service) convertTheme(_ *configuration, req *corepb.ConvertThemeRequest) (*corepb.ConvertThemeResponse, error) {
	if len(req.GetVscodeThemeJson()) == 0 {
		return nil, invalidArgument("a theme is required", "argument", "vscode_theme_json")
	}

	theme, err := highlight.LoadVSCodeTheme(bytes.NewReader(req.GetVscodeThemeJson()))
	if err != nil {
		return nil, err
	}

	styles := make(map[int32]string, len(theme))
	for kind, style := range theme {
		styles[int32(kind)] = style
	}

	return &corepb.ConvertThemeResponse{Styles: styles}, nil
}
