//! HTTP from a map of URLs, in process. No server, no socket.

use std::collections::BTreeMap;
use std::sync::{Arc, Mutex, MutexGuard};

use crate::backend::http::{Http, HttpError, HttpRequest, HttpResponse};

/// What the fake answers for one URL.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FakeRoute {
    /// A status and a body. A status that is not 2xx is
    /// [`HttpError::Status`].
    Body {
        /// The status code.
        status: u16,
        /// The body.
        body: Vec<u8>,
    },
    /// The request fails with this error.
    Fault(HttpError),
}

#[derive(Debug, Default)]
struct HttpState {
    routes: BTreeMap<String, FakeRoute>,
    requests: Vec<HttpRequest>,
}

/// Answers from a route map and records every request. A URL with no route
/// is a network error, so a test never reaches a real host by accident.
#[derive(Debug, Clone, Default)]
pub struct FakeHttp {
    state: Arc<Mutex<HttpState>>,
}

/// The chunk size the fake streams in, so progress gets several calls.
const CHUNK: usize = 16 * 1024;

impl FakeHttp {
    /// A fake with no routes.
    pub fn new() -> Self {
        Self::default()
    }

    /// Answers `url` with 200 and `body`.
    pub fn serve(&self, url: &str, body: impl Into<Vec<u8>>) -> &Self {
        self.route(
            url,
            FakeRoute::Body {
                status: 200,
                body: body.into(),
            },
        )
    }

    /// Sets the answer for `url`.
    pub fn route(&self, url: &str, route: FakeRoute) -> &Self {
        self.lock().routes.insert(url.to_owned(), route);
        self
    }

    /// Every request so far, in order.
    pub fn requests(&self) -> Vec<HttpRequest> {
        self.lock().requests.clone()
    }

    fn lock(&self) -> MutexGuard<'_, HttpState> {
        self.state.lock().unwrap_or_else(|e| e.into_inner())
    }
}

impl Http for FakeHttp {
    fn get(
        &self,
        request: &HttpRequest,
        body: &mut dyn std::io::Write,
        progress: &mut dyn FnMut(u64, Option<u64>),
    ) -> Result<HttpResponse, HttpError> {
        let url = request.uri.to_string();
        let route = {
            let mut s = self.lock();
            s.requests.push(request.clone());
            s.routes.get(&url).cloned()
        };
        let (status, bytes) = match route {
            None => return Err(HttpError::Network(format!("no fake route for {url}"))),
            Some(FakeRoute::Fault(e)) => return Err(e),
            Some(FakeRoute::Body { status, body }) => (status, body),
        };
        if !(200..300).contains(&status) {
            return Err(HttpError::Status(status));
        }
        let total = bytes.len() as u64;
        if total > request.body_cap {
            return Err(HttpError::TooLarge {
                cap: request.body_cap,
            });
        }
        let mut sent = 0u64;
        for chunk in bytes.chunks(CHUNK) {
            body.write_all(chunk)
                .map_err(|e| HttpError::Write(e.to_string()))?;
            sent += chunk.len() as u64;
            progress(sent, Some(total));
        }
        Ok(HttpResponse {
            status,
            bytes: sent,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn request(url: &str) -> HttpRequest {
        HttpRequest::new(url.parse().unwrap())
    }

    #[test]
    fn serves_bodies_with_progress() {
        let http = FakeHttp::new();
        let body = vec![1u8; 40_000];
        http.serve("https://example.invalid/a.dll", body.clone());
        let mut out = Vec::new();
        let mut seen = Vec::new();
        let resp = http
            .get(
                &request("https://example.invalid/a.dll"),
                &mut out,
                &mut |n, t| seen.push((n, t)),
            )
            .unwrap();
        assert_eq!(out, body);
        assert_eq!(resp.bytes, 40_000);
        assert_eq!(seen.len(), 3);
        assert_eq!(seen.last(), Some(&(40_000, Some(40_000))));
        assert_eq!(http.requests().len(), 1);
    }

    #[test]
    fn unknown_urls_statuses_faults_and_caps_fail() {
        let http = FakeHttp::new();
        http.route(
            "https://example.invalid/404",
            FakeRoute::Body {
                status: 404,
                body: vec![],
            },
        );
        http.route(
            "https://example.invalid/slow",
            FakeRoute::Fault(HttpError::Timeout),
        );
        http.serve("https://example.invalid/big", vec![0u8; 10]);
        let mut sink = Vec::new();
        let mut run = |req: HttpRequest| http.get(&req, &mut sink, &mut |_, _| {});
        assert!(matches!(
            run(request("https://example.invalid/none")),
            Err(HttpError::Network(_))
        ));
        assert_eq!(
            run(request("https://example.invalid/404")),
            Err(HttpError::Status(404))
        );
        assert_eq!(
            run(request("https://example.invalid/slow")),
            Err(HttpError::Timeout)
        );
        assert_eq!(
            run(request("https://example.invalid/big").with_body_cap(5)),
            Err(HttpError::TooLarge { cap: 5 })
        );
    }
}
