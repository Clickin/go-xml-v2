// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.xmlv2

package xmltext

import (
	"reflect"
	"unsafe"
)

// The v1 lexer makes Strict/Entity decisions as input is consumed, whereas the
// boundary engine reads a complete token first. Record only Reader-induced
// changes, then replay them at attribute/reference decisions, not per byte.
type legacySetting struct {
	loose    bool
	entities map[string]string // the user's map itself, not a copy
	id       unsafe.Pointer    // identity of entities, to see a replacement without looking inside
}

type legacyChange struct {
	at int64
	legacySetting
}

type legacyChanges struct {
	initial legacySetting
	changes []legacyChange
	ready   bool
}

func (d *Decoder) beginLegacy() {
	c := d.opts.LegacyDecoder
	d.loose = !*c.Strict
	d.unknownOK = d.loose
	d.opts.Entities = *c.Entity
	d.opts.CharsetReader = *c.CharsetReader
	if d.legacy != nil {
		d.legacy.initial = legacySetting{}
		clear(d.legacy.changes)
		d.legacy.ready = false
		d.legacy.changes = d.legacy.changes[:0]
	}
}

// snapshotLegacy runs on every Read, so it is O(1): Strict and the identity of
// the Entity map. A replaced map or a changed Strict is therefore seen exactly,
// at the byte the Reader changed it. The map is not copied or compared, so a
// change of its contents in place is not a change here: it applies to the token
// being read as a whole and to the ones after it, never to part of a token.
func (d *Decoder) snapshotLegacy() legacySetting {
	c := d.opts.LegacyDecoder
	s := legacySetting{loose: !*c.Strict, entities: *c.Entity}
	if s.entities != nil {
		s.id = reflect.ValueOf(s.entities).UnsafePointer()
	}
	return s
}

func (d *Decoder) beforeLegacyRead() legacySetting {
	s := d.snapshotLegacy()
	if d.legacy != nil && !d.legacy.ready {
		d.legacy.initial, d.legacy.ready = s, true
	}
	return s
}

func (d *Decoder) afterLegacyRead(before legacySetting, at int64) {
	after := d.snapshotLegacy()
	if before.loose == after.loose && before.id == after.id {
		return
	}
	if d.legacy == nil {
		d.legacy = new(legacyChanges)
	}
	if !d.legacy.ready {
		d.legacy.initial, d.legacy.ready = before, true
	}
	d.legacy.changes = append(d.legacy.changes, legacyChange{at: at, legacySetting: after})
}

func (d *Decoder) legacyAt(at int) {
	s := d.legacy.initial
	// Binary search for the first change after the position.
	pos := d.base + int64(at)
	lo, hi := 0, len(d.legacy.changes)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if d.legacy.changes[m].at > pos {
			hi = m
		} else {
			lo = m + 1
		}
	}
	i := lo
	if i > 0 {
		s = d.legacy.changes[i-1].legacySetting
	}
	d.loose, d.unknownOK, d.opts.Entities = s.loose, s.loose, s.entities
}
