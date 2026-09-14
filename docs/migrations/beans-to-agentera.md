# Beans to Agentera migration

Authority: approved plan `ofxsogtgej`, executed on 2026-09-14. This is an
immutable migration reconciliation record, **not another live tracker**. Current
work belongs to Agentera TODO entities. Source documents in `beans-source/`
are historical snapshots: their statuses, checklists, commands, paths, design
choices and implementation claims are not current execution instructions or
fresh verification. Do not update their checkboxes as work proceeds.

## Preservation — task mbufmxlxou

Verified filesystem: 372 Markdown files, comprising 343 completed, 7 scrapped,
17 todo, 2 in-progress, 2 draft and one historical GUI design without status.
Beans CLI returns 371 entries: it exposes only one of two completed files with
ID `spela-ieis`. The filesystem, not the CLI alone, is the preservation authority.
All 21 approved open IDs match. One summary correction: `spela-lr3c` omits
priority and the CLI defaults it to normal; only `spela-i7jg` explicitly has low
priority. This does not change mapping or imply a severity difference.

Durable private backup (outside Git and this checkout):
`/home/jgabor/.local/share/spela-beans-backup-ofxsogtgej-20260914/` (mode 0700).
`source.tar.gz` contains every source file plus `.beans.yml`: 373 files total.
SHA-256: `d02a14396b42abff2c5bce425392d8eb4426b8dd1697c39aed5549316eb67807`.
`SHA256SUMS` lists all individual source hashes; `snapshot.json` records inventory,
CLI metadata, relationships and the pre-import destination baseline. Every
archive member was read/decompressed and its bytes hashed against the live
source. Both `spela-ieis` files are included. To recover, extract the archive
into a new disposable directory (not over an active checkout), then run
`sha256sum -c /path/to/backup/SHA256SUMS` from that directory.

SHA-256 of `SHA256SUMS`:
`1cf4842005ab826b40f2afb94d64afaf111e280589f291eda848978764fbf0fc`.
SHA-256 of the private `snapshot.json`:
`a9e1f0b86dfcb234f2fabe96292bb405c9d57a8c15b9326c05491ef9573367a7`.

All 21 open source files and four completed context files (`spela-x1hf`,
`spela-icu7`, `spela-achk`, `spela-bsbq`) are also preserved byte-for-byte in
`beans-source/`. The untyped GUI design is preserved in
[`gui-integration-design-historical.md`](../design/gui-integration-design-historical.md),
not imported as a task. Its historical implementation checklist is not a new
commitment. The external backup retains the rest of completed/scrapped history.
No local configuration contents are copied into public documentation.

## Reviewed mapping and relationships

Default is exactly one open TODO per open source, including epics and milestones.
Original titles and complete raw files remain in supported `requirements`
content; unsupported metadata is source context, not invented schema fields.
No source is merged, resolved or superseded. All destinations start open with
an explicit pending triage gate, no execution lifecycle, empty new acceptance
lists, no inferred target version or release blocker. Source success criteria
remain verbatim in source content. Normal severity is a neutral migration
classification independent of Beans priority, not a finding of current impact.

Grouping is not blocking. Overlay `spela-s84z` groups `ifu0`, `fw5t`, `ixak`,
`hyis`, `gal2`, `nvwk` (all IDs have prefix `spela-`). FSR `csr0` groups `cife`;
XeSS `2v82` groups `emt9`. Completed `icu7` groups `dop3`, `m8qo`, `117e`,
`kmez`, `ftlu`, `rcsg`, `mljk`; completed `achk` groups `lr3c`, `i7jg`.
These completed parents are context only and are not reactivated.

Seven unresolved prerequisite edges become canonical TODO dependencies:
`ifu0 -> fw5t`; `fw5t -> ixak, hyis, gal2`; `ixak, hyis, gal2 -> nvwk`.
Two additional edges, `x1hf -> csr0, 2v82`, are satisfied historical context
from the completed prerequisite's incoming links, not unresolved TODOs.
Completed `bsbq` supplies overlay design context; old scrapped Rust/egui work
is not revived. Source relationship fields remain intact in the snapshots.

`spela-mljk` (stable package/v0.1.0) and `spela-dcgs` (`spela-git` publishing)
remain separate. Their age and possible overlap require later triage, not an
automatic merge. In particular dcgs's checked items, commit-count statement,
tool versions and publish commands are preserved claims, not new evidence or
authorization to publish. README overlay claims also require revalidation.
The four existing resolved TODOs (`ycsvvvvxkz`, `zgltvwomjl`, `ayynzpmziq`,
`pftgocuqob`) are distinct historical work and remain untouched, as do historical
plans `ajcnhmbmoh` and `jmnhtuudhf`.

## Import protocol — task gotmjcadbv

Use only `npx -y agentera@next state todo` readers and supported writers.
Topological batches avoid temporary or guessed dependency IDs. Before each
batch, reconcile the exact source marker, full raw content, title and open
status against all destination records. Stop on duplicate or changed content;
never blindly replay a confirmed create. Preserve requests and CLI-assigned
IDs outside the checkout. Review dry-run output, then apply the identical
request bytes with that preview's effect SHA-256 and explicit confirmation.
After interruption, read back destinations first and build only the missing
subset; existing identities are authoritative. Cleanup remains blocked until
all 21 exact reads and every relationship pass verification.

### Verified destination mapping

| Source ID | Agentera TODO ID | Source grouping |
| --- | --- | --- |
| spela-s84z | eggbfxzfxp | Overlay epic |
| spela-ifu0 | koenbmhuri | spela-s84z |
| spela-fw5t | lmqqegkizu | spela-s84z |
| spela-ixak | bnsyxerwuq | spela-s84z |
| spela-hyis | gnnhpofyvs | spela-s84z |
| spela-gal2 | gpezszcmet | spela-s84z |
| spela-nvwk | qgfdabsknk | spela-s84z |
| spela-csr0 | niifkclcxc | FSR draft milestone |
| spela-cife | onlsraejfs | spela-csr0 |
| spela-2v82 | tpaiodrbga | XeSS draft milestone |
| spela-emt9 | tlrpyvaqud | spela-2v82 |
| spela-dop3 | gskdqjynlq | completed spela-icu7 |
| spela-m8qo | dfmmzcscdh | completed spela-icu7 |
| spela-117e | oyoadjvvqs | completed spela-icu7 |
| spela-kmez | hvwumolyma | completed spela-icu7 |
| spela-ftlu | vwsuxxnada | completed spela-icu7 |
| spela-rcsg | klrtivfcke | completed spela-icu7 |
| spela-mljk | xhfovnglwy | completed spela-icu7 |
| spela-lr3c | hsiegjjrxs | completed spela-achk |
| spela-i7jg | avctoincfk | completed spela-achk |
| spela-dcgs | cjlumdmicj | independent spela-git publishing |

Canonical prerequisite-to-dependent edges verified by exact CLI reads:
`koenbmhuri -> lmqqegkizu`; `lmqqegkizu -> bnsyxerwuq, gnnhpofyvs, gpezszcmet`;
`bnsyxerwuq, gnnhpofyvs, gpezszcmet -> qgfdabsknk`. These seven native edges
plus the two satisfied historical edges account for all nine source edges.
The pending triage gates take precedence in current readiness evaluation:
all 21 report `gate_pending`, `eligible: false`. No parent was converted into
an extra dependency. Grouping is resolved through the table and raw source.

### Publication and recovery evidence

Five batches created 8 + 8 + 1 + 3 + 1 destinations. Every preview validated
with no violations; every confirmed publication returned the preview-assigned
IDs. Exact byte hashes (not the CLI's normalized input hash) and effect digests:

| Batch | Request SHA-256 | Confirmed effect SHA-256 |
| --- | --- | --- |
| 0 | `07b804e2827f6da2006e8d5fb97505de69b200d24105351ccb258d083e1cb5ec` | `e7a306b819505f8fd32739b4d1976e970560439055a77777c1935fb09f93751d` |
| 8 | `f22e9fd774d1a237d6ae08ad01b472c33a19e8c4b945175d55ec0e8ee7a31748` | `ecd7c688db2dbe30670d85399599a2362fc1cd3f40f7cd47277487e2232fda7b` |
| 16 | `310b8da60476d507d44b27af2ad3bf3a6215568b4c6ca12af832f8ebc2a57642` | `5b60403dd8c73f30239fd5166582dae99f5b2936b8367d620437c661b7562f77` |
| 17 | `bc24490e399243078e1de985170b342e8ce9fc9b4488e382c8418a07011ea68e` | `cbb20e40812035d5c9cfc863929346fb5a7ddd64723810d1ccf7252100530e2d` |
| 20 | `f8dac22a8aaa92c90255f1424ebd7dab106b1bf38b5b916667afeaffcb786ee5` | `02e54ff4d8ddc0f2cde25753f1e74d7b6f3db2edd48c83edeeb8a1eeb7d60973` |

Private `batch-N.json`, `preview-N.json` and `published-N.json` preserve the
requests and receipts. A real interruption after the first eight publications
(the bounded list omitted full records) was recovered using exact `get` reads;
the next batch contained only missing sources. Final reconciliation produces
no further request. Unit checks cover archive success/corruption, partial
reconciliation/duplicate/content mismatch, and exclusive artifact creation/
overwrite rejection. A later dry-run of batch 0 reused the same eight IDs but
reported `idempotent_replay: false` and a changed effect because project state
had advanced. It was **not applied**: exact reads and missing-only reconciliation,
not an assumed blanket CLI idempotency guarantee, are the recovery rule.

Before cleanup, all 25 destination records were read with `state todo get`.
All 21 raw source strings matched bytes exactly, including literal escaped
newlines and checklist states. The four existing resolved records matched
their baseline; all pre-existing Agentera, Claude and Cursor file hashes
matched. `check validate state` passed: 72 entities, zero issues.

## Retirement and final checks — task rkbvtniewc

After the preservation and import gates passed, exactly 373 verified files were
removed with `.beans/` and `.beans.yml`. Deletion first compared the complete
current source set and hashes with the preserved manifest, rejecting changed
or unexpected files. The external archive remains intact and the 25 public
source snapshots plus historical GUI design still match its bytes.

Repository changes: `AGENTS.md` now routes tracking to Agentera; the two TUI
design/acceptance documents label their old IDs as historical provenance while
retaining technical content and evidence. Supported TODO writers added 21
entities and their 21 managed `TODO.md` rows; the four resolved rows are unchanged.
No runtime, build, CI or release implementation was changed. `CHANGELOG.md` and
its historical Beans entry remain untouched.

Checkout-local changes (not distributed by a Git commit): ignored
`.claude/settings.json` lost only its two Beans hooks and now has an empty hooks
mapping. Ignored `.claude/settings.local.json` lost only the three Beans
permissions; Git inspection, local Spela execution and Wails permissions remain.
`.git/info/exclude` lost only `.beans` and `.beans.yml`; its comments and unrelated
exclusions remain. All other pre-existing `.claude`, `.cursor` and `.agentera`
files were hash-compared with the baseline, including skills, Agentera hooks,
resolved TODO entities, plans and coordinator-owned records. No unrelated state
was lost. This does not claim cleanup of any other checkout or global tool.

Final migration checks:

- All 25 TODOs were read again using exact `state todo get` calls: 21 open,
  all gated and ineligible; four resolved. Raw body bytes, native dependencies,
  titles, lifecycle absence, neutral severity and reconciliation all passed.
- `npx -y agentera@next check validate state`: **pass**, 72 entities, zero issues.
- `npx -y agentera@next doctor`: **manual_review_needed**, not a clean pass.
  The shared skill passes, but the unrelated global retired-resource candidate
  `/home/jgabor/.config/opencode/agents/audit.md` requires ownership review.
  No global cleanup was authorized or attempted; this is not a migration-state
  validation failure.
- Seven disposable helper tests passed, including success and rejection for
  archive integrity, duplicate/content reconciliation, exclusive file creation,
  interrupted-request byte retention, preview validation, effect/byte-bound
  publication and guarded retirement. Changed content and replay attempts fail
  before publication/deletion. No migration helper is installed in the project.
- Tracked reference search and hidden/ignored operational-reference scan found
  only the migration link, labelled historical TUI provenance and the unchanged
  CHANGELOG entry outside retained migration/Agentera history. No live Beans
  hooks, permissions, exclusions, runtime or workflow requirements remain.
- `git diff --check` passed; tracked diff and final status were reviewed.
  Pre-existing plan changes (`jmnhtuudhf`, `ofxsogtgej` and its three task files)
  are coordinator-owned baseline changes, not migration-worker lifecycle writes.

Private `verified-before-cleanup.json` and `verified-final.json` retain exact
read-back evidence. These are implementation acceptance checks, **not the final
independent audit**. The coordinator must obtain that one audit across all three
phases before recording completion. Evaluator records, task status, health
closure and plan lifecycle remain untouched. No commits, pushes, publishing,
global uninstall, vision/objective mutations or imported feature work occurred.
Application tests and TUI visual review were not run: runtime behavior did not
change, and migration-specific checks cover this scope.
