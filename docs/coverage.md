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

The target starts from an empty `coverage/` directory, installs the locked
frontend dependencies, runs the declared frontend `test:coverage` package
script, and writes the aggregate LCOV artifact to `coverage/lcov.info`. The
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

## Scope and classification

- **Go production:** working-tree `.go` files in `cmd/`, `internal/`, and `tools/`.
  Tests use `-coverpkg=./cmd/...,./internal/...,./tools/...`, so packages with
  no local tests remain in the denominator.
- **Go tests:** `*_test.go` files and the contract-test package under
  `tests/contracts/`. The tagged TUI tests in `tests/e2e/` remain end-to-end
  verification and are not production code or part of the headless unit
  coverage run.
- **Build tags:** four package graphs are measured and merged by source line:
  default; `dev,webkit2_41`; `wails,production,webkit2_41`; and
  `wails,production,embed_assets,webkit2_41`. Together they include the normal
  CLI graph, Wails process startup, development GUI code, non-embedded
  production assets, and embedded production assets in the maintained-source
  denominator. The embedded graph runs after a locked frontend build so the
  `go:embed` input exists in clean checkouts. A Go AST inventory scans every
  non-test source under the production roots and fails if a file containing an
  executable function body is absent from all four raw profiles. Declaration-only
  files are allowed to be absent because Go cannot instrument them. There are
  currently no generated or non-runtime Go source exclusions.
- **Frontend production:** every tracked `src/**/*.js` and `src/**/*.svelte`
  file is included, including files not imported by a test. `*.test.js` is
  classified as test code.
- **Generated/external/build files:** generated Wails bindings (`wailsjs/`) and
  `navContract.generated.js`, vendored dependencies (`vendor/` and
  `node_modules/`), frontend `dist/`, Playwright output, coverage output,
  binaries, fixtures, and other build output are excluded. They are generated
  adapters, third-party code, or test data rather than maintained production
  logic.

Go's coverprofile associates executable blocks with UTF-8 byte-column source
spans. The merger slices source as bytes, decodes complete spans, expands them
over executable source lines, excludes blank, comment-only, and delimiter-only
lines, and takes the maximum hit state for each source line across duplicate
records and build-tag runs. Frontend line data comes from Vitest's V8 LCOV
report. Serialized `DA` hit counts are normalized to `0` or `1`, paths and
records are sorted, and isolated equivalent runs therefore produce identical
artifacts. The final percentage is `sum(LH) / sum(LF)` across both ecosystems.

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
  profile field mutation, CLI/GUI patches, YAML round trips, and launch apply.
- **Unify global configuration operations:** `TestConfigYAMLContract`,
  `TestRoundtrip`, `TestConfigCLITextAndErrorContract`, and the `config` case of
  `TestWailsJSONKeyContracts` preserve YAML, CLI, and Wails keys.
- **Centralize DLL mutation operations:**
  `TestGUIBoundaryDLLPassInstallsThroughBoundary`,
  `TestGUIBoundaryDLLPassUpdatesThroughBoundary`, and
  `TestGUIBoundaryDLLPartialOutcomesAfterMutation` preserve successful and
  post-mutation scan/save failure outcomes.
- **Make GitHub Actions the sole release publisher:**
  `TestReleaseWorkflowArtifactContract` parses the actual release workflow and
  pins its `v*` tag trigger, `contents: write` permission, canonical build,
  binary rename, checksum command, changelog body extraction, release action
  inputs, and both stable and development AUR publications.
  `TestReleaseArtifactContractBuildsFromCleanArchiveAndVerifiesChecksum`
  complements it by executing the artifact path from a clean tracked-source
  snapshot. `TestReleaseBuildInputsContract` keeps tool pins, frozen Bun
  installs, PKGBUILD outputs, and the absence of local publishing helpers in
  the same nonpublishing contract.
- **Unify profile fields and field-level mutations:** `TestProfileYAMLContract`,
  `TestOverrides_YAMLRoundTrip`, `TestMigration_LegacyProfileRoundTrip`,
  `TestProfileCLITextJSONAndErrorContract`, and the remaining
  `TestWailsJSONKeyContracts` cases preserve profile persistence and interface
  projections.
- **Serialize raw profile mutation:** `TestMutateConcurrentDistinctSurfaceFieldsAndRollback`
  verifies that distinct concurrent field edits survive and callback failures
  leave the original YAML unchanged. `TestCreateAndDeleteShareMutationTransaction`
  covers create/delete serialization, while the TUI profile save serialization
  tests preserve the latest state across back-to-back inputs. CLI, TUI, and GUI
  writers use this profile-owned transaction.
