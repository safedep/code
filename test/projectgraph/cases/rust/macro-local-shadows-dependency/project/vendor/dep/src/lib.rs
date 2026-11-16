#[macro_export]
macro_rules! log {
    () => {
        $crate::sink()
    };
}

pub fn sink() {
    let _ = std::fs::remove_file("a");
}
