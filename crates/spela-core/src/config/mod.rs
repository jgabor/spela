//! The config file, `<config>/config.toml`.
//!
//! Every key is optional, and a missing key means its default, as in Go.
//! The keys keep their Go names. The TUI settings go in `[ui]`, as in mejl.
//!
//! | Key | Default | Accepted |
//! |---|---|---|
//! | `version` | `1` | 1, or higher read only with a warning |
//! | `log_level` | `info` | debug, info, warn, error |
//! | `check_updates` | `true` | bool |
//! | `rescan_on_startup` | `true` | bool. The TUI's one background scan at start |
//! | `auto_update_dlls` | `false` | bool |
//! | `confirm_destructive` | `true` | bool |
//! | `steam_path` | empty | any text. Empty finds Steam |
//! | `additional_library_paths` | `[]` | a list of folders |
//! | `dll_cache_path` | empty | absolute folder, or empty for `<cache>/dlls` |
//! | `backup_path` | empty | absolute folder, or empty for `<data>/backups` |
//! | `shader_cache` | empty | absolute folder, or empty for `<cache>/nvidia` |
//! | `dll_manifest_url` | empty | https, or http to loopback. Empty is the built-in URL |
//! | `auto_refresh_manifest` | `true` | bool |
//! | `manifest_refresh_hours` | `24` | whole number |
//! | `ui.theme` | `terminal` | terminal, neon, mono |
//! | `ui.mouse` | `true` | bool |
//! | `ui.ascii_glyphs` | `false` | bool |
//!
//! Loading validates every value and reports each problem with its line and
//! column, as mejl does. The value rules and error texts are Go's option
//! table (`internal/config/options.go`). Unknown keys are warnings with a
//! did-you-mean, not errors, and stay in the file. Go's `theme`,
//! `compact_mode`, `show_hints` and `preferred_dll_source` are unknown keys
//! here (decisions 3 and 46).
//!
//! `shader_cache` defaults to empty, which means `<cache>/nvidia`, so the
//! config needs no paths to build. [`Config::shader_cache_dir`] resolves it.

pub mod store;

use std::fmt;
use std::ops::Range;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};
use toml::Spanned;
use toml::de::{DeTable, DeValue};

use crate::paths::Paths;
use crate::profile::parse_bool;

/// The config format version this spela writes and reads fully.
pub const CONFIG_VERSION: u32 = 1;

/// The log level setting.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum LogLevel {
    /// Everything.
    Debug,
    /// The default.
    #[default]
    Info,
    /// Warnings and errors.
    Warn,
    /// Errors only.
    Error,
}

impl LogLevel {
    /// Every level, in order.
    pub const ALL: [LogLevel; 4] = [
        LogLevel::Debug,
        LogLevel::Info,
        LogLevel::Warn,
        LogLevel::Error,
    ];

    /// The config word, such as `warn`.
    pub fn name(self) -> &'static str {
        match self {
            LogLevel::Debug => "debug",
            LogLevel::Info => "info",
            LogLevel::Warn => "warn",
            LogLevel::Error => "error",
        }
    }

    /// The level with this word. Exact, as in Go.
    pub fn from_name(name: &str) -> Option<LogLevel> {
        LogLevel::ALL.into_iter().find(|l| l.name() == name)
    }
}

/// The TUI theme (decision 27). `NO_COLOR` forces mono whatever this says.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum Theme {
    /// The terminal's own ANSI 16 palette.
    #[default]
    Terminal,
    /// spela's own colours.
    Neon,
    /// No colour.
    Mono,
}

impl Theme {
    /// Every theme, in order.
    pub const ALL: [Theme; 3] = [Theme::Terminal, Theme::Neon, Theme::Mono];

    /// The config word, such as `neon`.
    pub fn name(self) -> &'static str {
        match self {
            Theme::Terminal => "terminal",
            Theme::Neon => "neon",
            Theme::Mono => "mono",
        }
    }

    /// The theme with this word.
    pub fn from_name(name: &str) -> Option<Theme> {
        Theme::ALL.into_iter().find(|t| t.name() == name)
    }
}

/// The `[ui]` table.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct UiSettings {
    /// The theme.
    pub theme: Theme,
    /// Capture the mouse. Off gives the terminal's own text selection back.
    pub mouse: bool,
    /// Draw boxes, markers and sparklines in plain ASCII.
    pub ascii_glyphs: bool,
}

impl Default for UiSettings {
    fn default() -> UiSettings {
        UiSettings {
            theme: Theme::Terminal,
            mouse: true,
            ascii_glyphs: false,
        }
    }
}

/// The whole config, with defaults filled in.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct Config {
    /// The format version of the file.
    pub version: u32,
    /// The log level when neither `--log-level` nor `SPELA_LOG` sets one.
    pub log_level: LogLevel,
    /// Ask GitHub once a day for a newer release. TUI only (decision 39).
    pub check_updates: bool,
    /// The TUI's one background scan at start (decision 24).
    pub rescan_on_startup: bool,
    /// Update stale DLLs when the TUI starts. TUI only (decision 39).
    pub auto_update_dlls: bool,
    /// Ask before a DLL update. Install, restore, whole-page resets and
    /// hardware actions always ask.
    pub confirm_destructive: bool,
    /// The Steam install to scan. Empty finds it.
    pub steam_path: String,
    /// More Steam library folders.
    pub additional_library_paths: Vec<String>,
    /// The DLL cache folder. Empty means `<cache>/dlls`.
    pub dll_cache_path: String,
    /// The folder for new backup files. Empty means `<data>/backups`.
    pub backup_path: String,
    /// The default shader cache folder. Empty means `<cache>/nvidia`.
    pub shader_cache: String,
    /// The DLL manifest URL. Empty means the built-in URL (decision 14).
    pub dll_manifest_url: String,
    /// Fetch a new manifest when the cached one is old.
    pub auto_refresh_manifest: bool,
    /// Hours before the cached manifest counts as old.
    pub manifest_refresh_hours: i64,
    /// The TUI settings.
    pub ui: UiSettings,
}

impl Default for Config {
    fn default() -> Config {
        Config {
            version: CONFIG_VERSION,
            log_level: LogLevel::Info,
            check_updates: true,
            rescan_on_startup: true,
            auto_update_dlls: false,
            confirm_destructive: true,
            steam_path: String::new(),
            additional_library_paths: Vec::new(),
            dll_cache_path: String::new(),
            backup_path: String::new(),
            shader_cache: String::new(),
            dll_manifest_url: String::new(),
            auto_refresh_manifest: true,
            manifest_refresh_hours: crate::dll::manifest::DEFAULT_REFRESH_HOURS,
            ui: UiSettings::default(),
        }
    }
}

impl Config {
    /// The DLL cache folder: `dll_cache_path` when it is absolute, else
    /// `<cache>/dlls`.
    pub fn dll_cache_dir(&self, paths: &Paths) -> PathBuf {
        configured_dir(&self.dll_cache_path).unwrap_or_else(|| paths.default_dll_cache_dir())
    }

    /// The folder new backup files go to: `backup_path` when it is
    /// absolute, else `<data>/backups`. Backup indexes stay in
    /// `<data>/backups` whatever this says.
    pub fn backup_files_dir(&self, paths: &Paths) -> PathBuf {
        configured_dir(&self.backup_path).unwrap_or_else(|| paths.backups_dir())
    }

    /// The shader cache folder for a profile that turns the cache on and
    /// leaves its own path empty: `shader_cache` when it is absolute, else
    /// `<cache>/nvidia`.
    pub fn shader_cache_dir(&self, paths: &Paths) -> PathBuf {
        configured_dir(&self.shader_cache).unwrap_or_else(|| paths.default_shader_cache_dir())
    }

    /// One setting's value as people edit it, by its `config set` key: a
    /// list reads as folders joined by `:`. `None` for an unknown key.
    pub fn get(&self, key: &str) -> Option<String> {
        let b = |v: bool| v.to_string();
        Some(match key {
            "log_level" => self.log_level.name().to_owned(),
            "check_updates" => b(self.check_updates),
            "rescan_on_startup" => b(self.rescan_on_startup),
            "auto_update_dlls" => b(self.auto_update_dlls),
            "confirm_destructive" => b(self.confirm_destructive),
            "steam_path" => self.steam_path.clone(),
            "additional_library_paths" => self.additional_library_paths.join(LIST_SEPARATOR),
            "dll_cache_path" => self.dll_cache_path.clone(),
            "backup_path" => self.backup_path.clone(),
            "shader_cache" => self.shader_cache.clone(),
            "dll_manifest_url" => self.dll_manifest_url.clone(),
            "auto_refresh_manifest" => b(self.auto_refresh_manifest),
            "manifest_refresh_hours" => self.manifest_refresh_hours.to_string(),
            "ui.theme" => self.ui.theme.name().to_owned(),
            "ui.mouse" => b(self.ui.mouse),
            "ui.ascii_glyphs" => b(self.ui.ascii_glyphs),
            _ => return None,
        })
    }

    /// Sets one setting from text, as `config set <key> <value>` and the
    /// TUI's Settings do, after checking it. Nothing changes on an error.
    /// Bools take spela's one bool vocabulary. A list takes folders joined
    /// by `:` or a JSON list, as Go's option parser does.
    pub fn set(&mut self, key: &str, raw: &str) -> Result<(), ConfigSetError> {
        let option =
            option(key).ok_or_else(|| ConfigSetError(format!("unknown config key: {key}")))?;
        let boolean = || {
            parse_bool(raw)
                .ok_or_else(|| ConfigSetError(format!("invalid boolean for {key}: {raw}")))
        };
        let text = || -> Result<String, ConfigSetError> {
            check_text(option.key, raw).map_err(ConfigSetError)?;
            Ok(raw.to_owned())
        };
        match option.key {
            "log_level" => {
                self.log_level =
                    LogLevel::from_name(raw).ok_or_else(|| ConfigSetError(log_level_error(raw)))?;
            }
            "check_updates" => self.check_updates = boolean()?,
            "rescan_on_startup" => self.rescan_on_startup = boolean()?,
            "auto_update_dlls" => self.auto_update_dlls = boolean()?,
            "confirm_destructive" => self.confirm_destructive = boolean()?,
            "steam_path" => self.steam_path = text()?,
            "additional_library_paths" => self.additional_library_paths = parse_list(key, raw)?,
            "dll_cache_path" => self.dll_cache_path = text()?,
            "backup_path" => self.backup_path = text()?,
            "shader_cache" => self.shader_cache = text()?,
            "dll_manifest_url" => self.dll_manifest_url = text()?,
            "auto_refresh_manifest" => self.auto_refresh_manifest = boolean()?,
            "manifest_refresh_hours" => {
                self.manifest_refresh_hours = raw
                    .trim()
                    .parse()
                    .map_err(|_| ConfigSetError(format!("invalid integer for {key}: {raw}")))?;
            }
            "ui.theme" => {
                self.ui.theme =
                    Theme::from_name(raw).ok_or_else(|| ConfigSetError(theme_error(raw)))?;
            }
            "ui.mouse" => self.ui.mouse = boolean()?,
            "ui.ascii_glyphs" => self.ui.ascii_glyphs = boolean()?,
            other => unreachable!("option {other} has no setter"),
        }
        Ok(())
    }
}

fn configured_dir(value: &str) -> Option<PathBuf> {
    let path = Path::new(value);
    (!value.is_empty() && path.is_absolute()).then(|| path.components().collect())
}

/// The separator of a list's one-line text form.
pub const LIST_SEPARATOR: &str = ":";

fn parse_list(key: &str, raw: &str) -> Result<Vec<String>, ConfigSetError> {
    if raw.trim_start().starts_with('[') {
        return serde_json::from_str(raw)
            .map_err(|e| ConfigSetError(format!("invalid string list for {key}: {e}")));
    }
    Ok(raw
        .split(LIST_SEPARATOR)
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .map(str::to_owned)
        .collect())
}

/// Checks a text setting. The messages are Go's.
fn check_text(key: &str, value: &str) -> Result<(), String> {
    match key {
        "dll_cache_path" | "backup_path" | "shader_cache" => check_folder(key, value),
        "dll_manifest_url" if !value.is_empty() => check_manifest_url(value),
        _ => Ok(()),
    }
}

fn check_folder(key: &str, value: &str) -> Result<(), String> {
    if value.is_empty() || Path::new(value).is_absolute() {
        Ok(())
    } else {
        Err(format!("{key} must be an absolute path: {value}"))
    }
}

/// Checks a manifest URL: https, or http to this machine for a local
/// manifest server. The manifest decides which DLLs and checksums spela
/// installs, so it never travels over plain http to another host.
pub fn check_manifest_url(value: &str) -> Result<(), String> {
    let error = || "dll_manifest_url must be an https URL".to_owned();
    let uri: http::Uri = value.parse().map_err(|_| error())?;
    let host = uri.host().filter(|h| !h.is_empty()).ok_or_else(error)?;
    match uri.scheme_str() {
        Some("https") => Ok(()),
        Some("http") if is_loopback(host) => Ok(()),
        _ => Err(error()),
    }
}

/// `localhost`, `::1`, or an IPv4 address in 127.0.0.0/8, as Go's
/// `IsLoopback` counts them for a host name in a URL.
fn is_loopback(host: &str) -> bool {
    let host = host.trim_start_matches('[').trim_end_matches(']');
    if host.eq_ignore_ascii_case("localhost") || host == "::1" {
        return true;
    }
    let octets: Vec<&str> = host.split('.').collect();
    octets.len() == 4
        && octets[0] == "127"
        && octets
            .iter()
            .all(|o| !o.is_empty() && o.len() <= 3 && o.parse::<u8>().is_ok())
}

fn log_level_error(value: &str) -> String {
    format!("unsupported log level: {value}")
}

fn theme_error(value: &str) -> String {
    format!("unsupported theme: {value} (use terminal, neon or mono)")
}

/// A setting that cannot be set to a value. The text is for the user.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("{0}")]
pub struct ConfigSetError(pub String);

/// A Settings section, in the TUI's order.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum SettingsSection {
    /// Theme, mouse, glyphs and confirmations.
    Display,
    /// What happens when spela starts.
    Startup,
    /// Folders.
    Paths,
    /// The DLL manifest.
    DllPolicy,
    /// The log level.
    Logging,
}

impl SettingsSection {
    /// Every section, in order.
    pub const ALL: [SettingsSection; 5] = [
        SettingsSection::Display,
        SettingsSection::Startup,
        SettingsSection::Paths,
        SettingsSection::DllPolicy,
        SettingsSection::Logging,
    ];

    /// The heading, as Go's TUI shows it.
    pub fn title(self) -> &'static str {
        match self {
            SettingsSection::Display => "Display",
            SettingsSection::Startup => "Startup",
            SettingsSection::Paths => "Paths",
            SettingsSection::DllPolicy => "DLL policy",
            SettingsSection::Logging => "Logging",
        }
    }

    /// This section's settings, in display order.
    pub fn options(self) -> impl Iterator<Item = &'static ConfigOption> {
        OPTIONS.iter().filter(move |o| o.section == self)
    }
}

/// How a setting is edited.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum OptionKind {
    /// On or Off.
    Bool,
    /// One of these words.
    Choice(&'static [&'static str]),
    /// A whole number, with these suggested values.
    Int(&'static [&'static str]),
    /// Free text.
    Text,
    /// An absolute folder, or empty for the default.
    Folder,
    /// A URL, or empty for the default.
    Url,
    /// A list of folders.
    FolderList,
}

/// One setting: what `config set` takes and the TUI's Settings shows.
#[derive(Debug, PartialEq, Eq)]
pub struct ConfigOption {
    /// The `config set` key, such as `check_updates` or `ui.theme`.
    pub key: &'static str,
    /// The Settings section.
    pub section: SettingsSection,
    /// The label.
    pub label: &'static str,
    /// The help text.
    pub description: &'static str,
    /// How it is edited.
    pub kind: OptionKind,
}

/// The setting with this `config set` key.
pub fn option(key: &str) -> Option<&'static ConfigOption> {
    OPTIONS.iter().find(|o| o.key == key)
}

/// Every setting, by section and then in Go's Settings order. `version` is
/// not a setting.
pub static OPTIONS: [ConfigOption; 16] = [
    ConfigOption {
        key: "ui.theme",
        section: SettingsSection::Display,
        label: "Theme",
        description: "Colours. terminal uses your terminal's palette, neon is spela's own, mono uses none.",
        kind: OptionKind::Choice(&["terminal", "neon", "mono"]),
    },
    ConfigOption {
        key: "ui.mouse",
        section: SettingsSection::Display,
        label: "Mouse",
        description: "Clicks, the wheel and right click. Off gives the terminal's own text selection back.",
        kind: OptionKind::Bool,
    },
    ConfigOption {
        key: "ui.ascii_glyphs",
        section: SettingsSection::Display,
        label: "ASCII glyphs",
        description: "Draw boxes, markers and sparklines in plain ASCII, for fonts that show the symbols wrong.",
        kind: OptionKind::Bool,
    },
    ConfigOption {
        key: "confirm_destructive",
        section: SettingsSection::Display,
        label: "Confirm destructive actions",
        description: "Ask before updating DLLs. Restoring and installing DLLs always ask.",
        kind: OptionKind::Bool,
    },
    ConfigOption {
        key: "rescan_on_startup",
        section: SettingsSection::Startup,
        label: "Re-scan on startup",
        description: "Scan for games when spela next starts. Requires restart.",
        kind: OptionKind::Bool,
    },
    ConfigOption {
        key: "auto_update_dlls",
        section: SettingsSection::Startup,
        label: "Auto-update DLLs",
        description: "At startup, update DLLs older than the manifest's latest version. Running and deny-listed games and DLLs of unknown version are skipped.",
        kind: OptionKind::Bool,
    },
    ConfigOption {
        key: "check_updates",
        section: SettingsSection::Startup,
        label: "Check for updates",
        description: "At startup, ask GitHub once a day whether a newer spela release exists and show a notice. Development builds never ask.",
        kind: OptionKind::Bool,
    },
    ConfigOption {
        key: "steam_path",
        section: SettingsSection::Paths,
        label: "Steam path",
        description: "Use this Steam installation for future manual scans and when spela next starts.",
        kind: OptionKind::Text,
    },
    ConfigOption {
        key: "additional_library_paths",
        section: SettingsSection::Paths,
        label: "Additional library paths",
        description: "Additional Steam library folders, separated by colons.",
        kind: OptionKind::FolderList,
    },
    ConfigOption {
        key: "dll_cache_path",
        section: SettingsSection::Paths,
        label: "DLL cache path",
        description: "Folder for downloaded DLLs. Empty uses spela's cache folder. Earlier downloads are not moved.",
        kind: OptionKind::Folder,
    },
    ConfigOption {
        key: "backup_path",
        section: SettingsSection::Paths,
        label: "Backup path",
        description: "Folder for new DLL backup files. Empty uses spela's data folder. Earlier backups stay restorable.",
        kind: OptionKind::Folder,
    },
    ConfigOption {
        key: "shader_cache",
        section: SettingsSection::Paths,
        label: "Default shader cache path",
        description: "Folder for the NVIDIA shader cache when a profile turns Shader cache on and leaves its path empty. Empty uses spela's cache folder.",
        kind: OptionKind::Folder,
    },
    ConfigOption {
        key: "auto_refresh_manifest",
        section: SettingsSection::DllPolicy,
        label: "Auto-refresh manifest",
        description: "Fetch a new DLL manifest when the cached one is older than the refresh interval. Off keeps the cached manifest.",
        kind: OptionKind::Bool,
    },
    ConfigOption {
        key: "manifest_refresh_hours",
        section: SettingsSection::DllPolicy,
        label: "Refresh interval",
        description: "Hours before the cached DLL manifest counts as old.",
        kind: OptionKind::Int(&["1", "6", "12", "24", "48", "168"]),
    },
    ConfigOption {
        key: "dll_manifest_url",
        section: SettingsSection::DllPolicy,
        label: "DLL manifest URL",
        description: "Where spela gets the list of DLLs and checksums it installs. Empty uses spela's own manifest. A new URL is fetched at next use.",
        kind: OptionKind::Url,
    },
    ConfigOption {
        key: "log_level",
        section: SettingsSection::Logging,
        label: "Log level",
        description: "Control logging verbosity for troubleshooting.",
        kind: OptionKind::Choice(&["debug", "info", "warn", "error"]),
    },
];

/// A problem at a place in the file. Line and column start at 1; both are 0
/// when the place is unknown.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Diagnostic {
    /// 1-based line, or 0.
    pub line: usize,
    /// 1-based column in characters, or 0.
    pub column: usize,
    /// What is wrong, in plain words.
    pub message: String,
}

impl fmt::Display for Diagnostic {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.line == 0 {
            f.write_str(&self.message)
        } else {
            write!(
                f,
                "line {}, column {}: {}",
                self.line, self.column, self.message
            )
        }
    }
}

/// Why the config could not be loaded. The CLI exits 3 on either.
#[derive(Debug, thiserror::Error)]
pub enum ConfigError {
    /// The file exists but cannot be read.
    #[error("cannot read config file {path}: {source}")]
    Io {
        /// The file.
        path: PathBuf,
        /// The cause.
        source: std::io::Error,
    },
    /// The file is not valid.
    #[error("{}", format_invalid(path.as_deref(), errors))]
    Invalid {
        /// The file, when the text came from one.
        path: Option<PathBuf>,
        /// Every problem found, in file order.
        errors: Vec<Diagnostic>,
    },
}

fn format_invalid(path: Option<&Path>, errors: &[Diagnostic]) -> String {
    let mut out = match path {
        Some(p) => format!("config file {} has errors:", p.display()),
        None => "config has errors:".to_owned(),
    };
    for e in errors {
        out.push_str("\n  ");
        out.push_str(&e.to_string());
    }
    out
}

/// A loaded config and the warnings found on the way.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Loaded {
    /// The config, with defaults filled in.
    pub config: Config,
    /// Unknown keys and other things that do not stop spela, in file order.
    pub warnings: Vec<Diagnostic>,
    /// Whether a file was read. `false` means defaults only.
    pub from_file: bool,
    /// The file has a newer `version` than this spela knows, so spela must
    /// not write it.
    pub read_only: bool,
}

/// Reads the config at `path`. A missing file gives the defaults.
pub fn load(path: &Path) -> Result<Loaded, ConfigError> {
    match std::fs::read_to_string(path) {
        Ok(text) => match parse(&text) {
            Ok(mut loaded) => {
                loaded.from_file = true;
                Ok(loaded)
            }
            Err(ConfigError::Invalid { errors, .. }) => Err(ConfigError::Invalid {
                path: Some(path.to_owned()),
                errors,
            }),
            Err(other) => Err(other),
        },
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(Loaded {
            config: Config::default(),
            warnings: Vec::new(),
            from_file: false,
            read_only: false,
        }),
        Err(source) => Err(ConfigError::Io {
            path: path.to_owned(),
            source,
        }),
    }
}

/// The file's shape before checks: every key with its TOML type.
#[derive(Debug, Deserialize)]
#[serde(default)]
struct RawConfig {
    version: i64,
    log_level: String,
    check_updates: bool,
    rescan_on_startup: bool,
    auto_update_dlls: bool,
    confirm_destructive: bool,
    steam_path: String,
    additional_library_paths: Vec<String>,
    dll_cache_path: String,
    backup_path: String,
    shader_cache: String,
    dll_manifest_url: String,
    auto_refresh_manifest: bool,
    manifest_refresh_hours: i64,
    ui: RawUi,
}

#[derive(Debug, Deserialize)]
#[serde(default)]
struct RawUi {
    theme: String,
    mouse: bool,
    ascii_glyphs: bool,
}

impl Default for RawConfig {
    fn default() -> RawConfig {
        let c = Config::default();
        RawConfig {
            version: i64::from(c.version),
            log_level: c.log_level.name().to_owned(),
            check_updates: c.check_updates,
            rescan_on_startup: c.rescan_on_startup,
            auto_update_dlls: c.auto_update_dlls,
            confirm_destructive: c.confirm_destructive,
            steam_path: c.steam_path,
            additional_library_paths: c.additional_library_paths,
            dll_cache_path: c.dll_cache_path,
            backup_path: c.backup_path,
            shader_cache: c.shader_cache,
            dll_manifest_url: c.dll_manifest_url,
            auto_refresh_manifest: c.auto_refresh_manifest,
            manifest_refresh_hours: c.manifest_refresh_hours,
            ui: RawUi::default(),
        }
    }
}

impl Default for RawUi {
    fn default() -> RawUi {
        let u = UiSettings::default();
        RawUi {
            theme: u.theme.name().to_owned(),
            mouse: u.mouse,
            ascii_glyphs: u.ascii_glyphs,
        }
    }
}

/// Parses config text.
pub fn parse(text: &str) -> Result<Loaded, ConfigError> {
    let doc = DeTable::parse(text).map_err(|e| invalid(vec![from_toml_error(text, &e)]))?;
    let raw: RawConfig =
        toml::from_str(text).map_err(|e| invalid(vec![from_toml_error(text, &e)]))?;
    let doc = doc.get_ref();
    let mut warnings = Vec::new();
    let mut errors = Vec::new();
    check_unknown_keys(text, doc, &mut warnings);

    let mut read_only = false;
    let version = match u32::try_from(raw.version) {
        Ok(v) if v >= 1 => {
            if v > CONFIG_VERSION {
                read_only = true;
                warnings.push(at(
                    text,
                    span_of(doc, &["version"]),
                    format!(
                        "config version {v} is newer than this spela knows ({CONFIG_VERSION}), so spela reads it but never writes it"
                    ),
                ));
            }
            v
        }
        _ => {
            errors.push(at(
                text,
                span_of(doc, &["version"]),
                format!("`version` is {}, but must be 1 or higher", raw.version),
            ));
            CONFIG_VERSION
        }
    };
    let log_level = LogLevel::from_name(&raw.log_level).unwrap_or_else(|| {
        errors.push(at(
            text,
            span_of(doc, &["log_level"]),
            log_level_error(&raw.log_level),
        ));
        LogLevel::default()
    });
    let theme = Theme::from_name(&raw.ui.theme).unwrap_or_else(|| {
        errors.push(at(
            text,
            span_of(doc, &["ui", "theme"]),
            theme_error(&raw.ui.theme),
        ));
        Theme::default()
    });
    for (key, value) in [
        ("dll_cache_path", &raw.dll_cache_path),
        ("backup_path", &raw.backup_path),
        ("shader_cache", &raw.shader_cache),
        ("dll_manifest_url", &raw.dll_manifest_url),
    ] {
        if let Err(message) = check_text(key, value) {
            errors.push(at(text, span_of(doc, &[key]), message));
        }
    }

    warnings.sort_by_key(|d| (d.line, d.column));
    errors.sort_by_key(|d| (d.line, d.column));
    if !errors.is_empty() {
        return Err(invalid(errors));
    }
    Ok(Loaded {
        config: Config {
            version,
            log_level,
            check_updates: raw.check_updates,
            rescan_on_startup: raw.rescan_on_startup,
            auto_update_dlls: raw.auto_update_dlls,
            confirm_destructive: raw.confirm_destructive,
            steam_path: raw.steam_path,
            additional_library_paths: raw.additional_library_paths,
            dll_cache_path: raw.dll_cache_path,
            backup_path: raw.backup_path,
            shader_cache: raw.shader_cache,
            dll_manifest_url: raw.dll_manifest_url,
            auto_refresh_manifest: raw.auto_refresh_manifest,
            manifest_refresh_hours: raw.manifest_refresh_hours,
            ui: UiSettings {
                theme,
                mouse: raw.ui.mouse,
                ascii_glyphs: raw.ui.ascii_glyphs,
            },
        },
        warnings,
        from_file: false,
        read_only,
    })
}

fn invalid(errors: Vec<Diagnostic>) -> ConfigError {
    ConfigError::Invalid { path: None, errors }
}

fn from_toml_error(text: &str, e: &toml::de::Error) -> Diagnostic {
    let (line, column) = e.span().map(|s| line_col(text, s.start)).unwrap_or((0, 0));
    Diagnostic {
        line,
        column,
        message: e.message().trim().to_owned(),
    }
}

/// 1-based line and character column of a byte offset.
pub fn line_col(text: &str, offset: usize) -> (usize, usize) {
    let offset = offset.min(text.len());
    let mut start = offset;
    while !text.is_char_boundary(start) {
        start -= 1;
    }
    let before = &text[..start];
    let line = before.matches('\n').count() + 1;
    let line_start = before.rfind('\n').map_or(0, |i| i + 1);
    let column = before[line_start..].chars().count() + 1;
    (line, column)
}

fn at(text: &str, span: Range<usize>, message: String) -> Diagnostic {
    let (line, column) = line_col(text, span.start);
    Diagnostic {
        line,
        column,
        message,
    }
}

const TOP_KEYS: &[&str] = &[
    "version",
    "log_level",
    "check_updates",
    "rescan_on_startup",
    "auto_update_dlls",
    "confirm_destructive",
    "steam_path",
    "additional_library_paths",
    "dll_cache_path",
    "backup_path",
    "shader_cache",
    "dll_manifest_url",
    "auto_refresh_manifest",
    "manifest_refresh_hours",
    "ui",
];
const UI_KEYS: &[&str] = &["theme", "mouse", "ascii_glyphs"];

fn check_unknown_keys(text: &str, doc: &DeTable<'_>, warnings: &mut Vec<Diagnostic>) {
    unknown_in(text, doc, "", TOP_KEYS, warnings);
    if let Some(DeValue::Table(ui)) = doc.get("ui").map(Spanned::get_ref) {
        unknown_in(text, ui, "ui.", UI_KEYS, warnings);
    }
}

fn unknown_in(
    text: &str,
    table: &DeTable<'_>,
    prefix: &str,
    known: &[&str],
    warnings: &mut Vec<Diagnostic>,
) {
    for (key, _) in table.iter() {
        let name: &str = key.get_ref();
        if known.contains(&name) {
            continue;
        }
        let mut message = format!("unknown key `{prefix}{name}` is ignored");
        if let Some(guess) = closest(name, known) {
            message.push_str(&format!(" (did you mean `{prefix}{guess}`?)"));
        }
        warnings.push(at(text, key.span(), message));
    }
}

/// The known key closest to `name`, if it is a likely typo. mejl's rule.
pub fn closest<'a>(name: &str, known: &[&'a str]) -> Option<&'a str> {
    known
        .iter()
        .map(|k| (edit_distance(name, k), *k))
        .filter(|(d, k)| *d <= 2.max(k.len() / 4))
        .min_by_key(|(d, _)| *d)
        .map(|(_, k)| k)
}

fn edit_distance(a: &str, b: &str) -> usize {
    let a: Vec<char> = a.chars().collect();
    let b: Vec<char> = b.chars().collect();
    let mut prev: Vec<usize> = (0..=b.len()).collect();
    for (i, ca) in a.iter().enumerate() {
        let mut cur = vec![i + 1; b.len() + 1];
        for (j, cb) in b.iter().enumerate() {
            let cost = usize::from(ca != cb);
            cur[j + 1] = (prev[j] + cost).min(prev[j + 1] + 1).min(cur[j] + 1);
        }
        prev = cur;
    }
    prev[b.len()]
}

/// The span of a value by path, or of the closest table that exists.
fn span_of(doc: &DeTable<'_>, path: &[&str]) -> Range<usize> {
    let mut table = doc;
    let mut span = 0..0;
    for (i, part) in path.iter().enumerate() {
        let Some(value) = table.get(*part) else {
            break;
        };
        span = value.span();
        match value.get_ref() {
            DeValue::Table(t) if i + 1 < path.len() => table = t,
            _ => break,
        }
    }
    span
}

#[cfg(test)]
mod tests {
    use super::*;

    fn errors(text: &str) -> Vec<String> {
        match parse(text) {
            Err(ConfigError::Invalid { errors, .. }) => {
                errors.iter().map(ToString::to_string).collect()
            }
            other => panic!("expected errors, got {other:?}"),
        }
    }

    #[test]
    fn empty_text_is_every_default() {
        let loaded = parse("").unwrap();
        assert_eq!(loaded.config, Config::default());
        assert!(loaded.warnings.is_empty());
        assert!(!loaded.read_only);
        let c = &loaded.config;
        assert_eq!(c.log_level, LogLevel::Info);
        assert!(c.check_updates && c.rescan_on_startup && c.confirm_destructive);
        assert!(!c.auto_update_dlls);
        assert!(c.auto_refresh_manifest);
        assert_eq!(c.manifest_refresh_hours, 24);
        assert_eq!(
            c.ui,
            UiSettings {
                theme: Theme::Terminal,
                mouse: true,
                ascii_glyphs: false
            }
        );
    }

    #[test]
    fn every_key_reads() {
        let text = [
            "version = 1",
            "log_level = \"debug\"",
            "check_updates = false",
            "rescan_on_startup = false",
            "auto_update_dlls = true",
            "confirm_destructive = false",
            "steam_path = \"/opt/steam\"",
            "additional_library_paths = [\"/games\", \"/mnt/more games\"]",
            "dll_cache_path = \"/c/dlls\"",
            "backup_path = \"/d/backups\"",
            "shader_cache = \"/c/nvidia\"",
            "dll_manifest_url = \"http://127.0.0.1:8080/manifest.json\"",
            "auto_refresh_manifest = false",
            "manifest_refresh_hours = 0",
            "",
            "[ui]",
            "theme = \"neon\"",
            "mouse = false",
            "ascii_glyphs = true",
        ]
        .join("\n");
        let loaded = parse(&text).unwrap();
        assert!(loaded.warnings.is_empty(), "{:?}", loaded.warnings);
        let c = loaded.config;
        assert_eq!(c.log_level, LogLevel::Debug);
        assert_eq!(c.additional_library_paths, ["/games", "/mnt/more games"]);
        assert_eq!(c.manifest_refresh_hours, 0);
        assert_eq!(c.ui.theme, Theme::Neon);
        assert!(!c.ui.mouse && c.ui.ascii_glyphs);
        for option in &OPTIONS {
            assert!(c.get(option.key).is_some(), "{}", option.key);
        }
    }

    #[test]
    fn bad_values_report_line_and_column_with_go_s_text() {
        let text = "log_level = \"verbose\"\nshader_cache = \"cache\"\n\n[ui]\ntheme = \"dark\"\n";
        assert_eq!(
            errors(text),
            [
                "line 1, column 13: unsupported log level: verbose",
                "line 2, column 16: shader_cache must be an absolute path: cache",
                "line 5, column 9: unsupported theme: dark (use terminal, neon or mono)",
            ]
        );
        assert_eq!(
            errors("dll_manifest_url = \"http://example.com/m.json\""),
            ["line 1, column 20: dll_manifest_url must be an https URL"]
        );
        let wrong_type = errors("manifest_refresh_hours = \"daily\"\n");
        assert_eq!(wrong_type.len(), 1);
        assert!(
            wrong_type[0].starts_with("line 1, column 26: "),
            "{wrong_type:?}"
        );
        assert!(
            errors("version = 0")
                .iter()
                .any(|e| e.contains("must be 1 or higher"))
        );
        let broken = errors("log_level = \n");
        assert!(broken[0].starts_with("line 1"), "{broken:?}");
    }

    #[test]
    fn unknown_keys_warn_with_a_suggestion() {
        let text =
            "check_update = true\ntheme = \"light\"\ncompact_mode = true\n[ui]\nmose = false\n";
        let loaded = parse(text).unwrap();
        let w: Vec<String> = loaded.warnings.iter().map(ToString::to_string).collect();
        assert_eq!(
            w,
            [
                "line 1, column 1: unknown key `check_update` is ignored (did you mean `check_updates`?)",
                "line 2, column 1: unknown key `theme` is ignored",
                "line 3, column 1: unknown key `compact_mode` is ignored",
                "line 5, column 1: unknown key `ui.mose` is ignored (did you mean `ui.mouse`?)",
            ]
        );
        assert!(loaded.config.check_updates, "unknown keys change nothing");
    }

    #[test]
    fn a_newer_version_is_read_only() {
        let loaded = parse("version = 2\ncheck_updates = false\n").unwrap();
        assert!(loaded.read_only);
        assert_eq!(loaded.config.version, 2);
        assert!(!loaded.config.check_updates);
        assert!(loaded.warnings[0].message.contains("never writes it"));
    }

    #[test]
    fn load_names_the_file_and_defaults_when_missing() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("config.toml");
        let loaded = load(&path).unwrap();
        assert!(!loaded.from_file);
        std::fs::write(&path, "log_level = \"loud\"\n").unwrap();
        let e = load(&path).unwrap_err().to_string();
        assert!(
            e.starts_with(&format!("config file {} has errors:", path.display())),
            "{e}"
        );
        std::fs::write(&path, "log_level = \"warn\"\n").unwrap();
        let loaded = load(&path).unwrap();
        assert!(loaded.from_file);
        assert_eq!(loaded.config.log_level, LogLevel::Warn);
    }

    #[test]
    fn set_checks_and_parses_like_go() {
        let mut c = Config::default();
        c.set("check_updates", "off").unwrap();
        assert!(!c.check_updates);
        c.set("ui.theme", "mono").unwrap();
        assert_eq!(c.ui.theme, Theme::Mono);
        c.set("additional_library_paths", "/a: /b ::").unwrap();
        assert_eq!(c.additional_library_paths, ["/a", "/b"]);
        c.set("additional_library_paths", r#"["/x:y"]"#).unwrap();
        assert_eq!(c.additional_library_paths, ["/x:y"]);
        assert_eq!(c.get("additional_library_paths").unwrap(), "/x:y");
        c.set("manifest_refresh_hours", "48").unwrap();
        assert_eq!(c.manifest_refresh_hours, 48);
        c.set("dll_manifest_url", "").unwrap();
        c.set("dll_manifest_url", "https://example.com/m.json")
            .unwrap();
        c.set("dll_manifest_url", "http://localhost/m.json")
            .unwrap();
        c.set("dll_manifest_url", "http://[::1]:9/m.json").unwrap();
        c.set("dll_manifest_url", "http://127.1.2.3/m.json")
            .unwrap();
        assert!(
            c.set("dll_manifest_url", "http://127.0.0.1.evil/m")
                .is_err()
        );
        assert!(c.set("dll_manifest_url", "http://1270.0.0.1/m").is_err());
        let before = c.clone();
        let err = |k: &str, v: &str, c: &mut Config| c.set(k, v).unwrap_err().to_string();
        assert_eq!(
            err("not_a_key", "1", &mut c),
            "unknown config key: not_a_key"
        );
        assert_eq!(
            err("log_level", "verbose", &mut c),
            "unsupported log level: verbose"
        );
        assert_eq!(
            err("check_updates", "maybe", &mut c),
            "invalid boolean for check_updates: maybe"
        );
        assert_eq!(
            err("manifest_refresh_hours", "daily", &mut c),
            "invalid integer for manifest_refresh_hours: daily"
        );
        assert_eq!(
            err("backup_path", "rel", &mut c),
            "backup_path must be an absolute path: rel"
        );
        assert_eq!(
            err("dll_manifest_url", "http://example.com/m", &mut c),
            "dll_manifest_url must be an https URL"
        );
        assert_eq!(
            err("compact_mode", "true", &mut c),
            "unknown config key: compact_mode"
        );
        assert_eq!(err("version", "2", &mut c), "unknown config key: version");
        assert_eq!(c, before, "a failed set changes nothing");
    }

    #[test]
    fn folders_resolve_against_paths() {
        let paths = Paths::under(Path::new("/b"));
        let mut c = Config::default();
        assert_eq!(c.dll_cache_dir(&paths), PathBuf::from("/b/cache/dlls"));
        assert_eq!(c.backup_files_dir(&paths), PathBuf::from("/b/data/backups"));
        assert_eq!(c.shader_cache_dir(&paths), PathBuf::from("/b/cache/nvidia"));
        c.dll_cache_path = "/fast/dlls/".into();
        c.shader_cache = "relative".into();
        assert_eq!(c.dll_cache_dir(&paths), PathBuf::from("/fast/dlls"));
        assert_eq!(c.shader_cache_dir(&paths), PathBuf::from("/b/cache/nvidia"));
    }

    #[test]
    fn options_cover_each_section() {
        assert_eq!(OPTIONS.len(), 16);
        let count: usize = SettingsSection::ALL
            .iter()
            .map(|s| s.options().count())
            .sum();
        assert_eq!(count, 16);
        for o in &OPTIONS {
            assert_eq!(option(o.key), Some(o));
            assert!(Config::default().get(o.key).is_some(), "{}", o.key);
            assert!(!o.description.contains('\u{2014}'));
        }
        let mut sections: Vec<_> = OPTIONS.iter().map(|o| o.section).collect();
        let sorted = {
            let mut s = sections.clone();
            s.sort();
            s
        };
        assert_eq!(sections, sorted, "grouped by section");
        sections.dedup();
        assert_eq!(sections, SettingsSection::ALL);
    }
}
