//! SHA-256 digests as lowercase hex.
//!
//! sha2 0.11's output has no `LowerHex`, so `format!("{:x}", digest)` does
//! not compile. Every digest spela prints, stores or compares goes through
//! these helpers instead.

use std::io::{self, Read};

use sha2::{Digest, Sha256};

/// The length of a SHA-256 digest in hex digits.
pub const SHA256_HEX_LEN: usize = 64;

/// Lowercase hex of any bytes.
pub fn encode(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut out = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        out.push(char::from(DIGITS[usize::from(b >> 4)]));
        out.push(char::from(DIGITS[usize::from(b & 0x0f)]));
    }
    out
}

/// The SHA-256 of `bytes`, as lowercase hex.
pub fn sha256_hex(bytes: &[u8]) -> String {
    encode(&Sha256::digest(bytes))
}

/// The SHA-256 of everything `reader` yields, as lowercase hex.
pub fn sha256_reader(mut reader: impl Read) -> io::Result<String> {
    let mut hasher = Sha256::new();
    let mut buf = [0u8; 64 * 1024];
    loop {
        match reader.read(&mut buf) {
            Ok(0) => break,
            Ok(n) => hasher.update(&buf[..n]),
            Err(e) if e.kind() == io::ErrorKind::Interrupted => {}
            Err(e) => return Err(e),
        }
    }
    Ok(encode(&hasher.finalize()))
}

/// Parses a SHA-256 hex string, in either case, into its 32 bytes. Anything
/// that is not exactly 64 hex digits is `None`.
pub fn parse_sha256(hex: &str) -> Option<[u8; 32]> {
    let bytes = hex.as_bytes();
    if bytes.len() != SHA256_HEX_LEN {
        return None;
    }
    let mut out = [0u8; 32];
    let (pairs, _) = bytes.as_chunks::<2>();
    for (slot, [high, low]) in out.iter_mut().zip(pairs) {
        *slot = (digit(*high)? << 4) | digit(*low)?;
    }
    Some(out)
}

/// Whether `hex` is the SHA-256 of `bytes`, ignoring case.
pub fn sha256_matches(bytes: &[u8], hex: &str) -> bool {
    parse_sha256(hex).is_some_and(|want| Sha256::digest(bytes).as_slice() == want)
}

fn digit(c: u8) -> Option<u8> {
    match c {
        b'0'..=b'9' => Some(c - b'0'),
        b'a'..=b'f' => Some(c - b'a' + 10),
        b'A'..=b'F' => Some(c - b'A' + 10),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const EMPTY: &str = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855";
    const ABC: &str = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad";

    #[test]
    fn known_digests() {
        assert_eq!(sha256_hex(b""), EMPTY);
        assert_eq!(sha256_hex(b"abc"), ABC);
        assert_eq!(sha256_reader(&b"abc"[..]).unwrap(), ABC);
    }

    #[test]
    fn reader_matches_one_shot_over_many_chunks() {
        let data = vec![7u8; 200_000];
        assert_eq!(sha256_reader(&data[..]).unwrap(), sha256_hex(&data));
    }

    #[test]
    fn encode_pads_each_byte() {
        assert_eq!(encode(&[0x00, 0x0f, 0xa0, 0xff]), "000fa0ff");
        assert_eq!(encode(&[]), "");
    }

    #[test]
    fn parse_round_trips_and_ignores_case() {
        let bytes = parse_sha256(ABC).unwrap();
        assert_eq!(encode(&bytes), ABC);
        assert_eq!(parse_sha256(&ABC.to_uppercase()), Some(bytes));
        assert!(sha256_matches(b"abc", &ABC.to_uppercase()));
        assert!(!sha256_matches(b"abd", ABC));
    }

    #[test]
    fn parse_refuses_bad_input() {
        assert_eq!(parse_sha256(""), None);
        assert_eq!(parse_sha256(&ABC[..63]), None);
        assert_eq!(parse_sha256(&format!("{ABC}0")), None);
        assert_eq!(parse_sha256(&ABC.replace('a', "g")), None);
        // Multi-byte characters never panic on the byte split.
        assert_eq!(parse_sha256(&"é".repeat(32)), None);
        assert!(!sha256_matches(b"abc", "not hex"));
    }
}
