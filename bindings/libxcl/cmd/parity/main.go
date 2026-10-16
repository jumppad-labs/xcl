// Command parity runs one parity fixture through the Go API and writes a
// normalised JSON record of what happened, so a language binding can run the
// same fixture through its own API and compare. The fixtures live in
// ../../testdata/parity; see its README for their layout.
//
//	parity -fixture <dir> -plugins <dir> -out <file>
//
// The record covers, in order: validate, diff before the apply, apply,
// entities and outputs after it, the state written, the configuration text
// of every entity under each combination of the encode choices, the diff of
// the changed configuration when the fixture has one, entities after a load in a new
// process, and destroy. Temporary paths are replaced by <tmp> and the fixture
// directory by <fixture>, and nothing in the record depends on time, so two
// runs of one fixture give the same record.
//
// With -load, which the program runs itself with for the load stage, it
// loads the state in -state with the fixture's types and prints the entities.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/jumppad-labs/xcl"
	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/types"
)

// recordingPluginName is the file name of the recording fixture plugin
// binary, built from ../../testdata/plugins/recording, under -plugins
const recordingPluginName = "xcl-plugin-recording"

func main() {
	fixture := flag.String("fixture", "", "the fixture directory")
	plugins := flag.String("plugins", "", "the directory holding the fixture plugin binaries")
	out := flag.String("out", "", "the file the record is written to, stdout when empty")
	load := flag.Bool("load", false, "load the state in -state and print the entities")
	state := flag.String("state", "", "the state directory, with -load")
	tmp := flag.String("tmp", "", "the temporary directory of the run, with -load")
	flag.Parse()

	if *fixture == "" {
		fmt.Fprintln(os.Stderr, "parity: -fixture is required")
		os.Exit(2)
	}

	if *load {
		if err := runLoad(*fixture, *plugins, *state, *tmp); err != nil {
			fmt.Fprintln(os.Stderr, "parity:", err)
			os.Exit(1)
		}

		return
	}

	record, err := run(*fixture, *plugins)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parity:", err)
		os.Exit(1)
	}

	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "parity:", err)
		os.Exit(1)
	}

	if *out == "" {
		fmt.Println(string(encoded))
		return
	}

	if err := os.WriteFile(*out, append(encoded, '\n'), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "parity:", err)
		os.Exit(1)
	}
}

// fixtureFiles reads a fixture directory
type fixtureFiles struct {
	dir       string
	main      string
	changed   string
	types     [][]byte
	variables map[string]any

	// handBuilt lists the addresses of the entities a binding builds by hand
	// for the encode stage, from hand_built.json
	handBuilt []string
}

func readFixture(dir string) (*fixtureFiles, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	f := &fixtureFiles{dir: abs, main: filepath.Join(abs, "main.xcl")}

	if _, err := os.Stat(f.main); err != nil {
		return nil, fmt.Errorf("fixture %s has no main.xcl: %w", dir, err)
	}

	changed := filepath.Join(abs, "changed", "main.xcl")
	if _, err := os.Stat(changed); err == nil {
		f.changed = changed
	}

	typesJSON, err := os.ReadFile(filepath.Join(abs, "types.json"))
	if err != nil {
		return nil, fmt.Errorf("fixture %s: %w", dir, err)
	}

	var docs []json.RawMessage
	if err := json.Unmarshal(typesJSON, &docs); err != nil {
		return nil, fmt.Errorf("fixture %s: types.json: %w", dir, err)
	}

	for _, doc := range docs {
		f.types = append(f.types, doc)
	}

	variablesJSON, err := os.ReadFile(filepath.Join(abs, "variables.json"))
	if err == nil {
		if err := json.Unmarshal(variablesJSON, &f.variables); err != nil {
			return nil, fmt.Errorf("fixture %s: variables.json: %w", dir, err)
		}
	}

	handBuiltJSON, err := os.ReadFile(filepath.Join(abs, "hand_built.json"))
	if err == nil {
		if err := json.Unmarshal(handBuiltJSON, &f.handBuilt); err != nil {
			return nil, fmt.Errorf("fixture %s: hand_built.json: %w", dir, err)
		}
	}

	return f, nil
}

// slugify is the function every parity side registers: lower case, spaces
// to dashes
func slugify(title string) (string, error) {
	return strings.ReplaceAll(strings.ToLower(title), " ", "-"), nil
}

// newConfig builds the Config a fixture runs with: its schema types and
// slugify on a local registry, the recording plugin when a plugins directory
// is given, state in stateDir, and the fixture's variables with <tmp>
// expanded to tmp.
func newConfig(f *fixtureFiles, plugins, stateDir, tmp string) (*xcl.Config, error) {
	local := registry.NewLocal()

	for _, doc := range f.types {
		t, err := registry.TypeFromSchema(doc)
		if err != nil {
			return nil, err
		}

		if t.Subtype == "" {
			local.RegisterType(t.Prototype, t.Type)
		} else {
			local.RegisterType(t.Prototype, t.Type, t.Subtype)
		}
	}

	local.RegisterFunction("slugify", slugify)

	if plugins != "" {
		name := recordingPluginName
		if runtime.GOOS == "windows" {
			name += ".exe"
		}

		local.RegisterExternalPlugin(filepath.Join(plugins, name))
	}

	variables := map[string]any{}
	for k, v := range f.variables {
		if s, ok := v.(string); ok {
			v = strings.ReplaceAll(s, "<tmp>", tmp)
		}

		variables[k] = v
	}

	return xcl.NewConfig(
		xcl.WithRegistry(local),
		xcl.WithStatePath(stateDir),
		xcl.WithVariables(variables),
	)
}

// run runs every stage of a fixture and returns the record
func run(fixtureDir, plugins string) (map[string]any, error) {
	f, err := readFixture(fixtureDir)
	if err != nil {
		return nil, err
	}

	tmp, err := os.MkdirTemp("", "xcl-parity-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	// the plugin registry caches under HOME
	os.Setenv("HOME", filepath.Join(tmp, "home"))

	stateDir := filepath.Join(tmp, "state")

	n := &normaliser{tmp: tmp, fixture: f.dir}
	record := map[string]any{}

	c, err := newConfig(f, plugins, stateDir, tmp)
	if err != nil {
		return nil, err
	}

	record["validate"] = n.outcome(c.Validate(f.main))
	record["diff_before"] = n.diff(c.Diff([]string{f.main}))

	applyErr := c.Apply(f.main)
	record["apply"] = n.outcome(applyErr)
	record["entities"] = n.entities(c.Entities())
	record["outputs"] = n.value(c.Outputs())
	record["state"] = n.state(stateDir)
	record["encode"] = n.encode(c, stateDir, f.handBuilt)

	if f.changed != "" {
		record["diff_changed"] = n.diff(c.Diff([]string{f.changed}))
	}

	if applyErr == nil {
		loaded, err := loadInNewProcess(f.dir, plugins, stateDir, tmp)
		if err != nil {
			return nil, fmt.Errorf("load in a new process: %w", err)
		}

		record["load"] = n.value(loaded)

		record["destroy"] = n.outcome(c.Destroy())
		record["after_destroy"] = map[string]any{
			"entities": n.entities(c.Entities()),
			"state":    n.state(stateDir),
		}
	}

	return record, nil
}

// loadInNewProcess runs this program with -load and returns the entities it
// printed
func loadInNewProcess(fixtureDir, plugins, stateDir, tmp string) (any, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(exe, "-load", "-fixture", fixtureDir, "-plugins", plugins, "-state", stateDir, "-tmp", tmp)
	cmd.Env = append(os.Environ(), "HOME="+filepath.Join(tmp, "home"))
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var loaded any
	if err := json.Unmarshal(out, &loaded); err != nil {
		return nil, fmt.Errorf("the load printed %q: %w", string(out), err)
	}

	return loaded, nil
}

// runLoad is the -load mode: load the state and print the entities
func runLoad(fixtureDir, plugins, stateDir, tmp string) error {
	f, err := readFixture(fixtureDir)
	if err != nil {
		return err
	}

	c, err := newConfig(f, plugins, stateDir, tmp)
	if err != nil {
		return err
	}

	n := &normaliser{tmp: tmp, fixture: f.dir}

	result := map[string]any{"load": n.outcome(c.Load())}
	result["entities"] = n.entities(c.Entities())

	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}

	fmt.Println(string(encoded))

	return nil
}

// normaliser renders values with the run's paths replaced, so the record is
// the same wherever and whenever it is made
type normaliser struct {
	tmp     string
	fixture string
}

// outcome records whether an operation succeeded, and on failure the error's
// code and each problem it collects, positioned
func (n *normaliser) outcome(err error) map[string]any {
	if err == nil {
		return map[string]any{"ok": true}
	}

	out := map[string]any{"ok": false, "error": n.problem(err)}

	var ce *xclerrors.ConfigError
	if errors.As(err, &ce) {
		problems := []any{}
		for _, cause := range ce.Errors {
			problems = append(problems, n.problem(cause))
		}

		// problems are compared as a set: the parser reports them in the
		// order it finds them, which both sides share, but sorting keeps
		// the record stable
		sort.Slice(problems, func(i, j int) bool {
			return fmt.Sprint(problems[i]) < fmt.Sprint(problems[j])
		})

		out["problems"] = problems
	}

	return out
}

// problem renders one error: its code, and for a diagnostic its file, line,
// column, summary, detail and the block and field it is in
func (n *normaliser) problem(err error) map[string]any {
	var d *xclerrors.Diagnostic
	var ce *xclerrors.ConfigError

	switch {
	case errors.As(err, &d):
		record := map[string]any{
			"code":    "config",
			"summary": n.text(d.Summary),
			"detail":  n.text(d.Detail),
			"entity":  d.Entity,
			"field":   d.Field,
		}
		if d.Subject != nil {
			record["file"] = n.path(d.Subject.Filename)
			record["line"] = d.Subject.Start.Line
			record["column"] = d.Subject.Start.Column
		}

		return record
	case errors.As(err, &ce):
		return map[string]any{"code": "config"}
	default:
		return map[string]any{"code": "internal", "message": n.text(err.Error())}
	}
}

// diff renders a diff, or the error it failed with
func (n *normaliser) diff(d any, err error) any {
	if err != nil {
		return n.outcome(err)
	}

	return n.value(d)
}

// entities renders entities as JSON, sorted by id
func (n *normaliser) entities(entities []any) any {
	rendered := []any{}
	for _, e := range entities {
		rendered = append(rendered, n.value(e))
	}

	sortByID(rendered)

	return rendered
}

// state renders the records of the state directory, sorted by id
func (n *normaliser) state(stateDir string) any {
	path := filepath.Join(stateDir, "state.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var records []any
	if err := json.Unmarshal(data, &records); err != nil {
		return map[string]any{"error": "state is not a JSON array"}
	}

	rendered := []any{}
	for _, r := range records {
		rendered = append(rendered, n.walk(r))
	}

	sortByID(rendered)

	return rendered
}

// encodeChoice is one of the encode choices, named as the record names it
type encodeChoice struct {
	name   string
	option xcl.EncodeOption
}

// encodeChoices are the four encode choices, in the order their names are
// joined in a combination's name
var encodeChoices = []encodeChoice{
	{name: "computed", option: xcl.IncludeComputed()},
	{name: "empty", option: xcl.IncludeEmpty()},
	{name: "reveal", option: xcl.RevealSensitive()},
	{name: "references", option: xcl.ShowReferences()},
}

// encodeCombination is one combination of the encode choices
type encodeCombination struct {
	name    string
	options []xcl.EncodeOption
}

// encodeCombinations returns all sixteen combinations of the encode choices.
// A combination is named by the choices it turns on, joined with + in the
// order of encodeChoices, and none when it turns on none.
func encodeCombinations() []encodeCombination {
	combinations := []encodeCombination{}

	for mask := 0; mask < 1<<len(encodeChoices); mask++ {
		names := []string{}
		options := []xcl.EncodeOption{}

		for bit, choice := range encodeChoices {
			if mask&(1<<bit) != 0 {
				names = append(names, choice.name)
				options = append(options, choice.option)
			}
		}

		name := "none"
		if len(names) > 0 {
			name = strings.Join(names, "+")
		}

		combinations = append(combinations, encodeCombination{name: name, options: options})
	}

	return combinations
}

// isBuiltinType reports whether a block type is one of xcl's own, which the
// encoder refuses and the encode stage leaves out
func isBuiltinType(blockType string) bool {
	switch blockType {
	case "variable", "output", "module", "root":
		return true
	}

	return false
}

// encode records, for every entity that is not a builtin, keyed by its
// address, its configuration text under every combination of the encode
// choices: looked_up, the entity the Config holds, as a binding's lookup
// hands it out; saved, its record in state; and, for the addresses the
// fixture lists in hand_built.json, hand_built, the text a binding's hand
// built copy of it must give, which is the entity's own. A failure is
// recorded as its outcome.
func (n *normaliser) encode(c *xcl.Config, stateDir string, handBuilt []string) map[string]any {
	saved := n.savedRecords(stateDir)

	listed := map[string]bool{}
	for _, address := range handBuilt {
		listed[address] = true
	}

	encoded := map[string]any{}

	for _, entity := range c.Entities() {
		meta, err := types.GetMeta(entity)
		if err != nil || isBuiltinType(meta.Type) {
			continue
		}

		lookedUp := map[string]any{}
		savedTexts := map[string]any{}

		for _, combination := range encodeCombinations() {
			lookedUp[combination.name] = n.encodedText(xcl.EncodeEntity(entity, combination.options...))

			record, found := saved[meta.ID]
			if !found {
				savedTexts[combination.name] = map[string]any{"ok": false, "error": map[string]any{"code": "missing_saved_record"}}
				continue
			}

			savedTexts[combination.name] = n.encodedText(c.EncodeSavedEntity(record, combination.options...))
		}

		texts := map[string]any{"looked_up": lookedUp, "saved": savedTexts}
		if listed[meta.ID] {
			texts["hand_built"] = lookedUp
		}

		encoded[meta.ID] = texts
	}

	return encoded
}

// savedRecords returns the records of the state directory by entity id
func (n *normaliser) savedRecords(stateDir string) map[string][]byte {
	records := map[string][]byte{}

	data, err := os.ReadFile(filepath.Join(stateDir, "state.json"))
	if err != nil {
		return records
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return records
	}

	for _, record := range raw {
		var identity struct {
			Meta struct {
				ID string `json:"id"`
			} `json:"meta"`
		}

		if err := json.Unmarshal(record, &identity); err != nil {
			continue
		}

		records[identity.Meta.ID] = record
	}

	return records
}

// encodedText renders encoded text with the run's paths replaced, or the
// error it failed with
func (n *normaliser) encodedText(text []byte, err error) any {
	if err != nil {
		return n.outcome(err)
	}

	return n.text(string(text))
}

// value renders any value through JSON and replaces the run's paths
func (n *normaliser) value(v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return map[string]any{"error": err.Error()}
	}

	return n.walk(decoded)
}

// walk replaces paths in every string of a decoded JSON value
func (n *normaliser) walk(v any) any {
	switch value := v.(type) {
	case string:
		return n.text(value)
	case []any:
		for i := range value {
			value[i] = n.walk(value[i])
		}
		return value
	case map[string]any:
		for k := range value {
			value[k] = n.walk(value[k])
		}
		return value
	default:
		return v
	}
}

// path replaces the fixture and temporary directories in a path
func (n *normaliser) path(p string) string {
	return n.text(filepath.ToSlash(p))
}

// text replaces the fixture and temporary directories wherever they appear
func (n *normaliser) text(s string) string {
	for _, dir := range []string{n.fixture, filepath.ToSlash(n.fixture)} {
		s = strings.ReplaceAll(s, dir, "<fixture>")
	}

	for _, dir := range []string{n.tmp, filepath.ToSlash(n.tmp)} {
		s = strings.ReplaceAll(s, dir, "<tmp>")
	}

	return s
}

// sortByID sorts rendered entities by their meta id
func sortByID(rendered []any) {
	sort.SliceStable(rendered, func(i, j int) bool {
		return entityID(rendered[i]) < entityID(rendered[j])
	})
}

func entityID(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}

	meta, ok := m["meta"].(map[string]any)
	if !ok {
		return ""
	}

	id, _ := meta["id"].(string)

	return id
}
