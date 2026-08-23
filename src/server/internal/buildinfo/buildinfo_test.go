package buildinfo

import (
	"strings"
	"testing"
)

func TestStringIncludesMetadata(t *testing.T) {
	value := String()
	for _, part := range []string{"sing-box-observability", Version, Commit, BuildTime} {
		if !strings.Contains(value, part) {
			t.Fatalf("build information %q does not include %q", value, part)
		}
	}
}
