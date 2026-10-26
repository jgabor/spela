//! Preview launch (decision 44): the dry-run summary for one game, built on
//! the job pool by the same function as `spela launch --dry-run`, so the
//! two never differ. It takes no lock and never runs pkexec or
//! `kscreen-doctor`. A profile that does not load answers with its error,
//! which names the file.

use spela_core::config;
use spela_core::games::cache;
use spela_core::launch::ProfileState;
use spela_core::launch::summary::{self, Mode};
use spela_core::paths::Paths;
use spela_core::profile::{Effective, store};
use spela_ui::Msg;

use super::{SteamSource, Workers};
use crate::plan::SystemProbes;
use crate::tui::metrics::DriverSlot;

/// Builds the summary for `app_id` on the pool and answers the popup.
pub fn run(w: &mut Workers, request: u64, app_id: u64) {
    let paths = w.paths.clone();
    let steam = w.steam.clone();
    let driver = w.driver.clone();
    w.jobs.submit(
        &w.tx,
        move || Msg::PreviewReady {
            request,
            result: preview(&paths, &steam, &driver, app_id),
        },
        move |e| Msg::PreviewReady {
            request,
            result: Err(e),
        },
    );
}

/// The summary text, without colour.
pub fn preview(
    paths: &Paths,
    steam: &SteamSource,
    driver: &DriverSlot,
    app_id: u64,
) -> Result<String, String> {
    let config = config::load(&paths.config_file())
        .map(|l| l.config)
        .map_err(|e| e.to_string())?;
    let games = cache::load(paths);
    let game = games
        .get(app_id)
        .ok_or_else(|| format!("game not found: {app_id}"))?;
    let all_games = store::load_all_games(paths).map_err(|e| e.to_string())?;
    let own = store::load_game(paths, app_id).map_err(|e| e.to_string())?;
    let profile = ProfileState {
        effective: Effective::of_game(
            app_id,
            own.as_ref().map(|l| &l.profile.values),
            all_games.as_ref().map(|l| &l.profile.values),
        ),
        game_file: own.is_some(),
        all_games_file: all_games.is_some(),
    };
    let mut install = || steam.install(paths);
    let mut driver = || super::profile::wait_driver(driver);
    let mut probes = SystemProbes {
        install: &mut install,
        driver: &mut driver,
    };
    let plan = crate::plan::build(paths, &config, game, &profile, &mut probes);
    let text = summary::summary(&plan, &Mode::DryRun, crate::privilege::PKEXEC).plain();
    Ok(text.trim_end().to_owned())
}

#[cfg(test)]
mod tests {
    use super::*;
    use spela_core::games::{Game, GameCache};
    use spela_core::profile::{FieldKey, Scope, Value};

    fn setup() -> (tempfile::TempDir, Paths) {
        let dir = tempfile::tempdir().unwrap();
        let paths = Paths::under(dir.path());
        cache::update(&paths, |c: &mut GameCache| {
            c.upsert(Game {
                app_id: 9100010,
                name: "Glimmer Run".into(),
                install_dir: dir.path().join("Glimmer"),
                library_path: dir.path().to_owned(),
                prefix_path: None,
                scanned_at: jiff::Timestamp::UNIX_EPOCH,
                dlls: Vec::new(),
            });
        })
        .unwrap();
        (dir, paths)
    }

    #[test]
    fn the_preview_is_the_dry_run_text() {
        let (dir, paths) = setup();
        store::edit(
            &paths,
            Scope::AllGames,
            "",
            &[(FieldKey::GpuVrr, Value::Text("never".into()))],
            &[],
        )
        .unwrap();
        let steam = SteamSource::Demo(dir.path().join("none"));
        let text = preview(&paths, &steam, &DriverSlot::default(), 9100010).unwrap();
        assert!(
            text.starts_with("Game: Glimmer Run (AppID 9100010)\n"),
            "{text}"
        );
        assert!(
            text.contains("  KDE VRR:  never in the desktop session"),
            "{text}"
        );
        assert!(text.ends_with("restored on cleanup"), "{text}");
        assert_eq!(
            preview(&paths, &steam, &DriverSlot::default(), 42),
            Err("game not found: 42".into())
        );
    }

    #[test]
    fn a_broken_profile_names_its_file() {
        let (dir, paths) = setup();
        std::fs::create_dir_all(paths.profiles_dir()).unwrap();
        std::fs::write(paths.game_profile(9100010), "[gpu\n").unwrap();
        let steam = SteamSource::Demo(dir.path().join("none"));
        let e = preview(&paths, &steam, &DriverSlot::default(), 9100010).unwrap_err();
        assert!(e.contains("9100010.toml"), "{e}");
    }
}
