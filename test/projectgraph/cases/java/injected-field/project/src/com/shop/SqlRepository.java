package com.shop;

import java.sql.DriverManager;

public class SqlRepository implements Repository {
    @Override
    public void purge(String tag) {
        DriverManager.getConnection(tag);
    }
}
