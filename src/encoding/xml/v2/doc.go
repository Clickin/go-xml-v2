// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build goexperiment.xmlv2

// Package xml implements semantic processing of XML: it marshals Go values
// to XML and unmarshals XML into Go values. It is built on the syntactic
// layer in [encoding/xml/xmltext], as encoding/json/v2 is built on
// encoding/json/jsontext.
//
// # Elements and text
//
// A Go value is marshaled either as an element or as text. Text is what
// an attribute value or the character data of a simple element holds:
// booleans, numbers, strings and byte slices (written verbatim), and any
// type with an AppendText, MarshalText or a text function registered with
// [MarshalFunc]. Every other type is marshaled as an element whose name is
// given by the context: the field tag of the enclosing struct, or for the
// top-level value its XMLName field or its type name.
//
// A struct is marshaled as an element holding its fields: attributes first,
// then character data, comments and child elements in field order. The
// struct tag grammar is that of encoding/xml, with omitzero added.
//
// Names match exactly, namespace included. A tag without a namespace names
// an element or attribute in no namespace, as in Namespaces in XML 1.0,
// and does not match the same local name in another namespace; elements of
// a parent>child chain are in no namespace. An XMLName tag with a name
// requires the element to have that name and namespace. (encoding/xml
// matches a name without a namespace in any namespace.)
//
// Unmarshal puts the namespace declarations of an element into its
// ",any,attr" field when that field is a slice of attributes, named as the
// Decoder names them (Space XMLNSNamespace), and Marshal writes them back,
// so the element keeps its prefixes and a prefix used only in an attribute
// value or text (xsi:type="p:T") stays declared. Without such a field the
// declarations are dropped and Marshal generates prefixes (ns1, ns2, ...).
//
// A slice or array other than a byte slice is marshaled as one element per
// item, all with the same name; unmarshaling appends one item per element.
// A nil pointer or interface is omitted.
//
// # Methods and functions
//
// A type controls its own representation through [MarshalerTo] and
// [UnmarshalerFrom], whose methods write and read the content of an element
// whose start tag the caller has already handled, or through the text
// methods. [WithMarshalers] and [WithUnmarshalers] override a type's
// representation for a single call.
package xml
