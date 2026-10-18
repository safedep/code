package com.shop.net

val client = java.net.http.HttpClient.newHttpClient()

class Money(val v: Int) {
    operator fun plus(o: Money): Money {
        Math.addExact(v, o.v)
        return this
    }
}
