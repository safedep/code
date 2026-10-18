package com.shop.text

fun String.shout(): String = java.net.URLEncoder.encode(this, "UTF-8")

fun String.whisper(): String = java.net.URLDecoder.decode(this, "UTF-8")
