package template

import (
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// TemplatePlugin provides the block type template. It is compiled into the
// application, which registers it with RegisterPlugin. xcl names an
// in-process plugin after its Go type, so its log lines come from
// TemplatePlugin.
type TemplatePlugin struct {
	plugins.PluginBase
}

var _ plugins.Plugin = (*TemplatePlugin)(nil)

// Init registers template with no subtype, so a block is written
// template "<name>" {} and addressed template.<name>
func (p *TemplatePlugin) Init(logger logger.Logger, state plugins.State) error {
	logger.Debug("registering block types", "block_types", "template")

	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"template",
		"",
		&Template{},
		&provider{},
	)
}
