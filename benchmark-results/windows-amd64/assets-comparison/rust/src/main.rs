use quick_xml::{events::Event, escape::resolve_predefined_entity, Reader, XmlVersion};
use std::{env,fs,time::Instant};
fn fold(mut h:i32,s:&str)->i32 {for c in s.encode_utf16(){h=h.wrapping_mul(31).wrapping_add(c as i32)} h}
fn flush(p:&mut String,n:&mut u64,h:&mut i32){let s=p.trim();if !s.is_empty(){*n+=1;*h=fold(fold(*h,"T"),s);}p.clear();}
fn consume(path:&str,data:Option<&[u8]>)->(u64,i32){
 let owned; let b:&[u8]=match data{Some(d)=>d,None=>{owned=fs::read(path).unwrap();&owned}}; let mut r=Reader::from_reader(b); r.config_mut().expand_empty_elements=true;
 let(mut n,mut h,mut p)=(0u64,0i32,String::new());
 loop{match r.read_event().expect("event"){
  Event::Start(e)=>{flush(&mut p,&mut n,&mut h);n+=1;h=fold(fold(h,"S"),e.name().as_ref());
   let a:Vec<_>=e.attributes().map(|a|a.unwrap()).filter(|a|a.key.as_ref()!="xmlns"&&!a.key.as_ref().starts_with("xmlns:")).collect();
   h=(h^(a.len() as i32)).wrapping_mul(16777619);for x in a {h=fold(h,x.key.as_ref());h=fold(h,&x.normalized_value(XmlVersion::Implicit1_0).unwrap());}
  },
  Event::End(e)=>{flush(&mut p,&mut n,&mut h);n+=1;h=fold(fold(h,"E"),e.name().as_ref());},
  Event::Text(e)=>p.push_str(&e.xml10_content()),Event::CData(e)=>p.push_str(&e.xml10_content()),
  Event::GeneralRef(e)=>{if let Some(c)=e.resolve_char_ref().unwrap(){p.push(c)}else{p.push_str(resolve_predefined_entity(&e).expect("unknown entity"))}},
  Event::Eof=>break,_=>{}
 }}flush(&mut p,&mut n,&mut h);(n,h)
}
fn main(){let a:Vec<_>=env::args().collect();let w:i32=a[2].parse().unwrap();let n:i32=a[3].parse().unwrap();let reps:u32=a[4].parse().unwrap();let mem=a.get(5).map(|s|s=="mem").unwrap_or(false);let data=if mem{Some(fs::read(&a[1]).unwrap())}else{None};let mut v=Vec::new();for i in -w..n{let t=Instant::now();let mut r=None;for _ in 0..reps{let x=consume(&a[1],std::hint::black_box(data.as_deref()));if let Some(previous)=r{assert_eq!(x,previous)}r=Some(x);}let (events,checksum)=r.unwrap();let seconds=t.elapsed().as_secs_f64()/reps as f64;if i>=0{v.push(format!("{{\"events\":{events},\"checksum\":{checksum},\"seconds\":{seconds:.9}}}"));}}println!("{{\"samples\":[{}]}}",v.join(","));}
