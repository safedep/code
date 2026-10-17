package com.calc;

public class Main {
    public static void main(String[] args) {
        Op op = Op.valueOf(args[0]);
        op.apply(1, 2);
        Point p = new Point(1, 2);
        p.norm();
    }
}
