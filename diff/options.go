package diff

// Options are the settings of a diff, built from Option values by NewOptions.
type Options struct {
	// RevealSensitive includes sensitive values in the result. Without it a
	// changed sensitive value is reported with neither its before nor its
	// after value.
	RevealSensitive bool
}

// Option changes a setting of a diff.
type Option func(*Options)

// RevealSensitive includes the before and after values of sensitive values in
// the result. The changes are still marked sensitive.
func RevealSensitive() Option {
	return func(options *Options) {
		options.RevealSensitive = true
	}
}

// NewOptions resolves options into the settings of a diff. A nil option is
// ignored.
func NewOptions(options ...Option) Options {
	resolved := Options{}

	for _, option := range options {
		if option != nil {
			option(&resolved)
		}
	}

	return resolved
}
