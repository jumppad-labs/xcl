// Package release holds the decisions behind automated releases: which
// release size a pull request's labels name, the next version a merge
// produces, and how big the step between two versions is. The release
// workflows run them through cmd/xcl-release so the rules are tested here
// rather than written in workflow shell.
package release

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Size is how big a release a pull request makes, named by its release label.
type Size string

// The release sizes, each the name of the label that selects it.
const (
	SizeMajor     Size = "major"
	SizeMinor     Size = "minor"
	SizePatch     Size = "patch"
	SizeNoRelease Size = "no-release"
)

// Sizes are every release size, in the order they are listed to a user.
var Sizes = []Size{SizeMajor, SizeMinor, SizePatch, SizeNoRelease}

// ErrNoReleaseLabel is returned by SizeFromLabels when a pull request carries
// none of the release labels. Match it with errors.Is.
var ErrNoReleaseLabel = errors.New("no release label")

// ErrSeveralReleaseLabels is returned by SizeFromLabels when a pull request
// carries more than one release label. Match it with errors.Is.
var ErrSeveralReleaseLabels = errors.New("more than one release label")

// LabelError is the detail of a pull request's labels that do not name
// exactly one release size. It wraps ErrNoReleaseLabel or
// ErrSeveralReleaseLabels.
type LabelError struct {
	// Err is ErrNoReleaseLabel or ErrSeveralReleaseLabels
	Err error

	// Found are the release labels present, empty for ErrNoReleaseLabel
	Found []Size
}

// Error says what is wrong and which labels would fix it.
func (e *LabelError) Error() string {
	allowed := make([]string, len(Sizes))
	for i, size := range Sizes {
		allowed[i] = string(size)
	}

	if errors.Is(e.Err, ErrSeveralReleaseLabels) {
		found := make([]string, len(e.Found))
		for i, size := range e.Found {
			found[i] = string(size)
		}
		return fmt.Sprintf("%s: %s; keep exactly one of %s", e.Err, strings.Join(found, ", "), strings.Join(allowed, ", "))
	}

	return fmt.Sprintf("%s: add exactly one of %s", e.Err, strings.Join(allowed, ", "))
}

// Unwrap returns the sentinel the error carries.
func (e *LabelError) Unwrap() error {
	return e.Err
}

// SizeFromLabels returns the release size a pull request's labels name.
// Labels that are not release labels are ignored. It returns a *LabelError
// when no release label, or more than one, is present.
func SizeFromLabels(labels []string) (Size, error) {
	var found []Size
	for _, size := range Sizes {
		if slices.Contains(labels, string(size)) {
			found = append(found, size)
		}
	}

	switch len(found) {
	case 0:
		return "", &LabelError{Err: ErrNoReleaseLabel}
	case 1:
		return found[0], nil
	default:
		return "", &LabelError{Err: ErrSeveralReleaseLabels, Found: found}
	}
}
