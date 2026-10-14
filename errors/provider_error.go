package errors

import "fmt"

// ProviderError reports a provider call that failed during a lifecycle step:
// the step, such as create or destroy, the address of the entity it was for,
// and the error the provider returned. An operation reports it as the Cause of
// the diagnostic placed at the entity's block.
type ProviderError struct {
	Step    string
	Address string
	Err     error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s failed for %s: %s", e.Step, e.Address, e.Err)
}

// Unwrap returns the error the provider returned, so it answers errors.Is and
// errors.As
func (e *ProviderError) Unwrap() error { return e.Err }
