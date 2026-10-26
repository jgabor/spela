//! Subcommand or wrapped game, decided on the raw arguments before clap.
//!
//! Steam runs `spela %command%`, so spela must tell its own subcommands from
//! a game to wrap. Known subcommands always win (decision 21): Go checked
//! for a wrapped program first, so `spela overlay` ran `/usr/bin/overlay`.
//!
//! The rules, in order:
//!
//! 1. A first argument that starts with `-` and is not a bare `--` goes to
//!    clap, so `--help`, `--version` and the global flags work.
//! 2. A known subcommand name goes to clap.
//! 3. Otherwise the arguments are read as a wrapper invocation: leading
//!    `KEY=VALUE` tokens, with keys matching `[A-Za-z_][A-Za-z0-9_]*`, are
//!    environment, and a bare `--`, first or after them, ends them.
//! 4. The first remaining token is a game launch when it contains `/`, or a
//!    program on PATH has its name and it has no extension, `.exe` or
//!    `.sh`, in any case, as Go compares it.
//! 5. Anything else goes to clap, which reports an unknown command.

use std::ffi::{OsStr, OsString};
use std::os::unix::fs::PermissionsExt;
use std::path::Path;

/// Where the arguments go.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Route {
    /// clap parses them.
    Cli,
    /// spela wraps a game.
    Wrapper(WrapperInvocation),
}

/// A game to wrap, as Steam passes it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WrapperInvocation {
    /// The leading `KEY=VALUE` tokens. They beat every value spela sets.
    pub env: Vec<(String, String)>,
    /// The command to run, never empty.
    pub command: Vec<OsString>,
}

/// Decides the route for `args`, which excludes the program name.
/// `is_subcommand` knows spela's command names; `is_launchable` says
/// whether a token names a program (see [`is_launchable`]).
pub fn route(
    args: &[OsString],
    is_subcommand: impl Fn(&str) -> bool,
    is_launchable: impl Fn(&OsStr) -> bool,
) -> Route {
    let Some(first) = args.first() else {
        return Route::Cli;
    };
    let bytes = first.as_encoded_bytes();
    if bytes.first() == Some(&b'-') && bytes != b"--" {
        return Route::Cli;
    }
    if first.to_str().is_some_and(&is_subcommand) {
        return Route::Cli;
    }
    let (env, command) = split_env(args);
    match command.first() {
        Some(program) if !starts_with_dash(program) && is_launchable(program) => {
            Route::Wrapper(WrapperInvocation {
                env,
                command: command.to_vec(),
            })
        }
        _ => Route::Cli,
    }
}

/// Splits leading `KEY=VALUE` tokens from the command. A bare `--` ends
/// them and is dropped.
pub fn split_env(args: &[OsString]) -> (Vec<(String, String)>, &[OsString]) {
    let mut env = Vec::new();
    for (i, arg) in args.iter().enumerate() {
        if arg == "--" {
            return (env, &args[i + 1..]);
        }
        match arg.to_str().and_then(|a| a.split_once('=')) {
            Some((key, value)) if is_env_key(key) => env.push((key.to_owned(), value.to_owned())),
            _ => return (env, &args[i..]),
        }
    }
    (env, &[])
}

/// Whether `key` matches `[A-Za-z_][A-Za-z0-9_]*`.
pub fn is_env_key(key: &str) -> bool {
    let mut chars = key.chars();
    chars
        .next()
        .is_some_and(|c| c.is_ascii_alphabetic() || c == '_')
        && chars.all(|c| c.is_ascii_alphanumeric() || c == '_')
}

/// Whether `token` names a program: it contains `/`, or an executable file
/// with that name is on `path_var`, the colon-separated `PATH`, and the name
/// has no extension, `.exe` or `.sh`. The extension check ignores case, as
/// Go's `strings.ToLower` does. Empty `PATH` entries are skipped.
pub fn is_launchable(token: &OsStr, path_var: Option<&str>) -> bool {
    let bytes = token.as_encoded_bytes();
    if bytes.is_empty() || starts_with_dash(token) {
        return false;
    }
    if bytes.contains(&b'/') {
        return true;
    }
    let name = Path::new(token);
    let extension_ok = match name.extension() {
        None => true,
        Some(ext) => ext.eq_ignore_ascii_case("exe") || ext.eq_ignore_ascii_case("sh"),
    };
    extension_ok
        && path_var
            .into_iter()
            .flat_map(|p| p.split(':'))
            .filter(|dir| !dir.is_empty())
            .any(|dir| is_executable_file(&Path::new(dir).join(name)))
}

fn starts_with_dash(token: &OsStr) -> bool {
    token.as_encoded_bytes().first() == Some(&b'-')
}

fn is_executable_file(path: &Path) -> bool {
    std::fs::metadata(path).is_ok_and(|m| m.is_file() && m.permissions().mode() & 0o111 != 0)
}

#[cfg(test)]
mod tests {
    use super::*;

    const KNOWN: &[&str] = &["scan", "list", "tui", "overlay", "gui", "help"];

    fn os(args: &[&str]) -> Vec<OsString> {
        args.iter().map(OsString::from).collect()
    }

    /// Routes with a PATH that holds `games` and nothing else.
    fn route_with(args: &[&str], games: &[&str]) -> Route {
        route(
            &os(args),
            |name| KNOWN.contains(&name),
            |token| {
                token.as_encoded_bytes().contains(&b'/')
                    || token.to_str().is_some_and(|t| games.contains(&t))
            },
        )
    }

    fn wrapper(env: &[(&str, &str)], command: &[&str]) -> Route {
        Route::Wrapper(WrapperInvocation {
            env: env
                .iter()
                .map(|(k, v)| ((*k).to_owned(), (*v).to_owned()))
                .collect(),
            command: os(command),
        })
    }

    #[test]
    fn known_subcommands_win_over_programs_on_path() {
        // The Go bug: `overlay` from mesa-demos is on PATH on the author's
        // machine and ran instead of spela.
        for name in ["scan", "overlay", "gui", "help"] {
            assert_eq!(route_with(&[name], &[name]), Route::Cli, "{name}");
        }
    }

    #[test]
    fn flags_go_to_clap() {
        assert_eq!(route_with(&["--version"], &[]), Route::Cli);
        assert_eq!(route_with(&["-v"], &[]), Route::Cli);
        assert_eq!(route_with(&["--demo", "tui"], &[]), Route::Cli);
        assert_eq!(route_with(&[], &[]), Route::Cli);
    }

    #[test]
    fn paths_and_programs_are_launches() {
        assert_eq!(
            route_with(&["/games/Cyber/bin/game.exe", "-x"], &[]),
            wrapper(&[], &["/games/Cyber/bin/game.exe", "-x"])
        );
        assert_eq!(route_with(&["./run.sh"], &[]), wrapper(&[], &["./run.sh"]));
        assert_eq!(
            route_with(&["mygame", "--fullscreen"], &["mygame"]),
            wrapper(&[], &["mygame", "--fullscreen"])
        );
    }

    #[test]
    fn leading_double_dash_and_env_tokens() {
        assert_eq!(
            route_with(&["--", "mygame"], &["mygame"]),
            wrapper(&[], &["mygame"])
        );
        assert_eq!(
            route_with(&["FOO=1", "--", "mygame"], &["mygame"]),
            wrapper(&[("FOO", "1")], &["mygame"])
        );
        assert_eq!(
            route_with(&["A=1", "B_2=x=y", "/bin/game"], &[]),
            wrapper(&[("A", "1"), ("B_2", "x=y")], &["/bin/game"])
        );
        // `--` ends the env section, so a later KEY=VALUE is the command.
        assert_eq!(route_with(&["--", "X=1/2"], &[]), wrapper(&[], &["X=1/2"]));
    }

    #[test]
    fn anything_else_goes_to_clap() {
        assert_eq!(route_with(&["frobnicate"], &[]), Route::Cli);
        assert_eq!(route_with(&["FOO=1", "--", "missing"], &[]), Route::Cli);
        assert_eq!(route_with(&["FOO=1"], &[]), Route::Cli);
        assert_eq!(route_with(&["--"], &[]), Route::Cli);
        assert_eq!(route_with(&["--", "--weird"], &[]), Route::Cli);
        // A key that is not a shell name is not env: `1X=2` is a command.
        assert_eq!(route_with(&["1X=2"], &[]), Route::Cli);
    }

    #[test]
    fn env_keys() {
        for good in ["A", "_", "_x9", "SteamAppId", "PROTON_LOG"] {
            assert!(is_env_key(good), "{good}");
        }
        for bad in ["", "1A", "A-B", "A B", "é"] {
            assert!(!is_env_key(bad), "{bad}");
        }
    }

    #[test]
    fn launchable_checks_path_extension_and_mode() {
        let dir = tempfile::tempdir().unwrap();
        let make = |name: &str, mode: u32| {
            let p = dir.path().join(name);
            std::fs::write(&p, "#!/bin/sh\n").unwrap();
            std::fs::set_permissions(&p, std::fs::Permissions::from_mode(mode)).unwrap();
        };
        make("game", 0o755);
        make("game.sh", 0o755);
        make("game.exe", 0o755);
        make("game.py", 0o755);
        make("Game.EXE", 0o755);
        make("run.SH", 0o755);
        make("tool.Py", 0o755);
        make("notes", 0o644);
        std::fs::create_dir(dir.path().join("folder")).unwrap();
        let path = format!("::{}:/nonexistent", dir.path().display());
        let yes = |t: &str| is_launchable(OsStr::new(t), Some(&path));
        assert!(yes("game"));
        assert!(yes("game.sh"));
        assert!(yes("game.exe"));
        assert!(!yes("game.py"), "other extensions are not launches");
        assert!(yes("Game.EXE"), "the extension check ignores case");
        assert!(yes("run.SH"));
        assert!(!yes("tool.Py"));
        assert!(!yes("notes"), "not executable");
        assert!(!yes("folder"), "not a file");
        assert!(!yes("missing"));
        assert!(!yes("-game"));
        assert!(!yes(""));
        assert!(yes("relative/path"), "anything with a slash");
        assert!(!is_launchable(OsStr::new("game"), None));
        assert!(!is_launchable(OsStr::new("game"), Some("")));
    }

    #[test]
    fn non_utf8_paths_can_launch() {
        use std::os::unix::ffi::OsStringExt;
        let arg = OsString::from_vec(b"/games/\xff/run".to_vec());
        let r = route(
            std::slice::from_ref(&arg),
            |_| false,
            |t| is_launchable(t, None),
        );
        assert_eq!(
            r,
            Route::Wrapper(WrapperInvocation {
                env: vec![],
                command: vec![arg]
            })
        );
    }

    mod props {
        use super::*;
        use proptest::prelude::*;

        fn token() -> impl Strategy<Value = String> {
            prop_oneof![
                "[a-z]{1,8}",
                "--[a-z-]{1,8}",
                "[A-Z_]{1,4}=[a-z0-9/]{0,6}",
                Just("--".to_owned()),
                "/[a-z/]{1,10}",
            ]
        }

        proptest! {
            #[test]
            fn a_known_first_name_always_goes_to_clap(
                name in proptest::sample::select(KNOWN.to_vec()),
                rest in proptest::collection::vec(token(), 0..6),
            ) {
                let mut args = vec![name.to_owned()];
                args.extend(rest);
                let args: Vec<&str> = args.iter().map(String::as_str).collect();
                prop_assert_eq!(route_with(&args, KNOWN), Route::Cli);
            }

            #[test]
            fn a_flag_first_always_goes_to_clap(
                flag in "-[a-zA-Z-]{1,10}",
                rest in proptest::collection::vec(token(), 0..6),
            ) {
                prop_assume!(flag != "--");
                let mut args = vec![flag];
                args.extend(rest);
                let args: Vec<&str> = args.iter().map(String::as_str).collect();
                prop_assert_eq!(route_with(&args, &["game"]), Route::Cli);
            }

            #[test]
            fn a_wrapper_command_is_never_empty_or_a_flag(
                args in proptest::collection::vec(token(), 0..8),
            ) {
                let args: Vec<&str> = args.iter().map(String::as_str).collect();
                if let Route::Wrapper(w) = route_with(&args, &["game"]) {
                    let first = w.command[0].to_str().unwrap();
                    prop_assert!(!first.starts_with('-'));
                    prop_assert!(!w.command.is_empty());
                }
            }
        }
    }
}
