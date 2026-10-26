//! The startup DLL auto-update: Go's `dll.AutoUpdate`
//! (`internal/dll/autoupdate.go`, parity inventory DLL 2.9).
//!
//! Only the TUI runs it, when `auto_update_dlls` is on (decision 39). Nobody
//! confirms it, so it is stricter than a manual update: it leaves games a
//! deny list names alone, and games that are running, and touches only DLLs
//! that are stale by the one stale function (decision 38), so a DLL whose
//! version cannot be read is skipped. Each swap is the manual path's
//! [`ops::update`], with its backups and checksums.

use std::collections::BTreeSet;

use super::manifest::Manifest;
use super::ops::{self, BatchResult, DllContext, Progress, UpdateRequest};
use super::stale::{self, Freshness};
use crate::denylist::DenyLists;
use crate::games::{Game, GameCache};

/// What the auto-update did.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct AutoUpdate {
    /// The batch it ran, empty when nothing was stale.
    pub batch: BatchResult,
    /// Games left alone because they were running, by name.
    pub running: Vec<String>,
    /// Why it did not run at all, such as a missing manifest.
    pub error: Option<String>,
}

impl AutoUpdate {
    /// An auto-update that did not run.
    pub fn skipped(error: impl Into<String>) -> AutoUpdate {
        AutoUpdate {
            error: Some(error.into()),
            ..AutoUpdate::default()
        }
    }

    /// Go's one-line notice. Empty when every DLL was already current, so
    /// a quiet start stays quiet.
    pub fn notice(&self) -> String {
        if let Some(e) = &self.error {
            return format!("DLL auto-update skipped: {e}");
        }
        let mut parts = Vec::new();
        let updated = self.batch.updated;
        if updated > 0 {
            let unit = if updated == 1 { "DLL" } else { "DLLs" };
            parts.push(format!("updated {updated} {unit}"));
        }
        let failed = self.batch.failed;
        if failed > 0
            && let Some(e) = self
                .batch
                .items
                .iter()
                .find_map(|i| i.result.as_ref().err())
        {
            parts.push(format!("{failed} failed ({e})"));
        }
        if !self.running.is_empty() {
            parts.push(format!("skipped {} while running", self.running.join(", ")));
        }
        if parts.is_empty() {
            String::new()
        } else {
            format!("DLL auto-update: {}", parts.join("; "))
        }
    }
}

/// The games with stale DLLs that no deny list names, by name, each with
/// its requests to the manifest's latest version.
pub fn candidates<'g>(
    cache: &'g GameCache,
    manifest: &Manifest,
    lists: &DenyLists,
) -> Vec<(&'g Game, Vec<UpdateRequest>)> {
    let mut games: Vec<&Game> = cache.games.iter().collect();
    games.sort_by(|a, b| a.name.cmp(&b.name).then(a.app_id.cmp(&b.app_id)));
    games
        .into_iter()
        .filter(|g| !lists.check(g.app_id).is_denied())
        .filter_map(|g| {
            let requests: Vec<UpdateRequest> = g
                .dlls
                .iter()
                .filter_map(|d| match stale::of_dll(d, manifest) {
                    Freshness::Stale { latest } => Some(UpdateRequest {
                        app_id: g.app_id,
                        dll_type: d.dll_type,
                        path: d.path.clone(),
                        version: Some(latest.version.clone()),
                        cached_only: false,
                    }),
                    _ => None,
                })
                .collect();
            (!requests.is_empty()).then_some((g, requests))
        })
        .collect()
}

/// Updates every stale DLL of every game no deny list names and that is
/// not running. `running` is asked once, with the candidate games only.
pub fn auto_update(
    cx: &DllContext<'_>,
    cache: &GameCache,
    manifest: &Manifest,
    lists: &DenyLists,
    running: &mut dyn FnMut(&[&Game]) -> BTreeSet<u64>,
    progress: Progress<'_>,
) -> AutoUpdate {
    let found = candidates(cache, manifest, lists);
    let mut out = AutoUpdate::default();
    if found.is_empty() {
        return out;
    }
    let games: Vec<&Game> = found.iter().map(|(g, _)| *g).collect();
    let busy = running(&games);
    let mut requests = Vec::new();
    for (game, wanted) in found {
        if busy.contains(&game.app_id) {
            out.running.push(game.name.clone());
        } else {
            requests.extend(wanted);
        }
    }
    if !requests.is_empty() {
        out.batch = ops::update(cx, manifest, requests, progress);
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::dll::ops::{BatchItem, FailKind, OpError, OpResult, Outcome};
    use crate::games::DllType;

    #[test]
    fn the_notice_reads_as_go_s() {
        assert_eq!(AutoUpdate::default().notice(), "");
        assert_eq!(
            AutoUpdate::skipped("no DLL manifest in the cache").notice(),
            "DLL auto-update skipped: no DLL manifest in the cache"
        );
        let item = |result| BatchItem {
            request: UpdateRequest {
                app_id: 1,
                dll_type: DllType::Dlss,
                path: "/g/nvngx_dlss.dll".into(),
                version: None,
                cached_only: false,
            },
            game_name: "A".into(),
            from_version: None,
            to_version: None,
            result,
        };
        let ok = Ok(OpResult {
            outcome: Outcome::Changed,
            files_changed: true,
            metadata_persisted: true,
            game: None,
        });
        let failed = Err(OpError::Failed {
            kind: FailKind::Download,
            message: "boom".into(),
        });
        let done = AutoUpdate {
            batch: BatchResult {
                items: vec![item(ok), item(failed)],
                updated: 1,
                unchanged: 0,
                failed: 1,
            },
            running: vec!["Ember Tide".into(), "Lantern Drift".into()],
            error: None,
        };
        assert_eq!(
            done.notice(),
            "DLL auto-update: updated 1 DLL; 1 failed (boom); skipped Ember Tide, Lantern Drift while running"
        );
    }
}
