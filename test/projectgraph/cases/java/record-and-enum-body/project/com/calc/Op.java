package com.calc;

enum Op {
    PLUS {
        int apply(int a, int b) {
            return Math.addExact(a, b);
        }
    },
    POW {
        int apply(int a, int b) {
            return (int) Math.pow(a, b);
        }
    };

    abstract int apply(int a, int b);
}
