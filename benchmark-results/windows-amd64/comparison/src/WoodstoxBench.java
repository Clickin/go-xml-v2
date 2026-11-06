import java.nio.file.Files;
import java.nio.file.Path;
import java.io.ByteArrayInputStream;
import javax.xml.stream.*;
import com.ctc.wstx.stax.WstxInputFactory;

public class WoodstoxBench {
  static long scan(XMLInputFactory factory, byte[] data) throws Exception {
    var r = factory.createXMLStreamReader(new ByteArrayInputStream(data));
    long n = 0;
    while (r.hasNext()) {
      switch (r.next()) {
        case XMLStreamConstants.START_ELEMENT:
          n += r.getLocalName().length();
          for (int a = 0; a < r.getAttributeCount(); a++) n += r.getAttributeLocalName(a).length() + r.getAttributeValue(a).length();
          break;
        case XMLStreamConstants.END_ELEMENT: n += r.getLocalName().length(); break;
        case XMLStreamConstants.CHARACTERS: case XMLStreamConstants.CDATA: n += r.getTextLength(); break;
      }
    }
    r.close(); return n;
  }
  public static void main(String[] args) throws Exception {
    byte[] data = Files.readAllBytes(Path.of(args[0]));
    var factory = new WstxInputFactory();
    factory.setProperty(XMLInputFactory.SUPPORT_DTD, false);
    factory.setProperty(XMLInputFactory.IS_REPLACING_ENTITY_REFERENCES, true);
    long expected = -1;
    for (int i = -8; i < 10; i++) {
      long start = System.nanoTime(), n = scan(factory, data), elapsed = System.nanoTime() - start;
      if (expected < 0) expected = n; if (n != expected) throw new AssertionError("checksum changed");
      if (i >= 0) System.out.printf("woodstox,%d,%d,%d%n", i, elapsed, n);
    }
  }
}
