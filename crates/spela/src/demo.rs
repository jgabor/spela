//! `spela --demo`: the fake backends under a private temp dir.
//!
//! Every path comes from [`Paths::under`] a fresh temp dir that is removed
//! when the session drops. Only `<config>/config.toml` is copied in, so
//! theme and mouse settings carry over and the real file is never written.
//! The demo never reads Go YAML and never migrates (decision 18).

use std::io;

use spela_core::paths::Paths;

/// A demo run's private directories. Dropping it removes them.
#[derive(Debug)]
pub struct DemoSession {
    dir: tempfile::TempDir,
    paths: Paths,
}

impl DemoSession {
    /// Makes the temp dir and copies the user's config into it, if there is
    /// one.
    pub fn start(real: &Paths) -> io::Result<DemoSession> {
        let dir = tempfile::Builder::new().prefix("spela-demo-").tempdir()?;
        let paths = Paths::under(dir.path());
        let config = real.config_file();
        if config.is_file() {
            std::fs::create_dir_all(&paths.config_dir)?;
            std::fs::copy(&config, paths.config_file())?;
        }
        Ok(DemoSession { dir, paths })
    }

    /// The demo's directories.
    pub fn paths(&self) -> &Paths {
        &self.paths
    }

    /// The temp dir everything lives under.
    pub fn root(&self) -> &std::path::Path {
        self.dir.path()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn copies_only_the_config_and_cleans_up() {
        let base = tempfile::tempdir().unwrap();
        let real = Paths::under(base.path());
        std::fs::create_dir_all(&real.config_dir).unwrap();
        std::fs::write(real.config_file(), "[ui]\ntheme = \"mono\"\n").unwrap();
        std::fs::write(real.config_dir.join("config.yaml"), "go: true\n").unwrap();

        let session = DemoSession::start(&real).unwrap();
        let root = session.root().to_owned();
        assert_eq!(
            std::fs::read_to_string(session.paths().config_file()).unwrap(),
            "[ui]\ntheme = \"mono\"\n"
        );
        assert!(!session.paths().config_dir.join("config.yaml").exists());
        for dir in session.paths().bases() {
            assert!(dir.starts_with(&root));
        }
        drop(session);
        assert!(!root.exists());
    }

    #[test]
    fn no_config_is_fine() {
        let base = tempfile::tempdir().unwrap();
        let session = DemoSession::start(&Paths::under(base.path())).unwrap();
        assert!(!session.paths().config_file().exists());
    }
}
