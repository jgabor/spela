//! Advisory flock locks.
//!
//! Two kinds, both on std's `File::lock` and `File::try_lock`, which use
//! flock on Linux:
//!
//! - [`InstanceLock`]: the TUI lock and the launch lock. Taken without
//!   blocking. The holder writes its PID into the file, only so a second
//!   process can print `spela is already running (PID: N)` (decision 29).
//! - [`StoreLock`]: `config.lock` and `games.lock`, around read, modify and
//!   write cycles. Blocks until free.
//!
//! The kernel drops a lock when its process exits, so a stale lock cannot
//! exist and PID reuse cannot fake one. A lock file stays in place after
//! release: unlinking it would let two processes hold locks on different
//! inodes at once. std opens files with close-on-exec, so a game spela
//! starts never inherits a lock. Each lock opens a fresh descriptor, and
//! flock on separate opens excludes within one process too.

use std::fs::{File, OpenOptions, TryLockError};
use std::io::{self, Write};
use std::os::unix::fs::OpenOptionsExt;
use std::path::{Path, PathBuf};
use std::time::{Duration, Instant};

use crate::paths::create_private_dir;

/// How long [`InstanceLock::acquire`] retries a held lock. A process that
/// forks while holding a lock gives the child a copy of the descriptor until
/// the child's `exec` closes it, so a just-released lock can look held for
/// a moment.
pub const RETRY_FOR: Duration = Duration::from_millis(100);

/// Why a lock could not be taken.
#[derive(Debug, thiserror::Error)]
pub enum LockError {
    /// Another spela holds it.
    #[error("{}", held_message(*pid))]
    Held {
        /// The lock file.
        path: PathBuf,
        /// The other process, when it wrote its id.
        pid: Option<u32>,
    },
    /// The lock file cannot be opened or locked.
    #[error("cannot lock {}: {source}", path.display())]
    Io {
        /// The lock file.
        path: PathBuf,
        /// The cause.
        source: io::Error,
    },
}

fn held_message(pid: Option<u32>) -> String {
    match pid {
        Some(pid) => format!("spela is already running (PID: {pid})"),
        None => "spela is already running".to_owned(),
    }
}

/// A held instance lock. Released on drop.
#[derive(Debug)]
pub struct InstanceLock {
    path: PathBuf,
    // Kept open: closing the file releases the flock.
    file: File,
}

impl InstanceLock {
    /// Takes the lock at `path` without blocking, creating its folder with
    /// mode 0700 and the file with mode 0600. A held lock is retried for
    /// [`RETRY_FOR`], then fails with [`LockError::Held`].
    pub fn acquire(path: &Path) -> Result<InstanceLock, LockError> {
        let io_err = |source| LockError::Io {
            path: path.to_owned(),
            source,
        };
        if let Some(dir) = path.parent() {
            create_private_dir(dir).map_err(io_err)?;
        }
        let file = open_lock_file(path).map_err(io_err)?;
        let deadline = Instant::now() + RETRY_FOR;
        loop {
            match file.try_lock() {
                Ok(()) => break,
                Err(TryLockError::WouldBlock) if Instant::now() < deadline => {
                    std::thread::sleep(RETRY_FOR / 10);
                }
                Err(TryLockError::WouldBlock) => {
                    return Err(LockError::Held {
                        path: path.to_owned(),
                        pid: read_pid(path),
                    });
                }
                Err(TryLockError::Error(e)) if e.kind() == io::ErrorKind::Interrupted => {}
                Err(TryLockError::Error(e)) => return Err(io_err(e)),
            }
        }
        let mut lock = InstanceLock {
            path: path.to_owned(),
            file,
        };
        lock.write_pid();
        Ok(lock)
    }

    /// The lock file.
    pub fn path(&self) -> &Path {
        &self.path
    }

    fn write_pid(&mut self) {
        // The id is only for the message; a failure here changes nothing.
        let _ = self.file.set_len(0);
        let _ = writeln!(self.file, "{}", std::process::id());
        let _ = self.file.flush();
    }
}

/// A held store lock. Released on drop.
#[derive(Debug)]
pub struct StoreLock {
    _file: File,
}

impl StoreLock {
    /// Takes the lock at `path`, waiting while another process or thread
    /// holds it. Creates the folder with mode 0700 and the file with 0600.
    pub fn acquire(path: &Path) -> Result<StoreLock, LockError> {
        let io_err = |source| LockError::Io {
            path: path.to_owned(),
            source,
        };
        if let Some(dir) = path.parent() {
            create_private_dir(dir).map_err(io_err)?;
        }
        let file = open_lock_file(path).map_err(io_err)?;
        loop {
            match file.lock() {
                Ok(()) => return Ok(StoreLock { _file: file }),
                Err(e) if e.kind() == io::ErrorKind::Interrupted => {}
                Err(e) => return Err(io_err(e)),
            }
        }
    }
}

/// Whether another process or descriptor holds the lock at `path`. A
/// missing file is not held. Never blocks and never writes a PID.
pub fn is_held(path: &Path) -> bool {
    let Ok(file) = OpenOptions::new().read(true).open(path) else {
        return false;
    };
    matches!(file.try_lock(), Err(TryLockError::WouldBlock))
}

fn open_lock_file(path: &Path) -> io::Result<File> {
    OpenOptions::new()
        .read(true)
        .write(true)
        .create(true)
        .truncate(false)
        .mode(0o600)
        .open(path)
}

fn read_pid(path: &Path) -> Option<u32> {
    std::fs::read_to_string(path).ok()?.trim().parse().ok()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::os::unix::fs::PermissionsExt;

    #[test]
    fn second_instance_lock_fails_with_the_pid() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("locks/tui.lock");
        let first = InstanceLock::acquire(&path).unwrap();
        assert_eq!(first.path(), path);
        let err = InstanceLock::acquire(&path).unwrap_err();
        match &err {
            LockError::Held { pid, .. } => assert_eq!(*pid, Some(std::process::id())),
            other => panic!("expected Held, got {other:?}"),
        }
        assert_eq!(
            err.to_string(),
            format!("spela is already running (PID: {})", std::process::id())
        );
        assert!(is_held(&path));
        drop(first);
        assert!(!is_held(&path));
        assert!(path.exists(), "the lock file stays");
        drop(InstanceLock::acquire(&path).unwrap());
    }

    #[test]
    fn held_without_a_pid_says_so() {
        assert_eq!(held_message(None), "spela is already running");
    }

    #[test]
    fn modes_are_private() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("locks/launch.lock");
        let _lock = InstanceLock::acquire(&path).unwrap();
        let mode = |p: &Path| std::fs::metadata(p).unwrap().permissions().mode() & 0o777;
        assert_eq!(mode(&path), 0o600);
        assert_eq!(mode(path.parent().unwrap()), 0o700);
    }

    #[test]
    fn instance_lock_in_another_thread_conflicts() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("tui.lock");
        let held = InstanceLock::acquire(&path).unwrap();
        let other = {
            let path = path.clone();
            std::thread::spawn(move || InstanceLock::acquire(&path).is_err())
                .join()
                .unwrap()
        };
        assert!(other);
        drop(held);
    }

    #[test]
    fn store_lock_waits_for_the_holder() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("locks/config.lock");
        let first = StoreLock::acquire(&path).unwrap();
        assert!(is_held(&path));
        let (tx, rx) = std::sync::mpsc::channel();
        let waiter = {
            let path = path.clone();
            std::thread::spawn(move || {
                let lock = StoreLock::acquire(&path).unwrap();
                tx.send(()).unwrap();
                drop(lock);
            })
        };
        assert!(
            rx.recv_timeout(Duration::from_millis(100)).is_err(),
            "the second writer must wait"
        );
        drop(first);
        rx.recv_timeout(Duration::from_secs(10)).unwrap();
        waiter.join().unwrap();
    }

    #[test]
    fn missing_lock_file_is_not_held() {
        let dir = tempfile::tempdir().unwrap();
        assert!(!is_held(&dir.path().join("none.lock")));
    }

    #[test]
    fn unwritable_folder_is_an_io_error() {
        let dir = tempfile::tempdir().unwrap();
        let file = dir.path().join("taken");
        std::fs::write(&file, b"x").unwrap();
        let err = InstanceLock::acquire(&file.join("tui.lock")).unwrap_err();
        assert!(matches!(err, LockError::Io { .. }), "{err:?}");
    }
}
