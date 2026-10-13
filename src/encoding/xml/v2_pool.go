// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.xmlv2

package xml

import (
	"bytes"
	"sync"
	"unsafe"

	"encoding/xml/internal/xmlopts"
)

// Unmarshal, Marshal and MarshalIndent run on pooled state, as the top-level
// calls of encoding/json/v2 do: a Decoder (with its tokenizer and namespace
// map) or an Encoder (with its tokenizer-side buffers and output buffer) is
// taken from a sync.Pool for the duration of one call and put back when it
// ends. NewDecoder and NewEncoder are not pooled; their callers own the
// lifecycle.
//
// Nothing of the call survives in the pool. Before it is put back the state
// drops every reference to the caller's data (the input, the values, the
// Entity map, the CharsetReader and AutoClose settings, the namespace strings),
// resets the public settings that a custom UnmarshalXML or MarshalXML could
// have changed, and discards every buffer larger than maxPooled instead of
// keeping it.
//
// The Decoder or Encoder that Unmarshal and Marshal pass to UnmarshalXML and
// MarshalXML is the pooled one. As with encoding/json/v2, an implementation
// must not keep it, or the tokens it returned, after it returns.

// maxPooled is the largest buffer, in bytes, that a pooled Decoder or Encoder
// keeps. It is the limit encoding/json/v2 uses for its pooled buffers.
const maxPooled = 64 << 10

// maxPooledNS is the most namespace bindings a pooled Decoder's map may have
// held at once. A map keeps its buckets after clear, so one that has grown
// past this is dropped.
const maxPooledNS = 64

// maxPooledStack is the most stack records a pooled Decoder keeps for reuse.
const maxPooledStack = 64

var decoderPool sync.Pool // of *Decoder

// getDecoder returns a Decoder reading data. Its settings are those of
// NewDecoder.
func getDecoder(data []byte) *Decoder {
	d, _ := decoderPool.Get().(*Decoder)
	if d == nil {
		d = NewDecoder(nil)
	}
	if data == nil {
		data = []byte{}
	}
	d.data = data
	return d
}

// putDecoder gives d back to the pool after dropping what the call left in it.
// It returns the size in bytes of the buffers d keeps.
func putDecoder(d *Decoder) (retained int) {
	// The caller's data and what the tokens and errors refer to.
	d.t, d.src, d.data, d.in = nil, nil, nil, nil
	d.nextToken, d.err, d.toClose = nil, nil, Name{}
	d.needClose, d.saving, d.inUnmarshalXML = false, false, false
	// The public settings: UnmarshalXML can change them mid-call.
	d.Strict, d.AutoClose, d.Entity, d.CharsetReader, d.DefaultSpace = true, nil, nil, nil, ""
	d.settings = xmlopts.LegacyDecoder{}
	// The tokenizer.
	if d.xd != nil {
		retained += d.xd.Release()
		d.xd, d.spare = nil, d.xd
	} else if d.spare != nil {
		retained += d.spare.Release()
	}
	// The namespace map.
	if d.nsPeak > maxPooledNS {
		d.ns = make(map[string]string)
	} else {
		clear(d.ns)
	}
	d.nsPeak = 0
	// The stack: a few records are recycled through d.free, the rest is left
	// to the GC.
	var free *stack
	n := 0
	for _, list := range [2]*stack{d.stk, d.free} {
		for s := list; s != nil && n < maxPooledStack; n++ {
			next := s.next
			*s = stack{next: free}
			free = s
			s = next
		}
	}
	d.free, d.stk, d.stkDepth = free, nil, 0
	decoderPool.Put(d)
	return retained
}

// encoderState is a pooled Encoder with the buffer it writes to.
type encoderState struct {
	enc *Encoder
	buf bytes.Buffer
}

var encoderPool sync.Pool // of *encoderState

// getEncoder returns an Encoder that writes to an empty buffer.
func getEncoder() *encoderState {
	st, _ := encoderPool.Get().(*encoderState)
	if st == nil {
		st = new(encoderState)
		st.enc = NewEncoder(&st.buf)
		return st
	}
	st.enc.p.enc.Reset(&st.buf, xmlopts.LegacyOutput(true))
	return st
}

// result returns the output as a slice that the caller owns, nil if empty. A
// buffer that putEncoder will drop (it is over maxPooled) is handed over as it
// is; a pooled one is copied.
func (st *encoderState) result() []byte {
	switch {
	case st.buf.Len() == 0:
		return nil
	case st.buf.Cap() > maxPooled:
		return st.buf.Bytes()
	}
	return bytes.Clone(st.buf.Bytes())
}

// putEncoder gives st back to the pool after dropping what the call left in
// it. It returns the size in bytes of the buffers st keeps.
func putEncoder(st *encoderState) (retained int) {
	p := &st.enc.p
	retained += p.enc.Release()
	clear(p.tags[:cap(p.tags)])
	p.tags, p.closed = p.tags[:0], false
	if st.buf.Cap() > maxPooled {
		st.buf = bytes.Buffer{}
	} else {
		st.buf.Reset()
	}
	retained += st.buf.Cap() + cap(p.tags)*int(unsafe.Sizeof(Name{}))
	encoderPool.Put(st)
	return retained
}
