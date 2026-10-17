package com.shop;

import java.nio.file.Files;
import java.util.zip.GZIPInputStream;

class YamlStep extends Step {
    @Override
    protected void parse() {
        new org.yaml.snakeyaml.Yaml().load("a: 1");
    }
}

class ZipStep extends YamlStep {
    @Override
    protected void parse() {
        Files.readAllBytes(null);
    }
}

class Steps {
    static Step pick(String name) {
        return new YamlStep();
    }
}
