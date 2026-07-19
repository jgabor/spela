# DLL mutation operations

`internal/dll` owns the synchronous install, update, batch-update, and restore
workflows. Every workflow uses the checksum-verified cache, denylist, one-time
backup, and atomic replacement primitives. Adapters do not mutate files, rescan,
or persist the database.

## States and outcomes

A mutation moves through these states:

1. Resolve and validate the requested manifest entry or backup.
2. Acquire and checksum the payload when installing or updating.
3. Create the original-file backup when one does not already exist.
4. Atomically replace or restore the game file.
5. Rescan the game directory.
6. Save the game database.

Every detected-file update names the detected absolute path. A basename or DLL
family is never used to choose among multiple copies. Backups use a path-derived
filename, while metadata retains the original and backup paths; existing backup
metadata and legacy backup paths remain readable.

Per-game updates resolve the authoritative game inside the database transaction
and enumerate its current exact DLL paths there. Adapters pass only game IDs and
an optional DLL type; explicit catalog cells continue to pass exact paths.

The manifest entry is resolved before cache use, including cached-only updates.
Its SHA-256 is required and checked against the cached bytes before replacement.
A stale cached manifest may be used offline, but unverified cached payloads may
not be installed. HTTP requests have a whole-request timeout, including body
transfer, so a stalled server cannot retain the mutation transaction forever.

Validation, download, checksum, denylist, and backup failures before replacement
are ordinary failures: `FilesChanged` is false. An update whose installed
version is current is a successful `no-op`; it does not download, rescan, or
save. A completed replacement is fully successful only after both rescan and
save succeed.

If replacement succeeds but scan or save fails, the operation returns a
`PartialFailure`. Its result has `FilesChanged: true` and
`MetadataPersisted: false`, and its message states that files changed while
metadata did not fully persist. The operation does not roll files back. A scan
failure leaves the prior in-memory metadata intact. A save failure leaves the
rescanned in-memory metadata available while the on-disk database may be stale.
Restore preflights every backup source and target and atomically replaces each
file. If a later replacement still fails, the partial failure records that the
earlier files changed and skips rescan and save.

Batch update applies the same operation independently to every requested cell,
continues after errors, and reports each result. A cell is counted as updated
only when replacement, rescan, and persistence all succeed; partial failures
remain failures even though their files changed.

## Progress and adapter ownership

Operations may emit `resolving`, `downloading`, `installing`, `updating`,
`restoring`, `scanning`, and `saving` events. Events describe domain work; they
do not imply success. Terminal events are represented by the returned result or
error rather than a progress event.

- CLI owns command syntax, text/progress formatting, and process exit behavior.
- Per-game TUI and batch TUI own confirmation, Bubble Tea commands, busy state,
  and message rendering.
- GUI owns Wails asynchrony and progress presentation.

Confirmation always happens before an adapter starts an operation. Adapters may
format errors differently, but must preserve `PartialFailure` so no surface can
claim full success after stale metadata.

## Serialization

The game package owns one in-process mutex and one cross-process advisory lock
under the XDG runtime directory. Its transaction helper takes the lock, loads a
fresh `games.yaml`, applies a callback, and atomically saves when changed. DLL
mutations hold this transaction through manifest/cache acquisition, file
mutation, scan, and save. Steam rescans use the same transaction, including the
scan itself, so no stale database snapshot can overwrite a concurrent writer.
Raw database persistence is private to `internal/game`; production writers must
use the transaction, and transaction callbacks must not recursively start
another transaction. There is no additional DLL database lock or per-app/cache
lock layer.

After taking the lock, an operation loads a fresh game database and updates only
the requested game or games in that database. It returns detached game snapshots;
adapters apply snapshots on their event loop or under their own small memory
lock. Temporary cache, replacement, metadata, and database files have unique
names and become visible only through atomic rename.
