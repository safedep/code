package com.shop

fun persist(store: Store, key: String) {
    store.save(key)
}

fun main(args: Array<String>) {
    persist(DbStore(), args[0])
}
