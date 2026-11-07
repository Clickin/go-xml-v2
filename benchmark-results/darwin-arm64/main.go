// Command crossbench reads the cross-runtime reader fixture with Go's XML
// readers, under the rules of stax-xml's reader-cross-runtime.mjs: each
// timed run reads the file and folds every start and end element name,
// every attribute name and value, and each run of non-whitespace text
// into one checksum. The checksum and event count must match those of
// stax-xml, Woodstox and quick-xml.
package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

func fold(seed int32, s string) int32 {
	for i := 0; i < len(s); i++ {
		seed = seed*31 + int32(s[i])
	}
	return seed
}

func foldBytes(seed int32, b []byte) int32 {
	for _, c := range b {
		seed = seed*31 + int32(c)
	}
	return seed
}

func mix(seed int32, v int) int32 { return (seed ^ int32(v)) * 16777619 }

// state is the checksum of one run.
type state struct {
	events  int64
	sum     int32
	pending []byte
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

func (s *state) flush() {
	p := s.pending
	i, j := 0, len(p)
	for i < j && isSpace(p[i]) {
		i++
	}
	for j > i && isSpace(p[j-1]) {
		j--
	}
	if i < j {
		s.events++
		s.sum = foldBytes(fold(s.sum, "T"), p[i:j])
	}
	s.pending = s.pending[:0]
}

func (s *state) start(name string, nattr int) {
	s.flush()
	s.events++
	s.sum = mix(fold(fold(s.sum, "S"), name), nattr)
}

func (s *state) end(name string) {
	s.flush()
	s.events++
	s.sum = fold(fold(s.sum, "E"), name)
}

type result struct {
	Events   int64   `json:"events"`
	Checksum int32   `json:"checksum"`
	Seconds  float64 `json:"seconds"`
	Allocs   uint64  `json:"allocs"`
	Bytes    uint64  `json:"allocBytes"`
}

// readers are the cases; a case returns events and checksum, or for an
// Unmarshal case the number of records decoded in events.
var readers = map[string]func(path string) (int64, int32, error){
	"encoding/xml Token":     tokenV1,
	"encoding/xml Unmarshal": unmarshalV1,
}

func qname(n xml.Name) string { return n.Local }

func tokenV1(path string) (int64, int32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	var s state
	for {
		t, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
		switch t := t.(type) {
		case xml.StartElement:
			s.start(qname(t.Name), len(t.Attr))
			for _, a := range t.Attr {
				s.sum = fold(fold(s.sum, qname(a.Name)), a.Value)
			}
		case xml.EndElement:
			s.end(qname(t.Name))
		case xml.CharData:
			s.pending = append(s.pending, t...)
		}
	}
	s.flush()
	return s.events, s.sum, nil
}

type chapter struct {
	Number string `xml:"number,attr"`
	Text   string `xml:",chardata"`
}

type book struct {
	ID          string    `xml:"id,attr"`
	Lang        string    `xml:"lang,attr"`
	Code        string    `xml:"code,attr"`
	Title       string    `xml:"title"`
	Author      string    `xml:"author"`
	Description string    `xml:"description"`
	Chapters    []chapter `xml:"chapter"`
}

type catalog struct {
	XMLName xml.Name `xml:"root"`
	Books   []book   `xml:"book"`
}

// catalogSum folds the decoded fields, so that Unmarshal cases can be
// checked against each other.
func catalogSum(c *catalog) (int64, int32) {
	var sum int32
	for i := range c.Books {
		b := &c.Books[i]
		for _, s := range []string{b.ID, b.Lang, b.Code, b.Title, b.Author, b.Description} {
			sum = fold(sum, s)
		}
		for _, ch := range b.Chapters {
			sum = fold(fold(sum, ch.Number), ch.Text)
		}
	}
	return int64(len(c.Books)), sum
}

func unmarshalV1(path string) (int64, int32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var c catalog
	if err := xml.Unmarshal(data, &c); err != nil {
		return 0, 0, err
	}
	n, sum := catalogSum(&c)
	return n, sum, nil
}

func main() {
	file := flag.String("file", "", "fixture")
	warmups := flag.Int("warmups", 1, "untimed runs")
	runs := flag.Int("runs", 3, "timed runs")
	only := flag.String("cases", "", "comma-separated cases (default all)")
	cpuprofile := flag.String("cpuprofile", "", "write a CPU profile of the timed runs")
	memprofile := flag.String("memprofile", "", "write an allocation profile of the runs")
	flag.Parse()
	if *memprofile != "" {
		runtime.MemProfileRate = 4096
		defer func() {
			f, err := os.Create(*memprofile)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			pprof.Lookup("allocs").WriteTo(f, 0)
			f.Close()
		}()
	}
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
		pprof.StartCPUProfile(f)
	}
	out := map[string]any{"go": runtime.Version(), "goexperiment": goexperiment}
	cases := map[string][]result{}
	for name, fn := range readers {
		if *only != "" && !strings.Contains(","+*only+",", ","+name+",") {
			continue
		}
		for range *warmups {
			if _, _, err := fn(*file); err != nil {
				fmt.Fprintln(os.Stderr, name, err)
				os.Exit(1)
			}
		}
		for range *runs {
			runtime.GC()
			var m0, m1 runtime.MemStats
			runtime.ReadMemStats(&m0)
			start := time.Now()
			ev, sum, err := fn(*file)
			sec := time.Since(start).Seconds()
			runtime.ReadMemStats(&m1)
			if err != nil {
				fmt.Fprintln(os.Stderr, name, err)
				os.Exit(1)
			}
			cases[name] = append(cases[name], result{ev, sum, sec, m1.Mallocs - m0.Mallocs, m1.TotalAlloc - m0.TotalAlloc})
		}
	}
	out["cases"] = cases
	json.NewEncoder(os.Stdout).Encode(out)
}
