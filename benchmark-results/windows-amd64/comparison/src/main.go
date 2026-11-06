package main

import (
 "encoding/xml/xmltext"
 "fmt"
 "io"
 "os"
 "time"
)

func scan(data []byte) (int, error) {
 d := xmltext.NewDecoderBytes(data)
 n := 0
 for {
  t, err := d.ReadToken()
  if err == io.EOF { return n, nil }; if err != nil { return 0, err }
  switch t.Kind() {
  case xmltext.StartElement:
   n += len(t.Name().Local)
   for it := t.Attrs(); it.Next(); { n += len(it.Name().Local) + len(it.Value()) }
  case xmltext.EndElement: n += len(t.Name().Local)
  case xmltext.Text: n += len(t.Text())
  }
 }
}
func main() {
 data, err := os.ReadFile(os.Args[1]); if err != nil { panic(err) }
 var expected int
 for i := -8; i < 10; i++ {
  start := time.Now(); n, err := scan(data); elapsed := time.Since(start); if err != nil { panic(err) }
  if i == -8 { expected = n }; if n != expected { panic("checksum changed") }
  if i >= 0 { fmt.Printf("go,%d,%d,%d\n", i, elapsed.Nanoseconds(), n) }
 }
}
