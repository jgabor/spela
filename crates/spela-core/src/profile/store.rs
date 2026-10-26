//! The TOML profile files: `profiles/all-games.toml` and
//! `profiles/<appid>.toml`, read into [`super::OwnValues`] and written with
//! toml_edit under `config.lock`.

use crate::paths::Paths;

/// Whether a game has its own profile, the ⚑ marker: its file exists, even
/// when it holds no own values.
pub fn has_profile(paths: &Paths, app_id: u64) -> bool {
    paths.game_profile(app_id).is_file()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn an_empty_file_still_counts() {
        let base = tempfile::tempdir().unwrap();
        let paths = Paths::under(base.path());
        assert!(!has_profile(&paths, 9100010));
        std::fs::create_dir_all(paths.profiles_dir()).unwrap();
        std::fs::write(paths.game_profile(9100010), "").unwrap();
        assert!(has_profile(&paths, 9100010));
    }
}
