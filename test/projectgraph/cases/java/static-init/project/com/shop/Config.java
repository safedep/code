package com.shop;

class Config {
    static {
        System.loadLibrary("native");
    }

    static String get(String key) {
        return key;
    }
}
