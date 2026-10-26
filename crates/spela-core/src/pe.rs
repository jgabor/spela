//! PE version reading: the file version of a DLL, formatted as Go formats it.
//!
//! This ports Go's scan (`internal/dll/detect.go`): take the first `.rsrc`
//! section, find the UTF-16LE text `VS_VERSION_INFO`, then the
//! `VS_FIXEDFILEINFO` signature `BD 04 EF FE` after it, and read the file
//! version (not the product version) 8 bytes past the signature. object
//! finds the section; only the section headers and `.rsrc` are read from
//! the file, so a 50 MiB DLL costs a few small reads.
//!
//! [`FileVersion`] prints as Go did: `major.minor` when build and revision
//! are 0, `major.minor.build` when only the revision is 0, and all four
//! parts otherwise. A version of 0.0 counts as none. The game cache and the
//! DLL manifest keep these strings; compare versions as numbers, because a
//! file at 310.7.0.0 prints as `310.7`.

use std::fmt;
use std::fs::File;
use std::io;
use std::path::Path;

use object::read::pe::{ImageNtHeaders, PeFile32, PeFile64};
use object::read::{ReadCache, ReadRef};
use object::{FileKind, LittleEndian};

/// A DLL's file version from its `VS_FIXEDFILEINFO`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct FileVersion {
    /// The major part.
    pub major: u16,
    /// The minor part.
    pub minor: u16,
    /// The build part.
    pub build: u16,
    /// The revision part.
    pub revision: u16,
}

impl FileVersion {
    /// The version from `dwFileVersionMS` and `dwFileVersionLS`, or `None`
    /// for 0.0, which Go treated as no version.
    pub fn from_ms_ls(ms: u32, ls: u32) -> Option<FileVersion> {
        let version = FileVersion {
            major: (ms >> 16) as u16,
            minor: ms as u16,
            build: (ls >> 16) as u16,
            revision: ls as u16,
        };
        (version.major != 0 || version.minor != 0).then_some(version)
    }
}

impl fmt::Display for FileVersion {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let FileVersion {
            major,
            minor,
            build,
            revision,
        } = *self;
        match (build, revision) {
            (0, 0) => write!(f, "{major}.{minor}"),
            (_, 0) => write!(f, "{major}.{minor}.{build}"),
            _ => write!(f, "{major}.{minor}.{build}.{revision}"),
        }
    }
}

/// A file whose version cannot be read.
#[derive(Debug, thiserror::Error)]
pub enum PeError {
    /// The file cannot be opened.
    #[error("cannot open {}: {source}", path.display())]
    Open {
        /// The file.
        path: std::path::PathBuf,
        /// Why.
        source: io::Error,
    },
    /// The file is not a PE file, or its headers or `.rsrc` are broken.
    #[error("{} is not a readable PE file: {message}", path.display())]
    NotPe {
        /// The file.
        path: std::path::PathBuf,
        /// What object said.
        message: String,
    },
}

/// The file version of a PE file. `Ok(None)` when it has no `.rsrc`
/// section or no version in it.
pub fn read_file_version(path: &Path) -> Result<Option<FileVersion>, PeError> {
    let file = File::open(path).map_err(|source| PeError::Open {
        path: path.to_owned(),
        source,
    })?;
    let cache = ReadCache::new(file);
    rsrc_version(&cache).map_err(|e| PeError::NotPe {
        path: path.to_owned(),
        message: e.0,
    })
}

/// The version string a scan stores, as Go's formatter writes it, or
/// `None` when the file has no readable version. Errors count as no
/// version, as in Go's scan, and are logged.
pub fn version_string(path: &Path) -> Option<String> {
    match read_file_version(path) {
        Ok(version) => version.map(|v| v.to_string()),
        Err(e) => {
            tracing::debug!("no version for {}: {e}", path.display());
            None
        }
    }
}

/// Bytes that are not a PE file, or whose headers or `.rsrc` are broken.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("{0}")]
pub struct NotPe(pub String);

impl From<object::Error> for NotPe {
    fn from(e: object::Error) -> NotPe {
        NotPe(e.to_string())
    }
}

/// The file version of PE bytes already in memory.
pub fn file_version_of(data: &[u8]) -> Result<Option<FileVersion>, NotPe> {
    rsrc_version(data)
}

fn rsrc_version<'data, R: ReadRef<'data>>(data: R) -> Result<Option<FileVersion>, NotPe> {
    let rsrc = match FileKind::parse(data)? {
        FileKind::Pe32 => rsrc_section(&PeFile32::parse(data)?, data)?,
        FileKind::Pe64 => rsrc_section(&PeFile64::parse(data)?, data)?,
        _ => return Err(NotPe("not a PE file".to_owned())),
    };
    Ok(rsrc.and_then(version_in_resources))
}

/// The raw bytes of the first `.rsrc` section, as Go's `Section.Data`
/// reads them: `SizeOfRawData` bytes at `PointerToRawData`.
fn rsrc_section<'data, Pe: ImageNtHeaders, R: ReadRef<'data>>(
    pe: &object::read::pe::PeFile<'data, Pe, R>,
    data: R,
) -> Result<Option<&'data [u8]>, NotPe> {
    let Some(header) = pe
        .section_table()
        .iter()
        .find(|s| s.raw_name().split(|&b| b == 0).next() == Some(b".rsrc".as_slice()))
    else {
        return Ok(None);
    };
    let offset = header.pointer_to_raw_data.get(LittleEndian);
    let size = header.size_of_raw_data.get(LittleEndian);
    data.read_bytes_at(offset.into(), size.into())
        .map(Some)
        .map_err(|()| NotPe("the .rsrc section is outside the file".to_owned()))
}

/// Go's search: `VS_VERSION_INFO` in UTF-16LE, then the fixed file info
/// signature after it, then the two version words.
fn version_in_resources(data: &[u8]) -> Option<FileVersion> {
    let key: Vec<u8> = "VS_VERSION_INFO"
        .encode_utf16()
        .flat_map(u16::to_le_bytes)
        .collect();
    let key_at = find(data, &key)?;
    let signature_at = key_at + find(&data[key_at..], &[0xBD, 0x04, 0xEF, 0xFE])?;
    let at = signature_at + 8;
    let word = |offset: usize| -> Option<u32> {
        Some(u32::from_le_bytes(
            data.get(offset..offset + 4)?.try_into().ok()?,
        ))
    };
    FileVersion::from_ms_ls(word(at)?, word(at + 4)?)
}

fn find(haystack: &[u8], needle: &[u8]) -> Option<usize> {
    haystack.windows(needle.len()).position(|w| w == needle)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn formats_like_go() {
        let v = |major, minor, build, revision| FileVersion {
            major,
            minor,
            build,
            revision,
        };
        assert_eq!(v(310, 5, 3, 0).to_string(), "310.5.3");
        assert_eq!(v(310, 7, 0, 0).to_string(), "310.7");
        assert_eq!(v(1, 0, 0, 0).to_string(), "1.0");
        assert_eq!(v(1, 3, 1, 33).to_string(), "1.3.1.33");
        assert_eq!(v(1, 3, 0, 33).to_string(), "1.3.0.33");
        assert_eq!(v(0, 1, 0, 0).to_string(), "0.1");
    }

    #[test]
    fn zero_dot_zero_is_no_version() {
        assert_eq!(FileVersion::from_ms_ls(0, 0x0001_0002), None);
        assert_eq!(
            FileVersion::from_ms_ls(0x0136_0005, 0x0003_0000),
            Some(FileVersion {
                major: 310,
                minor: 5,
                build: 3,
                revision: 0
            })
        );
    }

    fn resources(ms: u32, ls: u32) -> Vec<u8> {
        let mut data = vec![0u8; 6];
        data.extend("VS_VERSION_INFO".encode_utf16().flat_map(u16::to_le_bytes));
        data.extend([0, 0, 0xBD, 0x04, 0xEF, 0xFE, 0, 0, 1, 0]);
        data.extend(ms.to_le_bytes());
        data.extend(ls.to_le_bytes());
        data
    }

    #[test]
    fn finds_the_fixed_file_info_after_the_key() {
        let data = resources(0x0003_0007, 0x000A_0000);
        assert_eq!(version_in_resources(&data).unwrap().to_string(), "3.7.10");
        // A signature before the key does not count.
        let mut early = vec![0xBD, 0x04, 0xEF, 0xFE, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9];
        early.extend(resources(0x0001_0000, 0));
        assert_eq!(version_in_resources(&early).unwrap().to_string(), "1.0");
        // Cut short, or no key: no version.
        assert_eq!(version_in_resources(&data[..data.len() - 2]), None);
        assert_eq!(version_in_resources(&data[8..]), None);
    }

    #[test]
    fn a_file_that_is_not_pe_is_an_error() {
        assert!(file_version_of(b"not a PE file at all").is_err());
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("x.dll");
        std::fs::write(&path, b"MZ but nothing else").unwrap();
        assert!(read_file_version(&path).is_err());
        assert_eq!(version_string(&path), None);
        assert!(matches!(
            read_file_version(&dir.path().join("missing.dll")),
            Err(PeError::Open { .. })
        ));
    }
}
