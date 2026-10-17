package com.shop;

public class Main {
    public static void main(String[] args) {
        new Greeter().greet(args[0]);
        TextKt.clean(args[0]);
    }
}
