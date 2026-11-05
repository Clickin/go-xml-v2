// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.xmlv2

package xml

import (
	"encoding"
	"errors"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"encoding/xml/internal/xmllegacy"
	"encoding/xml/xmltext"
)

// This file implements the unmarshaling of encoding/xml (version 1),
// reading tokens through a TokenSource over its Decoder, which applies its
// own nesting and name space rules (Strict, AutoClose). As for marshaling
// (arshal_legacy.go), it shares the field rules and scratch buffers of this
// package and follows encoding/xml in every detail, its errors included.

const (
	maxLegacyDepth     = 10000
	maxLegacyDepthWasm = 5000 // go.dev/issue/56498
)

var legacyStatePool sync.Pool // of *unmarshalState

func legacyUnmarshal(src xmllegacy.TokenSource, v reflect.Value, started bool) error {
	us, _ := legacyStatePool.Get().(*unmarshalState)
	if us == nil {
		us = new(unmarshalState)
	}
	us.src, us.legacy, us.hooks = src, true, xmllegacy.V1
	err := us.legacyRead(v, started)
	bufs := us.bufs
	clear(bufs[:cap(bufs)])
	*us = unmarshalState{bufs: bufs[:0]}
	legacyStatePool.Put(us)
	return err
}

func (us *unmarshalState) legacyRead(v reflect.Value, started bool) error {
	if !started {
		if us.src.Depth() > maxLegacyDepth || runtime.GOARCH == "wasm" && us.src.Depth() > maxLegacyDepthWasm {
			return us.hooks.ErrUnmarshalDepth
		}
		for {
			k, err := us.src.Next()
			if err != nil {
				return err
			}
			if k == xmltext.StartElement {
				break
			}
		}
	}
	return us.legacyValue(v, started)
}

// legacyValue reads the element whose start is the current token into
// val, as encoding/xml's Decoder.unmarshal does.
// The depth limit is checked unless check is false: encoding/xml checks it
// before reading the start of a top-level element, and after reading it
// for the elements inside.
func (us *unmarshalState) legacyValue(val reflect.Value, check bool) error {
	src, h := us.src, us.hooks
	if d := src.Depth(); check && (d > maxLegacyDepth || runtime.GOARCH == "wasm" && d > maxLegacyDepthWasm) {
		return h.ErrUnmarshalDepth
	}

	// Load value from interface, but only if the result will be
	// usefully addressable.
	if val.Kind() == reflect.Interface && !val.IsNil() {
		if e := val.Elem(); e.Kind() == reflect.Pointer && !e.IsNil() {
			val = e
		}
	}
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		val = val.Elem()
	}

	if ok, err := h.UnmarshalXML(src, val); ok {
		return err
	}
	if m, ok := legacyTextUnmarshaler(val); ok {
		// The character data of the element, not of its children.
		var buf []byte
		depth := 1
		for depth > 0 {
			k, err := src.Next()
			if err != nil {
				return err
			}
			switch k {
			case xmltext.Text, xmltext.CData:
				if depth == 1 {
					buf = append(buf, src.Text()...)
				}
			case xmltext.StartElement:
				depth++
			case xmltext.EndElement:
				depth--
			}
		}
		return m.UnmarshalText(buf)
	}

	var (
		data, comment []byte
		saveData      reflect.Value
		saveComment   reflect.Value
		saveXML       reflect.Value
		saveXMLIndex  int
		saveXMLData   []byte
		saveAny       reflect.Value
		sv            reflect.Value
		sf            *structFields
	)
	switch v := val; v.Kind() {
	default:
		return errors.New("unknown type " + v.Type().String())

	case reflect.Interface:
		return us.legacySkip()

	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			saveData = v
			break
		}
		n := v.Len()
		v.Grow(1)
		v.SetLen(n + 1)
		if err := us.legacyValue(v.Index(n), true); err != nil {
			v.SetLen(n)
			return err
		}
		return nil

	case reflect.Bool, reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.String:
		saveData = v

	case reflect.Struct:
		typ := v.Type()
		if typ == h.NameType {
			setName(v, src.Name())
			break
		}
		sv = v
		var err error
		if sf, err = cachedStructFields(typ, true); err != nil {
			return err
		}
		name := src.Name()
		if xn := sf.xmlname; xn != nil {
			if xn.name.Local != "" && xn.name.Local != name.Local {
				return h.UnmarshalError("expected element type <" + xn.name.Local + "> but have <" + name.Local + ">")
			}
			if xn.name.Space != "" && xn.name.Space != name.Space {
				e := "expected element <" + xn.name.Local + "> in name space " + xn.name.Space + " but have "
				if name.Space == "" {
					e += "no name space"
				} else {
					e += name.Space
				}
				return h.UnmarshalError(e)
			}
			fv, _ := fieldValue(addressableValue{Value: sv}, xn.index, true)
			if fv.Type() == h.NameType {
				setName(fv.Value, name)
			}
		}

		stable := src.StableAttrs()
		for {
			an, value, ok := src.NextAttr()
			if !ok {
				break
			}
			var s string
			if stable {
				s = unsafe.String(unsafe.SliceData(value), len(value))
			} else {
				s = string(value)
			}
			a := xmltext.Attr{Name: an, Value: s}
			handled, anyAttr := false, -1
			for i := range sf.list {
				f := &sf.list[i]
				switch f.flags & fMode {
				case fAttr:
					fv, _ := fieldValue(addressableValue{Value: sv}, f.index, true)
					if an.Local == f.name.Local && (f.name.Space == "" || f.name.Space == an.Space) {
						if err := us.legacyAttr(fv.Value, a); err != nil {
							return err
						}
						handled = true
					}
				case fAny | fAttr:
					if anyAttr == -1 {
						anyAttr = i
					}
				}
			}
			if !handled && anyAttr >= 0 {
				fv, _ := fieldValue(addressableValue{Value: sv}, sf.list[anyAttr].index, true)
				if err := us.legacyAttr(fv.Value, a); err != nil {
					return err
				}
			}
		}

		for i := range sf.list {
			f := &sf.list[i]
			switch f.flags & fMode {
			case fCDATA, fCharData:
				if !saveData.IsValid() {
					fv, _ := fieldValue(addressableValue{Value: sv}, f.index, true)
					saveData = fv.Value
				}
			case fComment:
				if !saveComment.IsValid() {
					fv, _ := fieldValue(addressableValue{Value: sv}, f.index, true)
					saveComment = fv.Value
				}
			case fAny, fAny | fElement:
				if !saveAny.IsValid() {
					fv, _ := fieldValue(addressableValue{Value: sv}, f.index, true)
					saveAny = fv.Value
				}
			case fInnerXML:
				if !saveXML.IsValid() {
					fv, _ := fieldValue(addressableValue{Value: sv}, f.index, true)
					saveXML = fv.Value
					if !src.Capturing() {
						saveXMLIndex = 0
						src.StartCapture()
					} else {
						saveXMLIndex = src.CaptureLen()
					}
				}
			}
		}
	}

	// Read through the end element, processing children along the way.
	collect := saveData.IsValid()
	if collect {
		data = us.takeBuf()
		defer func() { us.putBuf(data) }()
	}
	for {
		var savedOffset int
		if saveXML.IsValid() {
			savedOffset = src.CaptureLen()
		}
		k, err := src.Next()
		if err != nil {
			return err
		}
		switch k {
		case xmltext.StartElement:
			consumed := false
			if sv.IsValid() {
				if consumed, err = us.legacyPath(sf, sv, nil); err != nil {
					return err
				}
				if !consumed && saveAny.IsValid() {
					consumed = true
					if err := us.legacyValue(saveAny, true); err != nil {
						return err
					}
				}
			}
			if !consumed {
				if err := us.legacySkip(); err != nil {
					return err
				}
			}
			continue
		case xmltext.EndElement:
			if saveXML.IsValid() {
				if savedOffset > saveXMLIndex {
					saveXMLData = src.Captured(saveXMLIndex, savedOffset)
				}
				if saveXMLIndex == 0 {
					src.StopCapture()
				}
			}
		case xmltext.Text, xmltext.CData:
			if collect {
				data = append(data, src.Text()...)
			}
			continue
		case xmltext.Comment:
			if saveComment.IsValid() {
				comment = append(comment, src.Text()...)
			}
			continue
		default:
			continue
		}
		break
	}

	if saveData.IsValid() {
		if m, ok := legacyTextUnmarshaler(saveData); ok {
			text := data
			if len(text) == 0 {
				text = nil // as encoding/xml, which collects into a nil slice
			}
			if err := m.UnmarshalText(text); err != nil {
				return err
			}
			saveData = reflect.Value{}
		}
	}
	if err := legacyCopyValue(saveData, data, true); err != nil {
		return err
	}

	switch t := saveComment; t.Kind() {
	case reflect.String:
		t.SetString(string(comment))
	case reflect.Slice:
		t.Set(reflect.ValueOf(comment))
	}

	switch t := saveXML; t.Kind() {
	case reflect.String:
		t.SetString(string(saveXMLData))
	case reflect.Slice:
		if t.Type().Elem().Kind() == reflect.Uint8 {
			t.Set(reflect.ValueOf(saveXMLData))
		}
	}
	return nil
}

// legacyTextUnmarshaler returns val, or else its address, as an
// encoding.TextUnmarshaler, as encoding/xml looks for one.
func legacyTextUnmarshaler(val reflect.Value) (encoding.TextUnmarshaler, bool) {
	if val.CanInterface() {
		if m, ok := reflect.TypeAssert[encoding.TextUnmarshaler](val); ok {
			return m, true
		}
	}
	if val.CanAddr() {
		if pv := val.Addr(); pv.CanInterface() {
			return reflect.TypeAssert[encoding.TextUnmarshaler](pv)
		}
	}
	return nil, false
}

// legacyAttr sets val from attribute a, as encoding/xml's
// Decoder.unmarshalAttr does.
func (us *unmarshalState) legacyAttr(val reflect.Value, a xmltext.Attr) error {
	h := us.hooks
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		val = val.Elem()
	}
	if ok, err := h.UnmarshalXMLAttr(val, a); ok {
		return err
	}
	if m, ok := legacyTextUnmarshaler(val); ok {
		return m.UnmarshalText([]byte(a.Value))
	}
	if val.Kind() == reflect.Slice && val.Type().Elem().Kind() != reflect.Uint8 {
		n := val.Len()
		val.Grow(1)
		val.SetLen(n + 1)
		if err := us.legacyAttr(val.Index(n), a); err != nil {
			val.SetLen(n)
			return err
		}
		return nil
	}
	if val.Type() == h.AttrType {
		setAttr(val, a)
		return nil
	}
	// A string field takes the value as is and only a []byte field gets
	// its own copy; other kinds are parsed from a view of it.
	v := val
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(a.Value)
		return nil
	case reflect.Slice:
		return legacyCopyValue(val, []byte(a.Value), false)
	}
	return legacyCopyValue(val, unsafeBytes(a.Value), false)
}

// legacyCopyValue is encoding/xml's copyValue. If scratch, src is a
// scratch buffer, which a []byte field gets a copy of.
func legacyCopyValue(dst reflect.Value, src []byte, scratch bool) error {
	dst0 := dst
	if dst.Kind() == reflect.Pointer {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		dst = dst.Elem()
	}
	switch dst.Kind() {
	case reflect.Invalid:
		// Probably a comment.
	default:
		return errors.New("cannot unmarshal into " + dst0.Type().String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if len(src) == 0 {
			dst.SetInt(0)
			return nil
		}
		v, err := strconv.ParseInt(strings.TrimSpace(string(src)), 10, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if len(src) == 0 {
			dst.SetUint(0)
			return nil
		}
		v, err := strconv.ParseUint(strings.TrimSpace(string(src)), 10, dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetUint(v)
	case reflect.Float32, reflect.Float64:
		if len(src) == 0 {
			dst.SetFloat(0)
			return nil
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(src)), dst.Type().Bits())
		if err != nil {
			return err
		}
		dst.SetFloat(v)
	case reflect.Bool:
		if len(src) == 0 {
			dst.SetBool(false)
			return nil
		}
		v, err := strconv.ParseBool(strings.TrimSpace(string(src)))
		if err != nil {
			return err
		}
		dst.SetBool(v)
	case reflect.String:
		dst.SetString(string(src))
	case reflect.Slice:
		switch {
		case len(src) == 0:
			src = []byte{} // non-nil to flag presence
		case scratch:
			src = append([]byte(nil), src...)
		}
		dst.SetBytes(src)
	}
	return nil
}

// legacyPath reads the element whose start is the current token into the
// field of sf it belongs to at path parents, as encoding/xml's
// Decoder.unmarshalPath does. It reports whether it read the element.
func (us *unmarshalState) legacyPath(sf *structFields, sv reflect.Value, parents []string) (consumed bool, err error) {
	src := us.src
	name := src.Name()
	recurse := false
loop:
	for i := range sf.list {
		f := &sf.list[i]
		if f.flags&fElement == 0 || len(f.parents) < len(parents) || f.name.Space != "" && f.name.Space != name.Space {
			continue
		}
		for j := range parents {
			if parents[j] != f.parents[j] {
				continue loop
			}
		}
		if len(f.parents) == len(parents) && f.name.Local == name.Local {
			fv, _ := fieldValue(addressableValue{Value: sv}, f.index, true)
			return true, us.legacyValue(fv.Value, true)
		}
		if len(f.parents) > len(parents) && f.parents[len(parents)] == name.Local {
			recurse = true
			parents = f.parents[:len(parents)+1]
			break
		}
	}
	if !recurse {
		return false, nil
	}
	for {
		k, err := src.Next()
		if err != nil {
			return true, err
		}
		switch k {
		case xmltext.StartElement:
			consumed2, err := us.legacyPath(sf, sv, parents)
			if err != nil {
				return true, err
			}
			if !consumed2 {
				if err := us.legacySkip(); err != nil {
					return true, err
				}
			}
		case xmltext.EndElement:
			return true, nil
		}
	}
}

// legacySkip reads through the end of the element whose start is the
// current token, as encoding/xml's Decoder.Skip does.
func (us *unmarshalState) legacySkip() error {
	var depth int64
	for {
		k, err := us.src.Next()
		if err != nil {
			return err
		}
		switch k {
		case xmltext.StartElement:
			depth++
		case xmltext.EndElement:
			if depth == 0 {
				return nil
			}
			depth--
		}
	}
}
