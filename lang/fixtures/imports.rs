use std::collections::HashMap;
use serde::{Deserialize, Serialize as Ser};
use reqwest::Client as Http;
use tokio::*;
use async_openai::{self, types::{CreateChatCompletionRequestArgs}};
extern crate log;
extern crate rand as random;

mod utils;

#[tokio::main]
async fn main() {
    let body = serde_json::to_string(&HashMap::<String, String>::new()).unwrap();
    let again = serde_json::from_str::<Value>(&body);
    let local = utils::helper();
    let items: Vec<String> = Vec::new();
    crate::config::load();
}

fn helper(x: i32) -> i32 { x }

mod nested {
    use std::{io::{self, Read}, fmt};

    fn read(r: impl Read) -> io::Result<usize> {
        Ok(usize::MAX)
    }
}
