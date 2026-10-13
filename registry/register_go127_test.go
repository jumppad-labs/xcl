//go:build go1.27

package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterMethodAndFunctionDeclareTheSame(t *testing.T) {
	byMethod := NewLocal()
	byMethod.Register[plainThing]("thing")

	byFunction := NewLocal()
	Register[plainThing](byFunction, "thing")

	require.Equal(t, byFunction.Types(), byMethod.Types())
}
