package release

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNoVersionTags is returned by Next when none of the tags is a version, so
// there is nothing to count from. A repository adopting automated releases
// is given a seed tag by hand first. Match it with errors.Is.
var ErrNoVersionTags = errors.New("no version tags; push a seed tag such as v0.1.0 first")

// ErrNotAVersion is returned by ParseVersion and Bump for a tag that is not
// a plain version, vMAJOR.MINOR.PATCH. Match it with errors.Is.
var ErrNotAVersion = errors.New("not a version tag")

// Version is a release version, written as a tag vMAJOR.MINOR.PATCH.
type Version struct {
	Major int
	Minor int
	Patch int
}

// String returns the version as a tag, such as v0.7.1.
func (v Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Less reports whether v is an earlier version than other.
func (v Version) Less(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

// ParseVersion reads a tag of the form vMAJOR.MINOR.PATCH. Anything else,
// including a pre-release such as v0.8.0-rc1, wraps ErrNotAVersion.
func ParseVersion(tag string) (Version, error) {
	numbers, ok := strings.CutPrefix(tag, "v")
	if !ok {
		return Version{}, fmt.Errorf("%w: %q", ErrNotAVersion, tag)
	}

	parts := strings.Split(numbers, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%w: %q", ErrNotAVersion, tag)
	}

	values := make([]int, 3)
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 || strconv.Itoa(value) != part {
			return Version{}, fmt.Errorf("%w: %q", ErrNotAVersion, tag)
		}
		values[i] = value
	}

	return Version{Major: values[0], Minor: values[1], Patch: values[2]}, nil
}

// Latest returns the highest version among tags, ignoring every tag that is
// not a plain version. The boolean is false when no tag is a version.
func Latest(tags []string) (Version, bool) {
	var latest Version
	found := false
	for _, tag := range tags {
		version, err := ParseVersion(tag)
		if err != nil {
			continue
		}
		if !found || latest.Less(version) {
			latest = version
			found = true
		}
	}
	return latest, found
}

// Next returns the version a release of the given size makes after the
// latest version among tags. The boolean is false for SizeNoRelease, which
// makes no release. A major release before 1.0 releases 1.0.0. It returns
// ErrNoVersionTags when no tag is a version.
func Next(tags []string, size Size) (Version, bool, error) {
	if size == SizeNoRelease {
		return Version{}, false, nil
	}

	latest, found := Latest(tags)
	if !found {
		return Version{}, false, ErrNoVersionTags
	}

	switch size {
	case SizeMajor:
		return Version{Major: latest.Major + 1}, true, nil
	case SizeMinor:
		return Version{Major: latest.Major, Minor: latest.Minor + 1}, true, nil
	case SizePatch:
		return Version{Major: latest.Major, Minor: latest.Minor, Patch: latest.Patch + 1}, true, nil
	default:
		return Version{}, false, fmt.Errorf("unknown release size %q", size)
	}
}

// Bump returns the size of the release that went from one version tag to a
// later one: major when the major number changed, minor when the minor
// number changed, and patch otherwise.
func Bump(from, to string) (Size, error) {
	previous, err := ParseVersion(from)
	if err != nil {
		return "", err
	}
	next, err := ParseVersion(to)
	if err != nil {
		return "", err
	}
	if !previous.Less(next) {
		return "", fmt.Errorf("%s is not later than %s", to, from)
	}

	switch {
	case next.Major != previous.Major:
		return SizeMajor, nil
	case next.Minor != previous.Minor:
		return SizeMinor, nil
	default:
		return SizePatch, nil
	}
}
