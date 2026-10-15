package person

import "github.com/jumppad-labs/xcl/types"

// Person is an example resource, written resource "person" "<name>" {}. Its
// label fields hold the block's two labels, the subtype and the name; a type
// that reads its metadata declares a Meta field of type types.Meta instead
type Person struct {
	Subtype string `xcl:"subtype,label" json:"subtype"`
	Name    string `xcl:"name,label" json:"name"`

	// Basic person fields
	FirstName   string `xcl:"first_name" json:"first_name"`
	LastName    string `xcl:"last_name" json:"last_name"`
	Age         int    `xcl:"age,optional" json:"age,omitempty"`
	Email       string `xcl:"email,optional" json:"email,omitempty"`
	Address     string `xcl:"address,optional" json:"address,omitempty"`
	Description string `xcl:"description,optional" json:"description,omitempty"`

	// Token is sensitive: it shows only as "(sensitive)" anywhere it is
	// printed, logged or reported, and the provider reads it with Reveal
	Token types.Sensitive[string] `xcl:"token,optional" json:"token"`

	// PersonID is owned by the provider: it is set when the person is created,
	// can not be set in configuration, and is carried over by xcl on every apply
	PersonID string `xcl:"person_id,optional,computed" json:"person_id,omitempty"`
}
