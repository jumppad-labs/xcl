package logger

// WithTag returns a Logger that adds the detail key=value to every event it
// emits, the key "resource" fills in the event's ResourceID instead. Tags
// added by nested calls are all kept, a later tag with the same key wins. A
// Logger not returned by New, or a nil l, is returned unchanged.
func WithTag(l Logger, key string, value any) Logger {
	el, ok := l.(*eventLogger)
	if !ok {
		return l
	}

	return el.withTag(key, value)
}

// WithSource returns a Logger whose events name source as their Source, i.e.
// the name of the plugin that writes them. A Logger not returned by New, or a
// nil l, is returned unchanged.
func WithSource(l Logger, source string) Logger {
	el, ok := l.(*eventLogger)
	if !ok {
		return l
	}

	sourced := el.withTag("", nil)
	sourced.base.Source = source

	return sourced
}
