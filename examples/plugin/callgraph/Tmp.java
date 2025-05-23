import java.awt.Dialog;
import java.awt.Frame;
import java.awt.LayoutManager;
import java.awt.Window;
import java.util.ArrayList;
import java.util.Map.Entry;
import com.sun.activation.registries.MailcapFile;

Dialog dg = new Dialog(new Window(new Frame()));
System.out.println(dg);

public void start() {
  System.out.println("Starting...");
}

public class Tmp {
  public static void main(String[] args) {
    try {
        MailcapFile mailcapFile = new MailcapFile();
        System.out.println(mailcapFile.getMimeTypes());
      LayoutManager lm = new java.awt.BorderLayout();
      java.awt.Component cb = new java.awt.Checkbox("label", new java.awt.CheckboxGroup(), false);
      cb = new java.awt.Dialog(new Dialog(new Window(new Frame()))); // Not assigned properly yet to cb
      java.awt.image.renderable.ParameterBlock pb = new java.awt.image.renderable.ParameterBlock();
      int a = 5;
      java.awt.Component cnv = new java.awt.Canvas();
      System.out.println(cnv);
    } catch (java.awt.AWTException e) {
      e.printStackTrace();
    }
    StringBuilder sb = new StringBuilder();
    sb.append("Hello, World!");
    System.out.println(sb.toString());
  }
}
