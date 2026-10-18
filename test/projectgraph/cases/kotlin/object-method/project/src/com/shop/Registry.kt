package com.shop

import java.util.logging.Logger

object Registry {
    private val log = Logger.getLogger("registry")

    fun register(name: String) {
        log.info(name)
    }
}

class Service {
    companion object {
        fun create(): Service = Service()

        fun unused() = System.exit(1)
    }
}
