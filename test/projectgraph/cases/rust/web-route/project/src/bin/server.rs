use actix_web::{web, App, HttpServer};
use web_app::handlers;

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    HttpServer::new(|| {
        App::new()
            .service(handlers::list_users)
            .route("/health", web::get().to(handlers::health))
    })
    .bind("0.0.0.0:8080")?
    .run()
    .await
}
