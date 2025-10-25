import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.awt.Canvas;
import java.awt.Dialog;
import java.awt.Window;
import java.awt.Frame;

public class testJavaArguments {

  public static void testMessageDigest() throws NoSuchAlgorithmException {
    // Test literal string argument matching - should match MD5 signature
    MessageDigest md5 = MessageDigest.getInstance("MD5");

    // Test different case variations
    MessageDigest md5Upper = MessageDigest.getInstance("md5");

    // Test SHA-256 hash algorithm
    MessageDigest sha256 = MessageDigest.getInstance("SHA-256");

    // Test SHA-1 (should match sha1 signature)
    MessageDigest sha1 = MessageDigest.getInstance("SHA-1");
  }

  public static void testCanvasSetSize() {
    Canvas canvas1 = new Canvas();
    // Test specific numeric literal arguments
    canvas1.setSize(32, 99);

    Canvas canvas2 = new Canvas();
    // Test different values (should not match 32, 99 signature)
    canvas2.setSize(64, 128);

    Canvas canvas3 = new Canvas();
    // Test first arg matches, second doesn't
    canvas3.setSize(32, 100);
  }

  public static void testDialogWithWindow() {
    // Test type resolution - Dialog constructor with Window argument
    Window window = new Window(new Frame());
    Dialog dialog = new Dialog(window);

    // Test nested type resolution
    Dialog nestedDialog = new Dialog(new Window(new Frame()));
  }

  public static void testMixedArguments() {
    Canvas canvas = new Canvas();
    // Variable arguments (should not match specific literal signatures)
    int width = 32;
    int height = 99;
    canvas.setSize(width, height);
  }

  public static void main(String[] args) {
    try {
      testMessageDigest();
      testCanvasSetSize();
      testDialogWithWindow();
      testMixedArguments();
    } catch (Exception e) {
      e.printStackTrace();
    }
  }
}
