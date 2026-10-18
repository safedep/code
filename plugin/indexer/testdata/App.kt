package com.shop

import org.yaml.snakeyaml.Yaml
import com.shop.util.*

val parser = Yaml()

@SpringBootApplication
class App(private val store: Store) : Base(), Runner {
    override fun run(path: String) {
        store.save(parser.load(path))
        this.audit()
        helper()
    }

    private fun audit() = log.info("ran")

    companion object {
        private val log = Logger.getLogger("app")

        @JvmStatic
        fun main(args: Array<String>) {
            App(Store()).run(args[0])
        }
    }
}

interface Runner {
    fun run(path: String)
}

fun String.trimmed(): String = this.trim()
