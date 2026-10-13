//go:build goexperiment.xmlv2

package xmltext

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	legacy "encoding/xml/internal/legacyxml"
)

var charsetInputModes = []string{"bytes", "buffer", "reader", "chunk1", "chunk2", "chunk3", "chunk7", "chunk16", "chunk4096"}

func charsetInput(in, mode string) io.Reader {
	switch mode {
	case "bytes":
		return bytes.NewReader([]byte(in))
	case "buffer":
		return bytes.NewBufferString(in)
	case "reader":
		return strings.NewReader(in)
	default:
		var n int
		fmt.Sscanf(mode, "chunk%d", &n)
		return &charsetChunkReader{r: strings.NewReader(in), n: n}
	}
}

func charsetDecoder(in, mode string, opts ...Options) *Decoder {
	if mode == "bytes" {
		return NewDecoderBytes([]byte(in), opts...)
	}
	return NewDecoder(charsetInput(in, mode), opts...)
}

type charsetChunkReader struct {
	r io.Reader
	n int
}

func (r *charsetChunkReader) Read(p []byte) (int, error) {
	return r.r.Read(p[:min(len(p), r.n)])
}

// A small actual Latin-1 converter keeps the tests independent of charset
// packages while exercising UTF-8 expansion across every source chunk size.
type charsetLatin1Reader struct {
	r       io.ByteReader
	pending byte
}

func (r *charsetLatin1Reader) ReadByte() (byte, error) {
	if r.pending != 0 {
		b := r.pending
		r.pending = 0
		return b, nil
	}
	b, err := r.r.ReadByte()
	if err != nil || b < 0x80 {
		return b, err
	}
	r.pending = 0x80 | b&0x3f
	return 0xc0 | b>>6, nil
}

func (r *charsetLatin1Reader) Read(p []byte) (int, error) {
	for i := range p {
		b, err := r.ReadByte()
		if err != nil {
			return i, err
		}
		p[i] = b
	}
	return len(p), nil
}

type charsetByteOnlyReader struct{ io.ByteReader }

func (r charsetByteOnlyReader) Read([]byte) (int, error) {
	panic("converter Read called instead of ReadByte")
}

func charsetConverter(t *testing.T, mode string, labels *[]string, remainder *string) func(string, io.Reader) (io.Reader, error) {
	t.Helper()
	return func(label string, input io.Reader) (io.Reader, error) {
		*labels = append(*labels, label)
		rb, ok := input.(io.ByteReader)
		if !ok {
			t.Fatal("CharsetReader input does not implement io.ByteReader")
		}
		switch mode {
		case "eager":
			data, err := io.ReadAll(input)
			if err != nil {
				return nil, err
			}
			*remainder = string(data)
			converted, err := io.ReadAll(&charsetLatin1Reader{r: bytes.NewReader(data)})
			return strings.NewReader(string(converted)), err
		case "byte":
			return charsetByteOnlyReader{&charsetLatin1Reader{r: rb}}, nil
		default:
			// Hide ReadByte so the decoder must also support ordinary readers.
			return struct{ io.Reader }{&charsetLatin1Reader{r: rb}}, nil
		}
	}
}

func charsetLegacyTokens(d *legacy.Decoder) ([]tok, error) {
	var out []tok
	for {
		tk, err := d.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		var x tok
		switch v := tk.(type) {
		case legacy.StartElement:
			x.kind, x.name = StartElement, Name{v.Name.Space, v.Name.Local}
			for _, a := range v.Attr {
				x.attrs = append(x.attrs, Attr{Name{a.Name.Space, a.Name.Local}, a.Value})
			}
		case legacy.EndElement:
			x.kind, x.name = EndElement, Name{v.Name.Space, v.Name.Local}
		case legacy.CharData:
			x.kind, x.text = Text, string(v)
		case legacy.Comment:
			x.kind, x.text = Comment, string(v)
		case legacy.ProcInst:
			x.kind, x.name, x.text = ProcInst, Name{Local: v.Target}, string(v.Inst)
		case legacy.Directive:
			x.kind, x.text = Directive, string(v)
		}
		out = append(out, x)
	}
}

func TestCharsetReaderConversion(t *testing.T) {
	// Longer than the decoder window: both buffered read-ahead and the unread
	// underlying reader must be converted, without losing or duplicating bytes.
	body := `<r a="caf` + "\xe9" + `"><!--caf` + "\xe9" + `--><?note caf` + "\xe9" + `?>` + strings.Repeat("caf\xe9", 20000) + `<b/></r>`
	for _, label := range []string{"ISO-8859-1", "x-Latin1-MiXeD"} {
		decl := `<?xml version="1.0" encoding="` + label + `"?>`
		for _, inputMode := range charsetInputModes {
			for _, conversion := range []string{"reader", "byte", "eager"} {
				t.Run(label+"/"+inputMode+"/"+conversion, func(t *testing.T) {
					var gotLabels, wantLabels []string
					var gotRemainder, wantRemainder string
					d := charsetDecoder(decl+body, inputMode, WithCharsetReader(charsetConverter(t, conversion, &gotLabels, &gotRemainder)))
					ref := legacy.NewDecoder(charsetInput(decl+body, inputMode))
					ref.CharsetReader = charsetConverter(t, conversion, &wantLabels, &wantRemainder)
					first, err := d.ReadToken()
					if err != nil {
						t.Fatal(err)
					}
					refFirst, err := ref.Token()
					if err != nil {
						t.Fatal(err)
					}
					pi := refFirst.(legacy.ProcInst)
					if first.Kind() != ProcInst || first.Name().Local != pi.Target || string(first.Text()) != string(pi.Inst) {
						t.Fatalf("declaration view corrupted: %v %v %q; want %v", first.Kind(), first.Name(), first.Text(), pi)
					}
					if !reflect.DeepEqual(gotLabels, []string{label}) || !reflect.DeepEqual(gotLabels, wantLabels) {
						t.Fatalf("callback timing/labels: %v, legacy %v", gotLabels, wantLabels)
					}
					if conversion == "eager" && (gotRemainder != body || gotRemainder != wantRemainder) {
						t.Fatal("callback did not receive exactly the unread declaration remainder")
					}
					got, err := readAll(t, d)
					want, refErr := charsetLegacyTokens(ref)
					if err != nil || refErr != nil {
						t.Fatalf("conversion errors: %v, legacy %v", err, refErr)
					}
					if !reflect.DeepEqual(normalize(got), normalize(want)) {
						t.Fatal("converted tokens differ from legacy")
					}
					if len(gotLabels) != 1 {
						t.Fatalf("callback called %d times", len(gotLabels))
					}
				})
			}
		}
	}
}

func TestCharsetReaderUTF8AndNoEncoding(t *testing.T) {
	// The default profile rejects the last two (empty EncName, reserved target,
	// no version); they only match encoding/xml under Permissive.
	for _, decl := range []string{
		`<?xml version="1.0"?>`,
		`<?xml version="1.0" encoding="utf-8"?>`,
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<?xml version="1.0" encoding="UtF-8"?>`,
		`<?xml version="1.0" encoding=""?>`,
		`<?XML encoding="not-a-charset"?>`,
	} {
		permissive := strings.Contains(decl, `encoding=""`) || strings.HasPrefix(decl, "<?XML")
		for _, mode := range charsetInputModes {
			t.Run(decl+"/"+mode, func(t *testing.T) {
				called := 0
				cb := func(string, io.Reader) (io.Reader, error) {
					called++
					return nil, errors.New("unexpected conversion")
				}
				in := decl + `<r>café</r>`
				got, err := readAll(t, charsetDecoder(in, mode, WithCharsetReader(cb), Permissive(permissive)))
				ref := legacy.NewDecoder(charsetInput(in, mode))
				ref.CharsetReader = cb
				want, refErr := charsetLegacyTokens(ref)
				if err != nil || refErr != nil || called != 0 || !reflect.DeepEqual(normalize(got), normalize(want)) {
					t.Fatalf("tokens/error/callbacks differ: %v %v %d", err, refErr, called)
				}
			})
		}
	}
}

func TestCharsetReaderErrors(t *testing.T) {
	const in = `<?xml version="1.0" encoding="x-MiXeD"?><r/>`
	sentinel := errors.New("converter unavailable")
	for _, mode := range charsetInputModes {
		t.Run(mode, func(t *testing.T) {
			called := 0
			cb := func(label string, input io.Reader) (io.Reader, error) {
				called++
				if label != "x-MiXeD" {
					t.Fatalf("label %q", label)
				}
				b := make([]byte, 1)
				if _, err := io.ReadFull(input, b); err != nil || b[0] != '<' {
					t.Fatalf("unread remainder: %q %v", b, err)
				}
				// A non-nil reader does not override a callback error.
				return strings.NewReader("ignored"), sentinel
			}
			d := charsetDecoder(in, mode, WithCharsetReader(cb))
			_, err := d.ReadToken()
			ref := legacy.NewDecoder(charsetInput(in, mode))
			ref.CharsetReader = cb
			_, refErr := ref.Token()
			if err == nil || refErr == nil || err.Error() != refErr.Error() || !errors.Is(err, sentinel) || !errors.Is(refErr, sentinel) || called != 2 {
				t.Fatalf("callback errors/timing: %v, legacy %v, calls %d", err, refErr, called)
			}
			_, again := d.ReadToken()
			if again != err || called != 2 {
				t.Fatal("callback error is not sticky")
			}
		})
	}
	for _, opts := range [][]Options{nil, {WithCharsetReader(nil)}} {
		_, err := NewDecoderBytes([]byte(in), opts...).ReadToken()
		if err == nil || !strings.Contains(err.Error(), "unsupported encoding: x-MiXeD") {
			t.Fatalf("missing converter must preserve unsupported-encoding error: %v", err)
		}
	}
	called := false
	d := NewDecoderBytes([]byte(`<?xml version="1.1" encoding="x-MiXeD"?><r/>`), WithCharsetReader(func(string, io.Reader) (io.Reader, error) {
		called = true
		return nil, nil
	}))
	if _, err := d.ReadToken(); err == nil || called {
		t.Fatalf("version must be checked before invoking converter: %v, called %v", err, called)
	}
}

func charsetPanic(fn func()) (value any) {
	defer func() { value = recover() }()
	fn()
	return nil
}

func TestCharsetReaderNilPanics(t *testing.T) {
	const in = `<?xml version="1.0" encoding="x-MiXeD"?><r/>`
	cb := func(string, io.Reader) (io.Reader, error) { return nil, nil }
	for _, mode := range charsetInputModes {
		t.Run(mode, func(t *testing.T) {
			d := charsetDecoder(in, mode, WithCharsetReader(cb))
			got := charsetPanic(func() { d.ReadToken() })
			ref := legacy.NewDecoder(charsetInput(in, mode))
			ref.CharsetReader = cb
			want := charsetPanic(func() { ref.Token() })
			if got == nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("panic %v, legacy %v", got, want)
			}
		})
	}
}

// Report a terminal error together with the final bytes, so read-ahead must
// preserve it even when the declaration and complete body share one window.
type charsetTerminalReader struct {
	r   *strings.Reader
	err error
	n   int
}

func (r *charsetTerminalReader) Read(p []byte) (int, error) {
	if r.n > 0 {
		p = p[:min(len(p), r.n)]
	}
	n, err := r.r.Read(p)
	if r.r.Len() == 0 {
		err = r.err
	}
	return n, err
}

func TestCharsetReaderDownstreamErrors(t *testing.T) {
	const decl = `<?xml version="1.0" encoding="latin-1"?>`
	const body = "<r>caf\xe9</r>"
	sentinel := errors.New("downstream read failed")
	for _, n := range []int{1, 2, 3, 7, 16, 4096} {
		for _, where := range []string{"source", "converted"} {
			t.Run(fmt.Sprintf("%s/chunk%d", where, n), func(t *testing.T) {
				makeReader := func() io.Reader {
					if where == "source" {
						return &charsetTerminalReader{r: strings.NewReader(decl + body), err: sentinel, n: n}
					}
					return &charsetChunkReader{r: strings.NewReader(decl + body), n: n}
				}
				cb := func(_ string, input io.Reader) (io.Reader, error) {
					converted := &charsetLatin1Reader{r: bufio.NewReader(input)}
					if where == "source" {
						return converted, nil
					}
					data, err := io.ReadAll(converted)
					if err != nil {
						return nil, err
					}
					return &charsetTerminalReader{r: strings.NewReader(string(data)), err: sentinel, n: n}, nil
				}
				d := NewDecoder(makeReader(), WithCharsetReader(cb))
				got, err := readAll(t, d)
				ref := legacy.NewDecoder(makeReader())
				ref.CharsetReader = cb
				want, refErr := charsetLegacyTokens(ref)
				if !errors.Is(err, sentinel) || !errors.Is(refErr, sentinel) || err.Error() != refErr.Error() || !reflect.DeepEqual(normalize(got), normalize(want)) {
					t.Fatalf("downstream tokens/errors differ: %v, legacy %v", err, refErr)
				}
				if _, again := d.ReadToken(); again != err {
					t.Fatalf("read error not sticky: %v, then %v", err, again)
				}
			})
		}
	}
}

func TestCharsetReaderMayConsumeSourceError(t *testing.T) {
	const decl = `<?xml version="1.0" encoding="replacement"?>`
	const body = `<r>replacement</r>`
	sentinel := errors.New("source ended with an error")
	makeReader := func() io.Reader {
		return &charsetTerminalReader{r: strings.NewReader(decl + body), err: sentinel}
	}
	cb := func(_ string, input io.Reader) (io.Reader, error) {
		data, err := io.ReadAll(input)
		if string(data) != body || !errors.Is(err, sentinel) {
			t.Fatalf("callback remainder/error: %q, %v", data, err)
		}
		return strings.NewReader(body), nil
	}
	got, err := readAll(t, NewDecoder(makeReader(), WithCharsetReader(cb)))
	ref := legacy.NewDecoder(makeReader())
	ref.CharsetReader = cb
	want, refErr := charsetLegacyTokens(ref)
	if err != nil || refErr != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("old source error survived conversion: %v, legacy %v", err, refErr)
	}
}

func TestCharsetReaderRepeatedDeclarations(t *testing.T) {
	// encoding/xml (and so Permissive) switches at every declaration; the default
	// profile only honors one at the start of the document.
	const decl = `<?xml version="1.0" encoding="First"?>`
	const body = `<r><?xml encoding="SeCoNd"?>ok</r>`
	for _, mode := range charsetInputModes {
		for _, capture := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/capture%v", mode, capture), func(t *testing.T) {
				var gotLabels, wantLabels []string
				cb := func(labels *[]string) func(string, io.Reader) (io.Reader, error) {
					return func(label string, input io.Reader) (io.Reader, error) {
						*labels = append(*labels, label)
						return input, nil
					}
				}
				d := charsetDecoder(decl+body, mode, WithCharsetReader(cb(&gotLabels)), Permissive(true))
				ref := legacy.NewDecoder(charsetInput(decl+body, mode))
				ref.CharsetReader = cb(&wantLabels)
				if capture {
					if _, err := d.ReadToken(); err != nil {
						t.Fatal(err)
					}
					raw, err := d.ReadElement()
					if err != nil || string(raw) != body {
						t.Fatalf("capture across charset switch: %q %v", raw, err)
					}
					if _, err := charsetLegacyTokens(ref); err != nil {
						t.Fatal(err)
					}
				} else {
					got, err := readAll(t, d)
					want, refErr := charsetLegacyTokens(ref)
					if err != nil || refErr != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("repeated declaration tokens/errors: %v, legacy %v", err, refErr)
					}
				}
				if !reflect.DeepEqual(gotLabels, []string{"First", "SeCoNd"}) || !reflect.DeepEqual(gotLabels, wantLabels) {
					t.Fatalf("declaration callbacks: %v, legacy %v", gotLabels, wantLabels)
				}
			})
		}
	}
}

func TestCharsetReaderResetAndCallerBytes(t *testing.T) {
	const in = `<?xml version="1.0" encoding="latin-1"?>` + "<r>caf\xe9</r>"
	for _, inputMode := range []string{"bytes", "buffer", "reader"} {
		for _, afterConversion := range []bool{false, true} {
			for _, resetMode := range []string{"bytes", "buffer", "reader"} {
				t.Run(fmt.Sprintf("%s/converted%v/reset%s", inputMode, afterConversion, resetMode), func(t *testing.T) {
					original := []byte(in)
					var labels []string
					var remainder string
					cb := charsetConverter(t, "byte", &labels, &remainder)
					var d *Decoder
					switch inputMode {
					case "bytes":
						d = NewDecoderBytes(original, WithCharsetReader(cb))
					case "buffer":
						d = NewDecoder(bytes.NewBuffer(original), WithCharsetReader(cb))
					default:
						d = NewDecoder(bytes.NewReader(original), WithCharsetReader(cb))
					}
					if _, err := d.ReadToken(); err != nil {
						t.Fatal(err)
					}
					if afterConversion {
						if _, err := readAll(t, d); err != nil {
							t.Fatal(err)
						}
					}
					if resetMode == "bytes" {
						d.ResetBytes([]byte(`<new>unconverted</new>`))
					} else {
						d.Reset(charsetInput(`<new>unconverted</new>`, resetMode))
					}
					got, err := readAll(t, d)
					want, refErr := stdTokens(t, `<new>unconverted</new>`)
					if err != nil || refErr != nil || !reflect.DeepEqual(got, want) || len(labels) != 1 {
						t.Fatalf("Reset did not discard conversion state: %v, labels %v", err, labels)
					}
					d.ResetBytes([]byte(in))
					if _, err := d.ReadToken(); err == nil || len(labels) != 1 {
						t.Fatal("Reset retained the old charset option")
					}
					d.Reset(strings.NewReader(in), WithCharsetReader(cb))
					if _, err := readAll(t, d); err != nil || len(labels) != 2 {
						t.Fatalf("Reset did not accept a new charset option: %v, labels %v", err, labels)
					}
					if string(original) != in {
						t.Fatal("charset conversion or Reset modified caller-owned bytes")
					}
				})
			}
		}
	}
}

func TestCharsetReaderNextOperations(t *testing.T) {
	const in = `<?xml version="1.0" encoding="latin-1"?>` + "<r>caf\xe9</r>"
	for _, operation := range []string{"peek", "element", "skip"} {
		t.Run(operation, func(t *testing.T) {
			var labels []string
			var remainder string
			d := NewDecoderBytes([]byte(in), WithCharsetReader(charsetConverter(t, "byte", &labels, &remainder)))
			pi, err := d.ReadToken()
			if err != nil || pi.Name().Local != "xml" || string(pi.Text()) != `version="1.0" encoding="latin-1"` {
				t.Fatalf("declaration: %v %v", pi, err)
			}
			switch operation {
			case "peek":
				if kind := d.PeekKind(); kind != StartElement {
					t.Fatalf("PeekKind = %v", kind)
				}
				got, err := readAll(t, d)
				want, _ := stdTokens(t, "<r>café</r>")
				if err != nil || !reflect.DeepEqual(normalize(got), normalize(want)) {
					t.Fatalf("tokens after PeekKind differ: %v", err)
				}
			case "element":
				raw, err := d.ReadElement()
				if err != nil || string(raw) != "<r>café</r>" {
					t.Fatalf("ReadElement = %q, %v", raw, err)
				}
			case "skip":
				if err := d.SkipElement(); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := d.ReadToken(); err != io.EOF {
				t.Fatalf("end of input: %v", err)
			}
		})
	}
}
