package com.shop

import org.yaml.snakeyaml.Yaml

class Loader(private val path: String) {
    fun load(): Any = Yaml().load(path)

    fun dump(value: Any): String = Yaml().dump(value)
}
