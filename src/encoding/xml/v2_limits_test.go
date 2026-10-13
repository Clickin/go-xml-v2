// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.xmlv2

package xml

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

// v1 keeps encoding/xml's behavior: no limit on a text run, a CDATA section or
// the attributes of a tag (xmltext's defaults of 16 MiB and 1000 do not apply).
func TestNoSizeLimits(t *testing.T) {
	var attrs strings.Builder
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&attrs, ` a%d="v"`, i)
	}
	big := strings.Repeat("x", 17<<20)
	for _, c := range []struct {
		name, in string
		check    func(Token) bool
	}{
		{"attributes", "<r" + attrs.String() + "/>", func(tk Token) bool { se, ok := tk.(StartElement); return !ok || len(se.Attr) == 3000 }},
		{"text", "<r>" + big + "</r>", func(tk Token) bool { cd, ok := tk.(CharData); return !ok || len(cd) == len(big) }},
		{"cdata", "<r><![CDATA[" + big + "]]></r>", func(tk Token) bool { cd, ok := tk.(CharData); return !ok || len(cd) == len(big) }},
	} {
		for _, reader := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reader=%v", c.name, reader), func(t *testing.T) {
				var d *Decoder
				if reader {
					d = NewDecoder(struct{ io.Reader }{strings.NewReader(c.in)})
				} else {
					d = NewDecoder(strings.NewReader(c.in))
				}
				n := 0
				for {
					tk, err := d.RawToken()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					if !c.check(tk) {
						t.Fatalf("token %d is wrong: %T", n, tk)
					}
					n++
				}
				if n != 3 && c.name != "attributes" || n != 1+1 && c.name == "attributes" {
					t.Fatalf("%d tokens", n)
				}
			})
		}
	}
}
