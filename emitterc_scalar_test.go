package yaml

import (
	"strings"
	"testing"
)

// yaml_emitter_analyze_scalar_reference is a copy of yaml_emitter_analyze_scalar
// at commit 4d2ac027, before the ASCII fast path.
// The tests check that both functions give the same result.
// Change this copy only for an intended change of behaviour.
func yaml_emitter_analyze_scalar_reference(emitter *yaml_emitter_t, value []byte) bool {
	var (
		block_indicators   = false
		flow_indicators    = false
		line_breaks        = false
		special_characters = false
		tab_characters     = false

		leading_space  = false
		leading_break  = false
		trailing_space = false
		trailing_break = false
		break_space    = false
		space_break    = false

		preceded_by_whitespace = false
		followed_by_whitespace = false
		previous_space         = false
		previous_break         = false
	)

	emitter.scalar_data.value = value

	if len(value) == 0 {
		emitter.scalar_data.multiline = false
		emitter.scalar_data.flow_plain_allowed = false
		emitter.scalar_data.block_plain_allowed = true
		emitter.scalar_data.single_quoted_allowed = true
		emitter.scalar_data.block_allowed = false
		return true
	}

	if len(value) >= 3 && ((value[0] == '-' && value[1] == '-' && value[2] == '-') || (value[0] == '.' && value[1] == '.' && value[2] == '.')) {
		block_indicators = true
		flow_indicators = true
	}

	preceded_by_whitespace = true
	for i, w := 0, 0; i < len(value); i += w {
		w = width(value[i])
		followed_by_whitespace = i+w >= len(value) || is_blank(value, i+w)

		if i == 0 {
			switch value[i] {
			case '#', ',', '[', ']', '{', '}', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
				flow_indicators = true
				block_indicators = true
			case '?', ':':
				flow_indicators = true
				if followed_by_whitespace {
					block_indicators = true
				}
			case '-':
				if followed_by_whitespace {
					flow_indicators = true
					block_indicators = true
				}
			}
		} else {
			switch value[i] {
			case ',', '?', '[', ']', '{', '}':
				flow_indicators = true
			case ':':
				flow_indicators = true
				if followed_by_whitespace {
					block_indicators = true
				}
			case '#':
				if preceded_by_whitespace {
					flow_indicators = true
					block_indicators = true
				}
			}
		}

		if value[i] == '\t' {
			tab_characters = true
		} else if !is_printable(value, i) || !is_ascii(value, i) && !emitter.unicode {
			special_characters = true
		}
		if is_space(value, i) {
			if i == 0 {
				leading_space = true
			}
			if i+width(value[i]) == len(value) {
				trailing_space = true
			}
			if previous_break {
				break_space = true
			}
			previous_space = true
			previous_break = false
		} else if is_break(value, i) {
			line_breaks = true
			if i == 0 {
				leading_break = true
			}
			if i+width(value[i]) == len(value) {
				trailing_break = true
			}
			if previous_space {
				space_break = true
			}
			previous_space = false
			previous_break = true
		} else {
			previous_space = false
			previous_break = false
		}

		// [Go]: Why 'z'? Couldn't be the end of the string as that's the loop condition.
		preceded_by_whitespace = is_blankz(value, i)
	}

	emitter.scalar_data.multiline = line_breaks
	emitter.scalar_data.flow_plain_allowed = true
	emitter.scalar_data.block_plain_allowed = true
	emitter.scalar_data.single_quoted_allowed = true
	emitter.scalar_data.block_allowed = true

	if leading_space || leading_break || trailing_space || trailing_break {
		emitter.scalar_data.flow_plain_allowed = false
		emitter.scalar_data.block_plain_allowed = false
	}
	if trailing_space {
		emitter.scalar_data.block_allowed = false
	}
	if break_space {
		emitter.scalar_data.flow_plain_allowed = false
		emitter.scalar_data.block_plain_allowed = false
		emitter.scalar_data.single_quoted_allowed = false
	}
	if space_break || tab_characters || special_characters {
		emitter.scalar_data.flow_plain_allowed = false
		emitter.scalar_data.block_plain_allowed = false
		emitter.scalar_data.single_quoted_allowed = false
	}
	if space_break || special_characters {
		emitter.scalar_data.block_allowed = false
	}
	if line_breaks {
		emitter.scalar_data.flow_plain_allowed = false
		emitter.scalar_data.block_plain_allowed = false
	}
	if flow_indicators {
		emitter.scalar_data.flow_plain_allowed = false
	}
	if block_indicators {
		emitter.scalar_data.block_plain_allowed = false
	}
	return true
}

type analyzeScalarResult struct {
	ok                    bool
	multiline             bool
	flow_plain_allowed    bool
	block_plain_allowed   bool
	single_quoted_allowed bool
	block_allowed         bool
}

func analyzeScalar(f func(*yaml_emitter_t, []byte) bool, unicode bool, value []byte) analyzeScalarResult {
	emitter := &yaml_emitter_t{unicode: unicode}
	ok := f(emitter, value)
	d := emitter.scalar_data
	return analyzeScalarResult{ok, d.multiline, d.flow_plain_allowed, d.block_plain_allowed, d.single_quoted_allowed, d.block_allowed}
}

func checkAnalyzeScalar(t *testing.T, value []byte) {
	t.Helper()
	for _, unicode := range []bool{false, true} {
		want := analyzeScalar(yaml_emitter_analyze_scalar_reference, unicode, value)
		got := analyzeScalar(yaml_emitter_analyze_scalar, unicode, value)
		if got != want {
			t.Fatalf("value %q, unicode %v: got %+v, want %+v", value, unicode, got, want)
		}
	}
}

// analyzeScalarRunes holds plain characters and characters with special rules in the analysis.
// The multi-byte characters take the general path.
// They check the state that passes from one path to the other, for example a space after a NEL.
var analyzeScalarRunes = []rune{
	'a', '0', ' ', '\t', '\n', '\r', 0, 0x7f, 0x1f, '-', '.', ':', '?', '#', ',', '[', ']', '{', '}',
	'&', '*', '!', '|', '>', '\'', '"', '%', '@', '`',
	'\u00e9', '\u0085', '\u00a0', '\u2028', '\u2029', '\ud7ff', '\ue000', '\ufeff', '\ufffe', '\U0001F600',
}

func TestAnalyzeScalarMatchesReference(t *testing.T) {
	checkAnalyzeScalar(t, nil)
	// All strings of 1, 2 or 3 ASCII bytes.
	for a := 0; a < 0x80; a++ {
		checkAnalyzeScalar(t, []byte{byte(a)})
		for b := 0; b < 0x80; b++ {
			checkAnalyzeScalar(t, []byte{byte(a), byte(b)})
			for c := 0; c < 0x80; c++ {
				checkAnalyzeScalar(t, []byte{byte(a), byte(b), byte(c)})
			}
		}
	}
	// All strings of 3 characters from analyzeScalarRunes.
	// Each string also gets a fourth character whose rules depend on the character before it.
	// A padded copy checks the same characters far from the start and the end of the value.
	pad := strings.Repeat("a", 16)
	for _, a := range analyzeScalarRunes {
		for _, b := range analyzeScalarRunes {
			for _, c := range analyzeScalarRunes {
				s := string([]rune{a, b, c})
				checkAnalyzeScalar(t, []byte(s))
				checkAnalyzeScalar(t, []byte(pad+s+"a"))
				for _, d := range []rune{' ', '\t', '\n', ':', '#', '\u00e9'} {
					checkAnalyzeScalar(t, []byte(s+string(d)))
				}
			}
		}
	}
}

// analyzeScalarBenchValues are typical rule values, plus one value with multi-byte characters.
var analyzeScalarBenchValues = [][]byte{
	[]byte(`sum by (job, instance) (rate(http_requests_total{job="api", status=~"5.."}[5m])) / sum by (job, instance) (rate(http_requests_total{job="api"}[5m]))`),
	[]byte("{{ $labels.job }} on {{ $labels.instance }} is {{ $value | humanize }}.\nCheck the dashboard."),
	[]byte("job_instance:http_requests_errors:ratio_rate5m"),
	[]byte("critical"),
	[]byte("caf\u00e9 \u65e5\u672c\u8a9e on {{ $labels.instance }}"),
}

func BenchmarkAnalyzeScalar(b *testing.B) {
	for _, impl := range []struct {
		name string
		f    func(*yaml_emitter_t, []byte) bool
	}{
		{"reference", yaml_emitter_analyze_scalar_reference},
		{"fast", yaml_emitter_analyze_scalar},
	} {
		b.Run("impl="+impl.name, func(b *testing.B) {
			emitter := &yaml_emitter_t{unicode: true}
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for _, v := range analyzeScalarBenchValues {
					impl.f(emitter, v)
				}
			}
		})
	}
}
