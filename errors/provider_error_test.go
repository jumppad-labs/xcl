package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderErrorMessageNamesTheStepAndTheAddress(t *testing.T) {
	err := &ProviderError{Step: "create", Address: "resource.container.web", Err: errors.New("no such image")}

	require.Equal(t, "create failed for resource.container.web: no such image", err.Error())
}

func TestProviderErrorMatchesWhatTheProviderReturned(t *testing.T) {
	cause := errors.New("no such image")
	err := fmt.Errorf("apply: %w", &ProviderError{Step: "create", Address: "resource.container.web", Err: cause})

	require.ErrorIs(t, err, cause)

	var provider *ProviderError
	require.ErrorAs(t, err, &provider)
	require.Equal(t, "create", provider.Step)
}
