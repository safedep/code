package com.shop.store

import java.nio.file.Files
import java.nio.file.Paths

class Store {
    fun save(key: String) {
        Files.write(Paths.get(key), byteArrayOf())
    }

    fun drop(key: String) {
        Files.delete(Paths.get(key))
    }
}

fun checkKey(key: String) = System.getenv(key)
