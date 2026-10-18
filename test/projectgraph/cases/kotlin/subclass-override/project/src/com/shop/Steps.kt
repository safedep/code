package com.shop

import java.nio.file.Files

open class YamlStep : Step() {
    override fun parse() {
        org.yaml.snakeyaml.Yaml().load("a: 1")
    }
}

class ZipStep : YamlStep() {
    override fun parse() {
        Files.readAllBytes(null)
    }
}

fun pick(name: String): Step = YamlStep()
