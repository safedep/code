#[macro_export]
macro_rules! wipe {
    () => {
        $crate::sink()
    };
}
