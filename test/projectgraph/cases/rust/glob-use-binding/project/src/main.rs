mod jobs;
mod store;

fn main() {
    jobs::worker::go();
}
