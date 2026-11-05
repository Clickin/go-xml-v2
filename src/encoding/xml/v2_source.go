// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.xmlv2

package xml

import (
	"reflect"

	"encoding/xml/internal/xmllegacy"
	"encoding/xml/xmltext"
)

// decoderSource is the xmllegacy.TokenSource through which
// encoding/xml/v2 unmarshals from a Decoder: it reads with Token's nesting
// and name space rules, and captures innerxml with the Decoder's own
// recording.
type decoderSource struct {
	d  *Decoder
	k  xmltext.Kind
	st StartElement
	t  Token
	ai int
}

// start makes st the current token, for DecodeElement.
func (s *decoderSource) start(st StartElement) {
	s.k, s.st, s.t, s.ai = xmltext.StartElement, st, nil, 0
}

func (s *decoderSource) Next() (xmltext.Kind, error) {
	k, t, err := s.d.token(&s.st)
	if k == rawNone {
		s.k = xmltext.Invalid
		return s.k, err
	}
	s.t, s.ai = t, 0
	switch k {
	case rawStart:
		s.k = xmltext.StartElement
	case rawEnd:
		s.k = xmltext.EndElement
	default:
		switch t.(type) {
		case CharData:
			s.k = xmltext.Text
		case Comment:
			s.k = xmltext.Comment
		case ProcInst:
			s.k = xmltext.ProcInst
		default:
			s.k = xmltext.Directive
		}
	}
	return s.k, nil
}

func (s *decoderSource) Name() xmltext.Name { return xmltext.Name(s.st.Name) }

func (s *decoderSource) NextAttr() (xmltext.Name, []byte, bool) {
	if s.ai >= len(s.st.Attr) {
		return xmltext.Name{}, nil, false
	}
	a := &s.st.Attr[s.ai]
	s.ai++
	return xmltext.Name(a.Name), unsafeBytes(a.Value), true
}

// StableAttrs reports true: attribute values are strings of their own.
func (s *decoderSource) StableAttrs() bool { return true }

func (s *decoderSource) Text() []byte {
	switch t := s.t.(type) {
	case CharData:
		return t
	case Comment:
		return t
	}
	return nil
}

func (s *decoderSource) Offset() int64                { return s.d.InputOffset() }
func (s *decoderSource) PeekKind() xmltext.Kind       { return xmltext.Invalid }
func (s *decoderSource) Depth() int                   { return s.d.stkDepth }
func (s *decoderSource) Capturing() bool              { return s.d.saving }
func (s *decoderSource) StartCapture()                { s.d.startSaved() }
func (s *decoderSource) CaptureLen() int              { return s.d.savedOffset() }
func (s *decoderSource) Captured(from, to int) []byte { return s.d.savedBytes(from, to) }
func (s *decoderSource) StopCapture()                 { s.d.stopSaved() }

// decodeElement unmarshals into val with encoding/xml/v2, which implements
// the semantics of this package for it.
func (d *Decoder) decodeElement(val reflect.Value, start *StartElement) error {
	var src *decoderSource
	if d.dsrcBusy {
		src = new(decoderSource) // DecodeElement within UnmarshalXML
	} else {
		if d.dsrc == nil {
			d.dsrc = new(decoderSource) // kept by a pooled Decoder
		}
		src, d.dsrcBusy = d.dsrc, true
	}
	*src = decoderSource{d: d}
	if start != nil {
		src.start(*start)
	}
	err := xmllegacy.Unmarshal(src, val, start != nil)
	if src == d.dsrc {
		*src, d.dsrcBusy = decoderSource{}, false
	}
	return err
}

// setUnmarshalHooks sets the unmarshal parts of the hooks of
// encoding/xml/v2.
func setUnmarshalHooks(h *xmllegacy.Hooks) {
	h.UnmarshalXML = func(owner xmllegacy.TokenSource, v reflect.Value) (bool, error) {
		m, ok := methodOf[Unmarshaler](v)
		if !ok {
			return false, nil
		}
		s := owner.(*decoderSource)
		start := s.st
		return true, s.d.unmarshalInterface(m, &start)
	}
	h.UnmarshalXMLAttr = func(v reflect.Value, a xmltext.Attr) (bool, error) {
		m, ok := methodOf[UnmarshalerAttr](v)
		if !ok {
			return false, nil
		}
		return true, m.UnmarshalXMLAttr(Attr{Name(a.Name), a.Value})
	}
	h.UnmarshalError = func(msg string) error { return UnmarshalError(msg) }
	h.ErrUnmarshalDepth = errUnmarshalDepth
}

// methodOf returns v, or else its address, as a T, as encoding/xml looks
// for the methods it calls when unmarshaling.
func methodOf[T any](v reflect.Value) (T, bool) {
	if v.CanInterface() {
		// A method on a non-pointer receiver is likely to be incorrect,
		// but encoding/xml does what it is told.
		if m, ok := reflect.TypeAssert[T](v); ok {
			return m, true
		}
	}
	if v.CanAddr() {
		if pv := v.Addr(); pv.CanInterface() {
			return reflect.TypeAssert[T](pv)
		}
	}
	var zero T
	return zero, false
}
