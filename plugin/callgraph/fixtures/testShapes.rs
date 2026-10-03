use async_openai::{Client, types::CreateChatCompletionRequestArgs};
use reqwest::Client as Http;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new();
    let request = CreateChatCompletionRequestArgs::default().model("gpt-4o").build()?;
    let response = client.chat().create(request).await?;
    let http = Http::new();
    let body = http.get("https://example.com").send().await?;
    let text = serde_json::to_string(&body)?;
    let positions = candle_core::Tensor::arange(0.0, 8.0, &device)?.unsqueeze(1)?;
    println!("{}", text);
    Ok(())
}
