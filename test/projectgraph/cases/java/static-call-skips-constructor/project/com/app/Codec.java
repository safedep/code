package com.app;

import java.util.logging.Logger;
import org.yaml.snakeyaml.Yaml;

class Codec {
    static final Logger LOG = Logger.getLogger("codec");

    Codec() {
        Yaml yaml = new Yaml();
        yaml.load("a: 1");
    }

    static String name() {
        return "codec";
    }
}
