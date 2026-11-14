pub fn render() {}

#[macro_export]
macro_rules! render_now {
    () => {
        $crate::render()
    };
}
