use kit::Sink;

pub fn all<S: Sink>(sink: &S) {
    sink.flush();
}
