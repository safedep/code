package com.shop;

public class Outer {
    public static void main(String[] args) {
        Outer outer = new Outer();
        Inner inner = outer.new Inner();
        inner.work();
    }

    void report() {
        java.util.logging.Logger.getLogger("audit").severe("done");
    }

    class Inner {
        void work() {
            report();
        }
    }
}
