use crate::inner::save;
pub(crate) use crate::inner::load;

pub(crate) fn render() {
    let _ = std::fs::remove_dir("a");
}

pub fn page() {
    render();
    save();
    load();
}
