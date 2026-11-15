pub use self::inner::*;

mod inner {
    pub use self::mac::*;
    pub use self::unix::*;

    mod mac {
        use libc::mach::*;
        use libc::time::*;

        pub fn ticks() -> u64 {
            mach_absolute_time()
        }
    }

    mod unix {
        use libc::clock::*;
        use libc::posix::*;
        use libc::signal::*;
        use libc::stat::*;
        use libc::types::*;
        use libc::wait::*;

        pub fn now() {
            clock_gettime();
            let _ = std::fs::remove_file("x");
        }
    }
}
