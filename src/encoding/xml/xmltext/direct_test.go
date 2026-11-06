// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.xmlv2

package xmltext

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// traceTokens reads d to the end and records what a caller can observe of
// every token: kind, names, attributes, text, offset, depth and the error.
func traceTokens(d *Decoder) string {
	var sb strings.Builder
	for range 1 << 16 {
		tok, err := d.ReadToken()
		fmt.Fprintf(&sb, "%v off=%d depth=%d", tok.Kind(), d.InputOffset(), d.StackDepth())
		if err != nil {
			fmt.Fprintf(&sb, " err=%v\n", err)
			break
		}
		switch tok.Kind() {
		case StartElement:
			fmt.Fprintf(&sb, " %q %q", tok.Name(), tok.RawName())
			it := tok.Attrs()
			for it.Next() {
				fmt.Fprintf(&sb, " [%q %q %q]", it.Name(), it.RawName(), it.Value())
			}
		case EndElement:
			fmt.Fprintf(&sb, " %q %q", tok.Name(), tok.RawName())
		case ProcInst:
			fmt.Fprintf(&sb, " %q %q", tok.Name(), tok.Text())
		default:
			fmt.Fprintf(&sb, " partial=%v %q", tok.Partial(), tok.Text())
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

var directOptions = [][]Options{
	nil,
	{MaxTokenSize(16)},
	{MaxTokenSize(40)},
	{AllowFragments(true)},
	{MaxAttributes(2), MaxDepth(3)},
	{BufferSize(16)},
}

// checkDirect reads in with every reader and option set, once as usual and
// once with ReadToken's direct path turned off, and reports a difference.
func checkDirect(t *testing.T, in string) {
	t.Helper()
	readers := []func() io.Reader{
		nil, // NewDecoderBytes
		func() io.Reader { return iotest.OneByteReader(strings.NewReader(in)) },
		func() io.Reader { return &chunkReader{in, 7} },
		func() io.Reader { return strings.NewReader(in) },
	}
	for _, opts := range directOptions {
		for ri, mk := range readers {
			newDec := func() *Decoder {
				if mk == nil {
					return NewDecoderBytes([]byte(in), opts...)
				}
				return NewDecoder(mk(), opts...)
			}
			on := newDec()
			off := newDec()
			off.direct = false
			got, want := traceTokens(on), traceTokens(off)
			if got != want {
				t.Fatalf("input %q, options %v, reader %d: direct path differs\ndirect:\n%s\nengine:\n%s", in, opts, ri, got, want)
			}
		}
	}
}

func directSeeds() []string {
	seeds := append([]string(nil), corpus...)
	return append(seeds,
		"<r>\n  <a x=\"1\" y='2'>text</a>\n  <b/>\n</r>",
		"<r><a>t</a><b>u</b><c/></r>",
		`<r a="x>y" b='p>q'>z</r>`,
		`<r a="unterminated></r>`,
		`<r><a></b></r>`,
		`<r>text<`,
		`<r>text</r`,
		`<r a="1" a="2"/>`,
		`<r><p:a xmlns:p="u" p:x="1">t</p:a></r>`,
		"<r>a&amp;b\r\nc</r>",
		"<r>\x01</r>",
		"<r>]]></r>",
		"<r><!-- c --><?pi d?><![CDATA[x]]></r>",
		"<r>"+strings.Repeat("<a b=\"c\">d</a>", 40)+"</r>",
		"<r>"+strings.Repeat("x", 100)+"</r>",
		"<r "+strings.Repeat("a", 50)+"=\"1\"/>",
		"<r>é<é é=\"é\">é</é></r>",
		"<r></r>trailing",
		"<r/><s/>",
	)
}

func TestDirectMatchesEngine(t *testing.T) {
	for _, in := range directSeeds() {
		checkDirect(t, in)
	}
}

// TestDirectUsed checks that most tokens of a plain document are read
// through the direct path, so that TestDirectMatchesEngine compares two
// different paths.
func TestDirectUsed(t *testing.T) {
	in := []byte("<r>" + strings.Repeat("\n  <a x=\"1\">text</a><b/>", 50) + "</r>")
	d := NewDecoderBytes(in)
	total, batched := 0, 0
	for {
		_, err := d.ReadToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		total++
		if d.bi < d.bn {
			batched++
		}
	}
	if direct := total - batched; direct < total/2 {
		t.Fatalf("%d of %d tokens came from the direct path", direct, total)
	}
}

func FuzzDirect(f *testing.F) {
	for _, s := range directSeeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		checkDirect(t, in)
	})
}
