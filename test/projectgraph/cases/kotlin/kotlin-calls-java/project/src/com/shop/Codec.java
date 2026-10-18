package com.shop;

import java.util.Base64;

public class Codec {
    public static byte[] decode(String s) {
        return Base64.getDecoder().decode(s);
    }
}
