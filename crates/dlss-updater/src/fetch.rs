//! The one place the updater talks to the network: GET a download page,
//! then POST its form without following the redirect.
//!
//! Like Go's tool, the page request follows redirects and must end in a
//! 200, and the form request must answer 302 with a `Location`. Tests point
//! a [`Client::without_proxy`] at a loopback server with saved pages.

use std::time::Duration;

use ureq::Agent;

use crate::Latest;
use crate::scrape::{FetchError, SERVER_ID, SOURCE_NAME, Source, USER_AGENT, read_page};

/// Time allowed until a response header arrives, as Go's tool allowed.
pub const HEADER_TIMEOUT: Duration = Duration::from_secs(30);
/// Time allowed for a whole request. Go's tool had no limit, so a stalled
/// body could hang the job.
pub const TOTAL_TIMEOUT: Duration = Duration::from_secs(120);

/// An HTTP client for the download pages.
pub struct Client {
    agent: Agent,
}

impl Client {
    /// A client that honours the proxy variables, for CI.
    pub fn new() -> Client {
        Client::build(true)
    }

    /// A client that never uses a proxy, for a loopback server.
    pub fn without_proxy() -> Client {
        Client::build(false)
    }

    fn build(proxy_from_env: bool) -> Client {
        let mut config = Agent::config_builder()
            .user_agent(USER_AGENT)
            .http_status_as_error(false)
            .timeout_recv_response(Some(HEADER_TIMEOUT))
            .timeout_global(Some(TOTAL_TIMEOUT));
        if !proxy_from_env {
            config = config.proxy(None);
        }
        Client {
            agent: config.build().new_agent(),
        }
    }

    /// The newest version a source lists, with its download URL. `is_new`
    /// is left false for the caller to decide.
    pub fn latest(&self, source: &Source) -> Result<Latest, FetchError> {
        let html = self.page(&source.page_url)?;
        let page = read_page(source, &html)?;
        let download_url = self
            .download_url(&source.page_url, &page.file_id)
            .map_err(FetchError::DownloadUrl)?;
        Ok(Latest {
            dll_type: source.dll_type.as_str().to_owned(),
            version: page.version,
            download_url,
            filename: source.filename().to_owned(),
            is_new: false,
            source: SOURCE_NAME.to_owned(),
        })
    }

    fn page(&self, url: &str) -> Result<String, FetchError> {
        let mut response = self
            .agent
            .get(url)
            .call()
            .map_err(|e| FetchError::Network(e.to_string()))?;
        let status = response.status().as_u16();
        if status != 200 {
            return Err(FetchError::Status(status));
        }
        response
            .body_mut()
            .read_to_string()
            .map_err(|e| FetchError::Network(e.to_string()))
    }

    /// Posts the download form and returns the redirect's `Location`.
    fn download_url(&self, url: &str, file_id: &str) -> Result<String, String> {
        let response = self
            .agent
            .post(url)
            .config()
            .max_redirects(0)
            .build()
            .send_form([("id", file_id), ("server_id", SERVER_ID)])
            .map_err(|e| e.to_string())?;
        let status = response.status().as_u16();
        if status != 302 {
            return Err(format!("expected redirect, got {status}"));
        }
        match response.headers().get("location").map(|v| v.to_str()) {
            Some(Ok(location)) if !location.is_empty() => Ok(location.to_owned()),
            _ => Err("no redirect location".to_owned()),
        }
    }
}

impl Default for Client {
    fn default() -> Client {
        Client::new()
    }
}
