use quick_xml::{events::Event, escape::resolve_predefined_entity, XmlVersion, Reader};
use std::{env, fs, time::Instant};

fn scan(data: &[u8]) -> usize {
    let mut reader = Reader::from_reader(data);
    reader.config_mut().expand_empty_elements = true;
    let mut n = 0usize;
    loop {
        match reader.read_event().expect("XML event") {
            Event::Start(e) => {
                n += e.local_name().as_ref().len();
                for a in e.attributes() {
                    let a = a.expect("attribute");
                    n += a.key.local_name().as_ref().len() + a.normalized_value(XmlVersion::Implicit1_0).expect("attribute value").len();
                }
            }
            Event::End(e) => n += e.local_name().as_ref().len(),
            Event::Text(e) => n += e.xml10_content().len(),
            Event::CData(e) => n += e.xml10_content().len(),
            Event::GeneralRef(e) => {
                n += if let Some(c) = e.resolve_char_ref().expect("character reference") { c.len_utf8() }
                     else { resolve_predefined_entity(&e).expect("unknown entity").len() };
            }
            Event::Eof => break,
            _ => (),
        }
    }
    n
}
fn main() {
    let data = fs::read(env::args().nth(1).expect("XML file")).expect("read XML");
    let mut expected = 0;
    for i in -8..10 {
        let start = Instant::now(); let n = std::hint::black_box(scan(&data)); let elapsed = start.elapsed().as_nanos();
        if i == -8 { expected = n; } assert_eq!(n, expected);
        if i >= 0 { println!("quick-xml,{i},{elapsed},{n}"); }
    }
}
