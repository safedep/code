package com.shop

abstract class Step {
    fun run() {
        parse()
    }

    protected abstract fun parse()
}
