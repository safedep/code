mod unix {
    pub struct Thing;

    impl Thing {
        pub fn open() -> Thing {
            let _ = std::fs::remove_file("x");
            Thing
        }

        pub fn close(&self) {
            let _ = std::process::Command::new("sync");
        }
    }
}

mod windows {
    pub struct Thing;

    impl Thing {
        pub fn open() -> Thing {
            Thing
        }

        pub fn close(&self) {}
    }
}

#[cfg(unix)]
type Imp = unix::Thing;

#[cfg(windows)]
type Imp = windows::Thing;

fn work(t: &Imp) {
    t.close();
}

fn main() {
    let t = Imp::open();
    work(&t);
}
