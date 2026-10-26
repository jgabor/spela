# Canonical coverage measurement

Run the complete headless measurement from the repository root:

```bash
mage coverage
```

To regenerate the artifact and enforce the repository's 90% minimum in one
command, run:

```bash
mage coverageCheck
```

The target starts from an empty `coverage/` directory, runs the Go tests with
coverage, and writes the aggregate LCOV artifact to `coverage/lcov.info`. The
artifact contains `LF`/`LH` line totals and is directly consumable by the
Code Crusher gate:

```bash
/home/jgabor/.config/opencode/skills/code-crusher/scripts/check-gate.sh \
  --coverage-file coverage/lcov.info --json
```

`coverage/lcov.info` replaces untracked, ad hoc `coverage.out` files as the
project coverage source of truth. `mage coverageCheck` runs the converter's
regression suite before measuring coverage, so malformed span conversion cannot
silently pass the gate.

CI no longer runs this gate. While the Go tree stays as the parity reference
for the Rust rewrite, CI only runs `go build ./...` and `go test ./...`
(`docs/architecture.md`, CI step 8). The 90% minimum does not hold today:
`tools/goref`, the untested fixture generator, is inside the `./tools/...`
denominator. The gate and this file go with the Go tree in the final step.

## Scope and classification

- **Go production:** working-tree `.go` files in `cmd/`, `internal/`, and `tools/`.
  Tests use `-coverpkg=./cmd/...,./internal/...,./tools/...`, so packages with
  no local tests remain in the denominator.
- **Go tests:** `*_test.go` files. The tagged TUI tests in `tests/e2e/`
  remain end-to-end verification and are not production code or part of the
  headless unit coverage run. The Go release contract tests that lived in
  `tests/contracts/` were ported to `crates/dlss-updater/tests/release.rs` and
  `crates/dlss-updater/tests/aur_submit.rs` in Phase 7, and are gone.
- **Build tags:** two package graphs are measured and merged by source line:
  the default graph and the `e2e` graph of `internal/tui`, which holds the
  tagged E2E hooks. A Go AST inventory scans every non-test source under the
  production roots and fails if a file containing an executable function body
  is absent from both raw profiles. Declaration-only files are allowed to be
  absent because Go cannot instrument them. There are currently no generated
  or non-runtime Go source exclusions.
- **Generated/external/build files:** vendored dependencies (`vendor/`),
  coverage output, binaries, fixtures, and other build output are excluded.
  They are third-party code or test data rather than maintained production
  logic.

Go's coverprofile associates executable blocks with UTF-8 byte-column source
spans. The merger slices source as bytes, decodes complete spans, expands them
over executable source lines, excludes blank, comment-only, and delimiter-only
lines, and takes the maximum hit state for each source line across duplicate
records and build-tag runs. Serialized `DA` hit counts are normalized to `0` or `1`, paths and
records are sorted, and isolated equivalent runs therefore produce identical
artifacts. The final percentage is `sum(LH) / sum(LF)`.

## Simplification behavior baselines

The following named tests are the regression sets for the five implementation
epics. They intentionally assert persisted or public/package behavior rather
than the duplicated implementation that those epics will remove.

- **Delete the obsolete TUI profile editor:**
  `TestTUIGameDetail_ProfileMutations` exercises the compiled active detail
  flow against isolated XDG state; `TestDetail_ProfileSemantics*` covers the
  package-level inherited/override behavior.

  Game profiles use `ContentModel.detail`; the canonical defaults editor is
  `resourcePaneModel.defaultsDetail`. Both are `DetailModel` instances. The
  deleted `ProfileWidgetModel` had no constructor or update call in the
  compiled path. Its old assertions classify as follows:

  | Observable contract | Supported-path coverage |
  | --- | --- |
  | Every profile field renders in subsystem order and focus skips headings | `TestDetail_FieldEnumeration_*`, `TestDetail_JKCrossesGroupHeaders`, and `TestDetail_JKClampsAtEnds` |
  | Game values distinguish inherited and overridden state; reset, reset-all, and pin persist the intended overrides | `TestDetail_ProfileSemantics*`, `TestDetail_Reset*`, `TestDetail_Pin*`, and `TestTUIGameDetail_ProfileMutations` |
  | Root defaults render without inheritance markers and support direct cycle/reset/save | `TestDetail_Root*`, `TestResourcePane_RootMutationPersistsAndRetainsSelection`, and `TestTUISmoke_DefaultProfile` for compiled rendering |
  | Save success/failure is reported; root saves reload persisted defaults without losing selection | `TestResourcePane_RootMutationPersistsAndRetainsSelection`, `TestContentSupportedMessageAndKeyRouting`, and layout application-message assertions |

  Widget grid/edit mode, disabled “Coming soon” fields, its manual `s` binding,
  and its injected VKD3D inline callback were unreachable implementation
  details, not supported user-observable behavior. Value-format assertions now
  exercise `DetailModel` formatting rather than the deleted widget helpers.
  The DLSS preset modal and generic dialog compatibility were removed because
  no compiled input path opened them. Preset behavior remains covered through
  profile field mutation, CLI patches, YAML round trips, and launch apply.
- **Unify global configuration operations:** `TestConfigYAMLContract`,
  `TestRoundtrip` and `TestConfigCLITextAndErrorContract` preserve YAML and CLI
  keys.
- **Centralize DLL mutation operations:** the GUI boundary tests that covered
  this were deleted with the GUI in Phase 1 of the Rust rewrite.
- **Make GitHub Actions the sole release publisher:** the Go contract tests
  for this were ported to Rust in Phase 7. `crates/dlss-updater/tests/release.rs`
  parses the release workflows and pins the `v*` tag trigger, the tag
  validation, the scoped `contents: write` permission, the build with
  `SPELA_VERSION`, the binary and checksum, the changelog notes and the AUR
  package. `crates/dlss-updater/tests/aur_submit.rs` covers the dry-run-first
  AUR submit.
- **Unify profile fields and field-level mutations:** `TestProfileYAMLContract`,
  `TestOverrides_YAMLRoundTrip`, `TestMigration_LegacyProfileRoundTrip`,
  and `TestProfileCLITextJSONAndErrorContract` preserve profile persistence
  and the CLI projection.
- **Serialize raw profile mutation:** `TestMutateConcurrentDistinctSurfaceFieldsAndRollback`
  verifies that distinct concurrent field edits survive and callback failures
  leave the original YAML unchanged. `TestCreateAndDeleteShareMutationTransaction`
  covers create/delete serialization, while the TUI profile save serialization
  tests preserve the latest state across back-to-back inputs. CLI and TUI
  writers use this profile-owned transaction.
