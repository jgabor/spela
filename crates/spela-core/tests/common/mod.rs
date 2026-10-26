//! What the fixture tests in this crate share: where Go's fixtures are, and
//! how a case is copied into a temp folder.
//!
//! Go writes absolute paths, so every text file in `tests/fixtures/go` has
//! the case folder replaced by the token `/fixture-root` (the fixtures
//! README). A copy puts the temp folder back in its place. Only text is
//! rewritten: a file with a NUL byte or that is not UTF-8, such as a PE
//! stub, is copied byte for byte.

#![allow(dead_code)]

use std::path::{Path, PathBuf};

/// The token Go's absolute paths carry in the fixtures.
pub const FIXTURE_ROOT: &str = "/fixture-root";

/// `tests/fixtures/go`.
pub fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../tests/fixtures/go")
}

/// Copies the folder `from` into `to`, replacing [`FIXTURE_ROOT`] with
/// `root` in text files.
pub fn copy_fixture(from: &Path, to: &Path, root: &str) {
    std::fs::create_dir_all(to).unwrap();
    for entry in std::fs::read_dir(from).unwrap() {
        let entry = entry.unwrap();
        let target = to.join(entry.file_name());
        let kind = entry.file_type().unwrap();
        if kind.is_dir() {
            copy_fixture(&entry.path(), &target, root);
            continue;
        }
        let bytes = std::fs::read(entry.path()).unwrap();
        let bytes = match String::from_utf8(bytes) {
            Ok(text) if !text.contains('\0') => text.replace(FIXTURE_ROOT, root).into_bytes(),
            Ok(text) => text.into_bytes(),
            Err(e) => e.into_bytes(),
        };
        std::fs::write(&target, bytes).unwrap();
    }
}
