package com.calc;

record Point(int x, int y) {
    double norm() {
        return Math.sqrt(x * x + y * y);
    }
}
