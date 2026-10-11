package coreservice

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jumppad-labs/xcl"
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/registry"
)

// configuration is one xcl.Config a host created, behind a handle, with the
// state of the operation running on it.
type configuration struct {
	cfg *xcl.Config

	// events is true when the host asked for events, which are delivered
	// through Poll
	events bool

	// queue carries every task the host polls for
	queue *taskQueue

	mu sync.Mutex

	// inProgress is the operation in progress, nil between operations
	inProgress *operation

	// nextOperation is the id of the last operation started; ids are never
	// reused
	nextOperation uint64
}

// createConfig is CreateConfig: it turns the request into xcl.NewConfig
// options, creates the configuration and returns its handle.
func (s *Service) createConfig(_ *configuration, req *corepb.CreateConfigRequest) (*corepb.CreateConfigResponse, error) {
	options, conf, err := buildOptions(req)
	if err != nil {
		return nil, err
	}

	cfg, err := xcl.NewConfig(options...)
	if err != nil {
		return nil, err
	}

	conf.cfg = cfg

	return &corepb.CreateConfigResponse{Handle: s.add(conf)}, nil
}

// buildOptions maps every field of a CreateConfigRequest to its ConfigOption,
// returning the options and the configuration they belong to.
func buildOptions(req *corepb.CreateConfigRequest) ([]xcl.ConfigOption, *configuration, error) {
	conf := &configuration{events: req.GetEvents(), queue: newTaskQueue()}
	options := []xcl.ConfigOption{}

	if conf.events {
		options = append(options, xcl.WithEventHandler(conf.handleEvent))
	}

	for i, r := range req.GetRegistries() {
		built, err := buildRegistry(r, conf)
		if err != nil {
			return nil, nil, invalidArgument(fmt.Sprintf("registry %d: %s", i, err), "registry", fmt.Sprint(i))
		}

		options = append(options, xcl.WithRegistry(built))
	}

	switch store := req.GetStateStore().GetKind().(type) {
	case nil:
	case *corepb.StateStore_Path:
		options = append(options, xcl.WithStatePath(store.Path))
	case *corepb.StateStore_Host:
		if store.Host {
			options = append(options, xcl.WithStateStore(&hostStateStore{conf: conf}))
		}
	}

	if req.GetStateMask() != nil {
		m, err := buildMask(req.GetStateMask())
		if err != nil {
			return nil, nil, invalidArgument("state_mask: "+err.Error(), "option", "state_mask")
		}

		options = append(options, xcl.WithStateMask(m))
	}

	if size := req.GetEventBufferSize(); size != 0 {
		if size < 0 {
			return nil, nil, invalidArgument("event_buffer_size must not be negative", "option", "event_buffer_size")
		}

		options = append(options, xcl.WithEventBufferSize(int(size)))
	}

	switch req.GetEventData() {
	case corepb.EventDataLevel_EVENT_DATA_NONE:
	case corepb.EventDataLevel_EVENT_DATA_RAW:
		options = append(options, xcl.WithEventData(events.DataRaw))
	case corepb.EventDataLevel_EVENT_DATA_PROCESSED:
		options = append(options, xcl.WithEventData(events.DataProcessed))
	default:
		return nil, nil, invalidArgument(fmt.Sprintf("unknown event_data level %d", req.GetEventData()), "option", "event_data")
	}

	if req.GetEventMask() != nil {
		m, err := buildMask(req.GetEventMask())
		if err != nil {
			return nil, nil, invalidArgument("event_mask: "+err.Error(), "option", "event_mask")
		}

		options = append(options, xcl.WithEventMask(m))
	}

	if req.GetNoEventMask() {
		options = append(options, xcl.WithNoEventMask())
	}

	if vars := req.GetVariablesJson(); len(vars) > 0 {
		variables := map[string]any{}
		if err := json.Unmarshal(vars, &variables); err != nil {
			return nil, nil, invalidArgument("variables_json must be a JSON object: "+err.Error(), "option", "variables_json")
		}

		options = append(options, xcl.WithVariables(variables))
	}

	return options, conf, nil
}

// buildRegistry turns a Registry message into the registry it describes.
func buildRegistry(r *corepb.Registry, conf *configuration) (registry.Registry, error) {
	switch kind := r.GetKind().(type) {
	case *corepb.Registry_Local:
		return buildLocalRegistry(kind.Local, conf)
	case *corepb.Registry_Github:
		return buildGitHubRegistry(kind.Github)
	default:
		return nil, fmt.Errorf("a registry must be local or github")
	}
}

// hostRegistry is the registry a LocalRegistry message becomes: a
// registry.Local holding the declared types, external plugins and plugin
// directories, which also offers the host's functions to configuration.
type hostRegistry struct {
	*registry.Local

	functions []registry.Function
}

// Functions returns the host's functions, built from their declared
// signatures, beside any registered on the local registry.
func (h *hostRegistry) Functions() []registry.Function {
	return append(append([]registry.Function{}, h.Local.Functions()...), h.functions...)
}

// buildLocalRegistry turns a LocalRegistry message into a hostRegistry.
func buildLocalRegistry(local *corepb.LocalRegistry, conf *configuration) (registry.Registry, error) {
	options := []registry.LocalOption{}
	if pattern := local.GetPluginPattern(); pattern != "" {
		options = append(options, registry.PluginPattern(pattern))
	}

	h := &hostRegistry{Local: registry.NewLocal(options...)}

	for _, declared := range local.GetTypes() {
		if err := declareType(h.Local.RegisterType, declared); err != nil {
			return nil, err
		}
	}

	for _, path := range local.GetExternalPlugins() {
		h.Local.RegisterExternalPlugin(path)
	}

	for _, dir := range local.GetPluginDirectories() {
		h.Local.RegisterPluginDirectory(dir)
	}

	for _, declared := range local.GetFunctions() {
		fn, err := makeHostFunction(conf, declared)
		if err != nil {
			return nil, err
		}

		h.functions = append(h.functions, fn)
	}

	return h, nil
}

// buildGitHubRegistry turns a GitHubRegistry message into a registry.GitHub.
func buildGitHubRegistry(gh *corepb.GitHubRegistry) (registry.Registry, error) {
	options := []registry.GitHubOption{}
	if dir := gh.GetCacheDir(); dir != "" {
		options = append(options, registry.GitHubCacheDir(dir))
	}
	if keys := gh.GetTrustedKeys(); len(keys) > 0 {
		options = append(options, registry.GitHubTrustedKeys(keys...))
	}
	if token := gh.GetToken(); token != "" {
		options = append(options, registry.GitHubToken(token))
	}
	if url := gh.GetApiUrl(); url != "" {
		options = append(options, registry.GitHubAPIURL(url))
	}

	built := registry.NewGitHub(options...)

	for _, p := range gh.GetPlugins() {
		if p.GetRepository() == "" || p.GetVersion() == "" {
			return nil, fmt.Errorf("a github plugin needs a repository and a version")
		}

		built.RegisterPlugin(p.GetRepository(), p.GetVersion())
	}

	for _, declared := range gh.GetTypes() {
		if err := declareType(built.RegisterType, declared); err != nil {
			return nil, err
		}
	}

	return built, nil
}

// declareType builds a DeclaredType's schema into a Go type and registers it
// through register, under the type and subtype the message names. The message
// may leave type and subtype empty to take them from the schema.
func declareType(register func(prototype any, name ...string), declared *corepb.DeclaredType) error {
	t, err := registry.TypeFromSchema(declared.GetSchema())
	if err != nil {
		return fmt.Errorf("type %s: %w", declared.GetType(), err)
	}

	if declared.GetType() != "" {
		t.Type = declared.GetType()
		t.Subtype = declared.GetSubtype()
	}

	if t.Subtype == "" {
		register(t.Prototype, t.Type)
	} else {
		register(t.Prototype, t.Type, t.Subtype)
	}

	return nil
}

// buildMask turns a Mask message into a mask.Masker.
func buildMask(m *corepb.Mask) (mask.Masker, error) {
	switch kind := m.GetKind().(type) {
	case *corepb.Mask_Aes256GcmKey:
		return mask.EncryptAES256GCM(kind.Aes256GcmKey)
	case *corepb.Mask_HmacSha256Key:
		return mask.HashHMACSHA256(kind.HmacSha256Key)
	case *corepb.Mask_Redact:
		if !kind.Redact {
			return nil, fmt.Errorf("redact must be true")
		}
		return mask.Redact(), nil
	case *corepb.Mask_Omit:
		if !kind.Omit {
			return nil, fmt.Errorf("omit must be true")
		}
		return mask.Omit(), nil
	default:
		return nil, fmt.Errorf("a mask must be aes256gcm_key, hmac_sha256_key, redact or omit")
	}
}

// makeHostFunction builds the Go func a declared host function is offered to
// configuration as. Host functions are added by a later task.
func makeHostFunction(_ *configuration, declared *corepb.DeclaredFunction) (registry.Function, error) {
	return registry.Function{}, fmt.Errorf("function %s: host functions are not supported yet", declared.GetName())
}
