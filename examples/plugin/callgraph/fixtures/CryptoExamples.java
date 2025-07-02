package examples;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;

class CryptoExamples {
  public void hashData(String inputString) throws NoSuchAlgorithmException {
    byte[] inputBytes = inputString.getBytes();
    List<byte[]> hashes = new ArrayList<>();

    MessageDigest md5Digest = MessageDigest.getInstance("MD5");
    md5Digest.update(inputBytes);
    hashes.add(md5Digest.digest());

    String sha1AlgoID = "SHA-1";
    MessageDigest sha1Digest = MessageDigest.getInstance(sha1AlgoID);
    sha1Digest.update(inputBytes);
    hashes.add(sha1Digest.digest());

    String uncommonAlgo = "SHA-224";
    if (Math.random() < 0.5) {
      uncommonAlgo = "SHA-384";
    }
    MessageDigest uncommonDigest = MessageDigest.getInstance(uncommonAlgo);
    uncommonDigest.update(inputBytes);
    hashes.add(uncommonDigest.digest());
    
    MessageDigest sha256Digest = MessageDigest.getInstance((Math.random() < 0.5) ? "SHA-256" : "SHA-512");
    sha256Digest.update(inputBytes);
    hashes.add(sha256Digest.digest());  
  }

  public void encryptData(String inputString) throws NoSuchAlgorithmException {
  }

  public void decryptData(byte[] encryptedData, String key) {
    
  }
} 