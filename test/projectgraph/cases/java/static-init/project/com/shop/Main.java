package com.shop;

public class Main {
    public static void main(String[] args) {
        Config.get("region");
        Registry.INSTANCE.lookup("orders");
    }
}
