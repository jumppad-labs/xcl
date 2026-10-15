package schema

import (
	"fmt"

	"github.com/jumppad-labs/xcl/internal/plaintype"
)

// CreateInstanceFromSchema builds the type a plugin's type is stored as from
// data, the schema v1 document the plugin describes the type with, and
// returns a pointer to a new value of it. The schema gives a plain type,
// which is built into the stored type as a plain type an application
// declares is.
func CreateInstanceFromSchema(data []byte) (any, error) {
	if !IsV1(data) {
		return nil, fmt.Errorf("the schema is not a schema v1 document; the plugin was built against a version of xcl that describes its types otherwise, rebuild it against this one")
	}

	plain, s, err := BuildTypeV1(data)
	if err != nil {
		return nil, err
	}

	labels := 1
	if s.Subtype != "" {
		labels = 2
	}

	built, err := plaintype.Build(plain, labels)
	if err != nil {
		return nil, err
	}

	return built.New(), nil
}
