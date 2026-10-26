//! Small PE images with a file version, for `spela --demo` and tests: a
//! port of the Phase 0 generator's `peStub` (`tools/goref/pe.go`).
//!
//! An image is a PE32+ DLL with no code and one `.rsrc` section holding a
//! real resource tree with an `RT_VERSION` entry, so [`crate::pe`] reads its
//! version as it reads a real DLL's. The DOS stub says it is not a real
//! DLL. No vendor DLL is ever shipped or embedded.

/// Written into the DOS stub of every image.
pub const NOTE: &str = "spela demo PE stub, not a real DLL";

/// An image whose file version is the dotted `version`, up to four parts.
/// Parts that do not parse read as 0.
pub fn stub(version: &str) -> Vec<u8> {
    let mut parts = [0u16; 4];
    for (slot, part) in parts.iter_mut().zip(version.split('.')) {
        *slot = part.parse().unwrap_or(0);
    }
    image(parts)
}

fn put16(buf: &mut [u8], at: usize, v: u16) {
    buf[at..at + 2].copy_from_slice(&v.to_le_bytes());
}

fn put32(buf: &mut [u8], at: usize, v: u32) {
    buf[at..at + 4].copy_from_slice(&v.to_le_bytes());
}

fn put64(buf: &mut [u8], at: usize, v: u64) {
    buf[at..at + 8].copy_from_slice(&v.to_le_bytes());
}

fn image([major, minor, build, revision]: [u16; 4]) -> Vec<u8> {
    const PE_OFFSET: usize = 0x80;
    const FILE_ALIGNMENT: usize = 0x200;
    const SECTION_RVA: u32 = 0x1000;
    const SECTION_ALIGNMENT: u32 = 0x1000;
    const OPTIONAL_SIZE: usize = 240;

    let version = version_resource(major, minor, build, revision);
    // Resource tree: type -> name -> language -> data entry -> blob.
    let mut rsrc = Vec::with_capacity(256);
    let mut directory = |id: u32, offset: u32, subdirectory: bool| {
        let mut header = [0u8; 16];
        put16(&mut header, 14, 1);
        rsrc.extend_from_slice(&header);
        let mut entry = [0u8; 8];
        put32(&mut entry, 0, id);
        put32(
            &mut entry,
            4,
            if subdirectory {
                offset | 0x8000_0000
            } else {
                offset
            },
        );
        rsrc.extend_from_slice(&entry);
    };
    directory(16, 24, true); // RT_VERSION
    directory(1, 48, true);
    directory(0x409, 72, false); // en-US: the data entry at 72
    let mut data_entry = [0u8; 16];
    put32(&mut data_entry, 0, SECTION_RVA + 88);
    put32(&mut data_entry, 4, version.len() as u32);
    rsrc.extend_from_slice(&data_entry);
    rsrc.extend_from_slice(&version);

    let raw_size = rsrc.len().div_ceil(FILE_ALIGNMENT) * FILE_ALIGNMENT;
    let mut out = vec![0u8; FILE_ALIGNMENT + raw_size];
    out[0..2].copy_from_slice(b"MZ");
    put32(&mut out, 0x3c, PE_OFFSET as u32);
    out[0x40..0x40 + NOTE.len()].copy_from_slice(NOTE.as_bytes());

    let mut at = PE_OFFSET;
    out[at..at + 4].copy_from_slice(b"PE\0\0");
    at += 4;
    put16(&mut out, at, 0x8664); // AMD64
    put16(&mut out, at + 2, 1); // one section
    put16(&mut out, at + 16, OPTIONAL_SIZE as u16);
    put16(&mut out, at + 18, 0x2022); // executable, large address aware, DLL
    at += 20;

    let o = at;
    put16(&mut out, o, 0x20b); // PE32+
    put64(&mut out, o + 24, 0x1_8000_0000); // image base
    put32(&mut out, o + 32, SECTION_ALIGNMENT);
    put32(&mut out, o + 36, FILE_ALIGNMENT as u32);
    put16(&mut out, o + 40, 6); // OS major
    put16(&mut out, o + 48, 6); // subsystem major
    put32(&mut out, o + 56, SECTION_RVA + SECTION_ALIGNMENT); // image size
    put32(&mut out, o + 60, FILE_ALIGNMENT as u32); // headers size
    put16(&mut out, o + 68, 2); // Windows GUI
    put32(&mut out, o + 108, 16); // data directories
    put32(&mut out, o + 112 + 2 * 8, SECTION_RVA); // the resource table
    put32(&mut out, o + 112 + 2 * 8 + 4, rsrc.len() as u32);
    at += OPTIONAL_SIZE;

    out[at..at + 5].copy_from_slice(b".rsrc");
    put32(&mut out, at + 8, rsrc.len() as u32);
    put32(&mut out, at + 12, SECTION_RVA);
    put32(&mut out, at + 16, raw_size as u32);
    put32(&mut out, at + 20, FILE_ALIGNMENT as u32);
    put32(&mut out, at + 36, 0x4000_0040); // initialized data, readable

    out[FILE_ALIGNMENT..FILE_ALIGNMENT + rsrc.len()].copy_from_slice(&rsrc);
    out
}

/// A `VS_VERSIONINFO` block with only its `VS_FIXEDFILEINFO`.
fn version_resource(major: u16, minor: u16, build: u16, revision: u16) -> Vec<u8> {
    let mut block = vec![0u8; 6];
    for unit in "VS_VERSION_INFO\0".encode_utf16() {
        block.extend_from_slice(&unit.to_le_bytes());
    }
    while !block.len().is_multiple_of(4) {
        block.push(0);
    }
    let ms = u32::from(major) << 16 | u32::from(minor);
    let ls = u32::from(build) << 16 | u32::from(revision);
    for value in [
        0xFEEF_04BD,
        0x0001_0000, // signature, structure version
        ms,
        ls, // file version
        ms,
        ls, // product version
        0x3F,
        0, // flags mask, flags
        0x4_0004,
        2,
        0, // NT Windows32, DLL, no subtype
        0,
        0, // date
    ] {
        block.extend_from_slice(&u32::to_le_bytes(value));
    }
    let len = block.len() as u16;
    put16(&mut block, 0, len);
    put16(&mut block, 2, 52);
    block
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_version_reader_sees_the_stub_s_version() {
        for (version, read) in [
            ("310.5.3", "310.5.3"),
            ("310.2.1", "310.2.1"),
            ("2.0.1", "2.0.1"),
            ("1.3.1.33", "1.3.1.33"),
            ("3.7", "3.7"),
        ] {
            let data = stub(version);
            let got = crate::pe::file_version_of(&data).unwrap().unwrap();
            assert_eq!(got.to_string(), read);
        }
        assert_eq!(crate::pe::file_version_of(&stub("0.0")).unwrap(), None);
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("nvngx_dlss.dll");
        std::fs::write(&path, stub("310.4.0")).unwrap();
        assert_eq!(crate::pe::version_string(&path).as_deref(), Some("310.4"));
    }
}
