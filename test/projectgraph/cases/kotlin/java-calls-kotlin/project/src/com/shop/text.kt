package com.shop

fun clean(s: String): String = java.text.Normalizer.normalize(s, java.text.Normalizer.Form.NFC)
