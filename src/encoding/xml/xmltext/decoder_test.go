//go:build goexperiment.xmlv2

package xmltext

import (
	"bytes"
	stdxml "encoding/xml/internal/legacyxml"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

var corpus = []string{
	`<a/>`,
	`<?xml version="1.0" encoding="UTF-8"?><a x="1" y='2'>text &amp; &#x41;&#66; &lt;</a>`,
	"<a>line1\r\nline2\rline3</a>",
	`<root xmlns="urn:d" xmlns:p="urn:p"><p:b p:k="v" k="w">t</p:b><c xmlns="">x</c></root>`,
	`<a><![CDATA[<raw> & ]]]]><![CDATA[>]]></a>`,
	`<a><!-- a comment --><?pi some data?><b></b></a>`,
	`<!DOCTYPE a [<!ENTITY x "y>z"> <!ELEMENT a ANY>]><a/>`,
	`<a>한국어 텍스트 😀 &#x1F600;</a>`,
	`<a  x = "1"   y="a>b" ><b / ></a>`[:0] + `<a  x = "1"   y="a>b" ><b/></a>`,
	"<a>\n  <b>1</b>\n  <b>2</b>\n</a>\n",
	`<a x="&lt;&gt;&amp;&apos;&quot;&#10;"/>`,
	`<a>` + strings.Repeat("x", 100000) + `<b>` + strings.Repeat("y&amp;", 5000) + `</b></a>`,
}

type tok struct {
	kind  Kind
	name  Name
	attrs []Attr
	text  string
}

func (t tok) String() string {
	return fmt.Sprintf("%v %q %q %q", t.kind, t.name, t.attrs, t.text)
}

func readAll(t *testing.T, d *Decoder) ([]tok, error) {
	var out []tok
	for {
		tk, err := d.ReadToken()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		x := tok{kind: tk.Kind(), name: tk.Name()}
		for it := tk.Attrs(); it.Next(); {
			x.attrs = append(x.attrs, Attr{it.Name(), string(it.Value())})
		}
		if tk.Kind() == ProcInst {
			x.text = string(tk.Text())
		} else {
			x.text = string(tk.Text())
		}
		out = append(out, x)
	}
}

// stdRawAttrs returns the lexical attribute names (prefix in Space) of each
// start element of in. std's Token() reports a declaration as {"xmlns", prefix}
// or {"", "xmlns"}, which is also what q:foo gets when q is bound to the URI
// "xmlns", and it loses the prefix of an attribute bound to the xmlns URI; only
// RawToken's lexical names tell these apart. strict, autoClose and entity
// configure the decoder as for Token().
func stdRawAttrs(in string, strict bool, autoClose []string, entity map[string]string) [][]stdxml.Name {
	d := stdxml.NewDecoder(strings.NewReader(in))
	d.Strict, d.AutoClose, d.Entity = strict, autoClose, entity
	var out [][]stdxml.Name
	for {
		tk, err := d.RawToken()
		if err != nil {
			return out
		}
		if v, ok := tk.(stdxml.StartElement); ok {
			names := make([]stdxml.Name, len(v.Attr))
			for i, a := range v.Attr {
				names[i] = a.Name
			}
			out = append(out, names)
		}
	}
}

// stdAttrName converts the name of one of std's attributes (raw is its lexical
// name) to xmltext's convention: a declaration is named by the xmlns namespace
// (xmlns="u" is {XMLNSNamespace, "xmlns"}), and a permissive xmlns:xmlns keeps the
// prefix as its Space. Intended difference: an attribute whose prefix is bound
// to the xmlns URI is reported like an unbound one, by its prefix, because only
// declarations have that Space.
func stdAttrName(n, raw stdxml.Name) Name {
	switch {
	case raw.Space != "xmlns" && !(raw.Space == "" && raw.Local == "xmlns"):
		if n.Space == XMLNSNamespace {
			return Name{raw.Space, n.Local}
		}
		return Name{n.Space, n.Local}
	case n.Space == "":
		return Name{XMLNSNamespace, "xmlns"}
	case n.Local == "xmlns":
		return Name{"xmlns", "xmlns"}
	}
	return Name{XMLNSNamespace, n.Local}
}

func stdTokens(t *testing.T, in string) ([]tok, error) {
	d := stdxml.NewDecoder(strings.NewReader(in))
	raws := stdRawAttrs(in, true, nil, nil)
	var out []tok
	nStart := 0
	for {
		tk, err := d.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		switch v := tk.(type) {
		case stdxml.StartElement:
			x := tok{kind: StartElement, name: Name{v.Name.Space, v.Name.Local}}
			var raw []stdxml.Name
			if nStart < len(raws) {
				raw = raws[nStart]
			}
			nStart++
			for i, a := range v.Attr {
				var rn stdxml.Name
				if i < len(raw) {
					rn = raw[i]
				}
				x.attrs = append(x.attrs, Attr{stdAttrName(a.Name, rn), a.Value})
			}
			out = append(out, x)
		case stdxml.EndElement:
			out = append(out, tok{kind: EndElement, name: Name{v.Name.Space, v.Name.Local}})
		case stdxml.CharData:
			out = append(out, tok{kind: Text, text: string(v)})
		case stdxml.Comment:
			out = append(out, tok{kind: Comment, text: string(v)})
		case stdxml.ProcInst:
			out = append(out, tok{kind: ProcInst, name: Name{Local: v.Target}, text: string(v.Inst)})
		case stdxml.Directive:
			out = append(out, tok{kind: Directive, text: string(v)})
		}
	}
}

// normalize merges adjacent Text/CData (std reports both as CharData).
func normalize(in []tok) []tok {
	var out []tok
	for _, t := range in {
		if t.kind == CData {
			t.kind = Text
		}
		if t.kind == Text && len(out) > 0 && out[len(out)-1].kind == Text {
			out[len(out)-1].text += t.text
			continue
		}
		out = append(out, t)
	}
	return out
}

func readers(in string) map[string]func() *Decoder {
	return map[string]func() *Decoder{
		"bytes":   func() *Decoder { return NewDecoderBytes([]byte(in)) },
		"buffer":  func() *Decoder { return NewDecoder(bytes.NewBufferString(in)) },
		"reader":  func() *Decoder { return NewDecoder(strings.NewReader(in)) },
		"onebyte": func() *Decoder { return NewDecoder(iotest.OneByteReader(strings.NewReader(in))) },
		"half":    func() *Decoder { return NewDecoder(iotest.HalfReader(strings.NewReader(in))) },
		"dataerr": func() *Decoder { return NewDecoder(iotest.DataErrReader(strings.NewReader(in))) },
	}
}

func TestMatchesStd(t *testing.T) {
	for i, in := range corpus {
		want, err := stdTokens(t, in)
		if err != nil {
			t.Fatalf("corpus %d: std: %v", i, err)
		}
		want = normalize(want)
		// std emits EndElement for <a/> as well; xmltext does too.
		for name, mk := range readers(in) {
			got, err := readAll(t, mk())
			if err != nil {
				t.Errorf("corpus %d %s: %v", i, name, err)
				continue
			}
			got = normalize(got)
			if len(got) != len(want) {
				t.Errorf("corpus %d %s: %d tokens, want %d\n got %v\nwant %v", i, name, len(got), len(want), trunc(got), trunc(want))
				continue
			}
			for k := range got {
				if fmt.Sprint(got[k]) != fmt.Sprint(want[k]) && !(len(got[k].text) > 200 && got[k].text == want[k].text) {
					t.Errorf("corpus %d %s token %d:\n got %v\nwant %v", i, name, k, trunc(got[k:k+1]), trunc(want[k:k+1]))
					break
				}
			}
		}
	}
}

func trunc(ts []tok) string {
	s := fmt.Sprint(ts)
	if len(s) > 400 {
		s = s[:400] + "..."
	}
	return s
}

func TestErrors(t *testing.T) {
	bad := []string{
		``, `text`, `<a>`, `<a></b>`, `<a x="1" x="2"/>`, `<a x=1/>`, `<a/><b/>`, `<a>&bogus;</a>`,
		`<a>&#0;</a>`, `<p:a/>`, `<a x="<"/>`, `<a>]]></a>`, `<!-- a -- b --><a/>`, `<a><!-- x --- ></a>`,
		"<a>\x01</a>", "<a>\xff</a>", `<a b="1"c="2"/>`, `</a>`, `<1a/>`, `<a:b:c/>`, `<?xml version="2.0"?><a/>`,
		`<?xml version="1.0" encoding="latin1"?><a/>`, `<a><![CDATA[x</a>`, `<a x="1`, `<a><b></a></b>`,
	}
	for _, in := range bad {
		for name, mk := range readers(in) {
			_, err := readAll(t, mk())
			if err == nil {
				t.Errorf("%q (%s): no error", in, name)
			} else if _, ok := err.(*SyntaxError); !ok {
				t.Errorf("%q (%s): error %T %v, want *SyntaxError", in, name, err, err)
			}
		}
	}
}

func TestPositions(t *testing.T) {
	d := NewDecoder(iotest.OneByteReader(strings.NewReader("<a>\n<b>\n  </x>")))
	_, err := readAll(t, d)
	se, ok := err.(*SyntaxError)
	if !ok || se.Line != 3 || se.Column != 3 {
		t.Fatalf("got %#v, want line 3 col 3", err)
	}
}

func TestReadSkipElement(t *testing.T) {
	in := `<r><a x="1">t<b/>&amp;</a><c>` + strings.Repeat("z", 70000) + `</c><d/></r>`
	for name, mk := range readers(in) {
		d := mk()
		tk, _ := d.ReadToken() // <r>
		if tk.Kind() != StartElement {
			t.Fatal(name)
		}
		raw, err := d.ReadElement()
		if err != nil || string(raw) != `<a x="1">t<b/>&amp;</a>` {
			t.Fatalf("%s: raw %q err %v", name, raw, err)
		}
		raw, err = d.ReadElement()
		if err != nil || len(raw) != 70007 {
			t.Fatalf("%s: raw len %d err %v", name, len(raw), err)
		}
		if err := d.SkipElement(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if k := d.PeekKind(); k != EndElement {
			t.Fatalf("%s: peek %v", name, k)
		}
		if _, err := readAll(t, d); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestClone(t *testing.T) {
	d := NewDecoderBytes([]byte(`<a x="1&amp;">hi</a>`))
	tk, _ := d.ReadToken()
	c := tk.Clone()
	d.ReadToken()
	it := c.Attrs()
	if !it.Next() || string(it.Value()) != "1&" || c.Name().Local != "a" {
		t.Fatal("clone lost data")
	}
}

func TestFragments(t *testing.T) {
	d := NewDecoderBytes([]byte(`hello<a/><b/>`), AllowFragments(true))
	if _, err := readAll(t, d); err != nil {
		t.Fatal(err)
	}
}

func TestMaxDepth(t *testing.T) {
	d := NewDecoderBytes([]byte(strings.Repeat("<a>", 5)), MaxDepth(3))
	if _, err := readAll(t, d); err == nil || !strings.Contains(err.Error(), "max depth") {
		t.Fatalf("got %v", err)
	}
}

func TestInputNotModified(t *testing.T) {
	in := []byte("<a x=\"&amp;\r\n\">t\r\n&lt;<![CDATA[\r]]></a>")
	orig := append([]byte(nil), in...)
	d := NewDecoderBytes(in)
	if _, err := readAll(t, d); err != nil {
		t.Fatal(err)
	}
	d.ResetBytes(in)
	d.Reset(strings.NewReader("<a/>")) // must not reuse the caller's slice as a read buffer
	readAll(t, d)
	if !bytes.Equal(in, orig) {
		t.Fatal("decoder modified caller input")
	}
}

func TestPredefinedEntityEdges(t *testing.T) {
	cases := map[string]string{
		"&lt;&gt;&amp;&quot;&apos;": `<>&"'`,
		"a&lt":                      "ERR", "&amp": "ERR", "&quo;": "ERR", "&quot": "ERR", "&apos;&apos": "ERR",
		"x&#65;&amp;y\r\nz\rw": "xA&y\nz\nw",
	}
	for in, want := range cases {
		d := NewDecoderBytes([]byte("<a>" + in + "</a>"))
		got, err := readAll(t, d)
		if want == "ERR" {
			if err == nil {
				t.Errorf("%q: want error", in)
			}
			continue
		}
		if err != nil || got[1].text != want {
			t.Errorf("%q: got %v %v, want %q", in, got, err, want)
		}
	}
}
