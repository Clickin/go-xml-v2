// Asset benchmark for encoding/xml/xmltext.
//
// The checksum rules match AssetBench.java and rust/src/main.rs: names,
// non-namespace attribute names and resolved values, and trimmed coalesced
// text are folded as UTF-16 code units. This harness folds the decoder's
// byte slices and names in place instead of building strings, so the
// measured time is the file read, the parse and the fold, not harness
// allocations.
package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml/xmltext"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
	"unicode/utf8"
)

type sample struct {
	Events   int     `json:"events"`
	Checksum int32   `json:"checksum"`
	Seconds  float64 `json:"seconds"`
}

func foldRune(h int32, c rune) int32 {
	if c <= 0xffff {
		return h*31 + int32(c)
	}
	c -= 0x10000
	h = h*31 + int32(0xd800+(c>>10))
	return h*31 + int32(0xdc00+(c&0x3ff))
}

func foldBytes(h int32, b []byte) int32 {
	for i := 0; i < len(b); {
		if c := b[i]; c < utf8.RuneSelf {
			h = h*31 + int32(c)
			i++
			continue
		}
		c, n := utf8.DecodeRune(b[i:])
		h = foldRune(h, c)
		i += n
	}
	return h
}

func foldString(h int32, s string) int32 {
	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			h = h*31 + int32(c)
			i++
			continue
		}
		c, n := utf8.DecodeRuneInString(s[i:])
		h = foldRune(h, c)
		i += n
	}
	return h
}

// foldName folds "prefix:local", or "local" when there is no prefix,
// without concatenating.
func foldName(h int32, n xmltext.Name) int32 {
	if n.Space != "" {
		h = foldString(h, n.Space)
		h = h*31 + ':'
	}
	return foldString(h, n.Local)
}

type state struct {
	pending []byte // coalesced text and CDATA; reused across documents
}

// consume reads path, or parses data when it is not nil (the "mem" mode).
func (st *state) consume(path string, data []byte) (r sample, err error) {
	b := data
	if b == nil {
		if b, err = os.ReadFile(path); err != nil {
			return r, err
		}
	}
	d := xmltext.NewDecoderBytes(b)
	st.pending = st.pending[:0]
	flush := func() {
		s := bytes.TrimSpace(st.pending)
		st.pending = st.pending[:0]
		if len(s) != 0 {
			r.Events++
			r.Checksum = foldBytes(r.Checksum*31+'T', s)
		}
	}
	for {
		t, e := d.ReadToken()
		if e == io.EOF {
			break
		}
		if e != nil {
			return r, e
		}
		switch t.Kind() {
		case xmltext.StartElement:
			flush()
			r.Events++
			r.Checksum = foldName(r.Checksum*31+'S', t.RawName())
			count := 0
			for a := t.Attrs(); a.Next(); {
				if a.Name().Space != xmltext.XMLNSNamespace {
					count++
				}
			}
			r.Checksum = (r.Checksum ^ int32(count)) * 16777619
			for a := t.Attrs(); a.Next(); {
				if a.Name().Space != xmltext.XMLNSNamespace {
					r.Checksum = foldName(r.Checksum, a.RawName())
					r.Checksum = foldBytes(r.Checksum, a.Value())
				}
			}
		case xmltext.EndElement:
			flush()
			r.Events++
			r.Checksum = foldName(r.Checksum*31+'E', t.RawName())
		case xmltext.Text, xmltext.CData:
			st.pending = append(st.pending, t.Text()...)
		}
	}
	flush()
	return r, nil
}

func main() {
	path := os.Args[1]
	w, _ := strconv.Atoi(os.Args[2])
	n, _ := strconv.Atoi(os.Args[3])
	reps, _ := strconv.Atoi(os.Args[4])
	var st state
	var data []byte // optional fifth argument "mem": read the file once, time only the parse
	if len(os.Args) > 5 && os.Args[5] == "mem" {
		var err error
		if data, err = os.ReadFile(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	out := make([]sample, 0, n)
	for i := -w; i < n; i++ {
		start := time.Now()
		var r sample
		for j := 0; j < reps; j++ {
			x, e := st.consume(path, data)
			if e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
			if j > 0 && (x.Events != r.Events || x.Checksum != r.Checksum) {
				panic("checksum changed")
			}
			r = x
		}
		r.Seconds = time.Since(start).Seconds() / float64(reps)
		if i >= 0 {
			out = append(out, r)
		}
	}
	json.NewEncoder(os.Stdout).Encode(struct {
		Samples []sample `json:"samples"`
	}{out})
}
