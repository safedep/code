package com.shop

import com.shop.net.client
import com.shop.net.Money

fun process(s: String) {
    ProcessBuilder(s).start()
}

fun main(args: Array<String>) {
    args.forEach(::process)
    println("${java.net.InetAddress.getByName(args[0])}")
    val a = Money(1) + Money(2)
    client.send(null, null)
    Config.get()
    with(args[0]) { java.nio.file.Files.delete(null) }
    Holder.make()
}

object Config {
    init { System.loadLibrary("x") }
    fun get() = 1
}

class Holder {
    companion object {
        private val seed = java.util.Random(42)
        fun make() = Holder()
    }
}
