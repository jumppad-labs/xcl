package template

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/logger"
)

func TestPluginRegistersTemplateWithoutASubtype(t *testing.T) {
	p := &TemplatePlugin{}

	err := p.Init(logger.Nop(), nil)
	require.NoError(t, err)

	registered := p.GetTypes()
	require.Len(t, registered, 1)
	require.Equal(t, "template", registered[0].Type)
	require.Equal(t, "", registered[0].SubType)
}
