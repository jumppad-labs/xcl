package mask

import "encoding/json"

// OmitName is the name the omit masker writes into what it masks.
const OmitName = "omit"

// Omit returns a one-way masker that leaves no value. The envelope it writes
// still names it, so masked data is never mistaken for a field that was not
// set.
func Omit() Masker {
	return omit{}
}

type omit struct{}

func (omit) Name() string { return OmitName }

func (omit) Mask(json.RawMessage) (json.RawMessage, error) { return nil, nil }
