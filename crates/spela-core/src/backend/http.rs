//! Blocking HTTP, run on a worker thread.
//!
//! The real backend is ureq in the `spela` binary. It honours the proxy
//! variables and sends `User-Agent: spela/<version>` (decision 41).

use std::time::Duration;

/// The `User-Agent` every request sends.
pub const USER_AGENT: &str = concat!("spela/", env!("CARGO_PKG_VERSION"));

/// Time to the response header for the manifest and DLLs, as Go.
pub const HEADER_TIMEOUT: Duration = Duration::from_secs(30);
/// Total time for the manifest and DLLs, as Go.
pub const TOTAL_TIMEOUT: Duration = Duration::from_secs(120);
/// The default body cap. A DLL is tens of MiB.
pub const DEFAULT_BODY_CAP: u64 = 256 * 1024 * 1024;
/// The manifest's body cap. Go read it without one; today's manifest is a
/// few KiB.
pub const MANIFEST_BODY_CAP: u64 = 4 * 1024 * 1024;
/// The release check's timeout, for the header and the whole request, as Go.
pub const RELEASE_TIMEOUT: Duration = Duration::from_secs(5);
/// The release check's body cap, as Go.
pub const RELEASE_BODY_CAP: u64 = 1024 * 1024;

/// One GET.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HttpRequest {
    /// What to fetch.
    pub uri: http::Uri,
    /// The `Accept` header, if any.
    pub accept: Option<String>,
    /// Time allowed until the response header.
    pub header_timeout: Duration,
    /// Time allowed for the whole request.
    pub total_timeout: Duration,
    /// The most body bytes accepted.
    pub body_cap: u64,
}

impl HttpRequest {
    /// A request with Go's manifest and DLL timeouts and the default cap.
    pub fn new(uri: http::Uri) -> Self {
        HttpRequest {
            uri,
            accept: None,
            header_timeout: HEADER_TIMEOUT,
            total_timeout: TOTAL_TIMEOUT,
            body_cap: DEFAULT_BODY_CAP,
        }
    }

    /// Changes both timeouts.
    pub fn with_timeouts(mut self, header: Duration, total: Duration) -> Self {
        self.header_timeout = header;
        self.total_timeout = total;
        self
    }

    /// Changes the body cap.
    pub fn with_body_cap(mut self, cap: u64) -> Self {
        self.body_cap = cap;
        self
    }

    /// Sets the `Accept` header.
    pub fn with_accept(mut self, accept: &str) -> Self {
        self.accept = Some(accept.to_owned());
        self
    }
}

/// A finished response. The body went to the caller's writer.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HttpResponse {
    /// The status code, always 2xx: other codes are [`HttpError::Status`].
    pub status: u16,
    /// Body bytes written.
    pub bytes: u64,
    /// The raw `Link` header, if the server sent one. The release check
    /// follows its `next` page.
    pub link: Option<String>,
}

/// Why a request failed.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum HttpError {
    /// The server answered with a status that is not 2xx.
    #[error("the server answered HTTP {0}")]
    Status(u16),
    /// A timeout ran out.
    #[error("the request timed out")]
    Timeout,
    /// The body is larger than the cap.
    #[error("the response is larger than {cap} bytes")]
    TooLarge {
        /// The cap that was exceeded.
        cap: u64,
    },
    /// DNS, connect, TLS or a broken connection.
    #[error("network error: {0}")]
    Network(String),
    /// The caller's writer failed.
    #[error("cannot save the response: {0}")]
    Write(String),
}

/// Blocking GET with progress. Implementations are shared between threads.
pub trait Http: Send + Sync {
    /// Streams the body into `body`. `progress` gets the bytes so far and
    /// the total when the server sent one.
    fn get(
        &self,
        request: &HttpRequest,
        body: &mut dyn std::io::Write,
        progress: &mut dyn FnMut(u64, Option<u64>),
    ) -> Result<HttpResponse, HttpError>;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn user_agent_names_the_version() {
        assert_eq!(USER_AGENT, format!("spela/{}", crate::VERSION));
    }

    #[test]
    fn builders_set_fields() {
        let uri: http::Uri = "https://example.invalid/manifest.json".parse().unwrap();
        let req = HttpRequest::new(uri.clone());
        assert_eq!(req.header_timeout, HEADER_TIMEOUT);
        assert_eq!(req.total_timeout, TOTAL_TIMEOUT);
        let req = req
            .with_timeouts(Duration::from_secs(5), Duration::from_secs(5))
            .with_body_cap(1024 * 1024)
            .with_accept("application/json");
        assert_eq!(req.uri, uri);
        assert_eq!(req.total_timeout, Duration::from_secs(5));
        assert_eq!(req.body_cap, 1024 * 1024);
        assert_eq!(req.accept.as_deref(), Some("application/json"));
    }
}
