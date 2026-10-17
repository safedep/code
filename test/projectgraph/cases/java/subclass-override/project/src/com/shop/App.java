package com.shop;

public class App {
    public static void main(String[] args) {
        Step step = Steps.pick(args[0]);
        step.run();
    }
}
