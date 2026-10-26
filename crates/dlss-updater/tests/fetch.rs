//! The real ureq client against a loopback server with saved download
//! pages. No test reaches the network.
//!
//! Go's `tools/dlss-updater/main_test.go` pinned the same contract: the
//! page request sends Firefox's User-Agent, the form posts `id` and
//! `server_id=27`, and the download URL is the 302's `Location`, which is
//! never followed.

use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::Path;
use std::sync::{Arc, Mutex};

use dlss_updater::fetch::Client;
use dlss_updater::scrape::{FetchError, Source, USER_AGENT, find_source, sources};
use dlss_updater::{Latest, run};

/// One request the server saw.
#[derive(Debug, Clone)]
struct Request {
    method: String,
    path: String,
    headers: Vec<(String, String)>,
    body: String,
}

impl Request {
    fn header(&self, name: &str) -> Option<&str> {
        self.headers
            .iter()
            .find(|(k, _)| k.eq_ignore_ascii_case(name))
            .map(|(_, v)| v.as_str())
    }
}

/// A canned answer.
struct Response {
    status: u16,
    headers: Vec<(&'static str, String)>,
    body: String,
}

impl Response {
    fn ok(body: String) -> Response {
        Response {
            status: 200,
            headers: Vec::new(),
            body,
        }
    }

    fn status(status: u16) -> Response {
        Response {
            status,
            headers: Vec::new(),
            body: String::new(),
        }
    }

    fn redirect(status: u16, location: &str) -> Response {
        Response {
            status,
            headers: vec![("Location", location.to_owned())],
            body: String::new(),
        }
    }
}

type Handler = dyn Fn(&Request, &str) -> Response + Send + Sync;

/// An answer that needs no base URL.
type Answer = dyn Fn(&Request) -> Response + Send + Sync;

/// A loopback HTTP/1.1 server that answers each connection once.
struct Server {
    base: String,
    seen: Arc<Mutex<Vec<Request>>>,
}

impl Server {
    fn start(handler: impl Fn(&Request, &str) -> Response + Send + Sync + 'static) -> Server {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let base = format!("http://{}", listener.local_addr().unwrap());
        let seen = Arc::new(Mutex::new(Vec::new()));
        let handler: Arc<Handler> = Arc::new(handler);
        let (thread_seen, thread_base) = (seen.clone(), base.clone());
        std::thread::spawn(move || {
            for stream in listener.incoming() {
                let Ok(stream) = stream else { return };
                serve(stream, &*handler, &thread_seen, &thread_base);
            }
        });
        Server { base, seen }
    }

    fn requests(&self) -> Vec<Request> {
        self.seen.lock().unwrap().clone()
    }

    /// A source whose page is on this server.
    fn source(&self, name: &str) -> Source {
        let mut source = find_source(name).unwrap();
        source.page_url = source
            .page_url
            .replace("https://www.techpowerup.com", &self.base);
        source
    }
}

fn serve(stream: TcpStream, handler: &Handler, seen: &Mutex<Vec<Request>>, base: &str) {
    let mut reader = BufReader::new(stream.try_clone().unwrap());
    let mut line = String::new();
    if reader.read_line(&mut line).unwrap_or(0) == 0 {
        return;
    }
    let mut parts = line.split_whitespace();
    let method = parts.next().unwrap_or_default().to_owned();
    let path = parts.next().unwrap_or_default().to_owned();
    let mut headers = Vec::new();
    loop {
        let mut header = String::new();
        reader.read_line(&mut header).unwrap();
        let header = header.trim_end();
        if header.is_empty() {
            break;
        }
        let (k, v) = header.split_once(':').unwrap();
        headers.push((k.trim().to_owned(), v.trim().to_owned()));
    }
    let mut request = Request {
        method,
        path,
        headers,
        body: String::new(),
    };
    let length: usize = request
        .header("content-length")
        .map_or(0, |v| v.parse().unwrap());
    let mut body = vec![0; length];
    reader.read_exact(&mut body).unwrap();
    request.body = String::from_utf8(body).unwrap();
    seen.lock().unwrap().push(request.clone());

    let response = handler(&request, base);
    let mut head = format!(
        "HTTP/1.1 {} Fixture\r\nContent-Length: {}\r\nConnection: close\r\n",
        response.status,
        response.body.len()
    );
    for (k, v) in &response.headers {
        head.push_str(&format!("{k}: {v}\r\n"));
    }
    head.push_str("\r\n");
    let mut stream = stream;
    let _ = stream.write_all(head.as_bytes());
    let _ = stream.write_all(response.body.as_bytes());
}

fn fixture(name: &str) -> String {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests/fixtures/techpowerup")
        .join(name);
    std::fs::read_to_string(path).unwrap()
}

/// TechPowerUp as the fixtures record it: each page, then a 302 to the
/// download for its form post.
fn techpowerup(request: &Request, base: &str) -> Response {
    let page = sources()
        .into_iter()
        .find(|s| s.page_url.ends_with(&request.path))
        .map(|s| s.dll_type.as_str());
    match (request.method.as_str(), page) {
        ("GET", Some(name)) => Response::ok(fixture(&format!("{name}.html"))),
        ("POST", Some(name)) => Response::redirect(
            302,
            &format!("{base}/files/nvngx_{name}_v310.5.3.zip?token=a&b=1"),
        ),
        _ => Response::status(404),
    }
}

#[test]
fn each_family_reads_its_saved_page_and_the_redirect() {
    let server = Server::start(techpowerup);
    let client = Client::without_proxy();
    for (name, id, filename) in [
        ("dlss", "5309", "nvngx_dlss.dll"),
        ("dlssg", "5311", "nvngx_dlssg.dll"),
        ("dlssd", "5310", "nvngx_dlssd.dll"),
    ] {
        let latest = client.latest(&server.source(name)).unwrap();
        assert_eq!(
            latest,
            Latest {
                dll_type: name.into(),
                version: "310.5.3".into(),
                download_url: format!(
                    "{}/files/nvngx_{name}_v310.5.3.zip?token=a&b=1",
                    server.base
                ),
                filename: filename.into(),
                is_new: false,
                source: "techpowerup.com".into(),
            }
        );
        let requests = server.requests();
        let [.., get, post] = requests.as_slice() else {
            panic!("{requests:?}")
        };
        assert_eq!(get.method, "GET");
        assert_eq!(post.method, "POST");
        assert_eq!(get.path, post.path, "the form posts back to its page");
        for r in [get, post] {
            assert_eq!(r.header("user-agent"), Some(USER_AGENT), "{r:?}");
        }
        assert_eq!(
            post.header("content-type"),
            Some("application/x-www-form-urlencoded")
        );
        assert_eq!(post.body, format!("id={id}&server_id=27"));
    }
    assert!(
        server
            .requests()
            .iter()
            .all(|r| !r.path.starts_with("/files/")),
        "the download redirect is never followed"
    );
}

#[test]
fn the_page_request_follows_a_redirect_as_go_s_client_did() {
    let server = Server::start(|request, base| {
        if request.path == "/moved/" {
            Response::redirect(301, &format!("{base}/download/nvidia-dlss-dll/"))
        } else {
            techpowerup(request, base)
        }
    });
    let mut source = server.source("dlss");
    source.page_url = format!("{}/moved/", server.base);
    // The form is posted to the page URL as given, as Go did.
    let err = Client::without_proxy().latest(&source).unwrap_err();
    assert_eq!(
        err.to_string(),
        "could not get download URL: expected redirect, got 301"
    );
    let paths: Vec<String> = server.requests().into_iter().map(|r| r.path).collect();
    assert_eq!(paths, ["/moved/", "/download/nvidia-dlss-dll/", "/moved/"]);
}

#[test]
fn failures_keep_go_s_messages() {
    // tools/dlss-updater/main_test.go, TestTechPowerUpFailuresRemainActionable
    // and TestDownloadRedirectValidation.
    let page = fixture("dlss.html");
    let cases: Vec<(&str, Box<Answer>, &str)> = vec![
        (
            "HTTP status",
            Box::new(|_| Response::status(502)),
            "HTTP 502",
        ),
        (
            "missing version",
            Box::new(|_| Response::ok(r#"<input type="hidden" name="id" value="1">"#.into())),
            "could not find version in page title",
        ),
        (
            "missing file ID",
            Box::new(|_| Response::ok("<title>NVIDIA DLSS DLL 1.2.3 Download</title>".into())),
            "could not find file ID",
        ),
        (
            "post not redirected",
            Box::new(move |r| match r.method.as_str() {
                "GET" => Response::ok(page.clone()),
                _ => Response::ok("<html>pick a mirror</html>".into()),
            }),
            "could not get download URL: expected redirect, got 200",
        ),
        (
            "redirect without location",
            Box::new(|r| match r.method.as_str() {
                "GET" => Response::ok(fixture("dlss.html")),
                _ => Response::status(302),
            }),
            "could not get download URL: no redirect location",
        ),
        (
            "post refused",
            Box::new(|r| match r.method.as_str() {
                "GET" => Response::ok(fixture("dlss.html")),
                _ => Response::status(403),
            }),
            "could not get download URL: expected redirect, got 403",
        ),
    ];
    for (name, answer, want) in cases {
        let server = Server::start(move |request, _| answer(request));
        let err = Client::without_proxy()
            .latest(&server.source("dlss"))
            .unwrap_err();
        assert_eq!(err.to_string(), want, "{name}");
    }

    // Nothing listening is a network error, not a panic.
    let closed = TcpListener::bind("127.0.0.1:0").unwrap();
    let base = format!("http://{}", closed.local_addr().unwrap());
    drop(closed);
    let mut source = find_source("dlss").unwrap();
    source.page_url = format!("{base}/download/nvidia-dlss-dll/");
    assert!(matches!(
        Client::without_proxy().latest(&source),
        Err(FetchError::Network(_))
    ));
}

#[test]
fn a_whole_check_against_the_saved_pages() {
    let server = Server::start(techpowerup);
    let client = Client::without_proxy();
    let manifest = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data/manifest.json");
    let dir = tempfile::tempdir().unwrap();
    let older = dir.path().join("manifest.json");
    std::fs::write(
        &older,
        r#"{"version":"1","dlls":{"dlssd":[{"version":"310.5.0"}]}}"#,
    )
    .unwrap();
    let check = |name: &str, manifest: &Path| {
        let args: Vec<String> = ["-json", "-type", name, "-manifest"]
            .into_iter()
            .map(str::to_owned)
            .chain([manifest.display().to_string()])
            .collect();
        let (mut out, mut err) = (Vec::new(), Vec::new());
        let code = run(
            "dlss-updater",
            &args,
            &mut out,
            &mut err,
            &|source| client.latest(&server.source(source.dll_type.as_str())),
            "2026-10-26T12:00:00Z".parse().unwrap(),
        );
        let err = String::from_utf8(err).unwrap();
        assert_eq!(err, "");
        let line: serde_json::Value = serde_json::from_slice(&out).unwrap();
        (code, line)
    };
    // The shipped manifest already has 310.5.3 for every family.
    for name in ["dlss", "dlssg", "dlssd"] {
        let (code, line) = check(name, &manifest);
        assert_eq!(code, 2, "{name}: {line}");
        assert_eq!(line["is_new"], false);
        assert_eq!(line["version"], "310.5.3");
    }
    let (code, line) = check("dlssd", &older);
    assert_eq!(code, 0, "{line}");
    assert_eq!(
        line,
        serde_json::json!({
            "type": "dlssd",
            "version": "310.5.3",
            "download_url": format!("{}/files/nvngx_dlssd_v310.5.3.zip?token=a&b=1", server.base),
            "filename": "nvngx_dlssd.dll",
            "is_new": true,
            "source": "techpowerup.com",
        })
    );
}
