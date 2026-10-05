package schema

import (
	"encoding/json"

	"github.com/jumppad-labs/xcl/internal/wire"
)

// UnmarshalUntyped unmarshals an untyped struct created by reflection
// into a concrete struct type.
func UnmarshalUntyped(from any, into any) error {
	// convert the untyped struct to json
	d, err := wire.Marshal(from)
	if err != nil {
		return err
	}

	// unmarshal the json into the concrete struct
	err = json.Unmarshal(d, into)
	if err != nil {
		return err
	}

	return nil
}
