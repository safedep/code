package com.shop

class View {
    fun render() = 1
}

fun View.paint() = java.lang.Runtime.getRuntime()

fun main(args: Array<String>) {
    val danger = { 1 }
    danger()
    View().render()
    val v: View = View()
    v.render()
    Shadow().run()
}

fun danger() = java.lang.ProcessBuilder("x").start()

fun View.render() = java.nio.file.Files.delete(null)

class Shadow {
    fun run() {
        val exec = 1
        helper(exec)
    }
    fun helper(x: Int) {}
    fun exec() = java.lang.System.exit(1)
}
