package errors

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
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

// Traversal. ConfigError is what Apply and Destroy return, and a consumer meets
// a query failure through it, usually with a ParserError in between.

func TestConfigErrorIsFindsASentinelAmongItsErrors(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(&ParserError{Message: "unrelated"})
	ce.AppendError(&NotUniqueError{Segments: []string{"resource", "container"}, Count: 2})

	require.ErrorIs(t, ce, ErrNotUnique)
}

func TestConfigErrorAsRecoversADetailAmongItsErrors(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(&ParserError{Message: "unrelated"})
	ce.AppendError(&NotUniqueError{Segments: []string{"resource", "container"}, Count: 2})

	var detail *NotUniqueError
	require.True(t, errors.As(ce, &detail))
	require.Equal(t, 2, detail.Count)
}

func TestConfigErrorIsReachesThroughAParserErrorToASentinel(t *testing.T) {
	cause := &NotFoundError{Address: "resource.container.web"}

	ce := NewConfigError()
	ce.AppendError(NewParserErrorWrapping("main.xcl", 10, 2, cause, "unable to resolve reference"))

	require.ErrorIs(t, ce, ErrNotFound)
}

func TestConfigErrorAsReachesThroughAParserErrorToADetail(t *testing.T) {
	cause := &NotFoundError{Address: "resource.container.web"}

	ce := NewConfigError()
	ce.AppendError(NewParserErrorWrapping("main.xcl", 10, 2, cause, "unable to resolve reference"))

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

func TestParserErrorWithNoCauseUnwrapsToNil(t *testing.T) {
	pe := NewParserError("main.xcl", 10, 2, "something went wrong")

	require.Nil(t, pe.Unwrap())
}

func TestParserErrorWithNoCauseDoesNotMatchASentinel(t *testing.T) {
	pe := NewParserError("main.xcl", 10, 2, "something went wrong")

	require.NotErrorIs(t, pe, ErrNotFound)
	require.NotErrorIs(t, pe, ErrUnknownType)
	require.NotErrorIs(t, pe, ErrNotTypeable)
	require.NotErrorIs(t, pe, ErrNotRegistered)
	require.NotErrorIs(t, pe, ErrTypeMismatch)
	require.NotErrorIs(t, pe, ErrNotAnEntity)
	require.NotErrorIs(t, pe, ErrNotUnique)
}

func TestParserErrorWrappingCarriesTheCause(t *testing.T) {
	cause := &NotFoundError{Address: "resource.container.web"}
	pe := NewParserErrorWrapping("main.xcl", 10, 2, cause, "unable to resolve reference")

	require.Same(t, cause, pe.Unwrap())
	require.ErrorIs(t, pe, ErrNotFound)
}

// The cause is carried alongside the message, never folded into it. The
// messages were kept byte identical on purpose, so this pins them: anyone
// tempted to "tidy" the cause into the text breaks this test rather than every
// consumer reading the output.
func TestParserErrorMessageIsUnchangedByCarryingACause(t *testing.T) {
	plain := NewParserError("main.xcl", 10, 2, "unable to resolve reference")
	wrapping := NewParserErrorWrapping(
		"main.xcl", 10, 2,
		&NotFoundError{Address: "resource.container.web"},
		"unable to resolve reference",
	)

	require.Equal(t, plain.Error(), wrapping.Error())
}

func TestConfigErrorStringIsUnchangedByAParserErrorCarryingACause(t *testing.T) {
	plain := NewConfigError()
	plain.AppendError(NewParserError("main.xcl", 10, 2, "unable to resolve reference"))

	wrapping := NewConfigError()
	wrapping.AppendError(NewParserErrorWrapping(
		"main.xcl", 10, 2,
		&NotFoundError{Address: "resource.container.web"},
		"unable to resolve reference",
	))

	require.Equal(t, plain.Error(), wrapping.Error())
}
