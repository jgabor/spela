//! The deny list's commands, run off the UI thread: the load on the job
//! pool, answered as `Msg::Denylist`. Its writes go through the write
//! queue as `Write::Deny`, like every other save.

use spela_core::denylist::{self, ListError};
use spela_ui::Msg;
use spela_ui::screens::denylist::{Cmd, Msg as DenyMsg, Unreadable};

use super::Workers;

/// Runs one of the deny list's commands.
pub fn run(w: &mut Workers, cmd: Cmd) {
    match cmd {
        Cmd::Load { request } => {
            let paths = w.paths.clone();
            w.jobs.submit(
                &w.tx,
                move || {
                    let result = denylist::load(&paths).map_err(|e| unreadable(&e));
                    Msg::Denylist(DenyMsg::Loaded { request, result })
                },
                move |error| {
                    Msg::Denylist(DenyMsg::Loaded {
                        request,
                        result: Err(Unreadable {
                            file: "deny list".to_owned(),
                            error,
                        }),
                    })
                },
            );
        }
    }
}

/// The file a list error names, for `Denylist: unreadable (<file>)`, and
/// the whole error.
pub fn unreadable(e: &ListError) -> Unreadable {
    let path = match e {
        ListError::Read { path, .. }
        | ListError::Invalid { path, .. }
        | ListError::Newer { path, .. }
        | ListError::Write { path, .. }
        | ListError::NotMigrated { path, .. } => Some(path),
        ListError::AppIdTooLarge { .. } | ListError::Lock(_) => None,
    };
    let file = path.and_then(|p| p.file_name()).map_or_else(
        || "deny list".to_owned(),
        |n| n.to_string_lossy().into_owned(),
    );
    Unreadable {
        file,
        error: e.to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use spela_core::paths::Paths;

    #[test]
    fn a_list_that_does_not_parse_names_its_file() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        assert!(denylist::load(&paths).is_ok(), "no files is empty lists");
        std::fs::create_dir_all(paths.deny_overrides().parent().unwrap()).unwrap();
        std::fs::write(paths.deny_overrides(), "[[denid]]\n").unwrap();
        let e = denylist::load(&paths).unwrap_err();
        let u = unreadable(&e);
        assert_eq!(u.file, "overrides.toml");
        assert!(u.error.contains("overrides.toml"), "{}", u.error);
    }
}
