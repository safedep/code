package com.shop;

public abstract class Step {
    public void run() {
        parse();
    }

    protected abstract void parse();
}
