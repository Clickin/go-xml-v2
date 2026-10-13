//go:build goexperiment.xmlv2

package xmltext

import (
	"fmt"
	"strings"
	"testing"
	"testing/iotest"

	stdxml "encoding/xml/internal/legacyxml"
)

// FuzzAgainstStd checks chunked reads and compares accepted inputs with
// encoding/xml in both profiles, allowing only the documented differences.
func FuzzAgainstStd(f *testing.F) {
	for _, s := range corpus[:len(corpus)-1] {
		f.Add(s)
	}
	f.Add("<r>" + strings.Repeat("x", textWindow-2) + "&amp;한\r\n]]x</r>")
	for _, s := range []string{
		"<\u0132/>",
		`<?xml aversion="2.0"?><r/>`,
		`<?xml aencoding="latin1"?><r/>`,
		`<!DOCTYPE r [<!-- c --><!ELEMENT r ANY>]><r/>`,
		`<!DOCTYPE r "<!-- quoted -->"><r/>`,
		`<r><!-- c -->text</r>`,
		`<a xmlns:p="` + XMLNSNamespace + `" p:x="1"/>`,
		`<a xmlns:q="xmlns" q:foo="bar" xmlns:foo="x"/>`,
		"<r a=\"x\ny\tz\r\nw\rv&#9;&#10;&#13;&#x9;\" b='\t'/>",
		"<r xmlns:p=\"u\nv\"><p:c p:a=\"\r\n\"/></r>",
		"\ufeff<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<r>&#0000000065;&#x0000000041;</r>",
		"\ufeff <r/>",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		for _, permissive := range []bool{false, true} {
			got, err := readAll(t, NewDecoderBytes([]byte(in), Permissive(permissive)))
			got2, err2 := readAll(t, NewDecoder(iotest.OneByteReader(strings.NewReader(in)), Permissive(permissive)))
			// A streaming decoder can emit earlier Text pieces before discovering
			// an error later in the run; complete input validates the entire run.
			if (err == nil) != (err2 == nil) || err == nil && fmt.Sprint(normalize(got)) != fmt.Sprint(normalize(got2)) {
				t.Fatalf("permissive=%v chunked mismatch: %v vs %v\n%v\n%v", permissive, err, err2, got, got2)
			}
			if err != nil {
				continue
			}
			want, serr := stdTokens(t, in)
			if serr != nil {
				if !permissive {
					// Fifth-edition names include characters absent from std's
					// legacy tables. Do not excuse any other invalid-name error.
					if se, ok := serr.(*stdxml.SyntaxError); ok {
						if name, ok := strings.CutPrefix(se.Msg, "invalid XML name: "); ok && validNameString(name) {
							continue
						}
					}
				}
				t.Fatalf("permissive=%v std rejects accepted input: %v", permissive, serr)
			}
			if !permissive {
				want = strictStdView(t, in, want)
			}
			g, w := normalize(got), normalize(want)
			if !permissive {
				// The default profile preserves directive comments; std replaces
				// unquoted comments by one space. Compare every other token as-is.
				for i := range g {
					if g[i].kind == Directive {
						g[i].text = string(stripDirectiveComments(nil, []byte(g[i].text)))
					}
				}
			}
			if fmt.Sprint(g) != fmt.Sprint(w) {
				t.Fatalf("permissive=%v mismatch\n got %v\nwant %v", permissive, g, w)
			}
		}
	})
}

// strictStdView turns std's tokens for in into what the default profile reports
// where the two intentionally differ, and nothing else:
//   - attribute values: std keeps literal tab, newline and carriage return, the
//     default profile turns each into one space (a line end \r\n is one) and keeps
//     what a character reference produces (XML 1.0 section 3.3.3). The values are
//     taken from a second std parse of in with those characters (all
//     whitespace, so acceptance and structure do not change) replaced by spaces,
//     and so are the resolved element and attribute names;
//   - a UTF-8 byte order mark at the start of the stream is not text.
func strictStdView(t *testing.T, in string, want []tok) []tok {
	if strings.ContainsAny(in, "\t\n\r") {
		spaced := strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(in)
		if alt, err := stdTokens(t, spaced); err == nil && len(alt) == len(want) {
			out := append([]tok(nil), want...)
			for i := range out {
				// names too: a namespace declaration's value is an attribute value
				if k := out[i].kind; (k == StartElement || k == EndElement) && alt[i].kind == k {
					out[i] = alt[i]
				}
			}
			want = out
		}
	}
	if strings.HasPrefix(in, "\ufeff") && len(want) > 0 && want[0].kind == Text && strings.HasPrefix(want[0].text, "\ufeff") {
		want = append([]tok(nil), want...)
		if want[0].text = strings.TrimPrefix(want[0].text, "\ufeff"); want[0].text == "" {
			want = want[1:]
		}
	}
	return want
}
