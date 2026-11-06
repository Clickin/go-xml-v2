package main

import (
 "encoding/json"
 "encoding/xml/xmltext"
 "fmt"
 "io"
 "os"
 "strconv"
 "strings"
 "time"
)
type sample struct { Events int `json:"events"`; Checksum int32 `json:"checksum"`; Seconds float64 `json:"seconds"` }
func fold(h int32, s string) int32 { for _, c := range s { if c <= 0xffff { h=h*31+int32(c) } else { c-=0x10000; h=h*31+int32(0xd800+(c>>10)); h=h*31+int32(0xdc00+(c&0x3ff)) } }; return h }
func qname(n xmltext.Name) string { if n.Space != "" { return n.Space+":"+n.Local }; return n.Local }
func consume(path string) (r sample, err error) {
 b, err := os.ReadFile(path); if err != nil { return r,err }; d := xmltext.NewDecoderBytes(b); var pending strings.Builder
 flush := func() { s := strings.TrimSpace(pending.String()); pending.Reset(); if s!="" { r.Events++; r.Checksum=fold(fold(r.Checksum,"T"),s) } }
 for { t,e := d.ReadToken(); if e==io.EOF { break }; if e!=nil { return r,e }; switch t.Kind() {
 case xmltext.StartElement:
  flush(); r.Events++; r.Checksum=fold(fold(r.Checksum,"S"),qname(t.RawName())); count:=0; for a:=t.Attrs(); a.Next(); { if a.Name().Space!=xmltext.XMLNSNamespace { count++ } }; r.Checksum=(r.Checksum^int32(count))*16777619
  for a:=t.Attrs(); a.Next(); { if a.Name().Space!=xmltext.XMLNSNamespace { r.Checksum=fold(r.Checksum,qname(a.RawName())); r.Checksum=fold(r.Checksum,string(a.Value())) } }
 case xmltext.EndElement: flush(); r.Events++; r.Checksum=fold(fold(r.Checksum,"E"),qname(t.RawName()))
 case xmltext.Text,xmltext.CData: pending.Write(t.Text())
 } }; flush(); return r,nil
}
func main(){ path:=os.Args[1]; w,_:=strconv.Atoi(os.Args[2]); n,_:=strconv.Atoi(os.Args[3]); reps,_:=strconv.Atoi(os.Args[4]); out:=make([]sample,0,n); for i:=-w;i<n;i++ { start:=time.Now(); var r sample; for j:=0;j<reps;j++ { x,e:=consume(path); if e!=nil { fmt.Fprintln(os.Stderr,e); os.Exit(1) }; if j>0 && (x.Events!=r.Events || x.Checksum!=r.Checksum) { panic("checksum changed") }; r=x }; r.Seconds=time.Since(start).Seconds()/float64(reps); if i>=0 { out=append(out,r) } }; json.NewEncoder(os.Stdout).Encode(struct{Samples []sample `json:"samples"`}{out}) }
