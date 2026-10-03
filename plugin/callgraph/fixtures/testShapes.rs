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
    println!("{}", text);
    Ok(())
}
