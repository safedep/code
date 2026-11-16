mod config;

fn main() {
    let limit = config::LIMIT;
    config::load(limit);
    let mode = config::Mode::Fast;
    config::apply(mode);
}
