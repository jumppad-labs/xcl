package structs

import (
	"github.com/jumppad-labs/xcl/types"
)

// TypeCredential is the resource string for a Credential resource
const TypeCredential = "credential"

// Credential is a plugin type with sensitive fields, so a test can follow a
// sensitive value through a provider
type Credential struct {
	types.ResourceBase `xcl:",remain"`

	Username string                  `xcl:"username,optional" json:"username,omitempty"`
	Password types.Sensitive[string] `xcl:"password" json:"password"`
	Pin      types.Sensitive[int]    `xcl:"pin,optional" json:"pin"`
}
