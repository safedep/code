import java.awt.Dialog;
import java.awt.Frame;
import java.awt.GridLayout;
import java.awt.ScrollPane;
import java.awt.LayoutManager;
import java.awt.Window;
import com.sun.activation.registries.MailcapFile;
import static somelib.xyz.somelibfn;

public class CallgraphTestcases {
  public CallgraphTestcases() {
    com.custompkg.SomeClass.defaultConstructor();
  }
  public CallgraphTestcases(int i) {
    com.custompkg.SomeClass.someMethod(i);
  }
  public CallgraphTestcases(int i, String s) {
    com.custompkg.SomeClass.someOtherMethod(i, s);
  }

  public static void myfunc(){
    String.valueOf('c');
  }
  
  public static void main(String[] args) {
    // Member functions / sub-functions accessed
    Dialog dg = new Dialog(new Window(new Frame()));
    dg.setTitle("Test Dialog");
    dg.prop.getSomething();

    // Member functions / sub-functions accessed on fully qualified class
    java.awt.Component cnv = new java.awt.Canvas();
    cnv = new ScrollPane();
    cnv.setSize(100, 100);
    cnv.prop.subprop.subsubprop.getSomething();

    // Multiple classes assigned
    LayoutManager lm = new java.awt.BorderLayout();
    lm.addLayoutComponent("North", new java.awt.Button("North Button"));
    lm = new java.awt.FlowLayout();
    lm.minimumLayoutSize(new java.awt.Container());
    lm = new GridLayout();
    lm.toString();
    lm.prop.getSomething();

    // Standalone function calls
    somelibfn();
    myfunc();
    System.out.println("GG");
    System.out.xyz.println("GG");

    // Function call chain
    System.console().readPassword();
    System.getenv().keySet().iterator().hasNext();
    Math.atan(1.0);

    // Unknown standalone function call
    com.somecompany.customlib.datatransfer.DataTransferer.getInstance(); // remaining

    // Unknown object creation
    Object obj = new org.mycompany.mylib.SomeClass();
    obj.prop.someMethod("GG");
  }
}
