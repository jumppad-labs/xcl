package diff_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
)

func TestPathAliasStringPrintsAttribute(t *testing.T) {
	path := diff.Path{}.Attribute("x")

	require.Equal(t, "x", path.String())
}

func TestPathAliasMarshalJSONEncodesIndexedPathAsString(t *testing.T) {
	encoded, err := json.Marshal(diff.Path{}.Attribute("ports").Index(0).Attribute("host"))
	require.NoError(t, err)

	require.Equal(t, `"ports[0].host"`, string(encoded))
}
