import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.security.SecureRandom;
import java.util.Arrays;
import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.SecretKeySpec;

class CryptoExamples {
  public static void hashData(String inputString) throws NoSuchAlgorithmException {
    byte[] inputBytes = inputString.getBytes();

    MessageDigest md5Digest = MessageDigest.getInstance("MD5");
    System.out.printf("MD5: %s\n", bytesToHex(md5Digest.digest(inputBytes)));

    MessageDigest sha1Digest = MessageDigest.getInstance("SHA-1");
    System.out.printf("SHA-1: %s\n", bytesToHex(sha1Digest.digest(inputBytes)));

    MessageDigest sha224Digest = MessageDigest.getInstance("SHA-224");
    System.out.printf("SHA-224: %s\n", bytesToHex(sha224Digest.digest(inputBytes)));

    MessageDigest sha384Digest = MessageDigest.getInstance("SHA-384");
    System.out.printf("SHA-384: %s\n", bytesToHex(sha384Digest.digest(inputBytes)));

    MessageDigest sha256Digest = MessageDigest.getInstance("SHA-256");
    System.out.printf("SHA-256: %s\n", bytesToHex(sha256Digest.digest(inputBytes)));

    MessageDigest sha512Digest = MessageDigest.getInstance("SHA-512");
    System.out.printf("SHA-512: %s\n", bytesToHex(sha512Digest.digest(inputBytes)));

    // Examples from SHA-3 family (Java 9+)
    MessageDigest sha3_256Digest = MessageDigest.getInstance("SHA3-256");
    System.out.printf("SHA3-256: %s\n", bytesToHex(sha3_256Digest.digest(inputBytes)));

    MessageDigest sha3_224Digest = MessageDigest.getInstance("SHA3-224");
    System.out.printf("SHA3-224: %s\n", bytesToHex(sha3_224Digest.digest(inputBytes)));

    MessageDigest sha3_384Digest = MessageDigest.getInstance("SHA3-384");
    System.out.printf("SHA3-384: %s\n", bytesToHex(sha3_384Digest.digest(inputBytes)));

    MessageDigest sha3_512Digest = MessageDigest.getInstance("SHA3-512");
    System.out.printf("SHA3-512: %s\n", bytesToHex(sha3_512Digest.digest(inputBytes)));

    // Less common hash (MD2, included in Java but rarely used today)
    MessageDigest md2Digest = MessageDigest.getInstance("MD2");
    System.out.printf("MD2: %s\n", bytesToHex(md2Digest.digest(inputBytes)));
  }

  // Symmetric algorithms: dedicated encrypt/decrypt

  public static byte[] encryptAES(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("AES");
    c.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(key, "AES"));
    return c.doFinal(input);
  }

  public static byte[] decryptAES(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("AES");
    c.init(Cipher.DECRYPT_MODE, new SecretKeySpec(key, "AES"));
    return c.doFinal(input);
  }

  public static byte[] encryptDES(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("DES");
    c.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(key, "DES"));
    return c.doFinal(input);
  }

  public static byte[] decryptDES(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("DES");
    c.init(Cipher.DECRYPT_MODE, new SecretKeySpec(key, "DES"));
    return c.doFinal(input);
  }

  public static byte[] encrypt3DES(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("DESede");
    c.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(key, "DESede"));
    return c.doFinal(input);
  }

  public static byte[] decrypt3DES(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("DESede");
    c.init(Cipher.DECRYPT_MODE, new SecretKeySpec(key, "DESede"));
    return c.doFinal(input);
  }

  public static byte[] encryptBlowfish(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("Blowfish");
    c.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(key, "Blowfish"));
    return c.doFinal(input);
  }

  public static byte[] decryptBlowfish(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("Blowfish");
    c.init(Cipher.DECRYPT_MODE, new SecretKeySpec(key, "Blowfish"));
    return c.doFinal(input);
  }

  public static byte[] encryptRC2(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("RC2");
    c.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(key, "RC2"));
    return c.doFinal(input);
  }

  public static byte[] decryptRC2(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("RC2");
    c.init(Cipher.DECRYPT_MODE, new SecretKeySpec(key, "RC2"));
    return c.doFinal(input);
  }

  public static byte[] encryptRC4(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("RC4");
    c.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(key, "RC4"));
    return c.doFinal(input);
  }

  public static byte[] decryptRC4(byte[] key, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("RC4");
    c.init(Cipher.DECRYPT_MODE, new SecretKeySpec(key, "RC4"));
    return c.doFinal(input);
  }

  // Asymmetric RSA
  public static byte[] encryptRSA(PublicKey pub, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("RSA/ECB/PKCS1Padding");
    c.init(Cipher.ENCRYPT_MODE, pub);
    return c.doFinal(input);
  }

  public static byte[] decryptRSA(PrivateKey priv, byte[] input) throws Exception {
    Cipher c = Cipher.getInstance("RSA/ECB/PKCS1Padding");
    c.init(Cipher.DECRYPT_MODE, priv);
    return c.doFinal(input);
  }

  // Helpers
  public static SecretKey genKey(String alg, int size) throws Exception {
    KeyGenerator kg = KeyGenerator.getInstance(alg);
    kg.init(size);
    return kg.generateKey();
  }

  public static KeyPair genRSA(int size) throws Exception {
    KeyPairGenerator kpg = KeyPairGenerator.getInstance("RSA");
    kpg.initialize(size);
    return kpg.generateKeyPair();
  }

  private static String toHex(byte[] b) {
    var sb = new StringBuilder();
    for (byte x : b)
      sb.append(String.format("%02x", x));
    return sb.toString();
  }

  private static String bytesToHex(byte[] bytes) {
    StringBuilder sb = new StringBuilder();
    for (byte b : bytes) {
      sb.append(String.format("%02x", b));
    }
    return sb.toString();
  }

  public static void main(String[] args) {
    System.out.println("Sample code for docs");
  }
}
