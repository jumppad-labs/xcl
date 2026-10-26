package report

// Walk visits, in order, every leaf of the error tree under err: each error
// isProblem reports as a problem, and each other error that holds no problem.
// It descends through Unwrap() error and Unwrap() []error, as errors.Is and
// errors.As do, but only into errors that hold a problem somewhere beneath
// them, so an error wrapping only plain errors is visited whole, with its own
// message. It never descends into a problem, so a problem's cause is not
// visited.
func Walk(err error, isProblem func(error) bool, visit func(leaf error, problem bool)) {
	if err == nil {
		return
	}

	if isProblem(err) {
		visit(err, true)
		return
	}

	if !Contains(err, isProblem) {
		visit(err, false)
		return
	}

	switch wrapper := err.(type) {
	case interface{ Unwrap() []error }:
		for _, inner := range wrapper.Unwrap() {
			Walk(inner, isProblem, visit)
		}
	case interface{ Unwrap() error }:
		Walk(wrapper.Unwrap(), isProblem, visit)
	}
}

// Contains reports whether err, or any error beneath it, is one isProblem
// reports as a problem
func Contains(err error, isProblem func(error) bool) bool {
	if err == nil {
		return false
	}

	if isProblem(err) {
		return true
	}

	switch wrapper := err.(type) {
	case interface{ Unwrap() []error }:
		for _, inner := range wrapper.Unwrap() {
			if Contains(inner, isProblem) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return Contains(wrapper.Unwrap(), isProblem)
	}

	return false
}
