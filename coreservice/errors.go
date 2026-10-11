package coreservice

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/jumppad-labs/xcl"
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// The stable error codes a binding switches on. A code is never renamed or
// reused, only added.
const (
	CodeNotFound            = "not_found"
	CodeNotUnique           = "not_unique"
	CodeUnknownType         = "unknown_type"
	CodeNotTypeable         = "not_typeable"
	CodeNotRegistered       = "not_registered"
	CodeTypeMismatch        = "type_mismatch"
	CodeNotAnEntity         = "not_an_entity"
	CodeTypeNameClash       = "type_name_clash"
	CodeTypeForm            = "type_form"
	CodeInvalidType         = "invalid_type"
	CodeFunctionNameClash   = "function_name_clash"
	CodeInvalidFunction     = "invalid_function"
	CodePluginLoad          = "plugin_load"
	CodePluginNotFound      = "plugin_not_found"
	CodePluginVerification  = "plugin_verification"
	CodeEmptyConfiguration  = "empty_configuration"
	CodeUnregisteredType    = "unregistered_type"
	CodeInvalidSavedData    = "invalid_saved_data"
	CodeNotEncodable        = "not_encodable"
	CodeUnrecoverable       = "unrecoverable"
	CodeMaskNotReversible   = "mask_not_reversible"
	CodeConfig              = "config"
	CodeProvider            = "provider"
	CodeCancelled           = "cancelled"
	CodeOperationInProgress = "operation_in_progress"
	CodeInvalidArgument     = "invalid_argument"
	CodeInternal            = "internal"
)

// codedError is an error coreservice raises itself, with the code it crosses
// as and its details.
type codedError struct {
	code    string
	message string
	details map[string]string
}

func (e *codedError) Error() string { return e.message }

// invalidArgument is a request that parsed but makes no sense.
func invalidArgument(message string, details ...string) error {
	return &codedError{code: CodeInvalidArgument, message: message, details: pairs(details)}
}

// operationInProgress is a call refused because an operation is running on
// the handle.
func operationInProgress(what string) error {
	return &codedError{code: CodeOperationInProgress, message: what + " cannot run while an operation is in progress"}
}

// pairs turns alternating keys and values into a map.
func pairs(kv []string) map[string]string {
	if len(kv) == 0 {
		return nil
	}

	details := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		details[kv[i]] = kv[i+1]
	}

	return details
}

// providerFailure matches the error the parser reports for a failed provider
// call, "<step> failed for <address>: <the provider's error>", wherever it
// sits in a message: on its own, or inside the positioned configuration
// error an apply collects it into.
var providerFailure = regexp.MustCompile(`(?m)\b(\w+) failed for (\S+): `)

// toProto maps an error to the Error message a binding receives: its stable
// code, its message, the fields of its detail type, and the errors it
// collects as causes. Anything not matched is CodeInternal.
func toProto(err error) *corepb.Error {
	if err == nil {
		return nil
	}

	out := &corepb.Error{Message: err.Error(), Details: map[string]string{}}

	var coded *codedError
	var notFound *xcl.NotFoundError
	var notUnique *xcl.NotUniqueError
	var unknownType *xcl.UnknownTypeError
	var notTypeable *xcl.NotTypeableError
	var notRegistered *xcl.NotRegisteredError
	var typeMismatch *xcl.TypeMismatchError
	var notAnEntity *xcl.NotAnEntityError
	var typeNameClash *xcl.TypeNameClashError
	var typeForm *xcl.TypeFormError
	var invalidType *xcl.InvalidTypeError
	var functionNameClash *xcl.FunctionNameClashError
	var invalidFunction *xclerrors.InvalidFunctionError
	var pluginLoad *xcl.PluginLoadError
	var pluginInstall *xcl.PluginInstallError
	var unregisteredType *xclerrors.UnregisteredTypeError
	var invalidSavedData *xclerrors.InvalidSavedDataError
	var notEncodable *xclerrors.NotEncodableError
	var unrecoverable *xclerrors.UnrecoverableError
	var maskNotReversible *xclerrors.MaskNotReversibleError
	var configError *xclerrors.ConfigError
	var parserError *xclerrors.ParserError

	switch {
	case errors.As(err, &coded):
		out.Code = coded.code
		for k, v := range coded.details {
			out.Details[k] = v
		}

	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		out.Code = CodeCancelled

	case errors.Is(err, xcl.ErrEmptyConfiguration):
		out.Code = CodeEmptyConfiguration

	case errors.As(err, &configError):
		out.Code = CodeConfig
		for _, cause := range configError.Errors {
			out.Causes = append(out.Causes, toProto(cause))
		}

	case errors.As(err, &parserError):
		// a positioned problem in configuration, which a config error
		// collects as a cause; a provider that failed during a lifecycle
		// step is reported this way too, positioned at its block
		out.Code = CodeConfig
		if m := providerFailure.FindStringSubmatch(parserError.Message); m != nil {
			out.Code = CodeProvider
			out.Details["step"] = m[1]
			out.Details["address"] = m[2]
		}
		out.Details["file"] = parserError.Filename
		out.Details["line"] = strconv.Itoa(parserError.Line)
		out.Details["column"] = strconv.Itoa(parserError.Column)
		out.Details["summary"] = parserError.Message

	case errors.As(err, &notFound):
		out.Code = CodeNotFound
		out.Details["address"] = notFound.Address

	case errors.As(err, &notUnique):
		out.Code = CodeNotUnique
		out.Details["count"] = strconv.Itoa(notUnique.Count)
		out.Details["segments"] = strings.Join(notUnique.Segments, ".")

	case errors.As(err, &unknownType):
		out.Code = CodeUnknownType
		out.Details["name"] = unknownType.Name
		out.Details["segments"] = strings.Join(unknownType.Segments, ".")

	case errors.As(err, &notTypeable):
		out.Code = CodeNotTypeable
		out.Details["segments"] = strings.Join(notTypeable.Segments, ".")
		out.Details["use"] = notTypeable.Use

	case errors.As(err, &notRegistered):
		out.Code = CodeNotRegistered
		if notRegistered.Type != nil {
			out.Details["type"] = notRegistered.Type.String()
		}
		out.Details["use"] = notRegistered.Use

	case errors.As(err, &typeMismatch):
		out.Code = CodeTypeMismatch
		out.Details["address"] = typeMismatch.Address
		if typeMismatch.Want != nil {
			out.Details["want"] = typeMismatch.Want.String()
		}
		out.Details["got"] = typeMismatch.Got
		out.Details["field"] = typeMismatch.Field

	case errors.As(err, &notAnEntity):
		out.Code = CodeNotAnEntity
		if notAnEntity.Type != nil {
			out.Details["type"] = notAnEntity.Type.String()
		}
		out.Details["reached_through"] = notAnEntity.ReachedThrough

	case errors.As(err, &typeNameClash):
		out.Code = CodeTypeNameClash
		out.Details["name"] = typeNameClash.Name
		out.Details["provider"] = typeNameClash.Provider
		out.Details["registry"] = typeNameClash.Registry
		out.Details["existing"] = typeNameClash.Existing
		out.Details["existing_registry"] = typeNameClash.ExistingRegistry

	case errors.As(err, &typeForm):
		out.Code = CodeTypeForm
		out.Details["type"] = typeForm.Type
		out.Details["takes_subtype"] = strconv.FormatBool(typeForm.TakesSubtype)

	case errors.As(err, &invalidType):
		out.Code = CodeInvalidType
		out.Details["type"] = invalidType.Type
		out.Details["provider"] = invalidType.Provider
		out.Details["registry"] = invalidType.Registry

	case errors.As(err, &functionNameClash):
		out.Code = CodeFunctionNameClash
		out.Details["name"] = functionNameClash.Name
		out.Details["provider"] = functionNameClash.Provider
		out.Details["registry"] = functionNameClash.Registry
		out.Details["existing"] = functionNameClash.Existing
		out.Details["existing_registry"] = functionNameClash.ExistingRegistry

	case errors.As(err, &invalidFunction):
		out.Code = CodeInvalidFunction
		out.Details["name"] = invalidFunction.Name
		out.Details["registry"] = invalidFunction.Registry

	case errors.As(err, &pluginInstall):
		out.Code = CodePluginNotFound
		if errors.Is(err, xcl.ErrPluginVerification) {
			out.Code = CodePluginVerification
		}
		out.Details["repository"] = pluginInstall.Repository
		out.Details["version"] = pluginInstall.Version
		out.Details["platform"] = pluginInstall.Platform

	case errors.Is(err, xcl.ErrPluginVerification):
		out.Code = CodePluginVerification

	case errors.Is(err, xcl.ErrPluginNotFound):
		out.Code = CodePluginNotFound

	case errors.As(err, &pluginLoad):
		out.Code = CodePluginLoad
		out.Details["plugin"] = pluginLoad.Plugin
		out.Details["registry"] = pluginLoad.Registry

	case errors.Is(err, xcl.ErrPluginLoad):
		out.Code = CodePluginLoad

	case errors.As(err, &unregisteredType):
		out.Code = CodeUnregisteredType
		out.Details["type"] = unregisteredType.Type

	case errors.As(err, &invalidSavedData):
		out.Code = CodeInvalidSavedData
		out.Details["id"] = invalidSavedData.ID

	case errors.As(err, &notEncodable):
		out.Code = CodeNotEncodable
		out.Details["what"] = notEncodable.What
		out.Details["reason"] = notEncodable.Reason

	case errors.As(err, &unrecoverable):
		out.Code = CodeUnrecoverable
		out.Details["id"] = unrecoverable.ID
		out.Details["masked_by"] = unrecoverable.MaskedBy
		out.Details["reason"] = unrecoverable.Reason

	case errors.As(err, &maskNotReversible):
		out.Code = CodeMaskNotReversible
		out.Details["masker"] = maskNotReversible.Masker

	case errors.Is(err, xcl.ErrNotFound):
		out.Code = CodeNotFound
	case errors.Is(err, xcl.ErrNotUnique):
		out.Code = CodeNotUnique
	case errors.Is(err, xcl.ErrUnknownType):
		out.Code = CodeUnknownType
	case errors.Is(err, xcl.ErrNotTypeable):
		out.Code = CodeNotTypeable
	case errors.Is(err, xcl.ErrNotRegistered):
		out.Code = CodeNotRegistered
	case errors.Is(err, xcl.ErrTypeMismatch):
		out.Code = CodeTypeMismatch
	case errors.Is(err, xcl.ErrNotAnEntity):
		out.Code = CodeNotAnEntity
	case errors.Is(err, xcl.ErrUnregisteredType):
		out.Code = CodeUnregisteredType
	case errors.Is(err, xcl.ErrInvalidSavedData):
		out.Code = CodeInvalidSavedData
	case errors.Is(err, xcl.ErrNotEncodable):
		out.Code = CodeNotEncodable
	case errors.Is(err, xcl.ErrUnrecoverable):
		out.Code = CodeUnrecoverable
	case errors.Is(err, xcl.ErrMaskNotReversible):
		out.Code = CodeMaskNotReversible

	default:
		if m := providerFailure.FindStringSubmatch(err.Error()); m != nil {
			// a provider failed during a lifecycle step
			out.Code = CodeProvider
			out.Details["step"] = m[1]
			out.Details["address"] = m[2]
		} else {
			out.Code = CodeInternal
		}
	}

	if len(out.Details) == 0 {
		out.Details = nil
	}

	return out
}
