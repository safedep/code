pub type Handle = kit::Store;

pub fn run() {
    keep(&Handle::open());
}

fn keep(h: &Handle) {
    h.save();
}
