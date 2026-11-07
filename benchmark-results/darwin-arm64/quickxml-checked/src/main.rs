// The reader of stax-xml's reader-cross-runtime benchmark, with the work a
// conforming reader does on top of finding tokens: text is checked as
// UTF-8 and its references resolved, attribute values are unescaped, names
// are checked as UTF-8 and end names are matched (check_end_names, the
// default).
use quick_xml::escape::resolve_xml_entity;
use quick_xml::events::Event;
use quick_xml::Reader;
use std::fs;
use std::time::Instant;

fn fold(mut seed: i32, value: &[u8]) -> i32 {
    for byte in value {
        seed = seed.wrapping_mul(31).wrapping_add(*byte as i32);
    }
    seed
}

fn mix(seed: i32, value: usize) -> i32 {
    (seed ^ value as i32).wrapping_mul(16777619)
}

fn flush_text(pending: &mut Vec<u8>, events: &mut u64, checksum: &mut i32) {
    let start = pending.iter().position(|b| !b.is_ascii_whitespace()).unwrap_or(pending.len());
    let end = pending.iter().rposition(|b| !b.is_ascii_whitespace()).map(|i| i + 1).unwrap_or(start);
    if start < end {
        *events += 1;
        *checksum = fold(fold(*checksum, b"T"), &pending[start..end]);
    }
    pending.clear();
}

fn name(b: &[u8]) -> &[u8] {
    std::str::from_utf8(b).unwrap().as_bytes()
}

fn consume(path: &str) -> (u64, i32) {
    let bytes = fs::read(path).unwrap();
    let mut reader = Reader::from_reader(bytes.as_slice());
    let mut events = 0;
    let mut checksum = 0;
    let mut pending_text = Vec::new();
    loop {
        match reader.read_event().unwrap() {
            Event::Start(event) => {
                flush_text(&mut pending_text, &mut events, &mut checksum);
                events += 1;
                checksum = fold(fold(checksum, b"S"), name(event.name().as_ref()));
                let attributes: Vec<_> = event.attributes().map(|value| value.unwrap()).collect();
                checksum = mix(checksum, attributes.len());
                for attribute in attributes {
                    checksum = fold(checksum, name(attribute.key.as_ref()));
                    checksum = fold(checksum, attribute.unescape_value().unwrap().as_bytes());
                }
            }
            Event::End(event) => {
                flush_text(&mut pending_text, &mut events, &mut checksum);
                events += 1;
                checksum = fold(fold(checksum, b"E"), name(event.name().as_ref()));
            }
            Event::Text(event) => pending_text.extend_from_slice(event.decode().unwrap().as_bytes()),
            Event::CData(event) => pending_text.extend_from_slice(event.decode().unwrap().as_bytes()),
            Event::GeneralRef(event) => {
                if let Some(c) = event.resolve_char_ref().unwrap() {
                    let mut buf = [0u8; 4];
                    pending_text.extend_from_slice(c.encode_utf8(&mut buf).as_bytes());
                } else {
                    let n = event.decode().unwrap();
                    pending_text.extend_from_slice(resolve_xml_entity(&n).unwrap().as_bytes());
                }
            }
            Event::Eof => break,
            _ => {}
        }
    }
    flush_text(&mut pending_text, &mut events, &mut checksum);
    (events, checksum)
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let path = &args[1];
    let warmups: usize = args[2].parse().unwrap();
    let runs: usize = args[3].parse().unwrap();
    for _ in 0..warmups {
        consume(path);
    }
    let mut samples = Vec::new();
    for _ in 0..runs {
        let started = Instant::now();
        let (events, checksum) = consume(path);
        samples.push(format!(
            "{{\"events\":{events},\"checksum\":{checksum},\"seconds\":{:.9}}}",
            started.elapsed().as_secs_f64()
        ));
    }
    println!("{{\"samples\":[{}]}}", samples.join(","));
}
