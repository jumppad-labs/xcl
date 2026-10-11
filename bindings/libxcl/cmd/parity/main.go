// Command parity runs one parity fixture through the Go API and writes a
// normalised JSON record of what happened, so a language binding can run the
// same fixture through its own API and compare. The fixtures live in
// ../../testdata/parity; see its README for their layout.
//
//	parity -fixture <dir> -plugins <dir> -out <file>
//
// The record covers, in order: validate, diff before the apply, apply,
// entities and outputs after it, the state written, the diff of the changed
// configuration when the fixture has one, entities after a load in a new
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

// problem renders one error: its code, and for a positioned problem its
// file, line, column and summary
func (n *normaliser) problem(err error) map[string]any {
	var pe *xclerrors.ParserError
	var ce *xclerrors.ConfigError

	switch {
	case errors.As(err, &pe):
		return map[string]any{
			"code":    "config",
			"file":    n.path(pe.Filename),
			"line":    pe.Line,
			"column":  pe.Column,
			"summary": n.text(pe.Message),
		}
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
