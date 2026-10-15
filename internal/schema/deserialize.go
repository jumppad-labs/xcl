package schema

import (
	"fmt"
	"reflect"
)

// CreateInstanceFromSchema builds the Go struct a plugin's type is decoded
// into from data, the schema v1 document the plugin describes the type with,
// and returns a pointer to a new value of it.
func CreateInstanceFromSchema(data []byte) (any, error) {
	if !IsV1(data) {
		return nil, fmt.Errorf("the schema is not a schema v1 document; the plugin was built against a version of xcl that describes its types otherwise, rebuild it against this one")
	}

	built, _, err := BuildTypeV1(data)
	if err != nil {
		return nil, err
	}

	return reflect.New(built).Interface(), nil
}
