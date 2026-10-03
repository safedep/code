use hmac::{Hmac, Mac};
use sha2::Sha256;

fn sign(key: &[u8]) {
    let mac = Hmac::<Sha256>::new_from_slice(key);
}
