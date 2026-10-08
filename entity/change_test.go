package entity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoChangeStringIsNoChange(t *testing.T) {
	require.Equal(t, "no change", NoChange.String())
}

func TestUpdateStringIsUpdate(t *testing.T) {
	require.Equal(t, "update", Update.String())
}

func TestReplaceStringIsReplace(t *testing.T) {
	require.Equal(t, "replace", Replace.String())
}
