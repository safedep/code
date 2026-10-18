package com.shop

interface Store {
    fun save(key: String)
}

class DbStore : Store {
    override fun save(key: String) {
        java.sql.DriverManager.getConnection(key)
    }
}

class FileStore : Store {
    override fun save(key: String) {
        java.io.File(key).writeText("x")
    }
}
