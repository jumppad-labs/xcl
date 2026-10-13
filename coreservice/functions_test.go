package coreservice

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/state"
)

// slugSchema declares the app type the host function tests configure: a
// name, and a sensitive secret and token
const slugSchema = `{"schema_version":1,"type":"app","fields":[{"name":"name","type":"string","optional":true},{"name":"secret","type":"string","optional":true,"sensitive":true},{"name":"token","type":"string","optional":true,"sensitive":true},{"name":"number","type":"number","optional":true}]}`

// slugConfig calls slugify once with a literal and once with a sensitive
// value read from another block
const slugConfig = "app \"a\" {\n  secret = \"Hunter Two\"\n}\n\napp \"b\" {\n  name  = slugify(\"My App\")\n  token = slugify(app.a.secret)\n}\n"

// slugifySignature is slugify's signature: one string in, a string out
const slugifySignature = `{"params":[{"name":"title","type":"string"}],"return":"string"}`

// declare is a DeclaredFunction with the given name and signature
func declare(name, signature string) *corepb.DeclaredFunction {
	return &corepb.DeclaredFunction{Name: name, Signature: []byte(signature)}
}

// functionRequest is a CreateConfigRequest declaring the app type from
// slugSchema and the given host functions, keeping state under stateDir
func functionRequest(t *testing.T, stateDir string, functions ...*corepb.DeclaredFunction) *corepb.CreateConfigRequest {
	t.Helper()

	return &corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{localRegistry(&corepb.LocalRegistry{
			Types:     []*corepb.DeclaredType{{Schema: []byte(slugSchema)}},
			Functions: functions,
		})},
		StateStore: &corepb.StateStore{Kind: &corepb.StateStore_Path{Path: stateDir}},
	}
}

// functionResult is a FunctionResult carrying value encoded as JSON
func functionResult(t *testing.T, value any) *corepb.FunctionResult {
	t.Helper()

	encoded, err := json.Marshal(value)
	require.NoError(t, err)

	return &corepb.FunctionResult{Result: encoded}
}

// functionError is a FunctionResult carrying the host's error message
func functionError(message string) *corepb.FunctionResult {
	return &corepb.FunctionResult{Error: &corepb.Error{Code: CodeInternal, Message: message}}
}

// slugify is the host's implementation of slugify: lowercase, spaces to
// dashes
func slugify(t *testing.T, task *corepb.FunctionTask) *corepb.FunctionResult {
	t.Helper()

	var args []string
	require.NoError(t, json.Unmarshal(task.GetArgs(), &args))
	require.Len(t, args, 1)

	return functionResult(t, strings.ReplaceAll(strings.ToLower(args[0]), " ", "-"))
}

// applySlugConfig creates a configuration declaring slugify, answers every
// call with answer, applies slugConfig and returns the client, handle, state
// directory, the finished task and every task the host saw
func applySlugConfig(t *testing.T, answer func(*corepb.FunctionTask) *corepb.FunctionResult) (*testClient, uint64, string, *corepb.OperationFinished, []*corepb.FunctionTask) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	stateDir := t.TempDir()

	c := newTestClient(t)

	var tasks []*corepb.FunctionTask
	c.functions = func(task *corepb.FunctionTask) *corepb.FunctionResult {
		tasks = append(tasks, task)
		return answer(task)
	}

	handle := c.createConfig(functionRequest(t, stateDir, declare("slugify", slugifySignature)))

	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, slugConfig)}})
	finished := c.drive(handle, opID, nil, nil)

	return c, handle, stateDir, finished, tasks
}

// taskWithArgs reports whether one of tasks is a call of name whose args
// are the JSON array want
func taskWithArgs(t *testing.T, tasks []*corepb.FunctionTask, name, want string) bool {
	t.Helper()

	for _, task := range tasks {
		if task.GetName() != name {
			continue
		}

		var got, expected any
		require.NoError(t, json.Unmarshal(task.GetArgs(), &got))
		require.NoError(t, json.Unmarshal([]byte(want), &expected))

		if reflect.DeepEqual(got, expected) {
			return true
		}
	}

	return false
}

// mentions reports whether err, or any error it collects, carries text in
// its message or its summary
func mentions(err *corepb.Error, text string) bool {
	if err == nil {
		return false
	}

	if strings.Contains(err.GetMessage(), text) || strings.Contains(err.GetDetails()["summary"], text) {
		return true
	}

	for _, cause := range err.GetCauses() {
		if mentions(cause, text) {
			return true
		}
	}

	return false
}

func TestHostFunctionResultFilledIn(t *testing.T) {
	c, handle, _, finished, _ := applySlugConfig(t, func(task *corepb.FunctionTask) *corepb.FunctionResult {
		return slugify(t, task)
	})

	require.Nil(t, finished.GetError(), "apply failed: %s", finished.GetError().GetMessage())

	entity := c.find(handle, "app.b")
	require.Contains(t, string(entity), `"name":"my-app"`)
}

func TestHostFunctionTaskCarriesNameAndJSONArgs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	var tasks []*corepb.FunctionTask
	c.functions = func(task *corepb.FunctionTask) *corepb.FunctionResult {
		tasks = append(tasks, task)
		return slugify(t, task)
	}

	handle := c.createConfig(functionRequest(t, t.TempDir(), declare("slugify", slugifySignature)))

	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, slugConfig)}})
	finished := c.drive(handle, opID, nil, nil)
	require.Nil(t, finished.GetError(), "apply failed: %s", finished.GetError().GetMessage())

	require.NotEmpty(t, tasks)
	for _, task := range tasks {
		require.Equal(t, opID, task.GetOperationId())
		require.NotZero(t, task.GetTaskId())
	}

	require.True(t, taskWithArgs(t, tasks, "slugify", `["My App"]`), "no slugify task carried [\"My App\"]")
}

func TestHostFunctionErrorFailsApplyAtPosition(t *testing.T) {
	_, _, _, finished, _ := applySlugConfig(t, func(*corepb.FunctionTask) *corepb.FunctionResult {
		return functionError("no slug for you")
	})

	require.NotNil(t, finished.GetError())
	require.Equal(t, CodeConfig, finished.GetError().GetCode())
	require.NotEmpty(t, finished.GetError().GetCauses())

	var positioned *corepb.Error
	for _, cause := range finished.GetError().GetCauses() {
		if mentions(cause, "no slug for you") {
			positioned = cause
			break
		}
	}
	require.NotNil(t, positioned, "no cause mentions the host's error: %v", finished.GetError())

	require.NotEmpty(t, positioned.GetDetails()["file"])

	line, err := strconv.Atoi(positioned.GetDetails()["line"])
	require.NoError(t, err)
	require.Greater(t, line, 0)
}

func TestHostFunctionClashWithBuiltinFailsCreateConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", functionRequest(t, t.TempDir(), declare("upper", slugifySignature)))

	require.Equal(t, CodeFunctionNameClash, out.GetCode())
	require.Equal(t, "upper", out.GetDetails()["name"])
	require.Equal(t, "builtin", out.GetDetails()["existing"])
}

func TestTwoHostFunctionsSameNameFailCreateConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", functionRequest(t, t.TempDir(),
		declare("slugify", slugifySignature),
		declare("slugify", slugifySignature),
	))

	require.Equal(t, CodeFunctionNameClash, out.GetCode())
	require.Equal(t, "slugify", out.GetDetails()["name"])
}

func TestHostFunctionUnsupportedTypeFailsCreateConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", functionRequest(t, t.TempDir(),
		declare("anything", `{"params":[{"name":"value","type":"dynamic"}],"return":"string"}`),
	))

	require.Equal(t, CodeInvalidFunction, out.GetCode())
	require.Equal(t, "anything", out.GetDetails()["name"])
	require.Equal(t, "local", out.GetDetails()["registry"])
	require.Contains(t, out.GetMessage(), "dynamic")
}

func TestHostFunctionVariadicFailsCreateConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", functionRequest(t, t.TempDir(),
		declare("join", `{"params":[{"name":"first","type":"string"}],"variadic":{"name":"rest","type":"string"},"return":"string"}`),
	))

	require.Equal(t, CodeInvalidFunction, out.GetCode())
	require.Equal(t, "join", out.GetDetails()["name"])
	require.Contains(t, out.GetMessage(), "variadic")
}

func TestHostFunctionMalformedSignatureFailsCreateConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", functionRequest(t, t.TempDir(), declare("broken", `{"params":[`)))

	require.Equal(t, CodeInvalidFunction, out.GetCode())
	require.Equal(t, "broken", out.GetDetails()["name"])
	require.Contains(t, out.GetMessage(), "signature")
}

func TestHostFunctionMissingReturnFailsCreateConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", functionRequest(t, t.TempDir(),
		declare("noreturn", `{"params":[{"name":"title","type":"string"}]}`),
	))

	require.Equal(t, CodeInvalidFunction, out.GetCode())
	require.Equal(t, "noreturn", out.GetDetails()["name"])
	require.Contains(t, out.GetMessage(), "return")
}

func TestHostFunctionEmptyNameFailsCreateConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	out := c.fail(0, "CreateConfig", functionRequest(t, t.TempDir(), declare("", slugifySignature)))

	require.Equal(t, CodeInvalidFunction, out.GetCode())
	require.Contains(t, out.GetMessage(), "named")
}

func TestHostFunctionReceivesSensitiveArgumentUnmarked(t *testing.T) {
	_, _, _, finished, tasks := applySlugConfig(t, func(task *corepb.FunctionTask) *corepb.FunctionResult {
		return slugify(t, task)
	})

	require.Nil(t, finished.GetError(), "apply failed: %s", finished.GetError().GetMessage())
	require.True(t, taskWithArgs(t, tasks, "slugify", `["Hunter Two"]`), "no slugify task carried the plain sensitive value")
}

func TestHostFunctionResultMarkedSensitive(t *testing.T) {
	c, handle, stateDir, finished, _ := applySlugConfig(t, func(task *corepb.FunctionTask) *corepb.FunctionResult {
		return slugify(t, task)
	})

	require.Nil(t, finished.GetError(), "apply failed: %s", finished.GetError().GetMessage())

	entity := c.find(handle, "app.b")
	require.Contains(t, string(entity), `"token":"(sensitive)"`)
	require.NotContains(t, string(entity), "hunter-two")

	saved, err := os.ReadFile(filepath.Join(stateDir, state.StateFileName))
	require.NoError(t, err)
	require.Contains(t, string(saved), "hunter-two")
}

func TestCancelWithOutstandingFunctionTaskEndsPromptly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(functionRequest(t, t.TempDir(), declare("slugify", slugifySignature)))

	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, slugConfig)}})

	// the first call is left outstanding and the operation cancelled while
	// it waits; any further call is left outstanding too
	var outstanding uint64
	c.functions = func(task *corepb.FunctionTask) *corepb.FunctionResult {
		if outstanding == 0 {
			outstanding = task.GetTaskId()
			c.ok(handle, "Cancel", &corepb.CancelRequest{OperationId: opID}, &corepb.CancelResponse{})
		}

		return nil
	}

	began := time.Now()
	finished := c.drive(handle, opID, nil, nil)

	require.NotZero(t, outstanding, "no function task arrived before the operation finished")
	require.Less(t, time.Since(began), 5*time.Second)

	require.NotNil(t, finished.GetError())
	require.Equal(t, CodeCancelled, finished.GetError().GetCode())

	// the cancelled call is no longer waiting, so a late answer is refused
	out := c.fail(handle, "Complete", &corepb.CompleteRequest{
		TaskId: outstanding,
		Result: &corepb.CompleteRequest_Function{Function: functionResult(t, "late")},
	})
	require.Equal(t, CodeInvalidArgument, out.GetCode())
}

func TestHostFunctionListAndMapArguments(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	var tasks []*corepb.FunctionTask
	c.functions = func(task *corepb.FunctionTask) *corepb.FunctionResult {
		tasks = append(tasks, task)
		return functionResult(t, "described")
	}

	handle := c.createConfig(functionRequest(t, t.TempDir(),
		declare("describe", `{"params":[{"name":"names","type":["list","string"]},{"name":"ports","type":["map","number"]}],"return":"string"}`),
	))

	config := "app \"a\" {\n  name = describe([\"a\", \"b\"], { http = 80 })\n}\n"
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, config)}})
	finished := c.drive(handle, opID, nil, nil)
	require.Nil(t, finished.GetError(), "apply failed: %s", finished.GetError().GetMessage())

	require.True(t, taskWithArgs(t, tasks, "describe", `[["a","b"],{"http":80}]`), "no describe task carried the list and map")
	require.Contains(t, string(c.find(handle, "app.a")), `"name":"described"`)
}

func TestHostFunctionNumberAndBoolRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	var tasks []*corepb.FunctionTask
	c.functions = func(task *corepb.FunctionTask) *corepb.FunctionResult {
		tasks = append(tasks, task)
		return functionResult(t, 42)
	}

	handle := c.createConfig(functionRequest(t, t.TempDir(),
		declare("double", `{"params":[{"name":"value","type":"number"},{"name":"really","type":"bool"}],"return":"number"}`),
	))

	config := "app \"a\" {\n  number = double(21, true)\n}\n"
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, config)}})
	finished := c.drive(handle, opID, nil, nil)
	require.Nil(t, finished.GetError(), "apply failed: %s", finished.GetError().GetMessage())

	require.True(t, taskWithArgs(t, tasks, "double", `[21,true]`), "no double task carried [21,true]")
	require.Contains(t, string(c.find(handle, "app.a")), `"number":42`)
}

func TestHostFunctionBadResultFailsCall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	c.functions = func(*corepb.FunctionTask) *corepb.FunctionResult {
		return functionResult(t, "x")
	}

	handle := c.createConfig(functionRequest(t, t.TempDir(),
		declare("double", `{"params":[{"name":"value","type":"number"},{"name":"really","type":"bool"}],"return":"number"}`),
	))

	config := "app \"a\" {\n  number = double(21, true)\n}\n"
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, config)}})
	finished := c.drive(handle, opID, nil, nil)

	require.NotNil(t, finished.GetError())
	require.True(t, mentions(finished.GetError(), "double"), "the error does not name the function: %v", finished.GetError())
	require.True(t, mentions(finished.GetError(), "number"), "the error does not name the expected type: %v", finished.GetError())
}

func TestMakeHostFuncVariadic(t *testing.T) {
	sig := plugins.FunctionSignature{
		Params:   []plugins.FunctionParam{{Name: "first", Type: json.RawMessage(`"string"`)}},
		Variadic: &plugins.FunctionParam{Name: "rest", Type: json.RawMessage(`"string"`)},
		Return:   json.RawMessage(`"string"`),
	}

	fn, err := makeHostFunc(nil, "join", sig)
	require.Error(t, err)
	require.Nil(t, fn)
	require.Contains(t, err.Error(), "variadic")
}

func TestMakeHostFuncBuildsTypedFunc(t *testing.T) {
	sig := plugins.FunctionSignature{
		Params: []plugins.FunctionParam{
			{Name: "value", Type: json.RawMessage(`"number"`)},
			{Name: "names", Type: json.RawMessage(`["list","string"]`)},
		},
		Return: json.RawMessage(`"bool"`),
	}

	fn, err := makeHostFunc(nil, "check", sig)
	require.NoError(t, err)

	typ := reflect.TypeOf(fn)
	require.Equal(t, reflect.Func, typ.Kind())
	require.Equal(t, 2, typ.NumIn())
	require.Equal(t, reflect.TypeOf(float64(0)), typ.In(0))
	require.Equal(t, reflect.TypeOf([]string{}), typ.In(1))
	require.Equal(t, 2, typ.NumOut())
	require.Equal(t, reflect.TypeOf(true), typ.Out(0))
	require.Equal(t, errorType, typ.Out(1))
}

func TestGoTypeOfString(t *testing.T) {
	typ, err := goTypeOf(json.RawMessage(`"string"`))
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(""), typ)
}

func TestGoTypeOfBool(t *testing.T) {
	typ, err := goTypeOf(json.RawMessage(`"bool"`))
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(true), typ)
}

func TestGoTypeOfNumberIsFloat64(t *testing.T) {
	typ, err := goTypeOf(json.RawMessage(`"number"`))
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(float64(0)), typ)
}

func TestGoTypeOfListIsSlice(t *testing.T) {
	typ, err := goTypeOf(json.RawMessage(`["list","string"]`))
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf([]string{}), typ)
}

func TestGoTypeOfSetIsSlice(t *testing.T) {
	typ, err := goTypeOf(json.RawMessage(`["set","number"]`))
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf([]float64{}), typ)
}

func TestGoTypeOfMapIsStringKeyed(t *testing.T) {
	typ, err := goTypeOf(json.RawMessage(`["map","number"]`))
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(map[string]float64{}), typ)
}

func TestGoTypeOfNestedListOfMaps(t *testing.T) {
	typ, err := goTypeOf(json.RawMessage(`["list",["map","bool"]]`))
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf([]map[string]bool{}), typ)
}

func TestGoTypeOfDynamicIsRefused(t *testing.T) {
	typ, err := goTypeOf(json.RawMessage(`"dynamic"`))
	require.Error(t, err)
	require.Nil(t, typ)
	require.Contains(t, err.Error(), "dynamic")
}

func TestGoTypeOfObjectIsRefused(t *testing.T) {
	typ, err := goTypeOf(json.RawMessage(`["object",{"name":"string"}]`))
	require.Error(t, err)
	require.Nil(t, typ)
	require.Contains(t, err.Error(), "object")
}

func TestGoTypeOfEmptyIsRefused(t *testing.T) {
	typ, err := goTypeOf(nil)
	require.Error(t, err)
	require.Nil(t, typ)
}
