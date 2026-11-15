pub fn go() {
    log::info("x");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn runs() {
        go();
    }
}
