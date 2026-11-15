package com.x;

public class A extends Outer {
    public static void main(String[] args) {
        new A().go();
    }

    void go() {
        baseMethod();
    }
}
