package coreservice

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	xclerrors "github.com/jumppad-labs/xcl/errors"
)

func TestErrorCodeNilIsNil(t *testing.T) {
	require.Nil(t, toProto(nil))
}

func TestErrorCodeNotFound(t *testing.T) {
	out := toProto(&xclerrors.NotFoundError{Address: "app.nope"})

	require.Equal(t, CodeNotFound, out.GetCode())
	require.Equal(t, "app.nope", out.GetDetails()["address"])
	require.NotEmpty(t, out.GetMessage())
}

func TestErrorCodeNotUnique(t *testing.T) {
	out := toProto(&xclerrors.NotUniqueError{Count: 2, Segments: []string{"a", "b"}})

	require.Equal(t, CodeNotUnique, out.GetCode())
	require.Equal(t, "2", out.GetDetails()["count"])
	require.Equal(t, "a.b", out.GetDetails()["segments"])
}

func TestErrorCodeUnknownType(t *testing.T) {
	out := toProto(&xclerrors.UnknownTypeError{Name: "nope", Segments: []string{"nope", "x"}})

	require.Equal(t, CodeUnknownType, out.GetCode())
	require.Equal(t, "nope", out.GetDetails()["name"])
	require.Equal(t, "nope.x", out.GetDetails()["segments"])
}

func TestErrorCodeNotTypeable(t *testing.T) {
	out := toProto(&xclerrors.NotTypeableError{Segments: []string{"resource"}, Use: "FindEntitiesByType"})

	require.Equal(t, CodeNotTypeable, out.GetCode())
	require.Equal(t, "resource", out.GetDetails()["segments"])
	require.Equal(t, "FindEntitiesByType", out.GetDetails()["use"])
}

func TestErrorCodeNotRegistered(t *testing.T) {
	out := toProto(&xclerrors.NotRegisteredError{Type: reflect.TypeOf(""), Use: "FindOneEntity"})

	require.Equal(t, CodeNotRegistered, out.GetCode())
	require.Equal(t, "string", out.GetDetails()["type"])
	require.Equal(t, "FindOneEntity", out.GetDetails()["use"])
}

func TestErrorCodeTypeMismatch(t *testing.T) {
	out := toProto(&xclerrors.TypeMismatchError{Address: "app.web", Want: reflect.TypeOf(0), Got: "app", Field: "password"})

	require.Equal(t, CodeTypeMismatch, out.GetCode())
	require.Equal(t, "app.web", out.GetDetails()["address"])
	require.Equal(t, "int", out.GetDetails()["want"])
	require.Equal(t, "app", out.GetDetails()["got"])
	require.Equal(t, "password", out.GetDetails()["field"])
}

func TestErrorCodeNotAnEntity(t *testing.T) {
	out := toProto(&xclerrors.NotAnEntityError{Type: reflect.TypeOf(""), ReachedThrough: "app"})

	require.Equal(t, CodeNotAnEntity, out.GetCode())
	require.Equal(t, "string", out.GetDetails()["type"])
	require.Equal(t, "app", out.GetDetails()["reached_through"])
}

func TestErrorCodeTypeNameClash(t *testing.T) {
	out := toProto(&xclerrors.TypeNameClashError{
		Name: "app", Provider: "p", Registry: "r", Existing: "e", ExistingRegistry: "er",
	})

	require.Equal(t, CodeTypeNameClash, out.GetCode())
	require.Equal(t, "app", out.GetDetails()["name"])
	require.Equal(t, "p", out.GetDetails()["provider"])
	require.Equal(t, "r", out.GetDetails()["registry"])
	require.Equal(t, "e", out.GetDetails()["existing"])
	require.Equal(t, "er", out.GetDetails()["existing_registry"])
}

func TestErrorCodeTypeForm(t *testing.T) {
	out := toProto(&xclerrors.TypeFormError{Type: "app", TakesSubtype: true})

	require.Equal(t, CodeTypeForm, out.GetCode())
	require.Equal(t, "app", out.GetDetails()["type"])
	require.Equal(t, "true", out.GetDetails()["takes_subtype"])
}

func TestErrorCodeInvalidType(t *testing.T) {
	out := toProto(&xclerrors.InvalidTypeError{Type: "app", Provider: "p", Registry: "r", Err: errors.New("bad")})

	require.Equal(t, CodeInvalidType, out.GetCode())
	require.Equal(t, "app", out.GetDetails()["type"])
	require.Equal(t, "p", out.GetDetails()["provider"])
	require.Equal(t, "r", out.GetDetails()["registry"])
}

func TestErrorCodeFunctionNameClash(t *testing.T) {
	out := toProto(&xclerrors.FunctionNameClashError{
		Name: "greet", Provider: "p", Registry: "r", Existing: "e", ExistingRegistry: "er",
	})

	require.Equal(t, CodeFunctionNameClash, out.GetCode())
	require.Equal(t, "greet", out.GetDetails()["name"])
	require.Equal(t, "p", out.GetDetails()["provider"])
	require.Equal(t, "r", out.GetDetails()["registry"])
	require.Equal(t, "e", out.GetDetails()["existing"])
	require.Equal(t, "er", out.GetDetails()["existing_registry"])
}

func TestErrorCodeInvalidFunction(t *testing.T) {
	out := toProto(&xclerrors.InvalidFunctionError{Name: "greet", Registry: "r", Err: errors.New("bad")})

	require.Equal(t, CodeInvalidFunction, out.GetCode())
	require.Equal(t, "greet", out.GetDetails()["name"])
	require.Equal(t, "r", out.GetDetails()["registry"])
}

func TestErrorCodePluginLoad(t *testing.T) {
	out := toProto(&xclerrors.PluginLoadError{Plugin: "p", Registry: "r", Err: errors.New("boom")})

	require.Equal(t, CodePluginLoad, out.GetCode())
	require.Equal(t, "p", out.GetDetails()["plugin"])
	require.Equal(t, "r", out.GetDetails()["registry"])
}

func TestErrorCodePluginNotFound(t *testing.T) {
	install := &xclerrors.PluginInstallError{
		Repository: "acme/xcl-plugin-widget", Version: "v1.0.0", Platform: "linux/amd64",
		Err: fmt.Errorf("%w: no release", xclerrors.ErrPluginNotFound),
	}

	out := toProto(&xclerrors.PluginLoadError{Plugin: "widget", Registry: "github", Err: install})

	require.Equal(t, CodePluginNotFound, out.GetCode())
	require.Equal(t, "acme/xcl-plugin-widget", out.GetDetails()["repository"])
	require.Equal(t, "v1.0.0", out.GetDetails()["version"])
	require.Equal(t, "linux/amd64", out.GetDetails()["platform"])
}

func TestErrorCodePluginVerification(t *testing.T) {
	install := &xclerrors.PluginInstallError{
		Repository: "acme/xcl-plugin-widget", Version: "v1.0.0", Platform: "linux/amd64",
		Err: fmt.Errorf("%w: checksum mismatch", xclerrors.ErrPluginVerification),
	}

	out := toProto(&xclerrors.PluginLoadError{Plugin: "widget", Registry: "github", Err: install})

	require.Equal(t, CodePluginVerification, out.GetCode())
	require.Equal(t, "acme/xcl-plugin-widget", out.GetDetails()["repository"])
	require.Equal(t, "v1.0.0", out.GetDetails()["version"])
	require.Equal(t, "linux/amd64", out.GetDetails()["platform"])
}

func TestErrorCodeEmptyConfiguration(t *testing.T) {
	out := toProto(fmt.Errorf("apply: %w", xcl.ErrEmptyConfiguration))

	require.Equal(t, CodeEmptyConfiguration, out.GetCode())
	require.Nil(t, out.GetDetails())
}

func TestErrorCodeUnregisteredType(t *testing.T) {
	out := toProto(&xclerrors.UnregisteredTypeError{Type: "widget"})

	require.Equal(t, CodeUnregisteredType, out.GetCode())
	require.Equal(t, "widget", out.GetDetails()["type"])
}

func TestErrorCodeInvalidSavedData(t *testing.T) {
	out := toProto(&xclerrors.InvalidSavedDataError{ID: "app.web", Err: errors.New("bad")})

	require.Equal(t, CodeInvalidSavedData, out.GetCode())
	require.Equal(t, "app.web", out.GetDetails()["id"])
}

func TestErrorCodeNotEncodable(t *testing.T) {
	out := toProto(&xclerrors.NotEncodableError{What: "variable.region", Reason: "it is a builtin"})

	require.Equal(t, CodeNotEncodable, out.GetCode())
	require.Equal(t, "variable.region", out.GetDetails()["what"])
	require.Equal(t, "it is a builtin", out.GetDetails()["reason"])
}

func TestErrorCodeUnrecoverable(t *testing.T) {
	out := toProto(&xclerrors.UnrecoverableError{ID: "app.web", MaskedBy: "aes256gcm", Reason: "wrong key"})

	require.Equal(t, CodeUnrecoverable, out.GetCode())
	require.Equal(t, "app.web", out.GetDetails()["id"])
	require.Equal(t, "aes256gcm", out.GetDetails()["masked_by"])
	require.Equal(t, "wrong key", out.GetDetails()["reason"])
}

func TestErrorCodeMaskNotReversible(t *testing.T) {
	out := toProto(&xclerrors.MaskNotReversibleError{Masker: "redact"})

	require.Equal(t, CodeMaskNotReversible, out.GetCode())
	require.Equal(t, "redact", out.GetDetails()["masker"])
}

func TestErrorCodeConfig(t *testing.T) {
	out := toProto(&xclerrors.ConfigError{Errors: []error{
		&xclerrors.ParserError{Filename: "f", Line: 3, Column: 4, Message: "m"},
	}})

	require.Equal(t, CodeConfig, out.GetCode())
	require.Nil(t, out.GetDetails())
	require.Len(t, out.GetCauses(), 1)

	cause := out.GetCauses()[0]
	require.Equal(t, CodeConfig, cause.GetCode())
	require.Equal(t, "f", cause.GetDetails()["file"])
	require.Equal(t, "3", cause.GetDetails()["line"])
	require.Equal(t, "4", cause.GetDetails()["column"])
	require.Equal(t, "m", cause.GetDetails()["summary"])
}

func TestErrorCodeProvider(t *testing.T) {
	out := toProto(fmt.Errorf("create failed for resource.x: boom"))

	require.Equal(t, CodeProvider, out.GetCode())
	require.Equal(t, "create", out.GetDetails()["step"])
	require.Equal(t, "resource.x", out.GetDetails()["address"])
}

func TestErrorCodeCancelled(t *testing.T) {
	out := toProto(fmt.Errorf("apply: %w", context.Canceled))

	require.Equal(t, CodeCancelled, out.GetCode())
}

func TestErrorCodeCancelledOnDeadline(t *testing.T) {
	out := toProto(context.DeadlineExceeded)

	require.Equal(t, CodeCancelled, out.GetCode())
}

func TestErrorCodeInvalidArgument(t *testing.T) {
	out := toProto(invalidArgument("m", "k", "v"))

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "m", out.GetMessage())
	require.Equal(t, map[string]string{"k": "v"}, out.GetDetails())
}

func TestErrorCodeOperationInProgress(t *testing.T) {
	out := toProto(operationInProgress("Find"))

	require.Equal(t, CodeOperationInProgress, out.GetCode())
	require.Contains(t, out.GetMessage(), "Find")
	require.Nil(t, out.GetDetails())
}

func TestErrorCodeInternal(t *testing.T) {
	out := toProto(errors.New("x"))

	require.Equal(t, CodeInternal, out.GetCode())
	require.Equal(t, "x", out.GetMessage())
	require.Nil(t, out.GetDetails())
}

func TestConfigErrorListsCausesWithFileAndLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	conf := c.s.lookup(handle)
	require.NotNil(t, conf)

	file := writeConfig(t, "app \"web\" {\n}\n")

	err := conf.cfg.Apply(file)
	require.Error(t, err)

	out := toProto(err)
	require.Equal(t, CodeConfig, out.GetCode())
	require.Len(t, out.GetCauses(), 1)

	cause := out.GetCauses()[0]
	require.Equal(t, CodeConfig, cause.GetCode())
	require.Equal(t, file, cause.GetDetails()["file"])
	require.Equal(t, "1", cause.GetDetails()["line"])
	require.NotEmpty(t, cause.GetDetails()["column"])
	require.Contains(t, cause.GetDetails()["summary"], "image")
}
