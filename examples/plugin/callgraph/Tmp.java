import java.awt.Dialog;
import java.awt.Frame;
import java.awt.LayoutManager;
import java.awt.Window;
import java.util.ArrayList;
import java.util.Map.Entry;
import com.sun.activation.registries.MailcapFile;
import static somelib.xyz.somelibfn;


public class Tmp {
  public static void main(String[] args) {
    try {
        // Done --------------------
        somelibfn();
        Dialog dg = new Dialog(new Window(new Frame()));
        dg.setTitle("Test Dialog");
        dg.subobj.getSomething();
        dg.subobj.subobj2.getSomethingElse();
        System.out.println(dg);
        // Done ^^^^^^^^^^^^^


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
