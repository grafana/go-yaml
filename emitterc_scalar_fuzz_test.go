//go:build go1.18
// +build go1.18

package yaml

import (
	"strings"
	"testing"
)

func FuzzAnalyzeScalar(f *testing.F) {
	for _, s := range []string{
		"", "a", "- a", "a: b", "a #b", "---", "...", " lead", "trail ", "a\nb", "a \nb", "a\n b", "a\tb",
		`sum(rate(x{a="b"}[5m])) > 0.5`, "{{ $labels.job }} is down", "caf\u00e9\u2028",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// The encoder sends only valid UTF-8 to the emitter.
		checkAnalyzeScalar(t, []byte(strings.ToValidUTF8(s, "\ufffd")))
	})
}
