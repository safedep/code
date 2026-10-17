package com.shop.app

import com.shop.store.Store
import com.shop.store.checkKey

fun main() {
    checkKey("k")
    Store().save("k")
}
