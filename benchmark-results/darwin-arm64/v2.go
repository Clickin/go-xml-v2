//go:build goexperiment.xmlv2

package main

import (
	xmlv2 "encoding/xml/v2"
	"encoding/xml/xmltext"
	"io"
	"os"
)

const goexperiment = "xmlv2"

func init() {
	readers["xmltext ReadToken"] = tokenXMLText
	readers["encoding/xml/v2 Unmarshal"] = unmarshalV2
}

func rawName(n xmltext.Name) string {
	if n.Space != "" {
		return n.Space + ":" + n.Local
	}
	return n.Local
}

func tokenXMLText(path string) (int64, int32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	d := xmltext.NewDecoderBytes(data)
	var s state
	for {
		t, err := d.ReadToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
		switch t.Kind() {
		case xmltext.StartElement:
			it := t.Attrs()
			s.start(rawName(t.RawName()), it.Len())
			for it.Next() {
				s.sum = foldBytes(fold(s.sum, rawName(it.RawName())), it.Value())
			}
		case xmltext.EndElement:
			s.end(rawName(t.RawName()))
		case xmltext.Text, xmltext.CData:
			s.pending = append(s.pending, t.Text()...)
		}
	}
	s.flush()
	return s.events, s.sum, nil
}

func unmarshalV2(path string) (int64, int32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var c catalog
	if err := xmlv2.Unmarshal(data, &c); err != nil {
		return 0, 0, err
	}
	n, sum := catalogSum(&c)
	return n, sum, nil
}
