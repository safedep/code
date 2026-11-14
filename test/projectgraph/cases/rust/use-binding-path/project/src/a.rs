use crate::deep::helpers;
use helpers::run as go_run;
use helpers::Mode;

pub fn go() {
    go_run();
    Mode::start();
}
