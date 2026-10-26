//! The Proton resolver: which Proton build a game runs under, from `config.vdf` and the compatibility tool folders.
//!
//! Steam records the tool for each game in `<root>/config/config.vdf` at
//! `InstallConfigStore > Software > Valve > Steam > CompatToolMapping`. The
//! resolver tries the game's AppID, then the global default `"0"` (Go's
//! `internal/proton/resolver.go`). Keys are matched without regard to case.
//!
//! The mapped name is the tool's internal name, not its folder name
//! (verified on this machine, docs/phase0-checks.md). So the build folder
//! is looked for in this order:
//!
//! 1. A `compatibilitytools.d/*/compatibilitytool.vdf` under the Steam
//!    root that declares the name, at its `install_path`. This finds
//!    `GE-Proton11-7-x86_64` in the `GE-Proton11-7` folder.
//! 2. `compatibilitytools.d/<name>` under the Steam root, Go's rule.
//! 3. For a built-in short name such as `proton_experimental`, the tool's
//!    app manifest in any library, then its `installdir`.
//! 4. `steamapps/common/<name>` in every library. Go looked only under the
//!    Steam root, where most tools are not.
//! 5. For a built-in short name, its usual folder in every library.
//!
//! `/usr/share/steam/compatibilitytools.d` is not searched; Go never looked
//! there and it does not exist on the machine checked.
//!
//! The rest is Go's descriptor heap gate, kept as it was: the legacy
//! `PROTON_VKD3D_HEAP` marker in the build's `proton` script, the
//! Proton-CachyOS build tag, the NVIDIA driver version and the notices.

use std::cmp::Ordering;
use std::fmt;
use std::io;
use std::path::{Component, Path, PathBuf};
use std::sync::LazyLock;

use regex_lite::Regex;

use crate::steam::library::SteamInstall;
use crate::steam::manifest::{self, common_dir};
use crate::steam::vdf::{self, VdfFileError};

/// The first NVIDIA driver with `VK_EXT_descriptor_heap`.
pub const MIN_DRIVER_VERSION: &str = "580.94.16";
/// The first Proton-CachyOS build with descriptor_heap behind
/// `PROTON_VKD3D_HEAP`.
pub const MIN_PROTON_CACHYOS_BUILD: &str = "10.0-20260321";
/// The first Proton-CachyOS build with descriptor_heap built into
/// vkd3d-proton, which needs only `VKD3D_CONFIG`.
pub const MIN_PROTON_CACHYOS_INTEGRATED_BUILD: &str = "11.0-20260520";

/// The marker of the legacy gate in a build's `proton` script.
pub const LEGACY_HEAP_MARKER: &str = "PROTON_VKD3D_HEAP";

/// The tool folder under the Steam root.
pub const COMPAT_TOOLS_DIR: &str = "compatibilitytools.d";
/// The file in a tool folder that names its tools.
pub const COMPAT_TOOL_VDF: &str = "compatibilitytool.vdf";

/// The path to the mapping inside `config.vdf`.
const MAPPING_PATH: [&str; 5] = [
    "InstallConfigStore",
    "Software",
    "Valve",
    "Steam",
    "CompatToolMapping",
];

/// A Steam built-in compatibility tool: its short name in
/// `CompatToolMapping`, its AppID and its usual folder.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct BuiltInTool {
    /// The short name, such as `proton_experimental`.
    pub name: &'static str,
    /// The tool's AppID, whose app manifest names its folder.
    pub app_id: u64,
    /// The folder Steam usually installs it in.
    pub folder: &'static str,
}

/// The built-in tools the resolver knows. The first four names and folders
/// were seen on the machine checked (docs/phase0-checks.md); the AppIDs are
/// Go's tool list. The last three entries are inferred.
pub const BUILT_IN_TOOLS: [BuiltInTool; 7] = [
    BuiltInTool {
        name: "proton_experimental",
        app_id: 1493710,
        folder: "Proton - Experimental",
    },
    BuiltInTool {
        name: "proton_hotfix",
        app_id: 2180100,
        folder: "Proton Hotfix",
    },
    BuiltInTool {
        name: "proton_9",
        app_id: 2805730,
        folder: "Proton 9.0 (Beta)",
    },
    BuiltInTool {
        name: "proton_8",
        app_id: 2348590,
        folder: "Proton 8.0",
    },
    BuiltInTool {
        name: "proton_7",
        app_id: 1887720,
        folder: "Proton 7.0",
    },
    BuiltInTool {
        name: "proton_63",
        app_id: 1580130,
        folder: "Proton 6.3",
    },
    BuiltInTool {
        name: "proton_5",
        app_id: 1245040,
        folder: "Proton 5.0",
    },
];

/// The built-in tool with this short name.
pub fn built_in_tool(name: &str) -> Option<&'static BuiltInTool> {
    BUILT_IN_TOOLS.iter().find(|t| t.name == name)
}

/// A Proton build on disk.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Build {
    /// The tool's name in `CompatToolMapping`, such as `GE-Proton10-34`
    /// or `proton_experimental`. Never empty.
    pub name: String,
    /// The build folder, which holds the `proton` script.
    pub path: PathBuf,
}

/// Why no build was found. Every case means the check did not run, never
/// that the feature is off.
#[derive(Debug, thiserror::Error)]
pub enum ResolveError {
    /// No Steam root was configured or found.
    #[error("proton build not resolved: no Steam root")]
    NoSteamRoot,
    /// `config.vdf` cannot be read or parsed.
    #[error("proton build not resolved: {0}")]
    Config(VdfFileError),
    /// `config.vdf` has no `CompatToolMapping`.
    #[error("proton build not resolved: no CompatToolMapping in {}", .0.display())]
    NoMappingTable(PathBuf),
    /// Neither the game nor the global default has a tool.
    #[error("proton build not resolved: no mapping for appid {0} or global default")]
    NoMapping(u64),
    /// The mapped tool has no folder anywhere searched.
    #[error("proton build not resolved: build dir for {0:?} not found")]
    BuildNotFound(String),
}

/// The Proton build Steam launches `app_id` with.
pub fn resolve(install: &SteamInstall, app_id: u64) -> Result<Build, ResolveError> {
    let root = install.root.as_deref().ok_or(ResolveError::NoSteamRoot)?;
    let name = mapped_tool(root, app_id)?;
    let path =
        locate_build(install, &name).ok_or_else(|| ResolveError::BuildNotFound(name.clone()))?;
    Ok(Build { name, path })
}

/// The tool name `CompatToolMapping` gives `app_id`, else the global
/// default `"0"`.
pub fn mapped_tool(steam_root: &Path, app_id: u64) -> Result<String, ResolveError> {
    let path = steam_root.join("config").join("config.vdf");
    let root = vdf::read(&path).map_err(ResolveError::Config)?;
    let mapping = root
        .path(&MAPPING_PATH)
        .ok_or_else(|| ResolveError::NoMappingTable(path.clone()))?;
    [app_id.to_string(), "0".to_owned()]
        .iter()
        .find_map(|key| {
            let name = mapping.node(key)?.text("name")?;
            (!name.is_empty()).then(|| name.to_owned())
        })
        .ok_or(ResolveError::NoMapping(app_id))
}

/// A tool declared in a `compatibilitytool.vdf`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CompatTool {
    /// The internal name, the key under `compat_tools`.
    pub name: String,
    /// Its folder: the tool folder joined with `install_path`.
    pub path: PathBuf,
}

/// The tools declared under `<root>/compatibilitytools.d`, in folder name
/// order. A folder without a readable `compatibilitytool.vdf`, or an
/// `install_path` that is absolute or climbs out with `..`, is skipped.
pub fn compat_tools(steam_root: &Path) -> Vec<CompatTool> {
    let Ok(entries) = std::fs::read_dir(steam_root.join(COMPAT_TOOLS_DIR)) else {
        return Vec::new();
    };
    let mut folders: Vec<PathBuf> = entries.filter_map(Result::ok).map(|e| e.path()).collect();
    folders.sort();
    let mut tools = Vec::new();
    for folder in folders {
        let Ok(root) = vdf::read(&folder.join(COMPAT_TOOL_VDF)) else {
            continue;
        };
        let Some(declared) = root.path(&["compatibilitytools", "compat_tools"]) else {
            continue;
        };
        for (name, item) in declared.children() {
            let Some(tool) = item.as_node() else {
                continue;
            };
            let install_path = Path::new(tool.text("install_path").unwrap_or("."));
            let plain = install_path
                .components()
                .all(|c| matches!(c, Component::Normal(_) | Component::CurDir));
            if plain {
                tools.push(CompatTool {
                    name: name.to_owned(),
                    path: folder.join(install_path).components().collect(),
                });
            }
        }
    }
    tools
}

/// The folder of the tool named `name`, by the search order in the module
/// docs. `None` when no candidate folder exists.
pub fn locate_build(install: &SteamInstall, name: &str) -> Option<PathBuf> {
    let one_folder = is_folder_name(name);
    let mut candidates: Vec<PathBuf> = Vec::new();
    if let Some(root) = &install.root {
        candidates.extend(
            compat_tools(root)
                .into_iter()
                .filter(|t| t.name == name)
                .map(|t| t.path),
        );
        if one_folder {
            candidates.push(root.join(COMPAT_TOOLS_DIR).join(name));
        }
    }
    let libraries: Vec<&Path> = install.library_paths().collect();
    let built_in = built_in_tool(name);
    if let Some(tool) = built_in {
        for library in &libraries {
            if let Ok(Some(app)) = manifest::read(&manifest::manifest_path(library, tool.app_id))
                && let Some(dir) = app.full_install_dir()
            {
                candidates.push(dir);
            }
        }
    }
    if one_folder {
        candidates.extend(libraries.iter().map(|l| common_dir(l).join(name)));
    }
    if let Some(tool) = built_in {
        candidates.extend(libraries.iter().map(|l| common_dir(l).join(tool.folder)));
    }
    candidates.into_iter().find(|c| c.is_dir())
}

fn is_folder_name(name: &str) -> bool {
    let mut components = Path::new(name).components();
    matches!(
        (components.next(), components.next()),
        (Some(Component::Normal(_)), None)
    )
}

/// Whether descriptor_heap still needs `PROTON_VKD3D_HEAP=1`: the build's
/// `proton` script names it. A missing script is `false`, not an error.
pub fn uses_legacy_heap_gate(build: &Build) -> io::Result<bool> {
    if build.path.as_os_str().is_empty() {
        return Ok(false);
    }
    match std::fs::read(build.path.join("proton")) {
        Ok(script) => Ok(script
            .windows(LEGACY_HEAP_MARKER.len())
            .any(|w| w == LEGACY_HEAP_MARKER.as_bytes())),
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(false),
        Err(e) => Err(e),
    }
}

/// Whether a build can honour `VKD3D_CONFIG=descriptor_heap`: it has the
/// legacy gate, or its CachyOS tag is at least
/// [`MIN_PROTON_CACHYOS_INTEGRATED_BUILD`].
pub fn supports_vkd3d_heap(build: &Build) -> io::Result<bool> {
    if uses_legacy_heap_gate(build)? {
        return Ok(true);
    }
    let minimum = CachyOsTag::parse_minimum(MIN_PROTON_CACHYOS_INTEGRATED_BUILD);
    Ok(matches!(
        (CachyOsTag::parse(&build.name), minimum),
        (Some(tag), Some(min)) if tag >= min
    ))
}

/// Whether a launch with vkd3d_heap on should set `PROTON_VKD3D_HEAP=1`:
/// when the build is unknown, uses the legacy gate, or its script cannot
/// be read (Go's `applyVKD3DHeapEnv`).
pub fn needs_heap_env(build: &Result<Build, ResolveError>) -> bool {
    match build {
        Ok(build) => uses_legacy_heap_gate(build).unwrap_or(true),
        Err(_) => true,
    }
}

static CACHYOS_TAG: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?i)cachyos[-_]?(\d+)\.(\d+)-(\d{8})").expect("the tag pattern is valid")
});

/// A Proton-CachyOS build tag, `MAJOR.MINOR-YYYYMMDD`, as in
/// `cachyos-11.0-20260521-slr`. Ordered by major, minor, then date.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub struct CachyOsTag {
    /// The major version.
    pub major: u64,
    /// The minor version.
    pub minor: u64,
    /// The date, `YYYYMMDD` as a number.
    pub date: u64,
}

impl CachyOsTag {
    /// The tag in a tool name. `None` for other tools, such as GE-Proton.
    pub fn parse(name: &str) -> Option<CachyOsTag> {
        let caps = CACHYOS_TAG.captures(name)?;
        let part = |i: usize| caps.get(i)?.as_str().parse::<u64>().ok();
        Some(CachyOsTag {
            major: part(1)?,
            minor: part(2)?,
            date: part(3)?,
        })
    }

    /// A minimum written as `MAJOR.MINOR-YYYYMMDD`.
    pub fn parse_minimum(raw: &str) -> Option<CachyOsTag> {
        CachyOsTag::parse(&format!("cachyos-{raw}-slr"))
    }
}

/// An NVIDIA driver version, compared by major, minor, then patch.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DriverVersion {
    /// The major part.
    pub major: i64,
    /// The minor part.
    pub minor: i64,
    /// The patch part.
    pub patch: i64,
    /// The input, trimmed.
    pub raw: String,
}

/// A driver version that does not parse.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum DriverVersionError {
    /// The input is empty: no NVIDIA driver, or none probed.
    #[error("driver version unavailable")]
    Unavailable,
    /// The input is not 1 to 4 dot-separated numbers.
    #[error("driver version {0:?} does not parse")]
    Unparsable(String),
}

impl DriverVersion {
    /// Parses `580.94.16`, `580.94` or `585.0.0`, with space around it.
    /// One to four parts; missing parts are 0 and a fourth is ignored.
    pub fn parse(raw: &str) -> Result<DriverVersion, DriverVersionError> {
        let trimmed = raw.trim();
        if trimmed.is_empty() {
            return Err(DriverVersionError::Unavailable);
        }
        let bad = || DriverVersionError::Unparsable(trimmed.to_owned());
        let parts: Vec<i64> = trimmed
            .split('.')
            .map(|p| p.trim().parse::<i64>())
            .collect::<Result<_, _>>()
            .map_err(|_| bad())?;
        if parts.len() > 4 {
            return Err(bad());
        }
        let part = |i: usize| parts.get(i).copied().unwrap_or(0);
        Ok(DriverVersion {
            major: part(0),
            minor: part(1),
            patch: part(2),
            raw: trimmed.to_owned(),
        })
    }

    /// Whether this is at least [`MIN_DRIVER_VERSION`].
    pub fn meets_minimum(&self) -> bool {
        DriverVersion::parse(MIN_DRIVER_VERSION)
            .is_ok_and(|min| self.cmp_parts(&min) != Ordering::Less)
    }

    /// Compares major, minor and patch.
    pub fn cmp_parts(&self, other: &DriverVersion) -> Ordering {
        (self.major, self.minor, self.patch).cmp(&(other.major, other.minor, other.patch))
    }
}

impl fmt::Display for DriverVersion {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}.{}.{}", self.major, self.minor, self.patch)
    }
}

/// The prefix of a notice for a check that could not run.
pub const INFO_GLYPH: &str = "ⓘ ";
/// The prefix of a notice for a real incompatibility.
pub const WARN_GLYPH: &str = "⚠ ";
/// Why the Proton check did not run: no build was found.
pub const SKIP_NOT_RESOLVED: &str =
    "could not resolve active Proton for this game; skipping compatibility check";
/// Why the Proton check did not run: the build's script cannot be read.
pub const SKIP_PROBE_FAILED: &str = "Proton marker probe failed; skipping compatibility check";
/// Why the driver check did not run: the driver could not be asked.
pub const SKIP_DRIVER_PROBE_FAILED: &str =
    "NVIDIA driver probe failed; skipping driver compatibility check";
/// Why the driver check did not run: there is no NVIDIA driver.
pub const SKIP_DRIVER_NOT_DETECTED: &str =
    "NVIDIA driver not detected; skipping driver compatibility check";
/// Why the driver check did not run: the version does not parse.
pub const SKIP_DRIVER_UNPARSABLE: &str =
    "NVIDIA driver version unparsable; skipping driver compatibility check";

/// The outcome of the descriptor heap check, each axis on its own. An
/// axis that could not run is OK with a skip reason, so it never blocks.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Compatibility {
    /// The build supports descriptor_heap, or the check did not run.
    pub proton_ok: bool,
    /// The build's name, when one was found.
    pub proton_detected: String,
    /// Why the Proton check did not run.
    pub proton_skip: Option<&'static str>,
    /// The driver is new enough, or the check did not run.
    pub driver_ok: bool,
    /// The driver as `M.m.p`, or the raw text when it does not parse.
    pub driver_detected: String,
    /// Why the driver check did not run.
    pub driver_skip: Option<&'static str>,
}

/// Runs the descriptor heap check. `driver` is the driver version from
/// the GPU backend, `Some("")` when there is no NVIDIA driver, or `None`
/// when the backend could not be asked.
pub fn check(build: &Result<Build, ResolveError>, driver: Option<&str>) -> Compatibility {
    let (proton_ok, proton_detected, proton_skip) = match build {
        Err(_) => (true, String::new(), Some(SKIP_NOT_RESOLVED)),
        Ok(build) => match supports_vkd3d_heap(build) {
            Ok(supported) => (supported, build.name.clone(), None),
            Err(_) => (true, build.name.clone(), Some(SKIP_PROBE_FAILED)),
        },
    };
    let (driver_ok, driver_detected, driver_skip) = match driver.map(DriverVersion::parse) {
        None => (true, String::new(), Some(SKIP_DRIVER_PROBE_FAILED)),
        Some(Err(DriverVersionError::Unavailable)) => {
            (true, String::new(), Some(SKIP_DRIVER_NOT_DETECTED))
        }
        Some(Err(DriverVersionError::Unparsable(raw))) => (true, raw, Some(SKIP_DRIVER_UNPARSABLE)),
        Some(Ok(version)) => (version.meets_minimum(), version.to_string(), None),
    };
    Compatibility {
        proton_ok,
        proton_detected,
        proton_skip,
        driver_ok,
        driver_detected,
        driver_skip,
    }
}

impl Compatibility {
    /// The line to show next to the vkd3d_heap toggle, or `None` when all
    /// is well or both checks were skipped. A real incompatibility on one
    /// axis wins over a skip on the other.
    pub fn notice(&self) -> Option<String> {
        if let Some(skip) = self.proton_skip
            && self.driver_ok
            && self.driver_skip.is_none()
        {
            return Some(format!("{INFO_GLYPH}{skip}"));
        }
        if let Some(skip) = self.driver_skip
            && self.proton_ok
            && self.proton_skip.is_none()
        {
            return Some(format!("{INFO_GLYPH}{skip}"));
        }
        let proton_hard = !self.proton_ok && self.proton_skip.is_none();
        let driver_hard = !self.driver_ok && self.driver_skip.is_none();
        let (proton, driver) = (&self.proton_detected, &self.driver_detected);
        match (proton_hard, driver_hard) {
            (true, true) => Some(format!(
                "{WARN_GLYPH}descriptor_heap requires Proton-CachyOS {MIN_PROTON_CACHYOS_BUILD}+ and NVIDIA driver {MIN_DRIVER_VERSION}+ (detected: {proton}, driver {driver})"
            )),
            (true, false) => Some(format!(
                "{WARN_GLYPH}descriptor_heap requires Proton-CachyOS {MIN_PROTON_CACHYOS_BUILD}+ (detected: {proton})"
            )),
            (false, true) => Some(format!(
                "{WARN_GLYPH}descriptor_heap requires NVIDIA driver {MIN_DRIVER_VERSION}+ (detected: {driver})"
            )),
            (false, false) => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::steam::library::Library;

    fn build(name: &str, path: &Path) -> Build {
        Build {
            name: name.to_owned(),
            path: path.to_owned(),
        }
    }

    #[test]
    fn cachyos_tags_parse_and_compare() {
        let tag = CachyOsTag::parse("cachyos-11.0-20260521-slr").unwrap();
        assert_eq!((tag.major, tag.minor, tag.date), (11, 0, 20260521));
        assert_eq!(CachyOsTag::parse("GE-Proton10-34"), None);
        assert!(CachyOsTag::parse("proton-CachyOS_10.0-20260312-slr-x86_64_v3").is_some());
        let min = CachyOsTag::parse_minimum(MIN_PROTON_CACHYOS_INTEGRATED_BUILD).unwrap();
        assert!(tag >= min);
        assert!(CachyOsTag::parse("cachyos-10.0-20260410-slr").unwrap() < min);
        assert!(CachyOsTag::parse("cachyos-11.0-20260519-slr").unwrap() < min);
    }

    #[test]
    fn driver_versions_follow_go() {
        let v = DriverVersion::parse("  580.94.16  ").unwrap();
        assert_eq!((v.major, v.minor, v.patch), (580, 94, 16));
        assert!(v.meets_minimum());
        assert_eq!(v.raw, "580.94.16");
        assert!(!DriverVersion::parse("580.94").unwrap().meets_minimum());
        assert!(DriverVersion::parse("585.0.0").unwrap().meets_minimum());
        assert!(!DriverVersion::parse("580.94.15").unwrap().meets_minimum());
        assert_eq!(
            DriverVersion::parse("570.86").unwrap().to_string(),
            "570.86.0"
        );
        assert_eq!(
            DriverVersion::parse("1.2.3.4").unwrap().to_string(),
            "1.2.3"
        );
        assert_eq!(
            DriverVersion::parse("   "),
            Err(DriverVersionError::Unavailable)
        );
        assert!(matches!(
            DriverVersion::parse("not-a-version"),
            Err(DriverVersionError::Unparsable(_))
        ));
        assert!(DriverVersion::parse("1.2.3.4.5").is_err());
        let lo = DriverVersion::parse("580.94.15").unwrap();
        let hi = DriverVersion::parse("580.95.0").unwrap();
        assert_eq!(lo.cmp_parts(&hi), Ordering::Less);
        assert_eq!(hi.cmp_parts(&hi), Ordering::Equal);
    }

    fn proton_dir(dir: &Path, script: Option<&str>) -> PathBuf {
        std::fs::create_dir_all(dir).unwrap();
        if let Some(text) = script {
            std::fs::write(dir.join("proton"), text).unwrap();
        }
        dir.to_owned()
    }

    #[test]
    fn the_legacy_gate_and_the_integrated_tag() {
        let tmp = tempfile::tempdir().unwrap();
        let legacy = proton_dir(&tmp.path().join("a"), Some("# honours PROTON_VKD3D_HEAP\n"));
        let plain = proton_dir(&tmp.path().join("b"), Some("# nothing\n"));
        let empty = proton_dir(&tmp.path().join("c"), None);
        assert!(uses_legacy_heap_gate(&build("x", &legacy)).unwrap());
        assert!(!uses_legacy_heap_gate(&build("x", &plain)).unwrap());
        assert!(!uses_legacy_heap_gate(&build("x", &empty)).unwrap());
        assert!(!uses_legacy_heap_gate(&build("x", Path::new(""))).unwrap());
        assert!(supports_vkd3d_heap(&build("GE-Proton10-34", &legacy)).unwrap());
        assert!(!supports_vkd3d_heap(&build("GE-Proton10-34", &plain)).unwrap());
        assert!(supports_vkd3d_heap(&build("cachyos-11.0-20260521-slr", &plain)).unwrap());
        assert!(!supports_vkd3d_heap(&build("cachyos-10.0-20260410-slr", &plain)).unwrap());
        assert!(needs_heap_env(&Ok(build("x", &legacy))));
        assert!(!needs_heap_env(&Ok(build(
            "cachyos-11.0-20260521-slr",
            &plain
        ))));
        assert!(needs_heap_env(&Err(ResolveError::NoSteamRoot)));
    }

    fn checked(name: Option<&str>, supports: bool, driver: Option<&str>) -> Option<String> {
        let tmp = tempfile::tempdir().unwrap();
        let script = if supports { "PROTON_VKD3D_HEAP" } else { "" };
        let dir = proton_dir(&tmp.path().join("p"), Some(script));
        let resolved = match name {
            Some(n) => Ok(build(n, &dir)),
            None => Err(ResolveError::NoMapping(1)),
        };
        check(&resolved, driver).notice()
    }

    #[test]
    fn notices_match_go_s_tests() {
        assert_eq!(
            checked(Some("cachyos-10.0-20260410-slr"), true, Some("580.94.16")),
            None
        );
        assert_eq!(
            checked(Some("GE-Proton10-34"), false, Some("580.94.16")).unwrap(),
            "⚠ descriptor_heap requires Proton-CachyOS 10.0-20260321+ (detected: GE-Proton10-34)"
        );
        assert_eq!(
            checked(Some("cachyos-10.0-20260410-slr"), true, Some("570.86")).unwrap(),
            "⚠ descriptor_heap requires NVIDIA driver 580.94.16+ (detected: 570.86.0)"
        );
        assert_eq!(
            checked(Some("GE-Proton10-34"), false, Some("570.86")).unwrap(),
            "⚠ descriptor_heap requires Proton-CachyOS 10.0-20260321+ and NVIDIA driver 580.94.16+ (detected: GE-Proton10-34, driver 570.86.0)"
        );
        assert_eq!(
            checked(None, true, Some("580.94.16")).unwrap(),
            format!("ⓘ {SKIP_NOT_RESOLVED}")
        );
        assert_eq!(
            checked(Some("cachyos-10.0-20260410-slr"), true, Some("")).unwrap(),
            format!("ⓘ {SKIP_DRIVER_NOT_DETECTED}")
        );
        assert_eq!(
            checked(Some("cachyos-10.0-20260410-slr"), true, None).unwrap(),
            format!("ⓘ {SKIP_DRIVER_PROBE_FAILED}")
        );
        assert_eq!(
            checked(Some("cachyos-10.0-20260410-slr"), true, Some("x.y")).unwrap(),
            format!("ⓘ {SKIP_DRIVER_UNPARSABLE}")
        );
        let hard = checked(None, true, Some("570.86")).unwrap();
        assert!(
            hard.starts_with("⚠ ") && hard.contains("580.94.16"),
            "{hard}"
        );
        assert_eq!(
            checked(None, true, Some("")),
            None,
            "both skipped says nothing"
        );
    }

    fn write(path: &Path, text: &str) {
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, text).unwrap();
    }

    fn mapping(root: &Path, entries: &[(&str, &str)]) {
        let body: String = entries
            .iter()
            .map(|(id, name)| {
                format!(
                    "  \"{id}\" {{ \"name\" \"{name}\" \"config\" \"\" \"priority\" \"250\" }}\n"
                )
            })
            .collect();
        write(
            &root.join("config/config.vdf"),
            &format!(
                "\"InstallConfigStore\" {{ \"Software\" {{ \"Valve\" {{ \"Steam\" {{ \"CompatToolMapping\" {{\n{body} }} }} }} }} }}"
            ),
        );
    }

    fn install(root: &Path, libraries: &[&Path]) -> SteamInstall {
        SteamInstall {
            root: Some(root.to_owned()),
            libraries: libraries
                .iter()
                .map(|p| Library {
                    path: p.to_path_buf(),
                    label: String::new(),
                })
                .collect(),
            problems: Vec::new(),
        }
    }

    #[test]
    fn the_mapping_tries_the_game_then_the_default() {
        let tmp = tempfile::tempdir().unwrap();
        let root = tmp.path();
        assert!(matches!(mapped_tool(root, 1), Err(ResolveError::Config(_))));
        mapping(
            root,
            &[
                ("0", "proton_experimental"),
                ("9100010", "GE-Proton10-34"),
                ("4263293584", ""),
            ],
        );
        assert_eq!(mapped_tool(root, 9100010).unwrap(), "GE-Proton10-34");
        assert_eq!(mapped_tool(root, 9100099).unwrap(), "proton_experimental");
        assert_eq!(
            mapped_tool(root, 4263293584).unwrap(),
            "proton_experimental",
            "an empty name falls back"
        );
        mapping(root, &[("9100010", "GE-Proton10-34")]);
        assert!(matches!(
            mapped_tool(root, 2),
            Err(ResolveError::NoMapping(2))
        ));
        write(
            &root.join("config/config.vdf"),
            "\"InstallConfigStore\" { }",
        );
        assert!(matches!(
            mapped_tool(root, 2),
            Err(ResolveError::NoMappingTable(_))
        ));
        // Keys match without regard to case.
        write(
            &root.join("config/config.vdf"),
            "\"installconfigstore\" { \"software\" { \"valve\" { \"steam\" { \"compattoolmapping\" { \"0\" { \"name\" \"x\" } } } } } }",
        );
        assert_eq!(mapped_tool(root, 2).unwrap(), "x");
    }

    #[test]
    fn builds_are_found_by_internal_name_and_in_every_library() {
        let tmp = tempfile::tempdir().unwrap();
        let root = tmp.path().join("steam");
        let lib2 = tmp.path().join("library2");
        write(
            &root.join("compatibilitytools.d/GE-Proton11-7/compatibilitytool.vdf"),
            "\"compatibilitytools\" { \"compat_tools\" { \"GE-Proton11-7-x86_64\" { \"install_path\" \".\" } \"Other-Tool\" { \"install_path\" \"files\" } \"Escape\" { \"install_path\" \"../x\" } } }",
        );
        std::fs::create_dir_all(root.join("compatibilitytools.d/GE-Proton11-7/files")).unwrap();
        std::fs::create_dir_all(root.join("compatibilitytools.d/Plain-Folder")).unwrap();
        std::fs::create_dir_all(root.join("compatibilitytools.d/x")).unwrap();
        std::fs::create_dir_all(lib2.join("steamapps/common/SteamLinuxRuntime_sniper")).unwrap();
        std::fs::create_dir_all(lib2.join("steamapps/common/Proton Hotfix")).unwrap();
        std::fs::create_dir_all(lib2.join("steamapps/common/Proton Nine")).unwrap();
        write(
            &manifest::manifest_path(&lib2, 2805730),
            "\"AppState\" { \"appid\" \"2805730\" \"name\" \"Proton 9.0\" \"StateFlags\" \"4\" \"installdir\" \"Proton Nine\" }",
        );
        let steam = install(&root, &[&root, &lib2]);
        let tools = compat_tools(&root);
        let names: Vec<&str> = tools.iter().map(|t| t.name.as_str()).collect();
        assert_eq!(names, ["GE-Proton11-7-x86_64", "Other-Tool"]);

        let found = |name: &str| locate_build(&steam, name);
        assert_eq!(
            found("GE-Proton11-7-x86_64"),
            Some(root.join("compatibilitytools.d/GE-Proton11-7"))
        );
        assert_eq!(
            found("Other-Tool"),
            Some(root.join("compatibilitytools.d/GE-Proton11-7/files"))
        );
        assert_eq!(
            found("Plain-Folder"),
            Some(root.join("compatibilitytools.d/Plain-Folder"))
        );
        assert_eq!(
            found("SteamLinuxRuntime_sniper"),
            Some(lib2.join("steamapps/common/SteamLinuxRuntime_sniper"))
        );
        assert_eq!(
            found("proton_9"),
            Some(lib2.join("steamapps/common/Proton Nine")),
            "from the app manifest"
        );
        assert_eq!(
            found("proton_hotfix"),
            Some(lib2.join("steamapps/common/Proton Hotfix")),
            "the usual folder"
        );
        assert_eq!(found("proton-cachyos-10.0-20260312-slr-x86_64_v3"), None);
        assert_eq!(found("Escape"), None);
        assert_eq!(found("../steam"), None, "a name is never a path");

        mapping(
            &root,
            &[
                ("0", "proton_hotfix"),
                ("9100070", "GE-Proton11-7-x86_64"),
                ("9100010", "missing-tool"),
            ],
        );
        assert_eq!(
            resolve(&steam, 9100070).unwrap().name,
            "GE-Proton11-7-x86_64"
        );
        assert_eq!(
            resolve(&steam, 1).unwrap().path,
            lib2.join("steamapps/common/Proton Hotfix")
        );
        let err = resolve(&steam, 9100010).unwrap_err();
        assert!(
            matches!(err, ResolveError::BuildNotFound(ref n) if n == "missing-tool"),
            "{err}"
        );
        assert!(matches!(
            resolve(&SteamInstall::default(), 1),
            Err(ResolveError::NoSteamRoot)
        ));
    }
}
