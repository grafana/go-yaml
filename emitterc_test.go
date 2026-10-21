package yaml

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type queueTestItem struct {
	Name   string            `yaml:"name"`
	Expr   string            `yaml:"expr"`
	Labels map[string]string `yaml:"labels"`
}

func queueTestDoc(n int) []queueTestItem {
	doc := make([]queueTestItem, n)
	for i := range doc {
		doc[i] = queueTestItem{
			Name:   "item" + strconv.Itoa(i),
			Expr:   "sum(rate(requests_total[5m]))",
			Labels: map[string]string{"team": "a", "severity": "warning"},
		}
	}
	return doc
}

type queueTestNode struct {
	Name     string            `yaml:"name"`
	Empty    map[string]string `yaml:"empty"`
	Tags     []string          `yaml:"tags"`
	Children []queueTestItem   `yaml:"children"`
}

// queueTestNodes returns n nodes and the exact text that the encoder must write for them.
// The empty collections and the nested items use the lookahead of the emitter.
func queueTestNodes(n int) ([]queueTestNode, string) {
	nodes := make([]queueTestNode, n)
	var want strings.Builder
	for i := range nodes {
		name := "node" + strconv.Itoa(i)
		nodes[i] = queueTestNode{
			Name:     name,
			Empty:    map[string]string{},
			Tags:     []string{},
			Children: queueTestDoc(1),
		}
		fmt.Fprintf(&want, "- name: %s\n", name)
		want.WriteString("  empty: {}\n" +
			"  tags: []\n" +
			"  children:\n" +
			"    - name: item0\n" +
			"      expr: sum(rate(requests_total[5m]))\n" +
			"      labels:\n" +
			"        severity: warning\n" +
			"        team: a\n")
	}
	return nodes, want.String()
}

// The emitter needs a few events of lookahead.
// Its queue must not keep a slot for every event of the document.
// Deep nesting still grows the queue past initial_queue_size.
// This document is shallow enough to stay inside it.
func TestEmitterEventQueueStaysBounded(t *testing.T) {
	nodes, want := queueTestNodes(10000)
	e := newEncoder()
	defer e.destroy()
	e.marshalDoc("", reflect.ValueOf(nodes))
	e.finish()
	if string(e.out) != want {
		t.Fatal("output differs from the expected document")
	}
	if got := cap(e.emitter.events); got > initial_queue_size {
		t.Fatalf("event queue capacity is %d, want at most %d", got, initial_queue_size)
	}
}

// An Encoder keeps one emitter for all of its documents.
// Its queue must not keep a slot for every event of the stream.
func TestEncoderEventQueueStaysBounded(t *testing.T) {
	const docs = 100
	nodes, doc := queueTestNodes(10)
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	for i := 0; i < docs; i++ {
		if err := enc.Encode(nodes); err != nil {
			t.Fatal(err)
		}
	}
	if got := cap(enc.encoder.emitter.events); got > initial_queue_size {
		t.Fatalf("event queue capacity is %d, want at most %d", got, initial_queue_size)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != doc+strings.Repeat("---\n"+doc, docs-1) {
		t.Fatal("output differs from the expected stream")
	}
}

func BenchmarkMarshalSequence(b *testing.B) {
	for _, n := range []int{1000, 10000, 50000} {
		doc := queueTestDoc(n)
		b.Run("items="+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Marshal(doc); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
