package errors

import "strings"

// ConfigError defines an error that was encountered while parsing the config
type ConfigError struct {
	// Errors is a list of errors that were encountered while processing the
	// config which includes calling the process function on the resource or any
	// default callbacks
	Errors []error
}

func NewConfigError() *ConfigError {
	return &ConfigError{
		Errors: []error{},
	}
}

// AppendError adds a new parse error to the list of errors
func (p *ConfigError) AppendError(err error) {
	p.Errors = append(p.Errors, err)
}

// Unwrap returns the errors this collected so that errors.Is and errors.As
// find anything inside it. Without it a caller holding the result of Apply has
// to walk Errors by hand to discover what went wrong, which is the opposite of
// matching an error by identity.
func (p *ConfigError) Unwrap() []error {
	return p.Errors
}

// Error pretty prints the error message as a string
func (p *ConfigError) Error() string {
	err := strings.Builder{}

	for _, e := range p.Errors {
		err.WriteString(e.Error() + "\n")
	}

	return strings.TrimSuffix(err.String(), "\n")
}
