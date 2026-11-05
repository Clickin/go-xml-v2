// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.xmlv2

package xml

import (
	"testing"

	"encoding/xml/internal/xmllegacy"
)

// getTypeInfo converts the flags of xmllegacy.Field by value.
func TestLegacyFieldFlags(t *testing.T) {
	for _, c := range []struct {
		v1     fieldFlags
		legacy uint8
	}{
		{fElement, xmllegacy.FElement}, {fAttr, xmllegacy.FAttr}, {fCDATA, xmllegacy.FCDATA},
		{fCharData, xmllegacy.FCharData}, {fInnerXML, xmllegacy.FInnerXML}, {fComment, xmllegacy.FComment},
		{fAny, xmllegacy.FAny}, {fOmitEmpty, xmllegacy.FOmitEmpty},
	} {
		if c.v1 != fieldFlags(c.legacy) {
			t.Errorf("flag %#x: xmllegacy has %#x", c.v1, c.legacy)
		}
	}
}
