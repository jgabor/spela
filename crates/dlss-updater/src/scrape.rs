//! The TechPowerUp download pages and what is read from them.
//!
//! Each DLSS family has a download page. Its `<title>` names the newest
//! version, and its download form carries the file id in a hidden `id`
//! input. Posting that id with `server_id=27` (TechPowerUp's NL server)
//! answers with a 302 whose `Location` is the download. Go's
//! `tools/dlss-updater` read the same three things, with the same patterns.

use regex_lite::Regex;
use spela_core::dll::detect::family;
use spela_core::dll::version;
use spela_core::games::DllType;

/// The `User-Agent` of every request: Firefox, as the Go tool sent.
pub const USER_AGENT: &str =
    "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0";

/// The `source` every result names.
pub const SOURCE_NAME: &str = "techpowerup.com";

/// The download server the form is posted with.
pub const SERVER_ID: &str = "27";

/// One family's download page.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Source {
    /// The family.
    pub dll_type: DllType,
    /// The download page. The form is posted back to it.
    pub page_url: String,
    /// The words before the version in the page title, such as
    /// `NVIDIA DLSS DLL`.
    pub title: String,
}

impl Source {
    /// The DLL's file name in a game folder, such as `nvngx_dlssg.dll`.
    pub fn filename(&self) -> &'static str {
        family(self.dll_type).filename
    }

    /// The pattern that reads the version from the title.
    fn title_pattern(&self) -> Regex {
        let title = regex_lite::escape(&self.title);
        Regex::new(&format!(r"<title>{title} (\d+\.\d+\.\d+) Download"))
            .expect("an escaped title is a valid pattern")
    }
}

/// The families the updater checks, in the matrix order of
/// `dll-manifest.yml`. XeSS and FSR have no source.
pub fn sources() -> Vec<Source> {
    let source = |dll_type, page: &str, title: &str| Source {
        dll_type,
        page_url: format!("https://www.techpowerup.com/download/{page}/"),
        title: title.to_owned(),
    };
    vec![
        source(DllType::Dlss, "nvidia-dlss-dll", "NVIDIA DLSS DLL"),
        source(
            DllType::Dlssg,
            "nvidia-dlss-3-frame-generation-dll",
            "NVIDIA DLSS Frame Generation DLL",
        ),
        source(
            DllType::Dlssd,
            "nvidia-dlss-3-ray-reconstruction-dll",
            "NVIDIA DLSS Ray Reconstruction DLL",
        ),
    ]
}

/// The source of a family word such as `dlssg`.
pub fn find_source(name: &str) -> Option<Source> {
    sources().into_iter().find(|s| s.dll_type.as_str() == name)
}

/// What a download page says.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Page {
    /// The newest version, from the title.
    pub version: String,
    /// The file id the download form posts.
    pub file_id: String,
}

/// Why the newest version could not be found. The texts are Go's.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum FetchError {
    /// The page answered with a status that is not 200.
    #[error("HTTP {0}")]
    Status(u16),
    /// DNS, connect, TLS, a timeout or a broken connection.
    #[error("{0}")]
    Network(String),
    /// The title names no version.
    #[error("could not find version in page title")]
    NoVersion,
    /// The page has no file id.
    #[error("could not find file ID")]
    NoFileId,
    /// The title's version could not name a manifest entry.
    #[error("version {0:?} from the page title is not 1 to 4 numbers joined by dots")]
    BadVersion(String),
    /// The download form did not answer with a redirect.
    #[error("could not get download URL: {0}")]
    DownloadUrl(String),
}

/// Reads the version and the file id from a download page.
pub fn read_page(source: &Source, html: &str) -> Result<Page, FetchError> {
    let version = source
        .title_pattern()
        .captures(html)
        .map(|c| c[1].to_owned())
        .ok_or(FetchError::NoVersion)?;
    if !version::is_valid(&version) {
        return Err(FetchError::BadVersion(version));
    }
    let id =
        Regex::new(r#"<input type="hidden" name="id" value="(\d+)""#).expect("a valid pattern");
    let file_id = id
        .captures(html)
        .map(|c| c[1].to_owned())
        .ok_or(FetchError::NoFileId)?;
    Ok(Page { version, file_id })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn three_sources_with_go_s_pages_and_the_real_file_names() {
        let found: Vec<(&str, String, &str)> = sources()
            .iter()
            .map(|s| (s.dll_type.as_str(), s.page_url.clone(), s.filename()))
            .collect();
        assert_eq!(
            found,
            [
                (
                    "dlss",
                    "https://www.techpowerup.com/download/nvidia-dlss-dll/".to_owned(),
                    "nvngx_dlss.dll"
                ),
                (
                    "dlssg",
                    "https://www.techpowerup.com/download/nvidia-dlss-3-frame-generation-dll/"
                        .to_owned(),
                    "nvngx_dlssg.dll"
                ),
                (
                    "dlssd",
                    "https://www.techpowerup.com/download/nvidia-dlss-3-ray-reconstruction-dll/"
                        .to_owned(),
                    "nvngx_dlssd.dll"
                ),
            ]
        );
        for name in ["dlss", "dlssg", "dlssd"] {
            assert_eq!(find_source(name).unwrap().dll_type.as_str(), name);
        }
        for name in ["xess", "fsr", "unknown", "", "DLSS"] {
            assert_eq!(find_source(name), None, "{name}");
        }
    }

    #[test]
    fn the_title_and_the_hidden_input_give_the_version_and_id() {
        let source = find_source("dlssg").unwrap();
        let html = r#"<title>NVIDIA DLSS Frame Generation DLL 310.5.3 Download | TechPowerUp</title>
            <form method="post"><input type="hidden" name="id" value="5321"></form>"#;
        assert_eq!(
            read_page(&source, html),
            Ok(Page {
                version: "310.5.3".into(),
                file_id: "5321".into()
            })
        );
        // Another family's title is not this one's.
        assert_eq!(
            read_page(&find_source("dlss").unwrap(), html),
            Err(FetchError::NoVersion)
        );
    }

    #[test]
    fn a_page_without_either_part_fails_with_go_s_text() {
        let source = find_source("dlss").unwrap();
        let no_title = r#"<input type="hidden" name="id" value="1">"#;
        assert_eq!(
            read_page(&source, no_title).unwrap_err().to_string(),
            "could not find version in page title"
        );
        let no_id = "<title>NVIDIA DLSS DLL 1.2.3 Download</title>";
        assert_eq!(
            read_page(&source, no_id).unwrap_err().to_string(),
            "could not find file ID"
        );
        let huge = "<title>NVIDIA DLSS DLL 1.2.99999999999 Download</title>";
        assert_eq!(
            read_page(&source, huge),
            Err(FetchError::BadVersion("1.2.99999999999".into()))
        );
    }

    #[test]
    fn title_words_are_literal_text() {
        let source = Source {
            dll_type: DllType::Dlss,
            page_url: String::new(),
            title: "Fixture (beta) DLL.".into(),
        };
        let html = r#"<title>Fixture (beta) DLL. 3.8.10 Download</title><input type="hidden" name="id" value="9">"#;
        assert_eq!(read_page(&source, html).unwrap().version, "3.8.10");
        let other = r#"<title>Fixture xbetax DLLx 3.8.10 Download</title><input type="hidden" name="id" value="9">"#;
        assert_eq!(read_page(&source, other), Err(FetchError::NoVersion));
    }
}
