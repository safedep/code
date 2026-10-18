mod report;
mod upload;

use report::Report;

fn main() {
    let r = Report::new("q1");
    r.publish();
}
