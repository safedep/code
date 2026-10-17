package com.shop.app;

import com.shop.store.Store;

public class Main {
    public static void main(String[] args) {
        Store store = new Store();
        store.save("order");
    }
}
