package com.app;

import org.yaml.snakeyaml.Yaml;

class Task implements Runnable {
    public void run() {
        new Yaml().load("a: 1");
    }
}
