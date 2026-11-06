import com.ctc.wstx.stax.WstxInputFactory;
import java.io.ByteArrayInputStream;
import java.nio.file.*;
import java.util.*;
import javax.xml.stream.*;

public class AssetBench {
 record Result(long events, int checksum) {}
 static int fold(int h,String s) { for(int i=0;i<s.length();i++) h=h*31+s.charAt(i); return h; }
 static String q(String prefix,String local) { return prefix==null || prefix.isEmpty() ? local : prefix+":"+local; }
 static Result consume(Path path, byte[] data) throws Exception {
  byte[] b=data!=null?data:Files.readAllBytes(path); var f=new WstxInputFactory(); f.setProperty(XMLInputFactory.SUPPORT_DTD,false); var r=f.createXMLStreamReader(new ByteArrayInputStream(b)); long events=0; int h=0; StringBuilder pending=new StringBuilder();
  while(r.hasNext()) { int type=r.next(); if(type==XMLStreamConstants.START_ELEMENT || type==XMLStreamConstants.END_ELEMENT) {
   String s=pending.toString().strip(); pending.setLength(0); if(!s.isEmpty()) {events++; h=fold(fold(h,"T"),s);}
   events++; h=fold(fold(h,type==XMLStreamConstants.START_ELEMENT?"S":"E"),q(r.getPrefix(),r.getLocalName()));
   if(type==XMLStreamConstants.START_ELEMENT) { h=(h^r.getAttributeCount())*16777619; for(int i=0;i<r.getAttributeCount();i++) {h=fold(h,q(r.getAttributePrefix(i),r.getAttributeLocalName(i))); h=fold(h,r.getAttributeValue(i));} }
  } else if(type==XMLStreamConstants.CHARACTERS || type==XMLStreamConstants.CDATA) pending.append(r.getText()); }
  String s=pending.toString().strip(); if(!s.isEmpty()){events++;h=fold(fold(h,"T"),s);} r.close(); return new Result(events,h);
 }
 public static void main(String[] a) throws Exception { Path p=Path.of(a[0]); int w=Integer.parseInt(a[1]), n=Integer.parseInt(a[2]), reps=Integer.parseInt(a[3]); byte[] data=a.length>4&&a[4].equals("mem")?Files.readAllBytes(p):null; StringBuilder out=new StringBuilder("{\"samples\":["); for(int i=-w;i<n;i++){long start=System.nanoTime(); Result r=null; for(int j=0;j<reps;j++){Result x=consume(p,data); if(r!=null && !r.equals(x)) throw new AssertionError("checksum changed"); r=x;} double sec=(System.nanoTime()-start)/1e9/reps; if(i>=0){if(i>0)out.append(',');out.append(String.format(Locale.ROOT,"{\"events\":%d,\"checksum\":%d,\"seconds\":%.9f}",r.events(),r.checksum(),sec));}} System.out.println(out.append("]}")); }
}
