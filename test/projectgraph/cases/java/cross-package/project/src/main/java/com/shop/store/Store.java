package com.shop.store;

import org.yaml.snakeyaml.Yaml;

public class Store {
    public Store() {
        Codec.check();
    }

    public void save(String name) {
        new Yaml().dump(name);
    }

    public void load(String name) {
        new Yaml().load(name);
    }
}
