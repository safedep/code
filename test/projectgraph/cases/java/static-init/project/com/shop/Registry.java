package com.shop;

class Registry {
    static final Registry INSTANCE = new Registry();

    Registry() {
        java.rmi.registry.LocateRegistry.getRegistry();
    }

    void lookup(String name) {}
}
