//! The real HTTP backend: blocking ureq, run on a worker thread or by a CLI
//! command (decision 11).
//!
//! This is the one module of the binary that builds an HTTP agent or opens a
//! socket; `tests/safety.rs` fails on `ureq` or `std::net` anywhere else.
//!
//! - Every request sends `User-Agent: spela/<version>` (decision 41).
//! - The timeouts and the body cap come with each [`HttpRequest`]: Go's 30 s
//!   to the response header and 120 s in total for the manifest and DLLs, 5 s
//!   and 1 MiB for the release check. The body cap is counted here, so a
//!   server that sends more never gets more than the cap written.
//! - The proxy comes from the environment snapshot, with the variables and
//!   the order ureq itself reads (decision 41). ureq's own lookup reads the
//!   process environment behind the snapshot, so it is switched off. A host
//!   listed in `NO_PROXY` goes direct.
//! - TLS trusts the system's certificates through ureq's platform verifier,
//!   as Go trusts the system pool.
//! - Any status that is not 2xx is [`HttpError::Status`]. Redirects are
//!   followed, as Go's client follows them.

use std::io::{self, Read, Write};

use spela_core::backend::{Http, HttpError, HttpRequest, HttpResponse, USER_AGENT};
use spela_core::env::{Env, var};
use ureq::tls::{RootCerts, TlsConfig};
use ureq::{Agent, Proxy};

/// Blocking HTTP through ureq.
#[derive(Debug)]
pub struct UreqHttp {
    direct: Agent,
    proxied: Option<(Agent, NoProxy)>,
}

impl UreqHttp {
    /// The backend for this run, with the proxy the snapshot names.
    pub fn from_env(env: &Env) -> UreqHttp {
        UreqHttp::with_proxy(proxy_from_env(env), no_proxy_from_env(env))
    }

    /// A backend with an explicit proxy and `NO_PROXY` list.
    pub fn with_proxy(proxy: Option<Proxy>, no_proxy: NoProxy) -> UreqHttp {
        UreqHttp {
            direct: agent(None),
            proxied: proxy.map(|p| (agent(Some(p)), no_proxy)),
        }
    }

    fn agent_for(&self, host: &str) -> &Agent {
        match &self.proxied {
            Some((agent, no_proxy)) if !no_proxy.matches(host) => agent,
            _ => &self.direct,
        }
    }
}

fn agent(proxy: Option<Proxy>) -> Agent {
    let tls = TlsConfig::builder()
        .root_certs(RootCerts::PlatformVerifier)
        .build();
    let config = Agent::config_builder()
        .proxy(proxy)
        .user_agent(USER_AGENT)
        .http_status_as_error(true)
        .tls_config(tls)
        .build();
    Agent::new_with_config(config)
}

/// The first proxy variable that holds a proxy URL ureq accepts, in ureq's
/// order. A variable set to an empty or broken value is skipped, as ureq
/// skips it.
pub fn proxy_from_env(env: &Env) -> Option<Proxy> {
    var::PROXIES
        .iter()
        .filter_map(|name| env.non_empty(name))
        .find_map(|value| Proxy::new(value).ok())
}

/// The `NO_PROXY` list of the snapshot, `NO_PROXY` before `no_proxy`.
pub fn no_proxy_from_env(env: &Env) -> NoProxy {
    var::NO_PROXY
        .iter()
        .find_map(|name| env.get(name))
        .map(NoProxy::parse)
        .unwrap_or_default()
}

/// Hosts that skip the proxy, matched as ureq matches them: `*` is every
/// host, `.example.com` and `*.example.com` are its subdomains, anything
/// else is one host, ignoring case.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct NoProxy(Vec<String>);

impl NoProxy {
    /// Parses a comma-separated list.
    pub fn parse(text: &str) -> NoProxy {
        NoProxy(
            text.split(',')
                .map(|e| {
                    e.trim()
                        .trim_start_matches('[')
                        .trim_end_matches(']')
                        .to_ascii_lowercase()
                })
                .filter(|e| !e.is_empty())
                .collect(),
        )
    }

    /// Whether `host` skips the proxy.
    pub fn matches(&self, host: &str) -> bool {
        let host = host
            .trim_start_matches('[')
            .trim_end_matches(']')
            .to_ascii_lowercase();
        self.0.iter().any(|entry| {
            if entry == "*" {
                return true;
            }
            match entry.strip_prefix('*').unwrap_or(entry).strip_prefix('.') {
                Some(suffix) => host.ends_with(&format!(".{suffix}")),
                None => host == *entry,
            }
        })
    }
}

impl Http for UreqHttp {
    fn get(
        &self,
        request: &HttpRequest,
        body: &mut dyn Write,
        progress: &mut dyn FnMut(u64, Option<u64>),
    ) -> Result<HttpResponse, HttpError> {
        let host = request.uri.host().unwrap_or_default();
        let mut builder = self.agent_for(host).get(&request.uri);
        if let Some(accept) = &request.accept {
            builder = builder.header("Accept", accept);
        }
        let mut response = builder
            .config()
            .timeout_recv_response(Some(request.header_timeout))
            .timeout_global(Some(request.total_timeout))
            .build()
            .call()
            .map_err(from_ureq)?;
        let status = response.status().as_u16();
        let link = response
            .headers()
            .get("link")
            .and_then(|v| v.to_str().ok())
            .map(str::to_owned);
        let total = response.body().content_length();
        if total.is_some_and(|t| t > request.body_cap) {
            return Err(HttpError::TooLarge {
                cap: request.body_cap,
            });
        }
        let mut reader = response.body_mut().as_reader();
        let bytes = copy_capped(&mut reader, body, request.body_cap, |n| progress(n, total))?;
        Ok(HttpResponse {
            status,
            bytes,
            link,
        })
    }
}

/// Copies at most `cap` bytes, failing as soon as one more arrives.
fn copy_capped(
    reader: &mut dyn Read,
    out: &mut dyn Write,
    cap: u64,
    mut progress: impl FnMut(u64),
) -> Result<u64, HttpError> {
    let mut buf = vec![0u8; 64 * 1024];
    let mut sent = 0u64;
    loop {
        let n = match reader.read(&mut buf) {
            Ok(0) => return Ok(sent),
            Ok(n) => n,
            Err(e) if e.kind() == io::ErrorKind::Interrupted => continue,
            Err(e) => return Err(from_ureq(ureq::Error::from(e))),
        };
        if sent + n as u64 > cap {
            return Err(HttpError::TooLarge { cap });
        }
        out.write_all(&buf[..n])
            .map_err(|e| HttpError::Write(e.to_string()))?;
        sent += n as u64;
        progress(sent);
    }
}

fn from_ureq(e: ureq::Error) -> HttpError {
    match e {
        ureq::Error::StatusCode(code) => HttpError::Status(code),
        ureq::Error::Timeout(_) => HttpError::Timeout,
        ureq::Error::Io(io) if io.kind() == io::ErrorKind::TimedOut => HttpError::Timeout,
        other => HttpError::Network(other.to_string()),
    }
}

#[cfg(test)]
mod tests {
    //! Every test talks to a server on a loopback port that the test owns.
    //! Nothing here resolves a name or leaves the machine.

    use super::*;
    use std::net::{TcpListener, TcpStream};
    use std::sync::mpsc;
    use std::thread;
    use std::time::{Duration, Instant};

    /// What the loopback server does with one connection.
    enum Reply {
        /// Writes these bytes and closes.
        Bytes(Vec<u8>),
        /// Writes these bytes, then waits without closing.
        Stall(Vec<u8>),
    }

    /// A loopback server that answers `replies` in order, one per
    /// connection, and sends each request's head back on a channel.
    struct Server {
        url: String,
        heads: mpsc::Receiver<String>,
    }

    fn serve(replies: Vec<Reply>) -> Server {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let (tx, heads) = mpsc::channel();
        thread::spawn(move || {
            for reply in replies {
                let Ok((mut stream, _)) = listener.accept() else {
                    return;
                };
                let _ = tx.send(read_head(&mut stream));
                match reply {
                    Reply::Bytes(b) => {
                        let _ = stream.write_all(&b);
                    }
                    Reply::Stall(b) => {
                        let _ = stream.write_all(&b);
                        thread::sleep(Duration::from_secs(5));
                    }
                }
            }
        });
        Server {
            url: format!("http://127.0.0.1:{port}"),
            heads,
        }
    }

    fn read_head(stream: &mut TcpStream) -> String {
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap();
        let mut head = Vec::new();
        let mut byte = [0u8; 1];
        while !head.ends_with(b"\r\n\r\n") {
            match stream.read(&mut byte) {
                Ok(1) => head.push(byte[0]),
                _ => break,
            }
        }
        String::from_utf8_lossy(&head).into_owned()
    }

    fn ok(body: &[u8], extra: &str) -> Reply {
        let mut out = format!(
            "HTTP/1.1 200 OK\r\nContent-Length: {}\r\n{extra}Connection: close\r\n\r\n",
            body.len()
        )
        .into_bytes();
        out.extend_from_slice(body);
        Reply::Bytes(out)
    }

    fn request(url: &str) -> HttpRequest {
        HttpRequest::new(url.parse().unwrap())
    }

    fn direct() -> UreqHttp {
        UreqHttp::with_proxy(None, NoProxy::default())
    }

    fn get(http: &UreqHttp, req: &HttpRequest) -> (Result<HttpResponse, HttpError>, Vec<u8>) {
        let mut out = Vec::new();
        let result = http.get(req, &mut out, &mut |_, _| {});
        (result, out)
    }

    #[test]
    fn sends_the_user_agent_and_accept_and_streams_with_progress() {
        let body = vec![7u8; 200_000];
        let server = serve(vec![ok(&body, "Link: <http://x/2>; rel=\"next\"\r\n")]);
        let req = request(&format!("{}/manifest.json", server.url))
            .with_accept("application/vnd.github+json");
        let mut out = Vec::new();
        let mut seen = Vec::new();
        let resp = direct()
            .get(&req, &mut out, &mut |n, t| seen.push((n, t)))
            .unwrap();
        assert_eq!(out, body);
        assert_eq!(resp.status, 200);
        assert_eq!(resp.bytes, 200_000);
        assert_eq!(resp.link.as_deref(), Some("<http://x/2>; rel=\"next\""));
        assert_eq!(seen.last(), Some(&(200_000, Some(200_000))));
        let head = server.heads.recv().unwrap().to_ascii_lowercase();
        assert!(
            head.starts_with("get /manifest.json http/1.1\r\n"),
            "{head}"
        );
        assert!(
            head.contains(&format!("user-agent: spela/{}\r\n", spela_core::VERSION)),
            "{head}"
        );
        assert!(
            head.contains("accept: application/vnd.github+json\r\n"),
            "{head}"
        );
    }

    #[test]
    fn a_status_that_is_not_2xx_is_an_error() {
        let server = serve(vec![Reply::Bytes(
            b"HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".to_vec(),
        )]);
        let (result, _) = get(&direct(), &request(&format!("{}/missing", server.url)));
        assert_eq!(result, Err(HttpError::Status(404)));
    }

    #[test]
    fn redirects_are_followed() {
        let target = serve(vec![ok(b"payload", "")]);
        let redirect = serve(vec![Reply::Bytes(
            format!(
                "HTTP/1.1 302 Found\r\nLocation: {}/file.dll\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
                target.url
            )
            .into_bytes(),
        )]);
        let (result, out) = get(&direct(), &request(&format!("{}/dl", redirect.url)));
        assert_eq!(result.unwrap().bytes, 7);
        assert_eq!(out, b"payload");
        assert!(target.heads.recv().unwrap().starts_with("GET /file.dll "));
    }

    #[test]
    fn the_body_cap_stops_a_long_body_before_it_is_written() {
        // Announced too large: refused before reading.
        let server = serve(vec![ok(&[1u8; 2048], "")]);
        let req = request(&format!("{}/big", server.url)).with_body_cap(1024);
        let (result, out) = get(&direct(), &req);
        assert_eq!(result, Err(HttpError::TooLarge { cap: 1024 }));
        assert!(out.is_empty());

        // No length: counted while reading, never more than the cap kept.
        let mut chunked = b"HTTP/1.1 200 OK\r\nConnection: close\r\n\r\n".to_vec();
        chunked.extend_from_slice(&[2u8; 4096]);
        let server = serve(vec![Reply::Bytes(chunked)]);
        let req = request(&format!("{}/big", server.url)).with_body_cap(1024);
        let (result, out) = get(&direct(), &req);
        assert_eq!(result, Err(HttpError::TooLarge { cap: 1024 }));
        assert!(out.len() <= 1024, "{}", out.len());
    }

    #[test]
    fn a_server_that_never_answers_times_out() {
        let server = serve(vec![Reply::Stall(Vec::new())]);
        let req = request(&format!("{}/slow", server.url))
            .with_timeouts(Duration::from_millis(300), Duration::from_secs(10));
        let started = Instant::now();
        let (result, _) = get(&direct(), &req);
        assert_eq!(result, Err(HttpError::Timeout));
        assert!(started.elapsed() < Duration::from_secs(4));
    }

    #[test]
    fn a_stalled_body_hits_the_total_timeout() {
        let mut head = b"HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n".to_vec();
        head.extend_from_slice(&[0u8; 10]);
        let server = serve(vec![Reply::Stall(head)]);
        let req = request(&format!("{}/stall", server.url))
            .with_timeouts(Duration::from_secs(2), Duration::from_millis(500));
        let started = Instant::now();
        let (result, _) = get(&direct(), &req);
        assert_eq!(result, Err(HttpError::Timeout));
        assert!(started.elapsed() < Duration::from_secs(4));
    }

    #[test]
    fn a_closed_port_is_a_network_error() {
        let port = TcpListener::bind("127.0.0.1:0")
            .unwrap()
            .local_addr()
            .unwrap()
            .port();
        let (result, _) = get(&direct(), &request(&format!("http://127.0.0.1:{port}/")));
        assert!(matches!(result, Err(HttpError::Network(_))), "{result:?}");
    }

    #[test]
    fn the_proxy_from_the_snapshot_carries_the_request() {
        // The proxy is a loopback listener; the target is a loopback port
        // nobody listens on, so only the proxy can answer.
        let proxy = serve(vec![Reply::Bytes(
            b"HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".to_vec(),
        )]);
        let target = TcpListener::bind("127.0.0.1:0").unwrap();
        let target_port = target.local_addr().unwrap().port();
        drop(target);
        let env = Env::default().with("HTTPS_PROXY", &proxy.url);
        let http = UreqHttp::from_env(&env);
        let (result, _) = get(
            &http,
            &request(&format!("http://127.0.0.1:{target_port}/m")),
        );
        assert!(result.is_err());
        let head = proxy.heads.recv_timeout(Duration::from_secs(5)).unwrap();
        assert!(
            head.contains(&format!("127.0.0.1:{target_port}")),
            "the proxy saw the target: {head}"
        );
    }

    #[test]
    fn no_proxy_hosts_go_direct() {
        let server = serve(vec![ok(b"direct", "")]);
        // A proxy port nobody listens on: going through it would fail.
        let closed = TcpListener::bind("127.0.0.1:0").unwrap();
        let closed_url = format!("http://127.0.0.1:{}", closed.local_addr().unwrap().port());
        drop(closed);
        let env = Env::default()
            .with("http_proxy", &closed_url)
            .with("NO_PROXY", "localhost, 127.0.0.1");
        let (result, out) = get(&UreqHttp::from_env(&env), &request(&server.url));
        assert_eq!(result.unwrap().bytes, 6);
        assert_eq!(out, b"direct");
    }

    #[test]
    fn proxy_variables_follow_ureq_s_order() {
        assert!(proxy_from_env(&Env::default()).is_none());
        let env = Env::default()
            .with("HTTP_PROXY", "http://127.0.0.1:3")
            .with("HTTPS_PROXY", "http://127.0.0.1:2")
            .with("ALL_PROXY", "");
        assert_eq!(proxy_from_env(&env).unwrap().port(), 2);
        let env = env.with("all_proxy", "http://127.0.0.1:1");
        assert_eq!(proxy_from_env(&env).unwrap().port(), 1);
        let env = Env::default()
            .with("NO_PROXY", "a.test")
            .with("no_proxy", "b.test");
        assert_eq!(no_proxy_from_env(&env), NoProxy::parse("a.test"));
    }

    #[test]
    fn no_proxy_patterns_match_as_ureq_matches_them() {
        let np = NoProxy::parse("Localhost, .internal.test,*.corp.test,10.0.0.1, ,[::1]");
        for host in [
            "localhost",
            "LOCALHOST",
            "a.internal.test",
            "x.y.corp.test",
            "10.0.0.1",
            "[::1]",
        ] {
            assert!(np.matches(host), "{host}");
        }
        for host in ["internal.test", "corp.test", "example.test", "10.0.0.10"] {
            assert!(!np.matches(host), "{host}");
        }
        assert!(NoProxy::parse("*").matches("anything.test"));
        assert!(!NoProxy::default().matches("localhost"));
    }
}
