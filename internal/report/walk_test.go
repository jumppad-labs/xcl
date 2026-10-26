package report_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/report"
)

// problemError stands in for a diagnostic: a problem with an optional cause
type problemError struct {
	name  string
	cause error
}

func (p *problemError) Error() string { return p.name }
func (p *problemError) Unwrap() error { return p.cause }

func isProblem(err error) bool {
	_, ok := err.(*problemError)
	return ok
}

// collection holds several errors, as a ConfigError does
type collection struct {
	errs []error
}

func (c *collection) Error() string   { return "collection" }
func (c *collection) Unwrap() []error { return c.errs }

type visit struct {
	message string
	problem bool
}

func walk(err error) []visit {
	var visits []visit
	report.Walk(err, isProblem, func(leaf error, problem bool) {
		visits = append(visits, visit{message: leaf.Error(), problem: problem})
	})

	return visits
}

func TestWalkVisitsAProblem(t *testing.T) {
	require.Equal(t, []visit{{"first", true}}, walk(&problemError{name: "first"}))
}

func TestWalkFindsAProblemThroughAWrap(t *testing.T) {
	err := fmt.Errorf("loading: %w", &problemError{name: "first"})

	require.Equal(t, []visit{{"first", true}}, walk(err))
}

func TestWalkFindsEveryProblemThroughJoin(t *testing.T) {
	err := errors.Join(&problemError{name: "first"}, &problemError{name: "second"})

	require.Equal(t, []visit{{"first", true}, {"second", true}}, walk(err))
}

func TestWalkFindsEveryProblemInACollection(t *testing.T) {
	err := fmt.Errorf("loading: %w", &collection{errs: []error{
		&problemError{name: "first"},
		errors.New("plain"),
		&problemError{name: "second"},
	}})

	require.Equal(t, []visit{{"first", true}, {"plain", false}, {"second", true}}, walk(err))
}

func TestWalkNeverDescendsIntoAProblemsCause(t *testing.T) {
	err := &problemError{name: "outer", cause: &problemError{name: "inner"}}

	require.Equal(t, []visit{{"outer", true}}, walk(err))
}

func TestWalkVisitsAnErrorWithoutAProblemWhole(t *testing.T) {
	err := fmt.Errorf("loading: %w", errors.New("boom"))

	require.Equal(t, []visit{{"loading: boom", false}}, walk(err))
}

func TestWalkKeepsTheMessageOfAWrapperAroundPlainErrors(t *testing.T) {
	err := errors.Join(&problemError{name: "first"}, fmt.Errorf("closing: %w", errors.New("boom")))

	require.Equal(t, []visit{{"first", true}, {"closing: boom", false}}, walk(err))
}

func TestWalkVisitsNothingForNil(t *testing.T) {
	require.Empty(t, walk(nil))
}

func TestContainsFindsAProblemThroughWrapsAndJoins(t *testing.T) {
	err := fmt.Errorf("loading: %w", errors.Join(errors.New("plain"), &problemError{name: "first"}))

	require.True(t, report.Contains(err, isProblem))
}

func TestContainsIsFalseWithoutAProblem(t *testing.T) {
	err := fmt.Errorf("loading: %w", errors.Join(errors.New("plain"), errors.New("other")))

	require.False(t, report.Contains(err, isProblem))
}

func TestContainsIsFalseForNil(t *testing.T) {
	require.False(t, report.Contains(nil, isProblem))
}
