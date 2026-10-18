use actix_web::{get, HttpResponse};

#[get("/users")]
pub async fn list_users() -> HttpResponse {
    let out = std::process::Command::new("ls").output().unwrap();
    HttpResponse::Ok().body(out.stdout)
}

pub async fn health() -> HttpResponse {
    std::env::var("HEALTH").unwrap();
    HttpResponse::Ok().finish()
}
