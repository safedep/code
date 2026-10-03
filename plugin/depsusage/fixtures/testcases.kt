package app

import okhttp3.OkHttpClient
import com.openai.client.okhttp.OpenAIOkHttpClient as OpenAIClient
import kotlinx.coroutines.*

fun main() {
    val client = OkHttpClient.Builder().build()
}

class Service {
    fun run(prompt: String): String = prompt
}
