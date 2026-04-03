// Package updater handles self-update of the envctl binary.
package updater

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version (semver).
type Version struct {
	Major, Minor, Patch int
	Raw                 string
}

// ParseVersion parses a "v1.2.3" or "1.2.3" string into a Version.
func ParseVersion(s string) (Version, error) {
	raw := s
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version %q: expected X.Y.Z", raw)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid version %q: non-numeric major", raw)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid version %q: non-numeric minor", raw)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Version{}, fmt.Errorf("invalid version %q: non-numeric patch", raw)
	}

	return Version{
		Major: major,
		Minor: minor,
		Patch: patch,
		Raw:   raw,
	}, nil
}

// NewerThan returns true if v is newer than other.
func (v Version) NewerThan(other Version) bool {
	if v.Major != other.Major {
		return v.Major > other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor > other.Minor
	}
	return v.Patch > other.Patch
}

// String returns the version as "vX.Y.Z".
func (v Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}
