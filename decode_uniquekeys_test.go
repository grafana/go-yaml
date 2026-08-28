package yaml_test

import (
	"strconv"
	"strings"
	"testing"

	. "gopkg.in/check.v1"

	yaml "go.yaml.in/yaml/v3"
)

// keysAboveScanLimit must stay above uniqueKeysScanLimit in decode.go, which is
// what makes the padded half of every case take the map based scan.
const keysAboveScanLimit = 1000

func fillerKeys(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("filler" + strconv.Itoa(i) + ": 0\n")
	}
	return b.String()
}

var uniqueKeysTests = []struct {
	name  string
	doc   string
	error string
}{{
	name: "no duplicates",
	doc:  "a: 1\nb: 2\n",
}, {
	name:  "one duplicate",
	doc:   "a: 1\nb: 2\na: 3\n",
	error: "yaml: unmarshal errors:\n  line 3: mapping key \"a\" already defined at line 1",
}, {
	name: "interleaved duplicates, reported in first occurrence order",
	doc:  "a: 1\nb: 2\nb: 3\na: 4\n",
	error: "yaml: unmarshal errors:\n" +
		"  line 4: mapping key \"a\" already defined at line 1\n" +
		"  line 3: mapping key \"b\" already defined at line 2",
}, {
	name: "one key repeated three times, once per pair",
	doc:  "a: 1\na: 2\na: 3\n",
	error: "yaml: unmarshal errors:\n" +
		"  line 2: mapping key \"a\" already defined at line 1\n" +
		"  line 3: mapping key \"a\" already defined at line 1\n" +
		"  line 3: mapping key \"a\" already defined at line 2",
}, {
	name:  "merge keys are duplicates, they are compared before being expanded",
	doc:   "x: &x\n  p: 1\ny: &y\n  q: 2\n<<: *x\n<<: *y\n",
	error: "yaml: unmarshal errors:\n  line 6: mapping key \"<<\" already defined at line 5",
}, {
	name:  "non-scalar keys of the same kind are duplicates, both have an empty value",
	doc:   "? [1, 2]\n: v1\n? [3, 4]\n: v2\n",
	error: "yaml: unmarshal errors:\n  line 3: mapping key \"\" already defined at line 1",
}, {
	name:  "non-scalar keys of different kinds are not duplicates",
	doc:   "? [1, 2]\n: v1\n? {m: 3}\n: v2\n",
	error: "yaml: invalid map key: []interface {}{1, 2}",
}}

func (s *S) TestUniqueKeys(c *C) {
	for _, test := range uniqueKeysTests {
		// Run each case twice: once as written, and once with extra keys added
		// so that the mapping is big enough to use the map based scan.
		for _, filler := range []int{0, keysAboveScanLimit} {
			comment := Commentf("%s, %d filler keys", test.name, filler)
			var v map[interface{}]interface{}
			err := yaml.Unmarshal([]byte(test.doc+fillerKeys(filler)), &v)
			if test.error == "" {
				c.Assert(err, IsNil, comment)
				continue
			}
			c.Assert(err, NotNil, comment)
			c.Assert(err.Error(), Equals, test.error, comment)
		}
	}
}

func benchmarkUniqueKeys(b *testing.B, keys int) {
	var doc strings.Builder
	for i := 0; i < keys; i++ {
		doc.WriteString("key-" + strconv.Itoa(i) + "-0123456789: value\n")
	}
	data := []byte(doc.String())
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v map[string]string
		if err := yaml.Unmarshal(data, &v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUniqueKeys32(b *testing.B)    { benchmarkUniqueKeys(b, 32) }
func BenchmarkUniqueKeys1000(b *testing.B)  { benchmarkUniqueKeys(b, 1000) }
func BenchmarkUniqueKeys4000(b *testing.B)  { benchmarkUniqueKeys(b, 4000) }
func BenchmarkUniqueKeys16000(b *testing.B) { benchmarkUniqueKeys(b, 16000) }
func BenchmarkUniqueKeys32000(b *testing.B) { benchmarkUniqueKeys(b, 32000) }
