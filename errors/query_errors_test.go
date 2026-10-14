package errors

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// queryErrorFixture stands in for a registered resource type wherever a test
// needs a reflect.Type to put in a detail error.
type queryErrorFixture struct {
	Name string
}

// The seven tests below pin the distinction the vocabulary exists for: every
// way a lookup can fail must be tellable apart from every other, by identity
// alone. Each detail type gets its own test so a regression names the one
// condition that stopped being distinguishable.

func TestNotFoundErrorMatchesOnlyErrNotFound(t *testing.T) {
	err := &NotFoundError{Address: "resource.container.web"}

	require.ErrorIs(t, err, ErrNotFound)

	require.NotErrorIs(t, err, ErrUnknownType)
	require.NotErrorIs(t, err, ErrNotTypeable)
	require.NotErrorIs(t, err, ErrNotRegistered)
	require.NotErrorIs(t, err, ErrTypeMismatch)
	require.NotErrorIs(t, err, ErrNotAnEntity)
	require.NotErrorIs(t, err, ErrNotUnique)
}

func TestUnknownTypeErrorMatchesOnlyErrUnknownType(t *testing.T) {
	err := &UnknownTypeError{Segments: []string{"widget", "web"}, Name: "widget"}

	require.ErrorIs(t, err, ErrUnknownType)

	require.NotErrorIs(t, err, ErrNotFound)
	require.NotErrorIs(t, err, ErrNotTypeable)
	require.NotErrorIs(t, err, ErrNotRegistered)
	require.NotErrorIs(t, err, ErrTypeMismatch)
	require.NotErrorIs(t, err, ErrNotAnEntity)
	require.NotErrorIs(t, err, ErrNotUnique)
}

func TestNotTypeableErrorMatchesOnlyErrNotTypeable(t *testing.T) {
	err := &NotTypeableError{Segments: []string{"resource"}, Use: "Resources"}

	require.ErrorIs(t, err, ErrNotTypeable)

	require.NotErrorIs(t, err, ErrNotFound)
	require.NotErrorIs(t, err, ErrUnknownType)
	require.NotErrorIs(t, err, ErrNotRegistered)
	require.NotErrorIs(t, err, ErrTypeMismatch)
	require.NotErrorIs(t, err, ErrNotAnEntity)
	require.NotErrorIs(t, err, ErrNotUnique)
}

func TestNotRegisteredErrorMatchesOnlyErrNotRegistered(t *testing.T) {
	err := &NotRegisteredError{Type: reflect.TypeOf(queryErrorFixture{}), Use: "Get"}

	require.ErrorIs(t, err, ErrNotRegistered)

	require.NotErrorIs(t, err, ErrNotFound)
	require.NotErrorIs(t, err, ErrUnknownType)
	require.NotErrorIs(t, err, ErrNotTypeable)
	require.NotErrorIs(t, err, ErrTypeMismatch)
	require.NotErrorIs(t, err, ErrNotAnEntity)
	require.NotErrorIs(t, err, ErrNotUnique)
}

func TestTypeMismatchErrorMatchesOnlyErrTypeMismatch(t *testing.T) {
	err := &TypeMismatchError{
		Address: "resource.container.web",
		Want:    reflect.TypeOf(queryErrorFixture{}),
		Got:     "resources.Network",
	}

	require.ErrorIs(t, err, ErrTypeMismatch)

	require.NotErrorIs(t, err, ErrNotFound)
	require.NotErrorIs(t, err, ErrUnknownType)
	require.NotErrorIs(t, err, ErrNotTypeable)
	require.NotErrorIs(t, err, ErrNotRegistered)
	require.NotErrorIs(t, err, ErrNotAnEntity)
	require.NotErrorIs(t, err, ErrNotUnique)
}

func TestNotAnEntityErrorMatchesOnlyErrNotAnEntity(t *testing.T) {
	err := &NotAnEntityError{
		Type:           reflect.TypeOf(queryErrorFixture{}),
		ReachedThrough: "resource.container.web",
	}

	require.ErrorIs(t, err, ErrNotAnEntity)

	require.NotErrorIs(t, err, ErrNotFound)
	require.NotErrorIs(t, err, ErrUnknownType)
	require.NotErrorIs(t, err, ErrNotTypeable)
	require.NotErrorIs(t, err, ErrNotRegistered)
	require.NotErrorIs(t, err, ErrTypeMismatch)
	require.NotErrorIs(t, err, ErrNotUnique)
}

func TestNotUniqueErrorMatchesOnlyErrNotUnique(t *testing.T) {
	err := &NotUniqueError{Segments: []string{"resource", "container"}, Count: 3}

	require.ErrorIs(t, err, ErrNotUnique)

	require.NotErrorIs(t, err, ErrNotFound)
	require.NotErrorIs(t, err, ErrUnknownType)
	require.NotErrorIs(t, err, ErrNotTypeable)
	require.NotErrorIs(t, err, ErrNotRegistered)
	require.NotErrorIs(t, err, ErrTypeMismatch)
	require.NotErrorIs(t, err, ErrNotAnEntity)
}

// The seven tests below recover each detail through a wrap, because that is how
// a caller meets it: the error has been annotated by the time it is returned,
// and errors.As has to reach the value inside to hand back its fields.

func TestNotFoundErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("looking up entity: %w", &NotFoundError{Address: "resource.container.web"})

	var detail *NotFoundError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "resource.container.web", detail.Address)
}

func TestUnknownTypeErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("running query: %w", &UnknownTypeError{
		Segments: []string{"widget", "web"},
		Name:     "widget",
	})

	var detail *UnknownTypeError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, []string{"widget", "web"}, detail.Segments)
	require.Equal(t, "widget", detail.Name)
}

func TestNotTypeableErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("running query: %w", &NotTypeableError{
		Segments: []string{"resource"},
		Use:      "Resources",
	})

	var detail *NotTypeableError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, []string{"resource"}, detail.Segments)
	require.Equal(t, "Resources", detail.Use)
}

func TestNotRegisteredErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("deriving address: %w", &NotRegisteredError{
		Type: reflect.TypeOf(queryErrorFixture{}),
		Use:  "Get",
	})

	var detail *NotRegisteredError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, reflect.TypeOf(queryErrorFixture{}), detail.Type)
	require.Equal(t, "Get", detail.Use)
}

func TestTypeMismatchErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("converting entity: %w", &TypeMismatchError{
		Address: "resource.container.web",
		Want:    reflect.TypeOf(queryErrorFixture{}),
		Got:     "resources.Network",
	})

	var detail *TypeMismatchError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "resource.container.web", detail.Address)
	require.Equal(t, reflect.TypeOf(queryErrorFixture{}), detail.Want)
	require.Equal(t, "resources.Network", detail.Got)
}

func TestNotAnEntityErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("deriving address: %w", &NotAnEntityError{
		Type:           reflect.TypeOf(queryErrorFixture{}),
		ReachedThrough: "resource.container.web",
	})

	var detail *NotAnEntityError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, reflect.TypeOf(queryErrorFixture{}), detail.Type)
	require.Equal(t, "resource.container.web", detail.ReachedThrough)
}

// The count is the point of this one: a caller that asked for exactly one
// entity has to be able to learn how many actually matched, without parsing the
// message.
func TestNotUniqueErrorDetailReportsHowManyMatchedThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("running query: %w", &NotUniqueError{
		Segments: []string{"resource", "container"},
		Count:    3,
	})

	var detail *NotUniqueError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, []string{"resource", "container"}, detail.Segments)
	require.Equal(t, 3, detail.Count)
}

// Decode target. Decode reports a target it cannot fill with its own sentinel,
// kept apart from the seven lookup failures above: a caller handing Decode the
// wrong thing has made a different mistake from one asking a question with no
// answer, and must be able to tell which by identity alone.

func TestInvalidDecodeTargetErrorUnwrapsToItsSentinel(t *testing.T) {
	err := &InvalidDecodeTargetError{Type: reflect.TypeFor[queryErrorFixture]()}

	require.ErrorIs(t, err, ErrInvalidDecodeTarget)

	require.NotErrorIs(t, err, ErrNotFound)
	require.NotErrorIs(t, err, ErrUnknownType)
	require.NotErrorIs(t, err, ErrNotTypeable)
	require.NotErrorIs(t, err, ErrNotRegistered)
	require.NotErrorIs(t, err, ErrTypeMismatch)
	require.NotErrorIs(t, err, ErrNotAnEntity)
	require.NotErrorIs(t, err, ErrNotUnique)
}

func TestInvalidDecodeTargetErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("decoding configuration: %w", &InvalidDecodeTargetError{
		Type: reflect.TypeFor[queryErrorFixture](),
	})

	var detail *InvalidDecodeTargetError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, reflect.TypeFor[queryErrorFixture](), detail.Type)
	require.False(t, detail.Nil)
}

// The three tests below pin the message, one per shape of mistake. Each says
// what Decode expected and then what it was actually given, so a caller reading
// the output can fix the call without looking anything up.

func TestInvalidDecodeTargetErrorNamesTheTypePassed(t *testing.T) {
	err := &InvalidDecodeTargetError{Type: reflect.TypeFor[queryErrorFixture]()}

	require.Equal(
		t,
		"decode target must be a non-nil pointer to a struct, got errors.queryErrorFixture",
		err.Error(),
	)
}

func TestInvalidDecodeTargetErrorSaysNothingWasPassed(t *testing.T) {
	err := &InvalidDecodeTargetError{}

	require.Equal(t, "decode target must be a non-nil pointer to a struct, got nil", err.Error())
}

func TestInvalidDecodeTargetErrorNamesATypedNilPointer(t *testing.T) {
	err := &InvalidDecodeTargetError{Type: reflect.TypeFor[*queryErrorFixture](), Nil: true}

	require.Equal(
		t,
		"decode target must be a non-nil pointer to a struct, got nil *errors.queryErrorFixture",
		err.Error(),
	)
}

// Traversal. ConfigError is what Apply and Destroy return, and a consumer meets
// a query failure through it, usually with a diagnostic in between.

// problem is a diagnostic about resource.container.web, found through cause
func problem(cause error) *Diagnostic {
	d := NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Unable to resolve reference"}, nil)
	d.Entity = "resource.container.web"
	d.Cause = cause

	return d
}

func TestConfigErrorIsFindsASentinelAmongItsErrors(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(problem(nil))
	ce.AppendError(&NotUniqueError{Segments: []string{"resource", "container"}, Count: 2})

	require.ErrorIs(t, ce, ErrNotUnique)
}

func TestConfigErrorAsRecoversADetailAmongItsErrors(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(problem(nil))
	ce.AppendError(&NotUniqueError{Segments: []string{"resource", "container"}, Count: 2})

	var detail *NotUniqueError
	require.True(t, errors.As(ce, &detail))
	require.Equal(t, 2, detail.Count)
}

func TestConfigErrorIsReachesThroughADiagnosticToASentinel(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(problem(&NotFoundError{Address: "resource.container.web"}))

	require.ErrorIs(t, ce, ErrNotFound)
}

func TestConfigErrorAsReachesThroughADiagnosticToADetail(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(problem(&NotFoundError{Address: "resource.container.web"}))

	var detail *NotFoundError
	require.True(t, errors.As(ce, &detail))
	require.Equal(t, "resource.container.web", detail.Address)
}

func TestConfigErrorWithNoErrorsDoesNotMatchASentinel(t *testing.T) {
	ce := NewConfigError()

	require.NotPanics(t, func() {
		require.NotErrorIs(t, ce, ErrNotFound)
	})
}

func TestADiagnosticWithNoCauseDoesNotMatchASentinel(t *testing.T) {
	d := problem(nil)

	require.NotErrorIs(t, d, ErrNotFound)
	require.NotErrorIs(t, d, ErrUnknownType)
	require.NotErrorIs(t, d, ErrNotTypeable)
	require.NotErrorIs(t, d, ErrNotRegistered)
	require.NotErrorIs(t, d, ErrTypeMismatch)
	require.NotErrorIs(t, d, ErrNotAnEntity)
	require.NotErrorIs(t, d, ErrNotUnique)
}

// The cause is carried alongside the message, never folded into it, so the
// text a person reads says what the diagnostic says and nothing more.
func TestADiagnosticMessageIsUnchangedByCarryingACause(t *testing.T) {
	plain := problem(nil)
	wrapping := problem(&NotFoundError{Address: "resource.container.web"})

	require.Equal(t, plain.Error(), wrapping.Error())
}

func TestTypeMismatchErrorMessageWithoutFieldIsUnchanged(t *testing.T) {
	err := &TypeMismatchError{
		Address: "resource.container.web",
		Want:    reflect.TypeOf(queryErrorFixture{}),
		Got:     "resources.Network",
	}

	require.Equal(t, `entity "resource.container.web" is resources.Network, not errors.queryErrorFixture`, err.Error())
}

func TestTypeMismatchErrorMessageWithFieldNamesTheSensitiveField(t *testing.T) {
	err := &TypeMismatchError{
		Address: "resource.credential.db",
		Want:    reflect.TypeOf(queryErrorFixture{}),
		Got:     "anonymous",
		Field:   "login.password",
	}

	require.Equal(t, `entity "resource.credential.db" field "login.password" is sensitive, errors.queryErrorFixture declares it as a plain value`, err.Error())
}

func TestTypeMismatchErrorWithFieldStillMatchesErrTypeMismatch(t *testing.T) {
	err := &TypeMismatchError{
		Address: "resource.credential.db",
		Want:    reflect.TypeOf(queryErrorFixture{}),
		Got:     "anonymous",
		Field:   "password",
	}

	require.ErrorIs(t, err, ErrTypeMismatch)
}
