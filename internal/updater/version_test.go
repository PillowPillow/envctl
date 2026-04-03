package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Version
		wantErr bool
	}{
		{"with v prefix", "v1.2.3", Version{1, 2, 3, "v1.2.3"}, false},
		{"without v prefix", "1.2.3", Version{1, 2, 3, "1.2.3"}, false},
		{"zeros", "v0.0.0", Version{0, 0, 0, "v0.0.0"}, false},
		{"large numbers", "v10.20.30", Version{10, 20, 30, "v10.20.30"}, false},
		{"missing patch", "v1.2", Version{}, true},
		{"empty", "", Version{}, true},
		{"non-numeric major", "vx.2.3", Version{}, true},
		{"non-numeric minor", "v1.x.3", Version{}, true},
		{"non-numeric patch", "v1.2.x", Version{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersion_NewerThan(t *testing.T) {
	tests := []struct {
		name string
		v    Version
		o    Version
		want bool
	}{
		{"major newer", Version{Major: 2}, Version{Major: 1}, true},
		{"major older", Version{Major: 1}, Version{Major: 2}, false},
		{"minor newer", Version{Major: 1, Minor: 2}, Version{Major: 1, Minor: 1}, true},
		{"minor older", Version{Major: 1, Minor: 1}, Version{Major: 1, Minor: 2}, false},
		{"patch newer", Version{Major: 1, Minor: 1, Patch: 2}, Version{Major: 1, Minor: 1, Patch: 1}, true},
		{"patch older", Version{Major: 1, Minor: 1, Patch: 1}, Version{Major: 1, Minor: 1, Patch: 2}, false},
		{"equal", Version{Major: 1, Minor: 2, Patch: 3}, Version{Major: 1, Minor: 2, Patch: 3}, false},
		{"zero vs zero", Version{}, Version{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.v.NewerThan(tt.o))
		})
	}
}

func TestVersion_String(t *testing.T) {
	v := Version{Major: 1, Minor: 2, Patch: 3}
	assert.Equal(t, "v1.2.3", v.String())
}
