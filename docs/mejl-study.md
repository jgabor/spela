# What mejl is and how it was built

Date: 2026-10-25. Written by research agents that read the code and docs read only, then assembled without edits. Labels follow mejl's method: (verified: path:line), (inferred), (unverified). Paths without a repo prefix are relative to the repo the section covers. mejl lives at `~/git/mejl`. This doc covers how its research was done, what its docs say, and what its code actually does, with every mismatch between the two. Where they differ, treat mejl's code and `Cargo.toml` as the truth for what mejl built.

## Contents

1. Research method, doc conventions and the mockups
2. mejl-ui as built
3. The binary, terminal layer, tests and infrastructure

## mejl research method, doc conventions and reusable terminal UX

### Scope

- **Covered.** How mejl was researched and documented, plus the parts of its docs the coordinator had not read: email-clients.md, rust-crates.md from line 238, mockups.html, the three Fastmail research docs (for method only), AGENTS.md, README.md, the justfile, `.claude/` and `.github/`.
- **Rules followed.** Read only. No builds, no binaries, no network.
- **Labels.** (verified: path:line), (inferred), (unverified).
- **Doc versus code.** "The doc says" and "the code does" are kept apart. A few code checks outside the docs scope were made to test doc claims. They are marked "code".

All paths are under /home/jgabor/git/mejl unless they say otherwise.

---

### 1. The research method as a reusable template

#### 1.1 The doc set and its roles

| Doc | Role | Date and scope statement | Method or labels section | Probes or measurements | Fact-check section | Open questions form |
|---|---|---|---|---|---|---|
| docs/README.md | Index, vision, top 10 findings, decisions, consolidated open questions, status of gaps | "research done on 2026-10-23" (verified: docs/README.md:5); status dated 2026-10-25 (docs/README.md:138) | n/a | n/a | n/a | 42 numbered rows in 6 grouped tables, each with a verification step and the source docs (docs/README.md:54-136) |
| docs/architecture.md | Synthesis | "Date: 2026-10-23. This doc synthesizes the six research docs ... It adds no new web research" (verified: architecture.md:3) | Label carry-over rule (architecture.md:3) | Budgets table with a "Measured how" column (architecture.md:449-463) | none | Architecture-only table plus a pointer to the README (architecture.md:603-617) |
| docs/email-clients.md | Survey | "Research date: 2026-10-23. Scope: ..." (verified: email-clients.md:3) | "Method and labels" (email-clients.md:18-26) | None. GitHub API star and push dates only | none | Numbered list, each item saying "Resolve by ..." (email-clients.md:460-471) |
| docs/rust-crates.md | Catalogue | Date given in the summary prose (rust-crates.md:5) | "How this was gathered", with a verdict key (rust-crates.md:20-26) | Probe crate built, run, snapshot tested and timed (rust-crates.md:25, 409-434) | none | Bullets, each saying "Resolve: ..." (rust-crates.md:597-609) |
| docs/terminal-ux.md | Research plus UX spec sketch | "Research date: 2026-10-23. Scope: ..." (terminal-ux.md:18) | "Scope and labels" (terminal-ux.md:16-24) | Cited third-party measurements, dated (terminal-ux.md:375) | none | Table: question, why it matters, how to resolve (terminal-ux.md:605-622) |
| docs/fastmail-platform.md | Vendor platform | Date plus "No call was made to the user's real mailbox ..." (fastmail-platform.md:3) | "How to read this doc" (fastmail-platform.md:16-21) | Unauthenticated curl discovery (fastmail-platform.md:88, 99) | Yes: confirmed, changed, still uncertain (fastmail-platform.md:642-671) | Numbered, each with an exact command (fastmail-platform.md:625-640) |
| docs/fastmail-rules-sieve.md | Feasibility of one feature | Date in the summary (fastmail-rules-sieve.md:5) | "How to read this document" (fastmail-rules-sieve.md:18-23) | Local experiment table (fastmail-rules-sieve.md:276-293) | Yes: corrected, upgraded, added, unchanged, still uncertain (fastmail-rules-sieve.md:574-595) | Table: question, how to resolve (fastmail-rules-sieve.md:551-572) |
| docs/fastmail-masked-email.md | One API in depth | "Research date: 2026-10-23. All Fastmail pages were fetched on that date." (fastmail-masked-email.md:3) | "Sources and labels" (fastmail-masked-email.md:15-20) | Third-party probes, labeled unverified (fastmail-masked-email.md:346) | Yes: a changes list plus still uncertain (fastmail-masked-email.md:504-523) | Table (fastmail-masked-email.md:488-503) |
| docs/mockups.html | Visual target | Eyebrow "mejl research, docs/mockups.html, 2026-10-23" (mockups.html:170) | A "How to read the grids" section (mockups.html:200-222) | A grid self-check that runs on page load (mockups.html:415-418, 1283-1396) | n/a | n/a |
| docs/plans/*.md | Build plan for one feature | none | n/a | n/a | n/a | "Not known" paragraph (plan:25) |

Heading names vary from doc to doc: "Method and labels", "How this was gathered", "How to read this doc", "Sources and labels", "Scope and labels". Only fastmail-platform.md uses the formal confirmed, changed and still uncertain triple (verified: fastmail-platform.md:646-667). A grep counted "fact-check" zero times in README, rust-crates, architecture, terminal-ux and email-clients (verified: grep, this session).

#### 1.2 How docs state date and scope

- **One research date.** One date heads each doc (see the table). Volatile facts are dated per value:
  - star counts "show activity, not quality" (verified: email-clients.md:26);
  - crate metadata "fetched on 2026-10-23" (verified: rust-crates.md:5, 22-23);
  - "The HN thread and app ratings will drift" (verified: fastmail-platform.md:670);
  - terminal latency is a "dated measurement" (verified: docs/README.md:43, terminal-ux.md:375).
- **A boundary sentence.** Each doc says what was not touched:
  - "No call was made to the user's real mailbox or to Fastmail's MCP tools" (verified: fastmail-platform.md:3);
  - "I did not call any tool connected to the user's real Fastmail mailbox" (verified: fastmail-rules-sieve.md:23).
- **A scope sentence** names the doc's goal (verified: email-clients.md:3, terminal-ux.md:18).
- **Gap: dates were not revised.** Later edits did not update the header dates:
  - rust-crates.md says every value was fetched 2026-10-23 (rust-crates.md:5). It now also contains as-built facts, such as "As built, mejl has its own body editor" (rust-crates.md:9) and "superseded by mejl's own editor" (rust-crates.md:51).
  - architecture.md keeps "Date: 2026-10-23" (architecture.md:3) while it holds "As built (2026-10-25)" sections (architecture.md:257, 396).

#### 1.3 Labels and source policy

- **Core labels**, defined in every research doc:
  - "(verified: source)" means a primary source, or a check run by the researcher;
  - "(inferred)" means reasoned from verified facts;
  - "(unverified)" means a lead, not a fact.

  (verified: email-clients.md:21-24, fastmail-platform.md:18-20, fastmail-rules-sieve.md:20-22, terminal-ux.md:22-24, fastmail-masked-email.md:20)
- **Extra labels:**
  - "(project claim)": a project says it about itself and nobody has measured it (email-clients.md:25);
  - "verified absence" (fastmail-rules-sieve.md:11);
  - "verified: local experiment" (fastmail-rules-sieve.md:276);
  - "verified: probe build" (rust-crates.md:39);
  - "measured" (rust-crates.md:255);
  - "verified: curl, 2026-10-23" (fastmail-platform.md:88);
  - "inferred targets" for budgets (docs/README.md:43).
- **Primary sources** are the vendor's own pages, RFCs and drafts, source code that calls the live API, and HTTP responses the researcher fetched (verified: fastmail-platform.md:18). Third-party reviews are used only for user complaints, and those claims are labeled (verified: email-clients.md:20).
- **Weighting a weak source.** One self-contradicting source (maton-ai notes) has every derived claim pinned at "(unverified)", and the reason is given (verified: fastmail-masked-email.md:19).
- **Split labels.** One claim can carry two labels, for example "verified dates; dormancy inferred" (verified: email-clients.md:127). Tables carry a Label column (verified: fastmail-platform.md:27-54).
- **Absences.** Absence claims are relabeled inferred unless a source states the absence (verified: fastmail-platform.md:664).
- **Design statements** are labeled inferred: "Design decisions made here are (inferred) unless stated otherwise" (verified: architecture.md:3, fastmail-rules-sieve.md:345).

#### 1.4 "How this was gathered" sections

rust-crates.md:20-26 is the fullest example. It contains:
- the exact API endpoints, and which fields were read from them: `max_stable_version`, the version's creation date, license and `rust_version`, plus the dependencies endpoint;
- `gh api repos/<owner>/<repo>` for stars, open issues plus PRs, last push and the archived flag, with the caveat that GitHub's "open issues" includes PRs;
- the repos that were shallow-cloned and grepped;
- the probe crate's toolchain and profile, and what it proved (an insta snapshot of a ratatui `TestBackend`);
- variant builds that isolate the cost of one dependency each;
- a verdict key: Pick, Later, Alt, Ref, Avoid.

The Fastmail docs put the same information into their opening sentence and their labels section (verified: fastmail-platform.md:3, 21; fastmail-masked-email.md:15-20).

#### 1.5 Probe builds, local experiments and measurements

- **Probe crate.** It used edition 2024, rustc 1.99.1, 16 cores, `lto = "thin"`, `codegen-units = 1` and `strip = true` (verified: rust-crates.md:25, 411).
  - The environment was recorded: no RUSTC_WRAPPER, no sccache, no cargo config. Whether the linker is rust-lld is labeled unverified (verified: rust-crates.md:411).
  - It was run once per configuration, with "treat the numbers as rough" (verified: rust-crates.md:411).
  - The starter set is marked line by line: uncommented lines were in the probe, commented "add when needed" lines were only version-checked (verified: rust-crates.md:339).
- **Local experiment.** A throwaway crate ran the sieve-rs `Compiler` with and without capability checks. Results go in an input-to-result table, then a "What this means" list. The fact-checker "read the throwaway crate's source and reran it with identical results" (verified: fastmail-rules-sieve.md:276-302).
- **Unauthenticated live reads.** Discovery endpoints were fetched with curl, and only metadata came back (verified: fastmail-platform.md:3, 88, 99).
- **Measurement method for later work** is borrowed from Superhuman (verified: email-clients.md:205-210):
  - start the timer at the OS event;
  - stop at frame flush;
  - throw away samples taken while hidden;
  - report the share of events per bucket, not percentiles.

  architecture.md turns this into budgets, each with "Measured how" (architecture.md:449-463), and into rules: buckets under 8, 16, 50 and 100 ms; a ring buffer of 1,000 frames; a `:perf` overlay; and CI fails on p99 regressions (architecture.md:465-472).

#### 1.6 Fact-check notes

- **The opening sentence** states the date, what was re-fetched or rerun, and what was not done. Example: "It did not call the user's mailbox or Fastmail's MCP tools, and it did not register an OAuth client" (verified: fastmail-platform.md:644; fastmail-rules-sieve.md:576; fastmail-masked-email.md:506).
- **The body takes one of three shapes:**
  - "Confirmed without change", "Changed" and "Still uncertain" (verified: fastmail-platform.md:646-671);
  - "Corrected", "Upgraded", "Added" and "Unchanged and confirmed", plus "Still uncertain" (verified: fastmail-rules-sieve.md:578-595);
  - a flat list of changes plus "Still uncertain", ending with a style check: "no em or en dashes were found before or after edits. Headings were already in sentence case" (verified: fastmail-masked-email.md:506-523).
- **Typical outcomes:**
  - labels downgraded (verified: fastmail-platform.md:656, 664);
  - quotes corrected to the exact wording (verified: fastmail-platform.md:659-660);
  - a dead anchor fixed (verified: fastmail-platform.md:652);
  - an experiment rerun with the same result (verified: fastmail-rules-sieve.md:584);
  - a new risk outside the doc's scope recorded (verified: fastmail-rules-sieve.md:595).
- **Gap.** email-clients.md, rust-crates.md and terminal-ux.md had no fact-check pass (verified: grep counts this session).

#### 1.7 Open questions with verification steps and safety rules

- **Consolidation.** docs/README.md "consolidated from all docs and deduplicated" the open questions into 42 numbered rows (verified: docs/README.md:56).
  - They are grouped as: live API tests (1-13), questions for the vendor (14-21), rules and Sieve (22-28), Rust, storage and build (29-32), terminal and UX (33-40), and low-priority competitor facts (41-42) (verified: docs/README.md:64-136).
  - Each row has a "Verification step" and a "Source docs" column. Vendor questions name the contact address (verified: docs/README.md:84).
- **Safety rules come before the steps** (verified: docs/README.md:56-62):
  - read-only probes may use the user's own account, with a temporary token deleted afterwards;
  - every write test runs only on a trial account;
  - nothing goes through the vendor's MCP tools;
  - one private API is never called, not even read-only;
  - session output is redacted before it becomes a fixture.
- **Verification procedure.** fastmail-rules-sieve.md:437-549 is a full procedure:
  - copy-paste commands with an "Expected today" result per step;
  - an explicit "Never run `SieveScript/set` against the real account" (fastmail-rules-sieve.md:523) and "Do not call ... even read-only" (fastmail-rules-sieve.md:525);
  - a closing "Step 5: record and decide" that maps outcomes to design options (fastmail-rules-sieve.md:546-549).
- **Limits on probing.** Some questions carry an explicit "do not probe a real account" (verified: docs/README.md:93; fastmail-masked-email.md:497).

#### 1.8 Decisions for the user

- **Table.** "Decisions the user needs to make" has three columns: Decision, Options, Recommendation (inferred). It covers license, Basic-plan users, when to contact the vendor, and MSRV (verified: docs/README.md:45-52).
- **The synthesis does not decide.** "These are the user's calls. This doc recommends answers but does not make them." (verified: architecture.md:28). The open-questions table repeats them as "User decision ..." rows (verified: architecture.md:616-617).
- **Recording an answer.** The plan records answers under "Your answers:" (verified: plan:7-10). The status section dates a user decision: "a user decision on 2026-10-25" (verified: docs/README.md:147).

#### 1.9 The synthesis doc: architecture.md

- **No new research.** It names the six research docs and keeps each claim's label and link (verified: architecture.md:3-10).
- **Summary.** A feasibility verdict table: Goal, Verdict (Yes, Partial, Yes with gaps, Yes by design but unproven), One line (verified: architecture.md:17-23). Then policy, auth, stack, and the decisions that are needed (verified: architecture.md:25-28).
- **Guiding principles.** Ten numbered principles, each with its label and source doc (verified: architecture.md:30-41). Then product direction, non-goals and plan-gated features (verified: architecture.md:43-53).
- **Performance budgets.** A table of metric, budget and "Measured how" (verified: architecture.md:449-463).
- **Testing strategy.** Layer, What, How (verified: architecture.md:474-490). Notable rows:
  - fake servers with a switch for each reported quirk (architecture.md:481);
  - live contract tests that are opt-in and use a test account only (architecture.md:484);
  - view snapshots at 80, 120 and 200 columns (architecture.md:485).
- **Phased roadmap.**
  - "Each phase lists what it proves. A phase is done when its proof holds, not when its feature list is typed out." (verified: architecture.md:494)
  - Phase 0 is "verify and decide (no app code)" (verified: architecture.md:496-501).
  - Every phase ends with "**Proves:**" (verified: architecture.md:501, 509, 516, 522, 529, 537).
  - It finishes with "Later (only on evidence)" (verified: architecture.md:539-546).
- **Risk register.** Columns: #, Risk, Likelihood (with a label), Impact, Mitigation, Trigger to act. There are 20 rows (verified: architecture.md:548-571).
- **"Where the research docs disagree".**
  - "Each disagreement is resolved here or reported as open. Nothing was picked silently." (verified: architecture.md:575)
  - Columns: #, Topic, What the docs say, Resolution. The resolution is one of **Resolved**, **Open**, **Partly resolved**, **Resolved provisionally** or **Resolved in code**. There are 23 rows (verified: architecture.md:577-601).
  - Row 20 catches docs that point at docs which do not exist (verified: architecture.md:598).
- **As built.** After the build, dated "As built" subsections were added (verified: architecture.md:257, 396-402).

#### 1.10 Status of known gaps and as-built notes

- **The README section.** It is dated: "Status of the known gaps (2026-10-25)" (verified: docs/README.md:138).
  - "Fixed, each with tests:" lists behaviour with numbers, for example "p99 key to frame about 0.4 ms, was 10 ms" (verified: docs/README.md:140-154).
  - "Not done:" gives the reason for each item (verified: docs/README.md:156-160).
- **Inline notes.** Research docs get inline "As built:" notes rather than rewrites (verified: terminal-ux.md:78, 183, 193, 429).

#### 1.11 The plan doc format (docs/plans/clever-dreaming-donut.md)

- **Title:** "Plan: <feature>" (verified: plan:1). The file name is three random words (inferred: likely a Claude Code plan-mode name).
- **Context** (verified: plan:3-25) contains:
  - why now;
  - "Your answers:", recording the user's choices;
  - "Known facts", citing doc and line ranges with the verification date;
  - "Not known";
  - a bold safety line: "No code, test or tool call contacts Fastmail. You run the first live login yourself."
- **Design per crate** (verified: plan:27-96):
  - protocol layer, fake server and app, as bullets;
  - the fake gets switches that simulate vendor quirks (verified: plan:65).
- **"Rules from review (they go into both agent briefs)"** (verified: plan:98-125). These are numbered invariants, each with the test that proves it. Rule 6 is a no-network rule plus a grep to run before handing off.
- **Dependencies:** "I make these edits" (verified: plan:127-129).
- **Work order** (verified: plan:131-138):
  1. Me: the dependency edits.
  2. Agent A: the protocol layer and the fake.
  3. Me: review A's API before B starts.
  4. Agent B: the app integration.
  5. An independent review agent: security and correctness.
  6. Docs: the README and the architecture "as built" text.
- **Critical files** (verified: plan:140-145).
- **Verification** (verified: plan:147-170):
  - unit tests, fake tests and app tests;
  - the gate command plus the MSRV check;
  - "Live, by you only", which also names the open questions the live run settles.

#### 1.12 Skeletons

Research doc:

```text
# <Topic>: <the question it answers for the project>
Research date: YYYY-MM-DD. Scope: <goal>. <What was not touched: no live system, no MCP, no writes>.

## Summary
- **<Finding as a claim.>** <Evidence> (verified: <source>). <What it means for the project> (inferred).
- ... (5 to 10 bullets)
What this means for <project>: <one paragraph>.

## How this was gathered | Method and labels
- Sources: primary vs secondary, with fetch dates and exact endpoints or commands
- Probe: toolchain, machine, profile, what was built and run, runs per measurement
- Labels: (verified: source), (inferred), (unverified), (project claim)
- Verdict key, if this is a catalogue: Pick / Later / Alt / Ref / Avoid

## <Body sections> (tables carry a Label or Verdict column)
## Design options (ranked, with a comparison table and "What is not possible today")
## Verification procedure (copy-paste steps, expected result per step, explicit don'ts, a "record and decide" step)
## Open questions (table: question | why it matters | how to resolve)
## Fact-check notes
An adversarial fact-check ran on <date>. It re-fetched <...> and reran <...>. It did not <...>.
### Confirmed without change
### Changed
### Still uncertain
```

Synthesis doc:

```text
# <Project> architecture and product direction
Date. "This doc synthesizes the N research docs. It adds no new research. Claims keep their labels; design decisions are (inferred)."
## Summary (feasibility verdict table, decisions that are needed and whose they are)
## Guiding principles (numbered, each with label and source doc)
## Product direction (position, non-goals)
## Feasibility verdicts
## System architecture / data model / <domain sections>
## Crate picks (versions from the crates doc; "Excluded" with reasons)
## Performance budgets (metric | budget | measured how)
## Testing strategy (layer | what | how)
## Phased roadmap (Phase 0: verify and decide, no code; each phase ends "Proves: ..."; "Later (only on evidence)")
## Risk register (# | risk | likelihood (label) | impact | mitigation | trigger to act)
## Where the research docs disagree (# | topic | what the docs say | Resolved / Open / Resolved provisionally) "Nothing was picked silently."
## Open questions (architecture-only; pointer to the consolidated list)
(later) As built (date): ...
```

Index README:

```text
# <project> research docs
## Summary (date, 4 to 6 bullets, "start with architecture.md")
## Docs in this folder (file | what it covers)
## Project vision (one paragraph)
## Top 10 findings (bold claim, label, link)
## Decisions the user needs to make (decision | options | recommendation (inferred))
## Open questions (safety rules first; grouped tables: # | question | verification step | source docs)
## Status of the known gaps (date) (Fixed, each with tests / Not done, with reasons)
```

Plan:

```text
# Plan: <feature>
## Context (why now; Your answers:; Known facts (doc:lines, verified date); Not known; safety line)
## Design
### <Layer or crate 1> / ### Fake / ### App
### Rules from review (they go into every agent brief): numbered invariant + its test
### Dependencies (I make these edits)
## Work order (Me / Agent A / Me: review / Agent B / independent review agent / Docs)
## Critical files
## Verification (unit / fake / app tests; the gate command; "Live, by you only" and which open questions it settles)
```

Mockup file:

```text
<header: eyebrow with path and date; "not screenshots"; links to the spec docs; theme and size controls>
<nav: numbered screens>
<section About: how to read the grids, glyph legend>
<section per screen: grid figure; "Design choices"; "Keys"; "Mouse">
<section Grid check: automated exact-width, no-clip, safe-glyph check; "What these mock-ups cannot show">
<script: Grid class modelled on a ratatui buffer; shared pieces; fictional data; one build function per screen; checks; renderer>
```

#### 1.13 Gaps in the method worth fixing in spela's version (inferred)

- Run a fact-check pass on every research doc, not three of six.
- When adding as-built facts, add a dated "As built" note, and leave the original research claim and its date untouched. Do not mix the two (see conflicts).
- Keep crate picks in sync with the manifest, or add an "as built" column. Otherwise the doc's picks drift from what was built (section 3.10).
- Close decisions in the README when they are made. The license and MSRV are still listed as open (see conflicts).

#### 1.14 Applying the template to spela (inferred)

- **Candidate research docs:**
  - NVML from Rust (setters need root);
  - polkit and pkexec self-exec for the batched `apply-profile`;
  - Steam library, VDF and appmanifest parsing;
  - DLSS and DLL sources, versions and backups;
  - the anti-cheat denylist;
  - CPU governor and SMT through sysfs;
  - Rust crates (TUI, YAML, CLI, logging, XDG);
  - terminal UX, reusing mejl's terminal-ux.md where it is generic;
  - a "Go behaviour inventory" that serves as the parity spec.
- **Safety rules for spela probes:** never pkexec or sudo, never touch GPU, governor or display settings, and never touch game files. Any write test runs in a VM or against fake sysfs and NVML.

---

### 2. email-clients.md: what transfers to a game configuration TUI

#### 2.1 Method

- **Sources.** Project repos read through the GitHub API, plus a direct clone of aerc; official docs, blogs, release notes and package registries; third-party reviews only for complaints (verified: email-clients.md:20).
- **Breadth.** 16 terminal clients and 13 desktop, web and mobile clients, each with an "At a glance" table and per-client sections that end "For mejl" (verified: email-clients.md:32-49, 179-193, docs/README.md:23).
- **Feature comparison.** A legend explains cell meanings (verified: email-clients.md:436-458).

#### 2.2 What makes a client feel fast (email-clients.md:331-345), mapped to spela

| Technique | Evidence in the doc | Generic? | spela mapping (inferred) |
|---|---|---|---|
| Render only from a local store | Superhuman local DB, Fastmail IndexedDB, Marco (email-clients.md:335) | Yes | The TUI renders the game library and profiles from a cached store. It never waits on a Steam library rescan, NVML, a DLL download or pkexec |
| Optimistic mutations with an ordered queue; repeat edits collapse; failures roll back visibly | Superhuman modifiers, Fastmail patch log (email-clients.md:336, 211-215) | Partly | Profile edits are local YAML writes and can be instant. Privileged applies and DLL swaps cannot be optimistic, so show them as pending with a visible result |
| Push instead of polling | (email-clients.md:337) | Partly | Watch Steam library files rather than rescanning. GPU metrics still need polling |
| Incremental sync by state | JMAP `/changes` (email-clients.md:338) | Yes, as an idea | Rescan only the appmanifests whose mtime changed |
| Staged loading | IDs, then metadata, then bodies (email-clients.md:339, 255) | Yes | List rows first, then DLL versions, then expensive checks |
| Prefetch the likely next item | Superhuman preload (email-clients.md:340) | Yes | Detect the DLLs of the next N rows ahead of the cursor |
| Measure latency in buckets | Superhuman under 50, 100 and 1000 ms, plus "terrible" (email-clients.md:341, 205-210) | Yes | Key-to-flush buckets and a `:perf`-style overlay |
| Minimal animation | Superhuman (email-clients.md:342) | Yes | No spinners for local actions. Slow work is reported on the status line |
| Shortcut hints in the UI | Superhuman, aerc keyhint popup (email-clients.md:343) | Yes | Hint bar, which-key popup, `?` overlay |
| A fast search path | HEY, Fastmail (email-clients.md:344) | Yes | Instant local fuzzy filter over games and profiles |
| Separate background work from the UI | Mailspring, Fastmail shared worker (email-clients.md:345) | Yes | Worker tasks talk to the UI over channels. The UI thread never does I/O |

#### 2.3 Design principles (email-clients.md:349-387)

1. **Local first.** The cache must be disposable. Durable user intent is the one exception (verified: email-clients.md:351-356).
   - Generic (inferred): in spela, profiles are user intent and the scanned game DB is cache. The same holds for mejl's "cache.db is disposable, state.db is not" (verified: architecture.md:33).
2. **Design for the protocol.** Global immutable IDs, not IMAP-era per-folder keys (verified: email-clients.md:357-362).
   - Generic lesson (inferred): key everything by a global id, for spela the Steam AppID, and never by a per-library index.
3. **One provider, done deeply** (verified: email-clients.md:363). Email specific.
4. **Latency as a budget:** under 16 ms for any local action, under 50 ms to open a cached item. These are inferred budgets; the 50 ms figure is verified for Superhuman (email-clients.md:364-369). Generic.
5. **Every action undoable.** Confirm only what cannot be undone (verified: email-clients.md:370). Generic. In spela: DLL swaps with backups, and GPU settings restored on game exit (inferred).
6. **Three keyboard layers:**
   1. single-key verbs;
   2. a `:` command line with completion, plus a fuzzy palette;
   3. contextual `?` help plus a popup after prefix keys.

   "Every palette entry shows its key, so the palette teaches the shortcuts" (verified: email-clients.md:371-376). Generic.
7. **Mouse on by default, never required.** Every mouse action has a key (verified: email-clients.md:377-381). Generic.
8. **One small, stable config** with versioned migrations (verified: email-clients.md:382). Generic.
9. **Privacy by default.** No servers of its own; credentials in the keyring (verified: email-clients.md:383-387). Partly generic. spela's equivalent is no telemetry and clear provenance for DLL downloads (inferred).

#### 2.4 Concrete ideas (email-clients.md:389-419) that generalise

- **Default keymap.** Match the tool users already know (verified: email-clients.md:391-398). For spela that is likely the existing Go TUI's keys, for parity (inferred).
- **"From this item" actions.** "Rule from this message" pre-fills a builder from the selection and previews matches (verified: email-clients.md:399). spela's analogue is a profile from this game, or from the running game, with a preview of what will change (inferred).
- **Split views from saved queries** (verified: email-clients.md:407). spela's analogue is saved filters such as "has DLSS", "has profile" and "anti-cheat" (inferred).
- **Staged bulk actions,** after elma's `$` commit (verified: email-clients.md:410). In spela: mark several games for a DLL update, review, then apply (inferred).
- **Edit in `$EDITOR`,** plus a quick inline editor (verified: email-clients.md:411-416). In spela: edit the profile YAML in `$EDITOR` (inferred).
- **Latency overlay** (verified: email-clients.md:418). Generic.
- **Setup from one identifier.** mejl discovers the account from an email address (verified: email-clients.md:419). spela's analogue is Steam library auto-detection (inferred).
- **A scriptable JSON surface** is now normal. Keep the core in a library so a CLI is cheap to add (verified trend, email-clients.md:323-329). spela already has a CLI, which argues for a core library shared by the CLI and TUI (inferred).

#### 2.5 Anti-patterns (email-clients.md:421-434), all generic except where noted

| Anti-pattern | Evidence (doc) | spela reading (inferred) |
|---|---|---|
| Speed features off by default | NeoMutt caches, aerc `cache-state`, meli index | Caching and prefetch on by default |
| Caches that never clean up | aerc `cache-blobs` "is never purged automatically" | Cap and evict the DLL download cache and old backups |
| Per-folder databases and local keys | Thunderbird Panorama rewrite | One table keyed by AppID |
| Network I/O on the UI path | aerc advertises async I/O | No NVML, pkexec, network or filesystem scans on the UI thread |
| Search that depends on an OS indexer | Apple Mail | Own the index |
| Server-side relay of credentials or content | Spark | Email specific; for spela, no phoning home |
| Config and scope churn | himalaya v1 and v2 | Versioned config with automatic migration. A rewrite must read existing spela YAML |
| A bag of plugins in place of a core | Matcha | Build the core in. Add hooks after a second real use |
| Building on a platform's whim | Notion Mail | Stay small and independent |
| A mouse path that breaks unnoticed | aerc 0.22 fix | Test the mouse like keys, with snapshot tests |

#### 2.6 Not transferable

JMAP and the sync protocol, Sieve, Masked Email, HTML-to-text, push over EventSource, the Screener, Reply Later, and the vendor policy analysis (verified: email-clients.md:357-362, 399-417).

---

### 3. rust-crates.md from line 238 to the end (versions exact, as fetched 2026-10-23)

#### 3.1 Fuzzy matching and pickers (rust-crates.md:235-243)

| Crate | Version | Released | License | Verdict | Note |
|---|---|---|---|---|---|
| nucleo-matcher | 0.3.1 | 2024-02-20 | MPL-2.0 | Pick | Helix's matcher, about 6x faster than skim's on its own benchmark. The README calls it "finished". `Pattern::parse(...).match_list(...)` worked in the probe. MPL is "fine as an unmodified dependency (inferred, not legal advice)" |
| nucleo | 0.5.0 | 2024-04-02 | MPL-2.0 | Later | Multithreaded (rayon). The API may change |
| frizbee | 0.13.0 | 2026-08-13 | MIT | Alt | SIMD Smith-Waterman, MSRV 1.89 |
| fuzzy-matcher | 0.3.7 | 2020-10-04 | MIT | Avoid | Repo archived |
| skim | 5.8.0 | 2026-10-20 | MIT | Avoid | A whole application, too heavy |

#### 3.2 Config, secrets, paths and OAuth (rust-crates.md:245-262)

| Crate | Version | Released | License | Verdict | Note |
|---|---|---|---|---|---|
| serde / serde_json | 1.0.229 / 1.0.154 | 2026-07-18 / 2026-10-11 | MIT OR Apache-2.0 | Pick | The `raw_value` feature name is unverified for this version |
| toml | 1.1.8+spec-1.1.0 | 2026-10-09 | MIT OR Apache-2.0 | Pick | TOML 1.1 |
| toml_edit | 0.25.17+spec-1.1.0 | 2026-10-09 | MIT OR Apache-2.0 | Later | Writes config and keeps comments |
| figment | 0.10.19 | 2024-05-17 | MIT OR Apache-2.0 | Alt | Layered config |
| config | 0.15.27 | 2026-09-30 | MIT OR Apache-2.0 | Alt | MSRV 1.88 |
| clap | 4.6.7 | 2026-09-14 | MIT OR Apache-2.0 | Later | CLI flags and subcommands |
| keyring | 4.2.0 | 2026-08-29 | MIT OR Apache-2.0 | Pick | Wraps keyring-core 1.0.1. The `v1` feature keeps `Entry::new`. Cost: 65 crates and about 3 MB (216 vs 281 crates, 11 vs 14 MB, measured) |
| secrecy | 0.10.3 | 2024-10-09 | Apache-2.0 OR MIT | Pick | `SecretString` |
| zeroize | 1.9.1 | 2026-10-06 | Apache-2.0 OR MIT | Pick (transitive) | |
| etcetera | 0.11.0 | 2025-10-28 | MIT OR Apache-2.0 | Pick | `choose_base_strategy()`. Can force XDG on macOS (probe build) |
| dirs | 7.0.0 | 2026-09-05 | MIT OR Apache-2.0 | Alt | Moved to Codeberg |
| directories | 6.0.0 | 2025-01-12 | MIT OR Apache-2.0 | Alt | GitHub repo archived, moved to Codeberg |
| oauth2 | 5.0.0 | 2025-01-21 | MIT OR Apache-2.0 | Later | Its reqwest feature pins reqwest ^0.12 and pulls chrono and rand 0.8 |
| openidconnect | 4.0.1 | 2025-07-06 | MIT | Avoid | Not OIDC |

#### 3.3 Misc (rust-crates.md:264-301)

- **Logging and errors:**
  - tracing 0.1.44 (2025-12-18, MIT), Pick;
  - tracing-subscriber 0.3.23 (2026-03-13, MIT), Pick, with `EnvFilter`;
  - tracing-appender 0.2.5 (2026-04-17, MIT), Pick: "A TUI cannot log to stdout";
  - anyhow 1.0.104 (2026-07-18), Pick;
  - thiserror 2.0.21 (2026-09-23), Pick;
  - color-eyre 0.6.5 (2025-05-30), Alt;
  - miette 7.6.0 (2025-04-27, Apache-2.0), Later;
  - human-panic 2.0.8 (2026-04-02), Later.
- **Desktop and OS:**
  - notify-rust 4.18.2 (2026-10-07), Later;
  - arboard 3.7.0 (2026-10-15), Alt, which does not work over SSH;
  - crossterm `osc52` feature 0.29.0, Pick;
  - osc52 0.1.0 (2023-08-11, GPL-3.0-only), Avoid;
  - copypasta 0.10.2 (2025-04-25), Avoid;
  - open 5.4.5 (2026-10-22, MIT), Pick;
  - opener 0.9.0 (2026-09-29), Alt;
  - mime_guess 2.0.5 (2024-06-29), Later;
  - mime 0.3.17 (2023-03-20), Avoid.
- **Dates:**
  - jiff 0.2.38 (2026-10-06, Unlicense OR MIT), Pick;
  - chrono 0.4.45 (2026-06-04), Avoid as a direct dependency;
  - time 0.3.55 (2026-08-01), Avoid.
- **Text:**
  - unicode-width 0.2.2 (2025-10-06) and unicode-segmentation 1.13.3 (2026-06-01), Pick (transitive);
  - textwrap 0.16.4 (2026-09-13), Later;
  - syntect 5.3.0 (2025-09-27), Later;
  - two-face 0.5.2+bat-0.26.1 (2026-08-07), Later;
  - image 0.25.10 (2026-03-10), Later.
- **Test and bench:**
  - criterion 0.8.2 (2026-02-04), Alt;
  - divan 0.1.21 (2025-04-10), Pick (dev);
  - insta 1.49.0 (2026-10-03, Apache-2.0), Pick (dev): `assert_snapshot!(terminal.backend())` worked;
  - ratatui `TestBackend`, Pick (dev);
  - vt100 0.18.1 (2026-10-19), Later (dev), for running the real binary in a pty.
- **Other:**
  - signal-hook 0.4.5 (2026-10-04), Avoid as a direct dependency, because crossterm handles SIGWINCH and tokio handles SIGTERM, and "Suspend ... needs care";
  - compact_str 0.10.0 (2026-07-13), Later;
  - uuid 1.29.0 (2026-10-23), Avoid.

(verified: rust-crates.md:268-301)

#### 3.4 Prior art in Rust (rust-crates.md:303-317)

- **meli and melib** 0.8.13 (2026-01-05), EUPL-1.2 OR GPL-3.0-or-later. Uses termion 1.5 and no ratatui. Study only; do not copy.
- **himalaya** 2.2.1 (2026-10-02), MIT OR Apache-2.0. Built from sans-IO crates. A reference for config shape and secrets.
- **email-lib** 0.27.0 and **mml-lib** 1.1.2 (2026-02-19). Legacy.
- **pimalaya-tui** 0.3.1 (2026-02-19). Prompts on crossterm 0.27 plus inquire.
- **jmap-client** 0.4.3, Apache-2.0 OR MIT. Copyable with attribution.
- **Stalwart server.** AGPL-3.0; the exact SPDX id is unverified. Read only.
- **mujmap** 0.2.0, GPL-3.0.
- **io-jmap** 0.6.0. Its push model: "one state change per push frame with cooperative shutdown".
- **crate-jmap** 0.3.0.

#### 3.5 Version and license traps (rust-crates.md:319-335)

| Trap | Fix given |
|---|---|
| tui-textarea 0.7.0 needs ratatui ^0.29 | Use ratatui-textarea 0.9.3 |
| reqwest-eventsource 0.6.0 and jmap-base-client 0.3.0 need reqwest ^0.12 | sse-stream on reqwest 0.13 |
| oauth2 5.0.0's reqwest feature needs reqwest ^0.12 | A custom HTTP client, or hand-roll the exchange |
| ratatui-termina 0.1.0 needs termina ^0.3; the latest is 0.4.0 | Stay on crossterm |
| jmap-client 0.4.3's `websockets` needs tokio-tungstenite ^0.29; the latest is 0.30.0 | Only matters if jmap-client is adopted |
| ratatui-image's default `chafa-dyn` feature needs pkg-config and libchafa | `default-features = false, features = ["crossterm", "image-defaults"]` |
| reqwest 0.13 `rustls` uses aws-lc-rs, which has a C build step | Accept it, or try `rustls-no-provider` plus ring (unverified) |
| sieve-rs is AGPL-3.0-only | Do not link it |
| html2md (GPL-3.0+), osc52 (GPL-3.0-only), meli and melib (EUPL or GPL), mujmap (GPL-3.0) | Use the alternatives |
| nucleo and nucleo-matcher are MPL-2.0 | Fine unmodified (inferred) |
| MSRV outliers: eventsource-client 1.97, sqlx 1.94, skim 1.91, redb and fjall 1.90 | None are in the starter set. Direct deps declare at most 1.88 |

#### 3.6 Recommended starter set (rust-crates.md:337-407)

The doc's claim: the probe resolved, compiled, ran, passed an insta test and ran a divan bench on 2026-10-23 with rustc 1.99.1 (verified: rust-crates.md:339).

- **TUI:**
  - ratatui 0.30.2 (default features: crossterm, macros, layout-cache, underline-color, all-widgets);
  - crossterm 0.29.0 with `event-stream` and `osc52`;
  - ratatui-textarea 0.9.3;
  - tui-input 0.15.5;
  - tui-tree-widget 0.24.1.
- **Async and HTTP:**
  - tokio 1.54.1 with `rt-multi-thread`, `macros`, `sync`, `time` and `signal`;
  - futures 0.3.34;
  - reqwest 0.13.5 with `default-features = false` and `rustls`, `http2`, `json`, `stream`, `gzip`;
  - sse-stream 0.3.0;
  - serde 1.0.229 with `derive`;
  - serde_json 1.0.154.
- **Mail and HTML:** mail-parser 0.11.9, html2text 0.17.4, linkify 0.11.0.
- **Store:** rusqlite 0.40.2 with `bundled` (includes FTS5).
- **Pickers:** nucleo-matcher 0.3.1.
- **Config:** toml 1.1.8, etcetera 0.11.0, keyring 4.2.0, secrecy 0.10.3.
- **Misc:**
  - jiff 0.2.38 with `serde`;
  - anyhow 1.0.104, thiserror 2.0.21;
  - tracing 0.1.44;
  - tracing-subscriber 0.3.23 with `env-filter`;
  - tracing-appender 0.2.5;
  - open 5.4.5.
- **Add when needed** (versions checked, not built, except mail-builder):
  - mail-builder 1.0.0, ammonia 4.2.4, lol_html 3.0.2;
  - ratatui-image 11.1.0 (with `default-features = false`);
  - notify-rust 4.18.2, arboard 3.7.0, toml_edit 0.25.17;
  - clap 4.6.7 with `derive`;
  - tokio-util 0.7.20, tantivy 0.27.0;
  - tokio-tungstenite 0.30.0 with `rustls-tls-native-roots`.
- **Dev:** insta 1.49.0, divan 0.1.21.
- **Release profile:** `lto = "thin"`, `codegen-units = 1`, `strip = true`.
- **Left out on purpose:** jmap-client, sieve-rs, oauth2, chrono.

#### 3.7 Compile time and binary size (rust-crates.md:409-434)

| Build | Crates | Wall time | Binary |
|---|---|---|---|
| Starter probe, clean release, including downloads | 281 | 48.8 s (3 min 51 s CPU) | 14 MB |
| Incremental release after touching main.rs | 281 | 7.5 s | 14 MB |
| Clean debug, registry warm | 281 | 12.9 s | 185 MB, unstripped |
| Incremental debug | 281 | 0.5 s | |
| `opt-level = "s"` and `panic = "abort"`, clean | 281 | 25.7 s | 8.8 MB |
| Starter without keyring | 216 | 40.4 s | 11 MB |
| Starter plus jmap-client 0.4.3 | 304 | 49.2 s | 14 MB |
| Final starter set | 284 | not timed clean | 14 MB |

- **Machine:** 16 cores, 30 GB RAM, Linux x86_64. One run each (verified: rust-crates.md:411).
- **Duplicate versions in the tree:**
  - base64 0.22.1 (via crossterm's osc52) and 0.23.1 (via reqwest);
  - syn 2 and 3;
  - hashbrown 0.17.1, toml_datetime 1.1.2 and toml_parser 1.1.5 each appear twice at the same version, which is a feature split (inferred) (verified: rust-crates.md:426).
- **Notes:**
  - a workspace of a few crates keeps the 0.5 s edit loop;
  - most of the 14 MB is aws-lc-rs, bundled SQLite, html5ever, tokio, hyper and zbus, but only keyring was measured;
  - `opt-level = "s"` cut size about 37% but may slow hot loops;
  - ring, mold or lld, and nextest were not measured.

  (verified: rust-crates.md:428-434)

#### 3.8 Appendix: repo signals (rust-crates.md:436-595)

- **Columns:** one row per linked crate with Repo, Stars, Open (issues plus PRs), Last push and Archived, from `gh api` on 2026-10-23. Monorepo numbers repeat. Repos off GitHub are marked "not checked".
- **Archived:** directories-rs, fuzzy-matcher, robn/jmap-rs and meilisearch/milli (verified: rust-crates.md:595).
- **Rows relevant to spela** (verified: rust-crates.md:447, 463, 485, 529, 574, 591):
  - clap: 16,794 stars, pushed 2026-10-06;
  - etcetera: 153 stars, pushed 2026-08-04;
  - jiff: 2,958 stars;
  - ratatui: 23,058 stars, 205 open;
  - tracing: 6,940 stars, 900 open;
  - vt100: 123 stars, pushed 2026-10-19.

#### 3.9 Open questions in the doc (rust-crates.md:597-609)

Eleven questions. They cover:
- Fastmail capability probes;
- ManageSieve;
- the license;
- the permissive Sieve parsers;
- draft-14 against RFC 9661;
- FTS5 at 1M messages;
- ring against aws-lc-rs;
- OAuth registration;
- the html2text `css` feature;
- crossterm `KeyboardEnhancementFlags` names and terminal coverage;
- MSRV, with the resolution "`cargo +1.88 build`".

The generic ones for spela are the keyboard flags, MSRV and the TLS provider (only if spela downloads DLLs over HTTPS).

#### 3.10 Research picks against what mejl built (code)

| Item | rust-crates.md / architecture.md | As built (code) |
|---|---|---|
| Fuzzy matching | nucleo-matcher 0.3.1, Pick (rust-crates.md:239, 367; architecture.md:428) | Own matcher. "nucleo-matcher would be the obvious choice, but it is MPL-2.0 and mejl only takes permissive licences" (verified: crates/mejl-ui/src/fuzzy.rs:3-4). The workspace `# Pickers` section is empty (verified: Cargo.toml:53) |
| tui-input, tui-tree-widget | Pick (rust-crates.md:55-56, 347-348; architecture.md:420-421) | In no manifest (verified: grep of Cargo.toml and crates/*/Cargo.toml) |
| ratatui-textarea | "Test reference" (rust-crates.md:51) | Dev-dependency of mejl-ui (verified: crates/mejl-ui/Cargo.toml:22-23), but listed under `# TUI` in the workspace (Cargo.toml:26) |
| toml_edit, clap, ammonia, mail-builder, tokio-util | Later or add when needed | Used (verified: Cargo.toml:32, 42, 44, 58, 62; crates/mejl/Cargo.toml:29, 34) |
| signal-hook | Avoid as a direct dependency, latest 0.4.5 (rust-crates.md:299) | Used directly, `signal-hook = "0.3"`, locked at 0.3.18 (verified: Cargo.toml:76, crates/mejl/Cargo.toml:47, Cargo.lock:3465-3466) |
| vt100 for pty end-to-end tests | Later (dev) (rust-crates.md:298; architecture.md:435, 487) | Not used. Pty tests call `libc::openpty` and check escape sequences and termios (verified: crates/mejl/tests/terminal_pty.rs:1-12, 38-59) |
| Workspace layout | 4 to 5 crates (rust-crates.md:430; architecture.md:441) | 8 crates: jmap, fake, store, sync, sieve, render, ui, binary (verified: AGENTS.md:9-18) |
| Not in the research | | sha2 0.10, base64 0.22, getrandom 0.4, libc 0.2, bytes 1, mime_guess 2, tempfile 3, axum 0.8 (for the loopback fake), and pulldown-cmark 0.13.0. pulldown-cmark is declared at Cargo.toml:47, but no member manifest lists it and Cargo.lock has no entry (verified: grep) |
| Policy | MSRV and license listed as open decisions (docs/README.md:49, 52) | `rust-version = "1.88"`, `license = "MIT OR Apache-2.0"`, edition 2024 (verified: Cargo.toml:8-10) |
| Dev profile | not covered | `[profile.dev.package."*"] opt-level = 1` (verified: Cargo.toml:89-90) |

#### 3.11 Relevance to spela (inferred)

- **No YAML crate.** rust-crates.md does not cover YAML, and spela profiles and config are YAML (spela AGENTS.md, code style). The choice of YAML crate, and whether comment-preserving round trips are possible, needs a live crates.io check before any pick. Do not decide from memory.
- **Direct fits:**
  - etcetera matches spela's XDG rule;
  - tracing with tracing-appender replaces spela's centralized slog logging;
  - clap replaces cobra;
  - anyhow and thiserror cover error handling;
  - insta with `TestBackend` covers snapshot tests;
  - divan covers benches.
- **Likely YAGNI:** keyring and secrecy. spela has no credentials.
- **License.** The permissive-only stance in fuzzy.rs, and its exception for ammonia, is a decision spela needs to make explicitly.
- **Not covered at all:** NVML (for example nvml-wrapper), polkit and pkexec self-exec, Steam VDF parsing and sysfs access. Each needs its own research doc.

---

### 4. mockups.html: the visual language

#### 4.1 The file as an artifact

- **Self-contained page:**
  - a header that says these are design mock-ups, "not screenshots", drawn on "a fixed grid of monospace cells, the way ratatui draws a terminal frame" (verified: mockups.html:170-173);
  - theme and size buttons, and page keys `1` to `9` and `t` (verified: mockups.html:174-179, 1417-1430);
  - each grid is `role="img"` with an `aria-label` that includes "Grid of W by H cells" (verified: mockups.html:1358-1360);
  - a `<noscript>` note (verified: mockups.html:429);
  - localStorage wrapped in try/catch (verified: mockups.html:1401).
- **Grid model.** A `Grid` class keeps per-cell character, foreground class, background class and box-drawing bits (verified: mockups.html:472-536):
  - `put` clips at a pane limit and records an issue;
  - `hline`, `vline` and `box` merge lines into the right junction (┌ ┐ └ ┘ ├ ┤ ┬ ┴ ┼) through a bit mask (verified: mockups.html:436-442, 504-517);
  - `dimAll` applies the DIM attribute over everything already drawn (verified: mockups.html:526-528).
- **Self-check.** Run on load (verified: mockups.html:415-418, 1283-1296, 1366-1396):
  - every row must hold exactly W cells;
  - no text may be clipped;
  - only ASCII 0x20 to 0x7E, box drawing U+2500 to U+257F and the set ▸▾◦⚑✓✗↩↪⋯ may appear;
  - the rendered pixel width of each row must equal W times the cell width;
  - glyphs that fall back to a wider font are boxed into one cell (verified: mockups.html:1335-1351).

  The script also exports `BUILT` under Node, so the checks could run headless (verified: mockups.html:1439-1441).
- **"What these mock-ups cannot show":** speed, hover, drag and click, and the user's font and palette (verified: mockups.html:419-424).

#### 4.2 Colour tokens and type

- **The rule.** "The terminal palette maps one to one onto the 16 ANSI colours, so a real build can fall back to the user's own terminal theme." (verified: mockups.html:10-11)
- **Dark, the default** (verified: mockups.html:22-33):

  | Token | Hex | ANSI role |
  |---|---|---|
  | bg | #15171c | default background |
  | fg | #c8cdd5 | ANSI 7 |
  | bold | #eef0f3 | ANSI 15 |
  | dim | #666d79 | ANSI 8: borders, previews, hints |
  | red | #d98080 | ANSI 1 |
  | green | #93c393 | ANSI 2 |
  | yellow | #d8bb7e | ANSI 3 |
  | blue | #86aee0 | ANSI 4 |
  | magenta | #bb9fdc | ANSI 5 |
  | cyan | #80c4c4 | ANSI 6 |
  | cursor row | #252932 | ANSI 0 background; reverse video in the plain theme |
  | window chrome | #1f232b | page only |

- **Light** (verified: mockups.html:45-56): bg #fbfaf7, fg #30353d, bold #101318, dim #8b9099, red #b04848, green #3b7a3b, yellow #8a6716, blue #2c5f9e, magenta #7a4ca6, cyan #2a7a7a, cursor #ebe9e2, chrome #e7e5de.
- **Type.** A monospace stack at 12 px by default, adjustable from 9 to 16 px. Line height 1.25. Ligatures and contextual alternates off (verified: mockups.html:133-147, 1410-1415).
- **Cell style classes**, "only what a terminal can do (16 colours, bold, dim, underline, background)" (verified: mockups.html:150-163):

  | Class | Meaning |
  |---|---|
  | `b` | bold, bright foreground |
  | `d` | dim |
  | `acc` | blue |
  | `k` | blue bold: a key |
  | `brand` | background colour on a blue background, bold |
  | `red`, `grn`, `yel`, `mag`, `cyn` | the ANSI colours |
  | `u` | underline |
  | `cur` | cursor-row background |
  | `bc` | block cursor, reverse video |

- **Colour roles as used across the grids** (inferred from usage):
  - **green:** ok, synced, enabled, a passed check, a local result;
  - **yellow:** a warning, a prompt, the selection counter, "changed" markers, a server result, "unknown";
  - **red:** disabled, off, and the pin flag ⚑;
  - **blue:** focus, current item, active pane title, unread glyph, keys;
  - **cyan:** links, footnotes, chips, rule field names, Sieve tags;
  - **magenta:** the Masked Email feature badge;
  - **dim:** borders, previews, labels, hints;
  - **bold:** unread rows and emphasised values.

#### 4.3 Glyphs and legend (mockups.html:209-219, 1283)

| Glyph | Meaning |
|---|---|
| ◦ | unread; the row is also bold |
| ⚑ | pinned |
| @ | delivered to a mask |
| ✓ | selected, enabled or passed |
| ✗ | disabled |
| ▸ ▾ | collapsed or expanded, current item, menu |
| ⋯ | truncated or folded |
| ┃ | scrollbar thumb, drawn on a pane border |
| blue bold | a key; clickable in hint bars |
| `-` | deleted (masks screen), used without being in the legend (mockups.html:361) |

"State never depends on colour alone. Every state has a glyph or a word." The glyphs come from the Unicode Neutral width class, and a real build "ships an ASCII fallback set" (verified: mockups.html:205).

#### 4.4 Shared chrome (generic)

- **Frame.** Row 0 is the top bar. Rows 1 to h-3 are one outer box with a dim border. Row h-2 is the status line and row h-1 is the hint bar.
  - The box height is always h-3: box(0,1,160,37) at 160x40, box(0,1,120,33) at 120x36, box(0,1,80,21) at 80x24 (verified: mockups.html:718, 700, 1244).
- **Top bar** (verified: mockups.html:540-549):
  - " mejl " in `brand` (reverse video);
  - two spaces, then breadcrumbs: the account dim, three spaces, the location bold, then a dim count;
  - right-aligned context, ending one cell before the edge. Examples: "fastmail preset   ? help", "3 of 214", "draft in Drafts", "mejl block v7   last copied v6, Oct 20" (in yellow).

  The narrow layout drops the account and the preset (verified: mockups.html:1243).
- **Panes.** Vertical dividers are vlines inside the box, so they merge into ┬ and ┴. Pane titles sit in the top border at x+2 as " Title ", bold blue when focused and dim otherwise (verified: mockups.html:529, 719-723).
  - Sub-panels inside a pane use an hline with an embedded dim title, for example " Attachments " and " Sender " (verified: mockups.html:807, 811).
- **Scrollbar.** A ┃ thumb in the default foreground, drawn on the divider column (verified: mockups.html:566-568, 726).
- **Status line**, from column 1. The left side carries the transient message, prompt, toast or selection counter. The right side carries sync state, a position counter or a refresh age (verified: mockups.html:550-557). Examples:
  - "selection: 2" (yellow bold) with inline action keys; right: "✓ synced via JMAP push  14:02:31" (verified: mockups.html:728-729);
  - a prompt: a yellow bold question, then `y`/`N` keys (default shown capitalised), then follow-up keys; right: the pending key "@" dim (verified: mockups.html:818-820);
  - an undo toast: yellow text plus "z undo" (verified: mockups.html:1232);
  - a save toast: "✓ Draft saved 3 s ago (Drafts, via JMAP)", with a word count on the right (verified: mockups.html:886);
  - a pending prefix: right side "pending: g" (verified: mockups.html:1257).
- **Hint bar.** Starts at column 1. Each item is a key in blue bold, a space and a dim label, with two spaces between items (verified: mockups.html:558-565). Every screen sets its own hint list. "Below 20 rows the hint bar can be turned off" (verified: mockups.html:385).
- **Popups:**
  - clear the area, draw a box with an accent (blue) border, put the title at x+2 in bold blue, and right-align a dim hint in the top border (" : or Ctrl-k ", " Tab: next group ");
  - inner separators are accent hlines that merge into ├ ┤;
  - the cursor row is `cur`-shaded with a ▸ marker (verified: mockups.html:916-943, 857-884, 1259-1277);
  - modal popups dim the whole screen behind them; which-key does not (verified: mockups.html:913, 1256).
- **Forms.** Label and value rows with fixed-width dim labels: 9 cells in compose and the rule editor, 10 in the mask details, 14 in the paste-status box (verified: mockups.html:833, 1094, 1222, 1085).
- **Lists:**
  - one item per row, no blank rows;
  - dim group-header rows ("Today", "Yesterday", "This week", "Earlier");
  - a fixed 6-cell, right-aligned date column "so rows never jitter": a time for today, a weekday this week, "Oct 3" this year;
  - truncation with ⋯;
  - a `cur`-shaded cursor row.

  (verified: mockups.html:233, 446-456, 641-663)

#### 4.5 Screen by screen

1. **Inbox, wide, 160x40** (verified: mockups.html:715-733, 224-249).
   - **Layout.** Dividers at x=23 and x=97. Sidebar titled " Mailboxes " (dim), list " Inbox " (bold blue), reader preview " Re: ... " (dim).
   - **Sidebar.** Names with a 4-cell right-aligned count. The active item gets ▸ in blue, bold blue text and a shaded row. Dim headings "Folders", "Saved searches", "Tools". Tool rows show their key, such as `g r`, in key style.
   - **List row.** Optional select column (✓, yellow bold), unread ◦ (blue bold), pin ⚑ (red), mask @ (magenta), a space, then a 20-cell sender with "(n)", the subject (bold if unread) plus a dim preview when it fits, and the 6-cell date. The thumb covers rows 2 to 12.
   - **Reader pane:**
     - a dim summary line;
     - collapsed messages "▸ name(16) date(9)" with a dim preview;
     - the expanded message "▾ Name <addr>" with the time right-aligned and bold, and a dim "to" line;
     - a wrapped body with a cyan [1] footnote;
     - "⋯ 12 quoted lines (") " with `"` in key style;
     - "⋯ signature, 3 lines";
     - the footnote URL in cyan underline;
     - an action-hint row.
   - **Status:** "selection: 2". **Hint bar:** 14 items.
   - **Captions** give the breakpoints: three panes at 160 columns or more, sidebar as an overlay at 100 to 159, one stack under 100. Mouse: click to preview, double click to open, wheel, drag borders to resize (the ratio is saved), right click for the actions menu.
2. **Reader with a side panel, 120x36** (verified: mockups.html:737-824, 252-274).
   - **Layout.** Divider at x=83. Main pane titled with the subject; the side panel titled " Delivered to a masked address " in magenta.
   - **Body:**
     - a dim info line with message count, attachment count and label;
     - collapsed rows (name 20, date 13);
     - a recipient line with the mask address in magenta;
     - a notice line in yellow, "remote images off, 2 trackers blocked", with "O open in browser" right-aligned;
     - the body indented 6 cells, with a bold heading, a "[ Track your parcel ]" button in cyan underline, and a table with a bold header row, dim ─ rules and right-aligned numbers;
     - the quote fold and footnotes.
   - **Side panel:**
     - label and value rows: site, state "✓ enabled" in green, created, last, mail;
     - a check line in green ✓;
     - action rows: the key padded to 3 cells, two spaces, the label;
     - the sub-panels Attachments and Sender.
   - **Status line** is a confirm prompt with follow-up keys.
3. **Compose, 120x36** (verified: mockups.html:828-890, 276-296).
   - **Header form.** Fields on rows 2 to 5 with dim 9-cell labels. The focused field row is shaded. Placeholder text is dim. A full-width hline merges into ├ ┤ at rows 6 and 30. The body starts at column 4 with a block cursor. Attachment and format rows sit at the bottom with key hints.
   - **From picker popup,** 72x15 at (46,2):
     - title "Send as", right hint " Tab: next group ";
     - group headings in dim;
     - options with right-aligned dim tags (default, alias, identity);
     - a highlighted "▸ + New masked email for ..." with `^o m` on the right;
     - an inline form field ending "(required)" in dim;
     - explanatory dim lines;
     - a yellow "! Sending as a mask over JMAP is not yet tested".
   - **Hint bar** is all `^o` chords plus Tab and Esc.
4. **Command palette, 120x36** (verified: mockups.html:911-947, 298-321).
   - **Base.** The 120-column mail layout (list, divider at x=58, reader), fully dimmed.
   - **Popup,** 84x16 at (18,4), titled "Commands":
     - an input row "> new▮";
     - 8 rows, each with a ▸ marker, the name with matched letters in yellow bold, a key column (8 cells at px+56) and a dim kind column (14 cells at px+66);
     - a footer: "Each row shows its key, so the palette teaches the shortcuts."
   - **Status:** "8 of 312 commands match "new"". Ranking mixes the fuzzy score with use frequency (caption).
5. **Search, 120x36** (verified: mockups.html:951-1004).
   - A query row with a block cursor and "Fastmail search syntax" right-aligned.
   - A "Filters" row of cyan chips, plus "Tab edit chips".
   - The exact JMAP filter shown verbatim, "so the query is never a black box".
   - A result summary: "5 local in 3 ms" (green), then "3 more from the server" (yellow), with the time in dim.
   - A results table whose source column is coloured local or server.
   - A syntax-mapping table under a titled hline.
   - Footnote: "a result set is never silently wider than asked".
6. **Rules manager, 160x40** (verified: mockups.html:1022-1170, 323-351).
   - **Banner** on rows 2 and 3: "! Paste required." in yellow bold.
   - **Left column** (x 1 to 39):
     - the rule list: ▸, state glyph (✓ green or ✗ red), name in 28 cells, and a 4-cell tag (`*` changed in yellow bold, `off` dim);
     - a legend and key hints;
     - sub-panels: Fastmail UI rules (explanation), Where the block runs (an 8-row order with mejl's row highlighted, "Order from samples, unconfirmed"), and Paste status (local block, last copied, "on Fastmail: unknown" in yellow).
   - **Right top, "Edit rule":**
     - Name, with "✓ enabled Space toggle";
     - When "any of these match";
     - condition rows: field in cyan, operator dim, value, with the focused row shaded;
     - "+ add condition";
     - Then (the actions);
     - Stop, with a yellow warning;
     - Runs;
     - Checks: a row of green ✓ items.
   - **Right bottom, split at x=101:**
     - left, "Sieve preview, read-only": syntax highlighted (strings green, tags cyan, keywords blue, comments dim) with gutter ▸ on the focused rule's lines;
     - right, "Dry run": the target and size, "14 would match" in green bold, match rows "date sender subject -> Receipts", "⋯ 6 more", and the actions apply and test, with a dim note on what is not simulated.
   - **Status:** "Rule saved locally. 2 changes are not on Fastmail yet."
7. **Masked Email manager, 120x36** (verified: mockups.html:1174-1237, 353-374).
   - **Stats row** of coloured counts.
   - **Filter row:** "Show 1 ✓ enabled 2 ✓ disabled 3 ✓ pending 4 ✓ deleted", plus silent, created-by and sort settings.
   - **Search row** with a right-aligned note on the default view.
   - **Table** with a dim header: state 10, address 30, description 19, site 21, last 7 (right-aligned), created by 11, mail 6 (right-aligned). State is glyph plus word, coloured. The address is magenta, or dim when deleted or pending. "(no description)" is dim. A yellow `!` flag sits at column 116.
   - **Detail panel** titled with the selected address, with 10-cell labels.
   - **Status:** an undo toast. Right side: "refreshed 2 min ago (full MaskedEmail/get)".
8. **Narrow, 80x24** (verified: mockups.html:1241-1251, 376-393).
   - Single pane, sender 20 cells, no preview, no select column. The scrollbar is on the right border.
   - Status right side: "1 of 214". Hint bar of 9 items, with ":" shortened to "cmds".
   - Captions: the sidebar hides behind `g` or a click on "Inbox ▾". Enter opens the reader full width. The select column shows only while a selection is active.
9. **Which-key, 120x36** (verified: mockups.html:1255-1279, 395-413).
   - **Base.** The 120-column layout, not dimmed.
   - **Popup,** 50x16 at (70,17), flush with the right border, titled "g  go to":
     - two columns of key plus 18-cell label, at px+2 and px+25;
     - a dim group label "mejl tools";
     - a footer: "Esc cancel   no timeout: g does nothing alone" and "Same popup for . mark * select ^o compose".
   - **Hint bar:** "g go to: press a key  Esc cancel  ? all keys".
   - **Rule:** a key that is not listed cancels and does nothing else.

#### 4.6 Not mocked: settings, key editor, help overlay

- **Nothing mocked.** The page has exactly nine grids. The nav and the screens list name inbox, reader, compose, palette, search, rules, masked, narrow and whichkey (verified: mockups.html:183-196, 226-397, 1298-1308).
- **Settings** appears only as a sidebar tool entry and a which-key target (verified: mockups.html:616, 1274).
- **Prose descriptions only:**
  - the settings screen is "read-mostly view of effective config and detected terminal capabilities ... 'probe terminal again'" (verified: terminal-ux.md:498);
  - the key editor "works like the controls screen of a game": duplicate a preset, rebind, swap or replace on a clash, prefix-breaking keys refused, macros section (verified: README.md:125-156; docs/README.md:152, 154);
  - help is "Bindings for the current context, filterable, grouped" (verified: terminal-ux.md:497).
- **Code locations,** not reviewed here: crates/mejl-ui/src/settings.rs, keys_screen.rs, view/keys.rs, update/keys.rs and keymap/edit.rs (verified: file listing).
- **As-built semantic style names** (code, not the mockup; verified: crates/mejl-ui/src/theme.rs:60-121). There are 62 names. Generic families:
  - `text`, `dim`;
  - `app.brand`, `app.mailbox`, `app.info`;
  - `pane.border`, `pane.title`, `pane.title.focused`;
  - `scrollbar.thumb`;
  - `sidebar.item`, `sidebar.active`, `sidebar.count`, `sidebar.heading`, `sidebar.cursor`;
  - `list.row`, `list.unread`, `list.marker.{unread,pinned,masked,selected}`, `list.preview`, `list.date`, `list.date.unread`, `list.group`, `list.cursor`, `list.skeleton`;
  - `status.text`, `status.selection`, `status.ok`, `status.warning`, `status.error`;
  - `hint.key`, `hint.label`;
  - `popup.border`, `popup.title`, `popup.group`, `popup.cursor`;
  - `help.match`.

  The `reader.*` family (quote depths 1 to 4, link, footnote, notice, masked and others) is mostly email specific. The spec states the reason for semantic names: "so a theme cannot accidentally lose a state distinction" (verified: terminal-ux.md:603).

#### 4.7 Generic chrome against email-specific content, mapped to spela (mapping inferred)

| Element | Generic or email | spela use (inferred) |
|---|---|---|
| Top bar, framed panes, focused title in bold accent, status line, hint bar, scrollbar on the border | Generic | Same frame for Library, Details and Actions |
| Which-key popup with no timeout; palette with key and kind columns over a dimmed screen | Generic | Command palette over games, profiles and actions; prefixes like `g` |
| Status-line prompt with y/N and an undo toast | Generic | Confirm a DLL swap or restore; undo a profile change |
| Picker popup with groups, tags and inline create | Generic pattern | Pick a DLL version or a preset profile |
| Stats row, numeric state filters 1 to 4, table with a state glyph and word, detail panel | Generic pattern, email content | DLL or profile table with states (original, swapped, backup, update available) |
| Checks row, dry-run panel, "paste status" drift box, banner for a manual step | Generic pattern | Apply-profile preview before the pkexec batch: what changes, what was checked, "applied" against "profile" drift |
| List rows (sender, subject, preview), reader, quotes, footnotes, compose fields, Sieve preview, mask semantics, JMAP sync text, search-syntax mapping | Email | Not transferable as content |
| Breakpoints (160, 100, 80x24), fixed date column, ⋯ truncation, safe glyph set, self-checking grid | Generic | spela's own TUI checks run at 80x24 and 120x40 (spela AGENTS.md) |

---

### 5. AGENTS.md, README, justfile, .claude and .github

#### 5.1 AGENTS.md (37 lines)

- **Header.** A one-line product statement. It points to `docs/README.md` and `docs/architecture.md`, and makes `docs/mockups.html` "the visual target" (verified: AGENTS.md:1-5).
- **Layout table.** One row per crate with its role (verified: AGENTS.md:7-18).
- **Commands** (verified: AGENTS.md:20-25):
  - build;
  - the gate: `cargo fmt --all --check && cargo clippy --workspace --all-targets -- -D warnings && cargo test --workspace`;
  - demo: `cargo run -- --demo`;
  - MSRV: `cargo +1.88 check --workspace`.
- **Hard rules** (verified: AGENTS.md:27-37):
  1. Never call the `mcp__claude_ai_Fastmail__*` tools; they act on the real mailbox. These tools are loaded in this very session, so the rule guards a live risk (verified: session tool list).
  2. No code, test or tool call may contact a Fastmail host. Tests use `mejl-fake`. Live checks are run only by the user.
  3. The user's email address never appears in Cargo.toml, the User-Agent, fixtures, config defaults, comments or tool calls. The User-Agent is `mejl/<version>`.
  4. Tests never touch the real OS keyring or the real XDG config and data dirs. They use temp dirs and in-memory secret stores.
  5. Never call the private `dev/rules` API.
  6. Rendered Sieve never contains `keep` or `discard`, uses only allowlisted extensions, and escapes `"` and `\`.
  7. The UI thread never awaits and never blocks on the network.
  8. Docs and comments: plain words, sentence-case headings, no em dashes.
  9. Inline `#[cfg(test)] mod tests` blocks go last in their file, for the CodeQL filter.

#### 5.2 How these rules keep agents safe

- **Absolute and checkable.** Each rule is phrased so a reviewer can test it, and several are backed by tests:
  - Sieve golden tests for "never keep or discard" (verified: architecture.md:488);
  - "Doctor output contains no token" (verified: plan:166);
  - "The binary test sandbox still never touches the keyring" (verified: plan:168).
- **Fake-first.** A fake server with quirk switches stands in for every external system (verified: architecture.md:481; plan:54-65, 120-124). The plan turns "no network" into a mechanical check: "Grep the new tests for `transport: None` before handing off." (verified: plan:125)
- **Live checks belong to the user.** The plan ends with "Live, by you only" and names the open questions the live run settles (verified: plan:170). The README marks OAuth as "Built and tested against the fake only" (verified: docs/README.md:153).
- **Research-time safety** follows the same rules (verified: docs/README.md:56-62; fastmail-platform.md:3; fastmail-rules-sieve.md:23, 523, 525):
  - read-only probes with a temporary token;
  - writes only on a trial account;
  - no MCP;
  - redact session output.
- **The pty tests never touch the developer's terminal.** Each child gets a fresh pty as its stdin, and the one scenario that opens `/dev/tty` first makes the pty its controlling terminal (verified: crates/mejl/tests/terminal_pty.rs:1-12).
- **Mapping to spela** (inferred):
  - never run pkexec or sudo in tests;
  - fake NVML and fake sysfs;
  - temporary XDG dirs and a temporary Steam root;
  - no network in tests (DLL downloads faked);
  - the anti-cheat denylist as a tested invariant;
  - the UI thread never blocks on NVML, pkexec or filesystem scans;
  - only the user runs a privileged apply.

  spela's AGENTS.md already forbids approving privileged or destructive actions against real game files during visual review (spela AGENTS.md, TUI testing section, session context).

#### 5.3 The CodeQL test filtering rule

- **Workflow.** Matrix languages `rust` and `actions`, `build-mode: none`, `upload: never`, SARIF output. Then `python3 .github/codeql/filter-tests.py sarif`, then `upload-sarif`. It replaces GitHub's default setup, "which cannot leave tests out". Runs on push and PR to main, plus a weekly cron "17 4 * * 1". Permissions are minimal (verified: .github/workflows/codeql.yml:1-45).
- **Config.** `paths-ignore` covers `crates/*/tests/**` and `crates/*/benches/**` (verified: .github/codeql/codeql-config.yml:6-8).
- **Filter.** It drops a result if any of its locations is under `crates/<crate>/(tests|benches)/`. It also drops a result in a `.rs` file at or after the first `#[cfg(test)]` line whose next non-blank line matches `mod <name> {` (verified: .github/codeql/filter-tests.py:17-18, 21-51).
- **Why the "tests last" rule exists.** If an inline test module sat mid-file, every production line after it would be silently dropped from CodeQL (inferred from filter-tests.py:47-50).
- **Gap.** `#[cfg(test)] mod tests;`, with the tests in a separate file, does not match the `{` pattern. The separate files live under `src/`, not `tests/`. Two such modules exist (verified: crates/mejl-ui/src/keymap/mod.rs:802-803, crates/mejl-ui/src/compose/editor.rs:1230-1231). So CodeQL results inside crates/mejl-ui/src/keymap/tests.rs and compose/editor/tests.rs are not filtered (inferred). The effect is extra noise in the alerts, not a hidden production alert (inferred). AGENTS.md:37 does not mention this form.

#### 5.4 justfile (verified: justfile:1-64)

- **Recipes:** default (list), build, release-build (`--release --locked -p mejl`), run, demo, test, fmt, lint (clippy with `-D warnings`), msrv, check (the gate), keys, release.
- **msrv** is `cargo +1.88 check --workspace --all-targets`.
- **release `<version>`** depends on `check`. It refuses when the Cargo.toml version differs or the tree is dirty, then tags `v<version>` with an annotation and pushes the tag.

#### 5.5 Release workflow (verified: .github/workflows/release.yml:1-107)

- **Trigger:** a tag `v[0-9]+.[0-9]+.[0-9]+*`, or `workflow_dispatch` with an existing tag.
- **Check job:**
  - the Cargo.toml version must equal the tag;
  - fmt;
  - clippy with `--locked` and `-D warnings`;
  - test with `--locked`;
  - no MSRV check.
- **Build matrix:** x86_64-unknown-linux-gnu, aarch64-unknown-linux-gnu (ubuntu-24.04-arm) and aarch64-apple-darwin. Each tars the binary with the README and licenses, plus a sha256.
- **Publish:** `gh release create` with `--generate-notes` and `--verify-tag`.
- **For spela** (inferred): spela is Linux-first, so the darwin target would be dropped.

#### 5.6 README.md and .claude/

- **README.md is user-facing** (verified: README.md:1-166):
  - install from source (Rust 1.88);
  - demo mode against a local fake, with no internet and temp files deleted on quit;
  - login options;
  - the `doctor`, `keys --check` and `logout` commands;
  - a commented config example;
  - the settings screen writes `[ui]` back and keeps comments (README.md:102);
  - `NO_COLOR`;
  - keys and presets;
  - the key editor, which works "like the controls screen of a game";
  - macros;
  - rules;
  - the licence.
- **Demo mode** is a safety and onboarding device spela could copy, for example a demo library on fake data (inferred).
- **.claude/** holds only `scheduled_tasks.lock`, a session lock with a sessionId, pid and timestamp (verified: .claude/scheduled_tasks.lock:1). It has no settings, agents or commands. All agent guidance lives in AGENTS.md.

### Conflicts and mismatches

- **Fuzzy matcher pick versus what was built.** rust-crates.md and architecture.md pick nucleo-matcher 0.3.1. The code has its own matcher and rejects nucleo-matcher for being MPL-2.0. architecture.md contradicts itself: line 459 says the matcher is mejl-ui's own, while line 428 and disagreement row 16 say nucleo-matcher. Evidence: rust-crates.md:239, 367; architecture.md:428, 459, 594; crates/mejl-ui/src/fuzzy.rs:3-4; Cargo.toml:53 (empty # Pickers section)
- **Permissive-only licence stance has an unstated exception.** fuzzy.rs says mejl "only takes permissive licences", yet ammonia's two MPL-2.0 crates were accepted by a user decision. rust-crates.md calls MPL-2.0 fine as an unmodified dependency. No single written licence policy exists. Evidence: crates/mejl-ui/src/fuzzy.rs:3-4; docs/README.md:147; rust-crates.md:239, 334
- **tui-input and tui-tree-widget picked but never built.** Both are Pick in the starter set and are listed in architecture crate picks. Neither appears in any manifest. Evidence: rust-crates.md:55-56, 347-348; architecture.md:420-421; Cargo.toml:23-28; grep of crates/*/Cargo.toml
- **signal-hook: avoid as a direct dependency, but used directly at an older version.** rust-crates.md says to avoid direct use and lists 0.4.5 as the latest. The workspace declares 0.3, locked at 0.3.18, and the binary crate uses it directly. Evidence: rust-crates.md:299; Cargo.toml:76; crates/mejl/Cargo.toml:47; Cargo.lock:3465-3466
- **vt100 end-to-end harness planned, raw openpty built.** The docs plan vt100 to read the screen in pty tests. The as-built pty tests use libc::openpty and check escape sequences and termios. No manifest has vt100. Evidence: rust-crates.md:298; architecture.md:435, 487; crates/mejl/tests/terminal_pty.rs:1-12, 38-59
- **Workspace layout.** architecture.md plans 5 crates and rust-crates.md about 4. The built workspace has 8 crates, including a fake server, a sync engine and a render crate. Evidence: architecture.md:441; rust-crates.md:430; AGENTS.md:9-18
- **pulldown-cmark declared but unused.** The workspace declares pulldown-cmark 0.13.0, and architecture.md lists 0.13.5 as a markdown compose option. No member crate lists it and Cargo.lock has no entry, so markdown compose may not use it, or may not exist as described. Evidence: Cargo.toml:47; architecture.md:437; grep of crates/*/Cargo.toml and Cargo.lock (no match)
- **Synthesis doc claims no new research but carries later as-built content under the old date.** architecture.md is dated 2026-10-23 and "adds no new web research", but holds "As built (2026-10-25)" sections. rust-crates.md says every value was fetched 2026-10-23 but contains as-built edits. Header dates were not revised. Evidence: architecture.md:3, 257, 396; rust-crates.md:5, 9, 51
- **Decisions still listed as open after they were made.** docs/README.md lists licence and MSRV as decisions the user needs to make. The workspace already sets MIT OR Apache-2.0 and rust-version 1.88, and the README states both. Evidence: docs/README.md:45-52; architecture.md:616; Cargo.toml:9-10; README.md:7, 164-166
- **Settings screen scope.** terminal-ux.md specs a read-mostly settings view. The built settings screen writes config back and hosts the key editor and macros. Evidence: terminal-ux.md:498; README.md:102, 125-156; docs/README.md:151-154
- **Mockups are the visual target but miss built screens.** AGENTS.md makes mockups.html the visual target. It has no settings screen, key editor or help overlay, all of which were built. Evidence: AGENTS.md:5; mockups.html:183-196, 1298-1308; docs/README.md:152
- **Which-key timing.** The mockups decide that the popup appears at once with no timeout. Open question 38 and terminal-ux still treat the popup delay as undecided. Evidence: mockups.html:402, 1276; docs/README.md:127; terminal-ux.md:622
- **MSRV check command and CI coverage.** AGENTS.md checks MSRV without --all-targets, and the justfile and plan include it. The release workflow runs no MSRV check at all. Evidence: AGENTS.md:25; justfile:35-37; docs/plans/clever-dreaming-donut.md:169; .github/workflows/release.yml:42-44
- **Fact-check coverage is uneven.** Only the three Fastmail docs have adversarial fact-check sections. email-clients.md, rust-crates.md and terminal-ux.md have none, though their findings feed the top-10 list and the architecture. Evidence: grep counts: fastmail-platform.md 3, fastmail-rules-sieve.md 7, fastmail-masked-email.md 3, others 0; docs/README.md:32-43
- **CodeQL filter misses out-of-line test modules.** filter-tests.py only recognises '#[cfg(test)]' followed by 'mod name {'. Two modules use 'mod tests;' with the tests in separate files under src/, which match neither the tests/ path rule nor the inline rule. AGENTS.md:37 does not cover this form. Evidence: .github/codeql/filter-tests.py:17-18, 30-35; crates/mejl-ui/src/keymap/mod.rs:802-803; crates/mejl-ui/src/compose/editor.rs:1230-1231; AGENTS.md:37
- **Plan names two token endpoints for the fake.** The fake design bullet serves '/oauth/token'. Review rule 5 says the fake serves the token endpoint at '/oauth/refresh', as Fastmail does. The pre-review text was left in place. Evidence: docs/plans/clever-dreaming-donut.md:18, 61, 121
- **Plan's doc step versus the README.** The plan says to mark open questions 3 and 17 'implemented, untested live' in docs/README.md. The question rows are unchanged. The status section mentions it instead. Evidence: docs/plans/clever-dreaming-donut.md:138; docs/README.md:70, 91, 153
- **OAuth guidance in rust-crates.md is stale.** rust-crates.md says Fastmail registers clients by hand and day one should use an API token. architecture.md resolves this as disagreement row 1, and OAuth with open registration was built. The rust-crates.md text was not updated. Evidence: rust-crates.md:261, 606; architecture.md:381, 579; docs/README.md:153
- **Snapshot widths versus mock-up widths.** The testing strategy snapshots every screen at 80, 120 and 200 columns. The mock-ups and the three-pane breakpoint are at 160 columns. Evidence: architecture.md:485; mockups.html:231, 716
- **Copy key inside the masks UI.** The reader moved 'copy address' to the global Y menu because y answers the disable prompt. The masks screen still binds y to copy. Masked-email.md suggested u to restore; the mockups use z. Evidence: mockups.html:269, 369, 1229-1230, 1234; fastmail-masked-email.md:415, 436; architecture.md:588
- **Deleted-state glyph missing from the legend.** The masks screen uses '-' for deleted, which the glyph legend does not list. Pending reuses the unread glyph ◦. Evidence: mockups.html:209-219, 361, 1200

### Open questions

- Does spela's research use the full mejl doc set (dated research docs, an index README with consolidated open questions and a decisions table, a synthesis doc, a self-checking mockups page, plans), and which topics get their own doc (NVML in Rust, polkit and pkexec self-exec, Steam VDF and appmanifests, DLSS and DLL sources, anti-cheat denylist, sysfs governor and SMT, Rust crates, terminal UX, a Go behaviour inventory as the parity spec)?
- Must every spela research doc get an adversarial fact-check pass? mejl did this for only 3 of 6.
- Profiles and config: keep YAML for compatibility with existing user files, or move to TOML like mejl? If YAML, which Rust crate, and is comment-preserving write-back required? This needs a live crates.io check; rust-crates.md does not cover YAML.
- Licence policy: permissive only (which rules out MPL-2.0 nucleo-matcher, as mejl's code does) or MPL allowed (as mejl allowed for ammonia's dependencies)? What licence will the Rust spela use?
- Should spela make a mockups.html with mejl's grid self-check before building the TUI? Should it include the screens mejl never mocked (settings, key editor, help), given spela already has a Settings screen?
- Default keymap: keep the current Go TUI keys for parity, add which-key popups and a palette on top as mejl did, or follow mejl's keys where they overlap?
- Are mejl's key editor, presets and macros in scope? They are not in the Go spela, so YAGNI says no unless the user wants them.
- Mouse on by default, as mejl does, with hit testing and a key for every clickable label?
- Adopt mejl's latency budgets (local action p99 under 8 ms, warm start under 50 ms) and a perf overlay with bucketed measurements for spela?
- Adopt CodeQL with test filtering? If yes, fix the 'mod tests;' gap, for example by matching 'mod name;' too or by banning out-of-line test modules.
- For the rewrite, should mejl's as-built code (Cargo.toml, crate layout, own fuzzy matcher, raw openpty pty tests) be the reference rather than rust-crates.md and architecture.md picks where they differ?
- Live check: does mejl's mockups.html grid check pass in a browser today? It was not opened in this read-only pass.
- Live check: does CodeQL actually report and keep alerts in crates/mejl-ui/src/keymap/tests.rs and compose/editor/tests.rs? Only a workflow run can show it.
- Should spela copy mejl's demo mode (fake data, nothing written to real dirs, temp files deleted on quit) as a safe way to try the TUI and to run visual reviews?

## mejl-ui as built: the UI layer

Paths are relative to `/home/jgabor/git/mejl/crates/mejl-ui/` unless they start with `crates/`, `docs/` or `.github/` (then they are relative to `/home/jgabor/git/mejl/`) or with `spela/` (relative to `/home/jgabor/git/`). Each claim carries a label: (verified: path:line), (inferred) or (unverified). "The doc says" and "the code does" are kept apart. Every doc, test or code mismatch is listed in the conflicts output and repeated briefly at the end.

### Size, dependencies and boundary

- The crate has 38,861 lines: src 32,025 in 53 files, tests 5,478 in 17 files (16 test files plus `tests/common/mod.rs`), 37 snapshot files with 1,334 lines, and `Cargo.toml` with 24 lines (verified: wc).
- Runtime dependencies are mejl-render, mejl-sieve, serde_json, ratatui, crossterm, unicode-width, unicode-segmentation, toml, jiff and thiserror. Dev dependencies are ratatui-textarea and insta (verified: Cargo.toml:10-24). The crate has no tokio, no file or network crates, no toml_edit, no nucleo, no tui-input and no tui-tree-widget (verified: Cargo.toml; grep over all Cargo.toml files found none of the last four).
- The only crate that depends on it is the binary `crates/mejl` (verified: crates/mejl/Cargo.toml:24).
- The crate doc says it "does no I/O: it never awaits, never touches the network and never writes files" (verified: lib.rs:4-7). The code bears this out: no file, network or process calls in src (inferred from the dependency list and from reading). The one exception reads the environment: `theme::no_color_from_env` reads `NO_COLOR`, and only the shell calls it (verified: theme.rs:681-686).
- These live in the shell, not in this crate: the config writer that uses toml_edit (verified: crates/mejl/src/app/settings.rs), the `ui.mouse` setting with default true (verified: crates/mejl/src/config.rs:229,243), `doctor` and demo mode (verified: crates/mejl/src/cli.rs:480,905-928; crates/mejl/src/doctor.rs), and the theme `auto:` prefix (verified: crates/mejl/src/config.rs:703).

### The contract: Event, Cmd and MailData

The doc says the UI follows the Elm architecture with three types (verified: lib.rs:11-44):

```text
Event -> update(&mut AppState, &dyn MailData, Event) -> Vec<Cmd> -> shell
         render(&AppState, &dyn MailData, &mut Frame)
Msg <- results from sync, store and workers
```

#### Message types and their real names

- `Event` (verified: event.rs:39-53) has six variants: `Key(KeyEvent)`, `Mouse(MouseEvent)`, `Paste(String)`, `Resize(u16,u16)`, `Focus(bool)` and `Msg(Msg)`. `Event::from_crossterm` drops key releases (verified: event.rs:58-69). update also ignores releases (verified: update.rs:58-63).
- `Msg` (verified: event.rs:74-173) carries results from outside: `Sync`, `Push`, `DataChanged(DataChange)`, `BodyFetched`, `BodyFailed`, `BodyRendered`, `Toast`, `Error`, `MaskedAccess`, `Outbox`, `Perf(FrameSample)`, `Tick`, `DraftSaved`, `Send`, `EditorDone`, `Attached`, `ServerSearch`, `MaskedCreated`, `Config(Box<UiConfig>)` and `SettingRefused`.
- `Cmd` (verified: event.rs:399-554) holds every request to the outside world:
  - reading and rendering: `FetchBody`, `Prefetch`, `RenderBody`
  - changing mail: `Mutate(Mutation)`
  - opening, copying and syncing: `OpenUrl`, `CopyToClipboard`, `OpenInBrowser`, `OpenAttachment`, `RefreshNow`
  - app control: `SaveLayout`, `WakeAt`, `Quit`, `Suspend`, `SendNowAndQuit`
  - compose: `SaveDraft`, `DeleteDraft`, `Send`, `CancelSend`, `OpenEditor`, `AttachFile`
  - search and mailboxes: `ServerSearch`, `SaveSearch`, `CreateMailbox`
  - masked addresses: `MaskedCreate`, `MaskedSetState`, `MaskedSetDescription`
  - rules: `SaveRule`, `DeleteRule`, `ReorderRules`, `RecordRuleRender`, `ConfirmRulePaste`
  - settings and keys: `SaveSetting(Setting)`, `EditKeys(KeysEdit)`

  The shell answers each one asynchronously, never by calling back into the UI. lib.rs tabulates which `Msg` answers which `Cmd` (verified: lib.rs:82-112).
- `Action` (verified: action.rs:65-231) has 127 named commands, generated by a `macro_rules!` that emits `name()` (kebab-case config and palette name), `label()` (hint and which-key text), `description()` (help text) and `category()`. There are 14 `Category` values for grouping (verified: action.rs:12-63). `Action::from_name` and `Action::suggest` give "did you mean" hints by edit distance (verified: action.rs:234-247). `takes_count` limits vim counts to motions, and `is_repeatable` limits `.` repeat to triage changes (verified: action.rs:249-284). A unit test checks that names are unique and kebab-case (verified: action.rs:312-326).
- `Binding` is either `Action(Action)` or `Macro(MacroId)` (verified: keymap/mod.rs:41-46).
- `Mutation` (verified: event.rs:666-718) carries `Undo` and `Redo` as plain values. The undo stack lives in the shell's store, not in the UI. update pushes `Cmd::Mutate(Mutation::Undo)` (verified: update.rs:412-420). The test fake keeps its own snapshot stack (verified: data/memory.rs:162-163, 558-588).
- `Setting` (verified: event.rs:263-282) and `KeysEdit` (verified: event.rs:179-222) are the requests the settings screen and the key editor send to the shell.

#### The optimistic contract

The doc describes it (verified: event.rs:9-22; lib.rs:35-44). A triage key yields `Cmd::Mutate`. The shell applies it to the local store and feeds `Msg::DataChanged` back into update before it draws. update then re-reads the visible page. The UI keeps no copy of mail state beyond small caches (verified: state.rs:3-6).

#### update: dispatch, purity and output

- Signature: `pub fn update(state: &mut AppState, data: &dyn MailData, event: Event) -> Vec<Cmd>` (verified: update.rs:54).
- After dispatching any event, update always calls `state.refresh(data, &mut cmds)` (verified: update.rs:74). Refresh does several things (verified: update.rs:1673-1740):
  - it reloads mailboxes, the sidebar, identities and the account when they are dirty
  - it checks that the current mailbox still exists and falls back to the Inbox
  - it runs the draft autosave and the delayed mark-read
  - it clears expired toasts
  - on the mail screen only, it fixes focus for the layout, pages the list, prefetches bodies in the direction of travel and rebuilds the reader document
- Key path order in `on_key` (verified: update.rs:194-262):
  1. The suspend key (any global binding to `Action::Suspend`, default `ctrl-z`) wins before anything else (verified: update.rs:195-201, 264-276).
  2. The top overlay takes the key. Confirm accepts `y`/`Y`, and any other key cancels. CaptureKey records raw keys. Perf closes on any key. Help, Picker and Prompt have their own handlers (verified: update.rs:202-235).
  3. Screen text handlers run next: compose, search, masked, rules and keys (verified: update.rs:236-247).
  4. `?` during a pending prefix opens help (verified: update.rs:248-254).
  5. `Keymap::feed` runs with `state.contexts()`. `Resolution::Action` calls `apply`, and `Resolution::Macro` calls `run_macro` (verified: update.rs:255-261).
- `apply` tries `compose_action`, then `search_action`, then `screen_action`, and only then the mail-screen match. Screen actions outside their screen do nothing (verified: update.rs:371-629). `AppState::run_action` exposes the same path publicly (verified: update.rs:364-369).
- Is update pure? No (verified). It mutates `&mut AppState` in place. It reads through `&dyn MailData`, including `conversation_page`, `conversation`, `message_body` and `masked_for_address` (verified: update.rs:1049, 1107, 1652, 2010). It reads time from the injected `Arc<dyn Clock>` (verified: state.rs:779; update.rs:149, 1383). It is deterministic given its inputs, and it does no I/O itself (inferred).
- It returns `Vec<Cmd>`. Effects leave the crate only as `Cmd` values. Timers leave as `Cmd::WakeAt(Timestamp)`. `AppState::next_wakeup()` gives the earliest pending deadline, and `AppState::shows_clock()` tells the shell to tick once a second while the status clock or "saved N s ago" is visible (verified: event.rs:430-432; state.rs:1005-1104).
- `AppState::start` loads the first page (verified: state.rs:880-886). `AppState::set_pane_sizes` clamps sizes restored from the state file (verified: state.rs:888-901).

#### render and draw

- `render(&AppState, &dyn MailData, &mut Frame) -> RenderOutput { hits, cursor }` (verified: view/mod.rs:36-101). `draw(&mut AppState, ...)` stores the hits in state for the next mouse event (verified: view/mod.rs:46-49).
- render does not change state. It may read one page from `MailData` when the cache does not cover the window, for example on a draw before the first update (verified: view/panes.rs:161-187). A unit test asserts that one page call of at most 36 rows happens at offset 50,000 of 100k rows (verified: view/mod.rs:1037-1064).
- In accessible mode, render returns a cursor position on the focused row and calls `frame.set_cursor_position`. A new toast parks the cursor on the status line for one frame (verified: view/mod.rs:90-99; update.rs:164).

### State model (state.rs)

- `UiConfig` (verified: state.rs:47-91) is rebuilt by the shell and sent with `Msg::Config`. It holds `keymap`, `presets` (custom), `user_keys` (top-level `[keys]`), `macros`, `theme`, `layout`, `hints`, `accessible`, `time_zone`, `double_click` (default 400 ms), `wheel_lines` (default 3), `prefetch` (default 3), `toast_time` (default 5 s), `density`, `prefer_html`, `undo_send` (default 10 s), `remote_images`, `mark_read_delay` and `mark_read_on_archive` (verified: state.rs:124-148).
- `Screen` has seven values: Mail, Compose, Search, Masked, Rules, Settings and Keys (verified: state.rs:152-169). `Focus` has three: Sidebar, List and Reader (verified: state.rs:173-180).
- `Overlay` has six: Help, Perf, Confirm, Picker, Prompt and CaptureKey (verified: state.rs:184-199). `Prompt` carries a `LineInput` and a `PromptAction` (verified: state.rs:203-243). `ConfirmAction` lists the confirmed operations: permanent delete, discard draft, send anyway, disable or delete a mask, delete a rule, delete a preset, delete a macro (verified: state.rs:265-301).
- `Toast` has a level, an `undo` flag that shows the undo key, an expiry time, and an optional undo-send countdown (verified: state.rs:305-318).
- `Selection` stores either the selected rows or, after "select all", the rows left out. Select-all in a 100k mailbox stores nothing per row (verified: state.rs:327-398).
- `PageCache` keeps a few 64-row chunks around the viewport, at most four (verified: state.rs:400-493). `ListState` holds cursor, offset, total, anchor, a `Follow` mode (cursor or offset) and the prefetch direction (verified: state.rs:495-547). `ReaderState` holds the expanded and folded state and an `Arc<ReaderDoc>` (verified: state.rs:550-608).
- `AppState` (verified: state.rs:776-823) holds everything else. That includes `overlays: Vec<Overlay>`, `pending: Pending` (prefix keys and count), `hits: HitMap`, `last_click`, `drag`, `recent: Vec<Action>`, and one `Option` per screen (compose, search, results, masked, rules, settings, keys).
- `contexts()` gives the keymap contexts most-specific first (verified: state.rs:1119-1144):
  - help overlay: `[Help]`
  - compose while a field is being edited: `[Compose]`
  - compose otherwise: `[Compose, Global]`
  - search: `[Search]`
  - Masked, Rules, Settings and Keys screens: `[X, Global]`
  - mail screen: `[pane, Global]`

### Screens, overlays and how to back out

- Screens are a single value, not a stack. Each non-mail screen keeps its state when the user leaves, so typed text survives (verified: state.rs:8-12). Only the key editor remembers where it came from (Settings or Mail) (verified: keys_screen.rs:922-923; update/keys.rs:27-32). `leave_screen` saves the draft when the user leaves compose (verified: update.rs:951-963).
- Overlays are a Vec, and the top one gets the keys (verified: state.rs:182-199; update.rs:202-235). Only the top overlay is drawn. Help, Perf and Picker are drawn as popups. Confirm, Prompt and CaptureKey are drawn in the status line (verified: view/mod.rs:327-338, 612-665). The which-key popup is drawn only while a prefix is pending and no overlay is open (verified: view/mod.rs:329-331).
- `back()` returns false at the top level. It unwinds in this order (verified: update.rs:757-805):
  1. a pending prefix
  2. the top overlay
  3. the screen: compose closes and keeps the draft, search closes, Masked, Rules and Settings go to Mail, Keys goes to the screen it came from
  4. the sidebar overlay
  5. the full-screen reader
  6. focus that is not on the list
  7. a non-empty selection
  8. search results, which restore the mailbox and cursor
- Quit: `Action::Quit` calls `back()`. Only at the top level does it quit. While a sent message is still in its undo-send window, it opens a picker: Send now (Enter), keep queued, cancel the send, or stay (Esc) (verified: update.rs:402-406, 641-702). `Action::QuitNow` flushes compose and quits with no question (verified: update.rs:407-410).

Where Esc, q, Q and u go, in the fastmail preset unless noted (verified, cited per row):

| Where | Esc | u | q | Q |
|---|---|---|---|---|
| Mail screen | back() (preset.rs:142) | back() (preset.rs:141) | back(), else quit (update.rs:402-406) | quit now (preset.rs:144) |
| Pending prefix | cancels (keymap/mod.rs:521-524) | any unlisted key cancels (keymap/mod.rs:561-566) | same | same |
| Help overlay | closes (preset.rs:488-494; update.rs:286-288) | types into the filter | types into the filter (tests/screens.rs:62) | types |
| Picker | closes (update/overlays.rs:589-591) | types into the filter | types | types |
| Prompt | cancels, toast "Cancelled" (update/overlays.rs:790-793) | types | types | types |
| Confirm | cancels, as does any key but y (update.rs:203-212) | cancels | cancels | cancels |
| Key capture | cancels (update/keys.rs:382-386) | is captured | is captured | is captured |
| Compose, field being edited | leaves the field (preset.rs:512; update/compose.rs:251-262) | types | types | types |
| Compose, no field active | closes compose, keeps the draft (update/compose.rs:263-265) | back() | back(), closes compose | quit now |
| Search | closes search, restores the view (preset.rs:528; update/search.rs:168) | types | types | types |
| Masked | ends filtering, then clears the filter, then goes to Mail (update/masked.rs:281-290) | same as Esc | straight to Mail (update.rs:776-779) | quit now |
| Rules | form, then list, then Mail (update/rules.rs:717-723) | same as Esc | straight to Mail | quit now |
| Settings | Mail (update/screens.rs:71) | same | Mail | quit now |
| Key editor | clears the filter, then returns (update/keys.rs:119-126) | resets the row (preset.rs:604) | same as Esc | quit now |
| vim preset | as above | undo (preset.rs:373) | as above | as above |

Focus model:
- Tab and Shift-Tab cycle the panes the layout shows. With fewer than two panes they do nothing (verified: update.rs:807-828).
- In two-pane and stack layouts the sidebar is an overlay, toggled with `g f` or by clicking the mailbox title (verified: update.rs:830-871; preset.rs:127).
- Open on a list row: in three-pane layout, focus moves to the reader and the list stays visible. In two-pane and stack layouts, the reader fills the screen (verified: update.rs:1030-1085).
- refresh moves focus off a pane the layout no longer shows, and forces focus to the reader while it is full screen (verified: update.rs:1724-1734).
- Ctrl-z suspends from anywhere. The shell restores the terminal and sends `Event::Resize` when the process resumes (verified: event.rs:435-439; preset.rs:8-14).

### Views (view/*)

- The `Painter` is an immediate-mode writer (verified: view/mod.rs:103-245). It clips to the frame. Its `put` writes width-bounded text, and `span` writes a span and records its hit target. It also has `patch` (style only), `clear`, `line_cell`, which merges box-drawing joints, `frame_box`, vertical and horizontal separators with tees, and `title`.
- The mail screen has a top line, one bordered frame with draggable separators, the panes, a status line and a hint bar (verified: view/mod.rs:286-324).
  - The top line shows the brand, the account (at 100 columns or more), the mailbox name with a dropdown glyph when the sidebar is hidden, unread counts, the preset name and a clickable `? help`. The full reader instead shows a breadcrumb and "N of M" (verified: view/mod.rs:394-489).
- The status line (verified: view/mod.rs:533-757) has one left slot. Its priority order is:
  1. prompt
  2. key capture
  3. confirm (`y/N`)
  4. toast, with a clickable undo key when `undo` is set
  5. undo-send countdown
  6. selection counter, with clickable archive, trash and clear
  7. text the screen supplies

  Error toasts start with `! ` so failure is never shown by colour alone (verified: view/mod.rs:672-676). The right side shows the pending keys, or the sync glyph and text and the clock at 100 columns or more, or "N of M" when narrow.
- The hint bar (verified: view/mod.rs:759-1011) is one line. Which actions it lists is hand-coded per state:
  - list, reader and sidebar focus each have their own list
  - each overlay and each screen has a fixed list
  - during a prefix it shows "press a key", Esc and `?`

  The keys come from `keys_for` on the live keymap. The fitting algorithm first tries full labels, dropping non-important hints from the end. Then it tries short labels. Last, it keeps only important hints and pops until they fit. Every hint is clickable.
- Which-key (verified: view/overlays.rs:40-222) is a 50-column popup at the bottom right of the frame. Its title is the prefix and its label. Continuations appear in two columns, grouped by category, with nested prefixes shown as "label ...". The footer reads `Esc cancel   no timeout: g does nothing alone`, then "Same popup for" with the other prefixes. Every entry is clickable.
- Help (verified: view/overlays.rs:237-400):
  - It is a centred popup up to 80 columns wide, titled `Keys: <context>`.
  - It lists the bindings of the context it was opened from, plus Global, skipping bindings that an earlier context hides.
  - Rows are grouped by `Category`. Several keys for one binding are joined with commas.
  - The filter is a case-insensitive substring over keys, name, label, description and category. It is not fuzzy.
  - Compose help also lists the body editor's undo keys, which are not keymap bindings (verified: view/overlays.rs:326-350).
  - The last line reads "Hold Shift to select text with the mouse in most terminals" (verified: view/overlays.rs:384-387).
- The perf overlay shows p50, p95, p99, max, the 8, 16, 50 and 100 ms buckets, the five slowest frames and "budget: p99 under 8 ms" (verified: view/overlays.rs:402-495).
- The picker popup (verified: view/picker.rs:74-285):
  - It is 84 columns wide for the palette and 72 for other pickers. The From picker is anchored near the From field.
  - It has a `> ` filter line, rows with label, key and tag columns, matched characters highlighted, info rows that cannot be selected, and footer lines.
  - The status line shows "N of M commands match" (verified: view/picker.rs:48-72).
  - Its hints are fixed strings (verified: view/picker.rs:19-44).
- The settings screen (verified: view/screens.rs:1354-1470) has eight rows that change value, each with a one-line explanation. It has three read-only rows: Layout, Accessible mode and Hint bar, each marked "(set in the config file)". A row shows ", saving" until `Msg::Config` arrives.
- The key editor screen (verified: view/keys.rs:41-565; tests/snapshots/keys_editor__keys_editor_120x36.snap) has a preset line, a clickable section tab line (Mail, List only, Reader only, Global, Compose, Search, Sidebar, Masked, Rules, Settings, Help, Keys, Macros), rows grouped by category, and a two-line footer about the focused row.
- The search view (verified: view/search.rs:1-2) shows the query, operator chips, skipped terms, merged local and server results, and the JMAP filter preview. The compose view (verified: view/compose.rs:1-2) has header fields, the body editor, attachment and format rows, and the draft state in the status line. Both were only skimmed.
- Below 30x8 the frame shows only "mejl needs at least 30 by 8 cells" and records no hit regions (verified: view/mod.rs:61-71; layout.rs:141-144; view/mod.rs:1066-1078).

### Layout (layout.rs)

- `compute` is a pure function of size, preference, hints, the full-reader flag, the sidebar-overlay flag and pane sizes. update and render share it (verified: layout.rs:1-5, 147-230).
- Breakpoints (verified: layout.rs:21-26, 70-79):
  - 160 columns or more: three panes
  - 100 to 159: two panes, with the sidebar as an overlay
  - below 100: one pane, a stack
- The hint bar is dropped below 20 rows or when hints are off (verified: layout.rs:25-26, 160).
- A forced layout that does not fit degrades: three panes fall back to two below 14+1+48+1 inner columns, and two fall back to stack below 49 (verified: layout.rs:151-159).
- Sidebar width defaults to 22 and is clamped to 14..40. Each pane is at least 24 columns. The automatic list share is 0.545, giving the mock-up's 73/61 split at 160 columns, and the reader keeps at least 60 columns in the automatic split (verified: layout.rs:27-41; event.rs:577-584).
- The full reader adds a 35-column side panel at 100 columns or more (verified: layout.rs:27-30, 186-196).
- Row stack: top line, the box with the panes, the status line and the hint bar (verified: layout.rs:13-15, 161-166).
- Unit tests cover the breakpoints, the mock-up columns, the side panel, short terminals, clamped dragged ratios including NaN, and sizes 0..40 by 0..12 without panic (verified: layout.rs:287-380).

### Hit testing and mouse (hit.rs, update.rs)

- `HitMap` is a Vec of `(Rect, HitTarget)` regions. Later regions sit on top (draw order is the z order). It offers `hit(x,y)` for the top-most target and `hits(x,y)` for all of them (verified: hit.rs:88-149).
- `HitTarget` (verified: hit.rs:27-86):
  - generic: Pane, ListRow, ListMarker, SidebarRow, Splitter, Scrollbar, Action, Macro, Prefix, Link, PickerRow, Field, Row, Popup, Backdrop
  - email-specific: Fold, Attachment, Message, MailboxTitle, Mailbox
- Each list row records its own region, plus a two-cell marker region that toggles selection (verified: view/panes.rs:255, 279-283). The doc suggested a single list region plus the offset (terminal-ux.md:235), so the build differs.
- Clicks:
  - A click while a prefix is pending runs only an Action or Macro target. Anything else cancels the prefix (verified: update.rs:1392-1400).
  - A click while a confirm, prompt or capture is open cancels it (verified: update.rs:1401-1407).
  - A click on the backdrop pops the overlay or closes the sidebar overlay. A click inside a popup does nothing (verified: update.rs:1412-1420).
  - A click on a picker row moves the cursor. A second click, a double click, or any click on a non-label picker chooses the row (verified: update.rs:1515-1523).
- Double click means two presses on the same cell within `UiConfig::double_click`, 400 ms by default, timed by the injected clock (verified: update.rs:1383-1390; state.rs:136). A double click on a list row opens it.
- The wheel:
  - The help overlay scrolls by `wheel_lines`.
  - A picker moves its cursor by one.
  - Non-mail screens step by one.
  - On the mail screen the pane under the pointer scrolls by `wheel_lines`, default 3. The list then keeps its offset and pulls the cursor into view (verified: update.rs:1347-1377; state.rs:137).
- Drag:
  - A press on a splitter starts a drag.
  - Drag events update the pane sizes.
  - Release sends `Cmd::SaveLayout` (verified: update.rs:1323-1346, 1451).
- A right click on a list row, a pane or a message opens the actions menu for it (verified: update.rs:1286-1322).
- A click on the scrollbar track jumps there, but only for the list in stack layout (verified: view/panes.rs:311-320).
- The shell must coalesce mouse motion (verified: event.rs:43). The mouse modes and the capture default live in the shell (verified: crates/mejl/src/config.rs:229,243).
- The hit map is one frame old. One code comment works around this (verified: update/keys.rs:1083-1084).

### Keymap system (keymap/*)

#### Keys and sequences (key.rs, 603 lines)

- `Key` is a `KeyName` plus ctrl, alt and shift flags. Shift is folded into printable characters, so `X` means Shift+x, and Shift-Tab becomes BackTab. `Key::from_event` normalises legacy and kitty encodings to one form (verified: keymap/key.rs:1-7, 104-141; tests at keymap/key.rs:473-497).
- The config syntax accepts `ctrl-`, `control-`, `^`, `alt-`, `meta-` and `shift-` prefixes and names such as `space`, `enter`, `esc`, `tab`, `pgdn` and `f1`..`f24`. `shift-1` is refused as ambiguous, and control characters are refused (verified: keymap/key.rs:283-393).
- The display form is short: `j`, `^k`, `Enter`, `S-Tab`, `PgDn`, `Ctrl-Enter`. `to_config` round-trips (verified: keymap/key.rs:197-269, 499-558).
- A `KeySeq` holds at most 4 keys. `is_prefix_of` is a strict prefix test (verified: keymap/key.rs:395-457).
- `legacy_problem()` names keys that legacy encoding cannot deliver: any Alt, Ctrl-i, Ctrl-m, Ctrl-h, Ctrl-[, Ctrl+Shift+letter, Ctrl+digit, Ctrl+punctuation (except `] \ ^ _ @` and space), Shift-Space, and Ctrl or Shift with Enter, Tab, Backspace or Esc (verified: keymap/key.rs:152-195, 571-602).

#### Contexts and the engine (mod.rs, 803 lines)

- There are 11 contexts: Global, List, Reader, Sidebar, Help, Compose, Search, Masked, Rules, Settings and Keys. Every context except the three panes is "modal", meaning its bindings replace global ones on purpose (verified: keymap/mod.rs:58-131).
- `Keymap` keeps per-context `BTreeMap<KeySeq, Entry>` tables, prefix labels, macro info and warnings (verified: keymap/mod.rs:246-256).
- `feed(&mut Pending, contexts, key) -> Resolution` (verified: keymap/mod.rs:518-568):
  - Esc while a prefix is pending cancels it.
  - In presets that use counts (vim only), a digit starts or extends a count, unless the context binds that digit, as Masked binds 1..6.
  - Otherwise the first context that knows the sequence decides: an exact match resolves, and a strict prefix keeps the sequence pending.
  - Anything else cancels a pending sequence or is `Unbound`.
  - There are no timeouts anywhere (verified: keymap/mod.rs:10-12).
- `continuations(prefix, contexts)` lists what can follow a prefix, in preset order, using the same first-context-wins rule. It feeds which-key (verified: keymap/mod.rs:588-640).
- `keys_for(action, contexts)` returns the first binding of an action that no earlier context shadows, by preset order. Hints, the palette, menus and toasts all use it (verified: keymap/mod.rs:650-687).
- `check()` reports three kinds of problem (verified: keymap/mod.rs:704-792):
  - Errors: a key that is also a prefix in the same table, so neither can ever run.
  - Warnings: keys with a legacy-encoding problem.
  - Warnings: in pane contexts, bindings that hide a global key or prefix.

  `mejl keys --check` uses it (the doc says so: keymap/mod.rs:704-706; the command itself lives in the shell, unverified here).

#### Layers, presets and config

- `Keymap::build(KeymapSpec)` applies, in order: the built-in preset, a custom preset's overrides, the top-level `[keys]` overrides, then macro keys (verified: keymap/mod.rs:265-380).
  - Errors from `check()` fail the build.
  - A macro key never fails it. A key that would cut a prefix is skipped with a warning. A key that takes an action's key wins, with a warning (verified: keymap/mod.rs:392-423).
  - Macro keys bind only in List and Reader (verified: keymap/mod.rs:54-55).
- Presets (verified: keymap/preset.rs):
  - `fastmail` is the default. `gmail` and `vim` are the others (verified: keymap/preset.rs:20-59).
  - Only vim uses counts (verified: keymap/preset.rs:56-59).
  - Prefix labels (verified: keymap/preset.rs:88-117): fastmail has `g`, `.`, `*`, `b`, `@` and `ctrl-o`. gmail drops `.`. vim adds `r` (reply), `z` (view) and `space` (tools).
  - Tables (verified: keymap/preset.rs:120-616): global, triage (bound in both list and reader), list, reader, sidebar, help, compose, search, masked, rules, settings and keys.
  - The compose table binds only keys a text field never needs: the `ctrl-o` prefix, `ctrl-s`, Tab, Shift-Tab and Esc. Ctrl-Enter is left out because legacy encoding cannot deliver it (verified: keymap/preset.rs:496-513).
  - A test asserts that every preset has no conflicts and no legacy-only keys (verified: keymap/tests.rs:24-37).
- `[keys]` overrides (verified: keymap/config.rs:1-11, 168-236):
  - The format is `[keys.<context>] "key seq" = "action-name" | "none"`. "none", "unbind" and the empty string unbind.
  - Unbinding a key also removes every longer binding it is a prefix of, which frees a prefix (verified: keymap/mod.rs:510-516).
  - Errors are collected, never stop at the first, and carry a "did you mean" suggestion (verified: keymap/config.rs:22-80; keymap/tests.rs:247-276).
  - `to_table` writes the overrides back (verified: keymap/config.rs:137-151).
- Custom presets: `CustomPreset { name, base: Preset, overrides }`, stored as a diff from the base so actions added later keep their default keys (verified: keymap/config.rs:93-103). `Keymap::diff_from(base)` computes the diff, and applying it to the base round-trips. A test also round-trips through the config table (verified: keymap/edit.rs:106-139, 212-240).

#### Key editor (keys_screen.rs 1,652 lines; update/keys.rs 1,130; view/keys.rs 571)

- The editor works on the active preset's own map, its base plus its own overrides. The top-level `[keys]` and macro keys are not part of that map (verified: keys_screen.rs:1-21, 959-1004).
  - Built-in presets are read-only. The editor says "Press d to make your own copy" (verified: update/keys.rs:296-312).
  - Duplicating sends `KeysEdit::NewPreset { take_user: true }` and switches to the copy (verified: update/keys.rs:920-948).
- Each edit is a pure function from one map to the next. Bindings go through `bind`, `remove` and `reset`, and conflicts are checked with `check` (verified: keys_screen.rs:363-620).
  - The result is sent as `KeysEdit::SetPresetKeys { overrides, drop_user }`. `drop_user` drops the `[keys]` entries that the edit took over (verified: update/keys.rs:564-587; event.rs:182-189).
  - The editor never shows a change before the shell answers. A `saving` counter blocks new edits until `Msg::Config` or `Msg::SettingRefused` arrives (verified: update/keys.rs:1-7, 314-326, 1061-1069).
- Key capture is an overlay that reads raw keys before the keymap. It waits while the keys typed so far are a prefix, and Esc cancels (verified: update/keys.rs:378-409).
- Clash rules, as built (verified: keys_screen.rs:363-475):
  - Typing guard: in the Compose, Search and Help sections, a plain key is refused because it types into the field. In Compose, Enter, the arrows and the page keys are refused too. Ctrl, Alt, F-keys and Tab are always allowed (verified: keys_screen.rs:298-313, 371-382).
  - Same key: "already the key".
  - A clash with another row in the same table asks Swap, Replace or Cancel. Swap is offered only in Replace mode when this row has a key to give back (verified: keys_screen.rs:459-466; update/keys.rs:450-498).
  - A key that starts a longer binding in the same table, or whose head is already a key there, is refused (verified: keys_screen.rs:411-429).
  - Hiding a global key is a warning. Hiding a global prefix asks first, without Swap. In the Global section, keys that pane tables shadow get warnings (verified: keys_screen.rs:406-410, 417-419, 441-458, 467-473).
  - Lockout: `LOCKOUT = [Back, Help, Palette, QuitNow, GoSettings]`, plus every action in the Keys context. The editor refuses to take or remove the last key of any of these unless Swap gives one back (verified: keys_screen.rs:29-37, 277-280, 529-538, 559-575). Quit (`q`) is not protected; QuitNow (`Q`) is.
  - A preset row cannot take a key that a macro holds (verified: keys_screen.rs:622-644; update/keys.rs:418-421).
  - Reset is refused when another row now holds one of the base keys (verified: keys_screen.rs:585-620).
- Macros get their own section. You can make one (`n`), add steps through a step picker and a folder picker, reorder steps (`K`/`J`), rename it, bind a key (`b`), and delete it after a confirm (verified: update/keys.rs:127-155, 219-294, 697-849, 1041-1059).

#### One source for help, hints, which-key and the palette

All of these read keys from the live keymap:
- the hint bar (verified: view/mod.rs:278-284, 759-954)
- the help overlay (verified: view/overlays.rs:283-324)
- which-key (verified: view/overlays.rs:46)
- the palette key column (verified: update/overlays.rs:91-96, 124)
- the actions menu (verified: update/overlays.rs:492-497)
- the undo key shown in toasts (verified: view/mod.rs:685-696)
- the reader's inline key hints (verified: update.rs:2024-2026)

Some text does not come from the keymap: the hint bar's choice of actions, every overlay hint (`Up/Down`, `Esc`, `^n/^p`, `Tab complete`, `y confirm`), the pending-prefix hints `Esc` and `? all keys`, and screen hints such as `1-4` and `Left/Right` (verified: view/mod.rs:799-900; view/picker.rs:19-44; view/screens.rs:1508-1514, 1551-1557). The keys of pickers, prompts, confirms, capture and LineInput are hard-coded (verified: update/overlays.rs:584-624, 785-804; update.rs:207; update/keys.rs:382; input.rs:116-184).

#### Macros (macros.rs 505, update/macros.rs 319)

- A macro is a label, its keys, and up to 10 steps (`MAX_STEPS`). The steps are mark-read, mark-unread, pin, unpin, mute, archive, trash, spam, and the folder steps move, label and unlabel (verified: macros.rs:1-17, 26-30, 57-97).
- The TOML form is `[macros.<id>] label, keys, steps`, where a step is a name or a `{ move = "Path", id = "M1" }` table (verified: macros.rs:3-8, 138-203).
- Running a macro resolves every folder first. One missing folder changes nothing. Moves into the folder the mail is already in are skipped. All steps go out as one `Mutation::Batch`, so one undo reverts the whole macro (verified: update/macros.rs:1-107).

### Palette, pickers and the matcher

- `Picker` is a generic filter-over-items structure (verified: picker.rs:158-336):
  - The cursor wraps when stepping and does not wrap when paging. Info rows are skipped.
  - Move and label pickers add a "Create mailbox" row that is never ranked.
  - The sort is stable, so ties keep the given order (verified: picker.rs:269-271).
  - `PickValue` and `PickerKind` are email-specific (verified: picker.rs:19-103).
- The palette is built once when it opens (verified: update/overlays.rs:87-215):
  - Rows: recent actions first, then every action in scope for the screen, macros, "Go to:" mailboxes, saved searches, drafts, up to 200 contacts, every masked address and every rule. Each action row shows its key and a category tag.
  - It opens on `:`, `ctrl-k` and `ctrl-p` in fastmail and gmail. vim drops `ctrl-p` (verified: keymap/preset.rs:137-139, 366-367).
  - `in_scope` hides actions that belong to other screens (verified: update/overlays.rs:26-44).
  - In the palette, Tab completes the filter to the highlighted label (verified: update/overlays.rs:600-606).
- Recency: only actions chosen from the palette are remembered, most recent first, up to 8 (`RECENT`). Motions, Palette, Back, Help, Open and Repeat are never remembered (verified: update/overlays.rs:15-16, 72-85, 644; `remember` has one caller). There is no frecency score.
- The matcher is an in-house scorer (verified: fuzzy.rs:1-121):
  - Each pattern word must match on its own.
  - A literal substring scores 100 + 16n, with bonuses for a word-boundary start and a whole word, minus a start penalty of up to 30.
  - Otherwise a greedy subsequence matches, counting only characters at word starts or in a run.
  - Case folding is Unicode.
  - The picker also scores `label + tag` at -50 (verified: picker.rs:248-266).
  - The code comment gives the reason for writing it: nucleo-matcher is MPL-2.0 and mejl takes only permissive licences (verified: fuzzy.rs:3-7).

### Theme, glyphs, NO_COLOR and density (theme.rs 832 lines)

- There are 62 semantic `StyleName`s with dotted names such as `list.unread`, `status.error` and `reader.quote.1`. Widgets ask for a name, never a colour (verified: theme.rs:1-8, 36-122).
- Built-in themes are `terminal` (the default), `mono`, `high-contrast`, `dark` and `light` (verified: theme.rs:9-19, 302-317).
  - `terminal` uses the ANSI 16 colours, never fills a background, and shows the cursor row in reverse video (verified: theme.rs:357-417).
  - `dark` and `light` map the ANSI colours to a truecolor palette and give the cursor row a background (verified: theme.rs:476-517, 655-679).
- `named(name, no_color)` and `from_toml(doc, no_color)` force mono when `no_color` is true. NO_COLOR therefore always wins, even over a theme file (verified: theme.rs:305-317, 519-551; tests at theme.rs:726-746).
- A theme file sets `base`, `name` and `ascii`, plus a `[styles]` table of `{ fg, bg, bold, dim, italic, underline, reverse, strikethrough }`. Colours can be ANSI names, `#rrggbb` or an index 0-255 (verified: theme.rs:20-28, 554-635).
- Glyphs (verified: theme.rs:147-206):
  - `UNICODE`: ◦ ⚑ @ ✓ ✓ ✗ ▸ ▾ ⋯, with ┃ as the scrollbar thumb.
  - `ASCII`: * ! @ + + x > v ~ #.
  - Border sets are `BOX` and `ASCII` (verified: theme.rs:224-241). `with_ascii` switches both (verified: theme.rs:336-346).
  - A test checks that each glyph has `unicode-width` 1 (verified: theme.rs:812-831). It does not test East Asian Ambiguous width.
- Density lives in state.rs, not theme.rs (verified: state.rs:93-122):
  - Compact: one line per row, no date headings.
  - Comfortable: one line under date headings. This is the default.
  - Relaxed: a row line plus a preview line (verified: update.rs:92-127).
  - An inline preview after the subject shows only in three-pane layout and not in Relaxed (verified: view/panes.rs:219-224).

### Text, dates, clock, perf and input

- text.rs (266 lines) measures width per extended grapheme, matching ratatui's cell width, including the halfwidth katakana fix (verified: text.rs:39-58).
  - `sanitize` removes bidi controls, zero-width characters and soft hyphens. It turns control characters into single spaces, drops zero-width graphemes, replaces graphemes wider than 2 cells with U+FFFD, and trims (verified: text.rs:21-114).
  - `truncate`, `fit`, `rfit` and `wrap` work on graphemes and display columns (verified: text.rs:116-197).
- dates.rs (215 lines) computes everything against an explicit `Zoned` now, with Monday weeks (verified: dates.rs:1-9).
  - The list column is 6 cells: `14:02`, `Thu`, `Oct 17`, `Oct'24` (verified: dates.rs:15-16, 73-87).
  - Group headings are Today, Yesterday, This week and Earlier (verified: dates.rs:19-71).
  - There are separate formats for the reader pane, the full reader, headers and the clock (verified: dates.rs:89-131).
- clock.rs (82 lines) defines a `Clock` trait with `SystemClock` and `ManualClock`, which offers `set` and `advance` (verified: clock.rs:13-66). Toasts, relative dates, the status clock, double click and the timers all use it (verified: clock.rs:3-6).
- perf.rs (137 lines) keeps a `VecDeque` of the last 1000 `input_to_flush` samples, a total count, nearest-rank p50, p95 and p99, max, buckets (under 8, 16, 50, 100 ms, and slower) and the five slowest in the window (verified: perf.rs:14-91). The shell measures each sample and sends it as `Msg::Perf` (verified: perf.rs:3-4; event.rs:386-391).
- input.rs (315 lines) provides `LineInput`, one line of text with a byte cursor on grapheme boundaries (verified: input.rs:1-31, 81-110).
  - Keys: printable characters insert. Backspace and Delete delete. Ctrl-w and Ctrl or Alt Backspace delete a word. Ctrl-u and Ctrl-k kill. Left and Right move, and Ctrl or Alt with them move by word. Ctrl-b, Ctrl-f, Home, End, Ctrl-a and Ctrl-e also move (verified: input.rs:112-184).
  - Pasted text is folded to one line (verified: input.rs:224-241).
  - `view(cols)` scrolls the text so the cursor stays visible (verified: input.rs:186-221).

### Data abstraction and the fake (data/*)

- `MailData` is a synchronous, read-only trait with 18 methods, one of them defaulted (verified: data/mod.rs:574-653). The contract it states:
  - every call is a bounded, indexed read with a 2 ms p99 budget
  - no network
  - missing data comes back as `None` or empty, and the UI asks for it with a `Cmd`
  - results already include the pending optimistic changes

  (verified: data/mod.rs:1-17, 574-580).
- The UI asks only for the rows it is about to draw, through `conversation_page(mailbox, offset, limit)` (verified: data/mod.rs:588-595).
- Some methods return whole collections: `masked_emails`, `rules`, `drafts` and `saved_searches`. The palette calls them when it opens (verified: data/mod.rs:610-644; update/overlays.rs:152-207).
- The view structs are plain owned data, decoupled from JMAP and SQLite (verified: data/mod.rs:9-12, 130-572).
- `MemoryData` (data/memory.rs, 1,349 lines) is the in-memory fake (verified: data/memory.rs:1-10, 143-229, 398-494):
  - It stores emails, mailboxes, bodies, identities, masks, contacts, saved searches, rules, drafts and renders.
  - `apply(&Mutation) -> DataChange` plays the shell's part, with its own undo and redo snapshots.
  - Atomic read counters (`ReadStats`) let tests prove the list is virtualised.
  - `MemoryData::synthetic(n, now)` generates rows from their index, so 100k rows cost nothing to build and mutations are ignored.
  - `MemoryData::demo(now)` loads the fictional mock-up mailbox from data/fixture.rs (1,086 lines): 214 inbox conversations, 12 unread, all on example domains, with dates shifted to the requested now (verified: data/fixture.rs:1-7; data/memory.rs:204-209).

### Tests

- The harness (verified: tests/common/mod.rs:20-133) uses a `ManualClock` fixed at 2026-10-23T14:02:31Z, the demo data, `TestBackend` and a `run` loop that plays the shell. `Cmd::Mutate` goes through `MemoryData::apply` and comes back as `Msg::DataChanged`. `Cmd::RenderBody` goes through `render_body` and comes back as `Msg::BodyRendered`. A guard stops the loop after 50 rounds.
  - Keys are fed in config syntax. Uppercase letters carry SHIFT, as legacy terminals send them (verified: tests/common/mod.rs:198-245).
  - Screen text is read by joining cell symbols and skipping the second cell of wide graphemes (verified: tests/common/mod.rs:139-196).
- Snapshots: 37 insta text snapshots. 34 are full frames at 80x24, 120x36 and 160x40. The rest are single status lines: archive toast and offline status (verified: tests/snapshots listing; tests/screens.rs:17-117).
  - Styles are asserted cell by cell: bold unread rows, a reversed cursor row, marker colours, a dim preview, blue hint keys, cyan underlined links, the dark theme's cursor background, and no colour at all in ascii-mono (verified: tests/screens.rs:96-170).
- Robustness:
  - robust.rs sweeps 4 layout preferences, widths 30..200 in steps of 23, heights 8, 19, 24 and 40, and 7 key states, asserting that pane separators stay intact. It also clamps hostile pane sizes and resizes through every width (verified: tests/robust.rs:43-122).
  - robust_screens.rs sweeps 11 screen and popup setups over widths 30..200 in steps of 17 and heights 8..50 in steps of 7, asserting that the line count equals the height (verified: tests/robust_screens.rs:22-70).
- Width: hostile names (CJK, ZWJ families, combining marks, RLO tricks, newlines, Korean, halfwidth katakana) keep borders and dates in place. Bidi and control characters never reach the buffer, and wide names truncate on whole graphemes (verified: tests/width.rs:10-174).
- Scale: a 100k synthetic mailbox.
  - Start-up reads at most one chunk.
  - 400 `j` presses read at most 12 pages of up to 64 rows.
  - `G` reads at most 3 chunks.
  - 200 page-downs stay bounded.
  - `* a` reads nothing per row, and an `X` range beyond 5,000 rows is refused with a hint to use `* a` (verified: tests/scale.rs:27-109; update.rs:44-45, 1087-1112).
- Timing: every bound is wall-clock `Instant` around `update` plus `draw`, in debug builds:
  - p99 under 100 ms per key while scrolling 100k rows, 20 s in total, and `G` under 500 ms (verified: tests/scale.rs:63-80).
  - p99 under 60 ms for palette, compose, search, rules and masked screens (verified: tests/perf_screens.rs:23-128).
  - p99 under 30 ms per key in compose with 500 and 5,000 lines, and the 5,000-line median under 3 times the 500-line median (verified: tests/compose_editor.rs:65-105).

  The comments say the real budget, p99 under 8 ms, "is checked in release benchmarks" (verified: tests/scale.rs:63-64). No UI benches exist. Only `crates/mejl-store/benches/store.rs` exists, and it benches store reads (verified: directory listing; crates/mejl-store/benches/store.rs:68-105). CI runs `cargo test --workspace` in debug, and only on release tags (verified: .github/workflows/release.yml:44).
- Mouse tests cover hit map coordinates, click and double click with clock advances, the marker column, wheel over the pane under the pointer, splitter drag with `SaveLayout`, clicks on the hint bar and top line, sidebar and which-key clicks, the mailbox title, folds, links and attachments, the scrollbar, right click, and a click cancelling a confirm (verified: tests/mouse.rs:15-265).
- Keyboard tests cover moves, paging, open with mark-read, the narrow reader, optimistic archive with undo and redo, new mail keeping the cursor, ranges and select-all, pin and mark, permanent delete with confirm, the prefix popup, the help filter, Tab cycling, the sidebar overlay, quotes, headers and expand, quit, perf, the vim preset, overrides, and Ctrl in both legacy and kitty encodings (verified: tests/keys.rs:30-551).
- The key editor has 26 integration tests and 5 snapshots (verified: tests/keys_editor.rs outline).
- The keymap has 15 unit tests (verified: keymap/tests.rs).

### Module classification

"Generic" means it lifts into another app as is or nearly so. "Adaptable" means the mechanism transfers but the enums and content are mejl's. "Email-specific" means it does not transfer. Line counts are verified with wc; the buckets are inferred.

| Module | Lines | Class | Note |
|---|---|---|---|
| clock.rs | 82 | generic | Clock seam |
| perf.rs | 137 | generic | depends only on `FrameSample` |
| text.rs | 266 | generic | width, sanitize, truncate, wrap |
| input.rs | 315 | generic | uses `keymap::Key` |
| fuzzy.rs | 165 | generic | matcher |
| keymap/key.rs | 603 | generic | Key, KeySeq, parse, display, legacy check |
| keymap/edit.rs | 241 | generic | lookup and diff |
| keymap/config.rs | 237 | generic | `[keys]` TOML; refers to Action and Context |
| compose/editor.rs (+tests 324) | 1,231 | generic | multi-line editor with soft wrap and undo |
| keymap/mod.rs | 803 | adaptable | engine generic; `Context` enum app-specific |
| keymap/preset.rs | 616 | adaptable | mechanism generic; tables are mail keys |
| keymap/tests.rs | 449 | adaptable | tests tied to presets |
| action.rs | 334 | adaptable | macro pattern generic; 127 mail actions |
| theme.rs | 832 | adaptable | mechanism generic; 62 mail style names |
| picker.rs | 401 | adaptable | Picker generic; PickValue and PickerKind specific |
| hit.rs | 171 | adaptable | HitMap generic; some HitTarget variants mail |
| layout.rs | 381 | adaptable | breakpoint pattern; mail panes |
| dates.rs | 215 | adaptable | relative dates and column |
| state.rs | 1,242 | adaptable | pattern; mostly mail fields |
| event.rs | 727 | adaptable | Event/Msg/Cmd pattern; mail payloads |
| lib.rs | 207 | adaptable | contract docs |
| settings.rs | 218 | adaptable | cycle-value rows pattern |
| keys_screen.rs | 1,652 | adaptable | key editor logic; sections tie to contexts |
| update.rs | 2,059 | adaptable | dispatch, back(), quit, mouse skeleton; triage is mail |
| update/keys.rs | 1,130 | adaptable | key editor update |
| update/screens.rs | 212 | adaptable | screen dispatch |
| update/overlays.rs | 967 | adaptable | palette, picker and prompt handling; menus mail |
| view/mod.rs | 1,079 | adaptable | Painter generic; status line and hint bar adaptable |
| view/overlays.rs | 506 | adaptable | which-key, help, perf |
| view/picker.rs | 286 | adaptable | picker popup |
| view/keys.rs | 571 | adaptable | key editor view |
| macros.rs | 505 | email | mail-change steps |
| update/macros.rs | 319 | email | runs mail macros |
| compose.rs | 1,249 | email | header form, quoting, drafts |
| update/compose.rs | 831 | email | |
| view/compose.rs | 571 | email | |
| search.rs | 330 | email | |
| update/search.rs | 364 | email | |
| view/search.rs | 519 | email | |
| rules.rs | 539 | email | Sieve rules |
| update/rules.rs | 728 | email | |
| masked.rs | 447 | email | Masked Email |
| update/masked.rs | 397 | email | |
| view/screens.rs | 1,620 | email | masked and rules views; settings view adaptable |
| view/panes.rs | 523 | email | sidebar, list, reader |
| reader.rs | 880 | email | conversation document |
| body.rs | 239 | email | body render cache, cap 64 |
| address.rs | 217 | email | address parsing |
| data/mod.rs | 653 | email | MailData; trait-plus-view-model pattern adaptable |
| data/memory.rs | 1,349 | email | fake; read-counter pattern adaptable |
| data/fixture.rs | 1,086 | email | demo mailbox |

Totals (inferred from the table): generic 3,601 lines including the editor tests, adaptable 15,058, email-specific 13,366. The sum is 32,025, the src total.

### UX patterns versus spela's contracts

For each pattern: what mejl does, then what spela's documents require (spela side verified at the cited lines; whether the Go code matches those documents was not checked).

- **Fuzzy palette on `:` and Ctrl-k.** Built (verified: keymap/preset.rs:137-139). Spela requires that commands need only numbers, arrows, Tab, Enter and Space, and that punctuation is plain text only inside an input (spela/AGENTS.md:79-81; spela/docs/tui-testing.md:127-131). `:` as a command key collides. Ctrl-k is a modifier key, so it is outside that list too.
- **`q` back then quit, `Q` quits at once.** Built (verified: update.rs:402-410). Spela says Help and Quit are Actions rows, and Quit must wait for a save and stay unavailable during a scan or DLL change (spela/docs/tui-testing.md:120-122, 150). Collision: a letter key versus an Actions row. mejl's quit question appears only for sends held in the undo window (verified: update.rs:641-674).
- **Esc and `u` back out one level.** Built, with exceptions: help, pickers and fields own typing; `u` is reset in the key editor and undo in vim; `q` skips levels on Masked and Rules (verified: see the table above). Spela uses Esc to cancel an editor or picker (spela/docs/tui-testing.md:127-128), which is compatible. `u` is a letter key, which collides with spela's key list.
- **Non-overlapping prefixes with which-key.** Built (verified: keymap/mod.rs:10-12; view/overlays.rs:40-222). Spela uses digits as destination keys (`text:0` opens Actions, spela/AGENTS.md:84-85). Letter prefixes collide with the "numbers, arrows, Tab, Enter, Space" rule. mejl's vim counts and the Masked screen's 1..6 filters also use digits (verified: keymap/mod.rs:525-540; keymap/preset.rs:539-544).
- **Named presets, custom presets, in-app key editor.** Built, about 4,600 lines across keys_screen.rs, update/keys.rs, view/keys.rs, keymap/edit.rs and part of keymap/mod.rs (inferred). Whether spela needs it is open.
- **Mouse capture on by default with in-house hit testing.** Built in the shell (verified: crates/mejl/src/config.rs:229,243). Spela's review procedure is keyboard-only through termctrl (spela/AGENTS.md:62-114). That is not a stated collision, but capture takes text selection away from the user (terminal-ux.md:224).
- **One-line hint bar that changes with focus.** Built (verified: view/mod.rs:759-1011). Spela requires sending a command only when its key is visible in the frame, and selecting Actions rows by their visible labels (spela/docs/tui-testing.md:128-129; spela/AGENTS.md:95-96). mejl's hint bar drops and shortens entries by width (verified: view/mod.rs:966-991), so some keys are not visible at 80 columns.
- **Filterable `?` help.** Built (verified: view/overlays.rs:237-400). Spela makes Help an Actions row (spela/docs/tui-testing.md:150). `?` is punctuation.
- **Undo instead of confirmation for reversible actions.** Built for mail triage (verified: update.rs:525-542; tests/keys.rs:110-136). Spela says confirmations start on Cancel, need an explicit selection, and that with "Confirm destructive actions" off, DLL updates start without a confirmation while restore and install still ask (spela/docs/tui-testing.md:176-180). mejl's confirm is a status-line `y/N` where any other key cancels (verified: update.rs:203-212; view/mod.rs:652-664). It has no Cancel button and no scrollable details, so it collides. Spela profile edits save at once, and the same Space key undoes a toggle (spela/docs/tui-testing.md:8-10), which fits mejl's pattern.
- **TOML config edited in place with toml_edit by a settings screen.** The UI sends `Cmd::SaveSetting` and shows the value only after `Msg::Config` returns (verified: settings.rs:1-6; update/screens.rs:56-70). The writer is in the shell (verified: crates/mejl/src/app/settings.rs). Spela uses YAML (spela/AGENTS.md:39-40), so the toml_edit part does not transfer directly. A YAML equivalent that preserves comments is unverified.
- **Breakpoints at 160 and 100 columns.** Built (verified: layout.rs:21-24). Spela requires that at 80x24 the compact header and active pane fit and Tab reveals the other pane, that 120x40 shows both panes, and that below 80x24 a bounded resize prompt replaces the workspace (spela/docs/tui-testing.md:201-203; spela/AGENTS.md:95-96). mejl stacks below 100 columns. In stack layout Tab does nothing, because cycling needs two panes, and Enter opens a full-screen reader instead (verified: update.rs:818-820, 1078-1081). mejl's floor is 30x8 (verified: layout.rs:142-144). Collision.
- **A `doctor` command and a demo mode backed by a fake.** Both are in the shell (verified: crates/mejl/src/cli.rs:480, 905-928; crates/mejl/src/doctor.rs). The UI crate supplies `MemoryData::demo` (verified: data/memory.rs:204-209).
- **Text fields own typing.** Built (verified: state.rs:1125-1132; update/compose.rs:179-199; update/search.rs:64-90). Spela requires that typing `01234` in a text field leaves the destination unchanged (spela/docs/tui-testing.md:169-171). This is compatible.

### Doc versus code mismatches (short list; details in conflicts)

1. update signature and where the keymap runs (architecture.md:170-171; terminal-ux.md:395; versus update.rs:54, 255-261).
2. Fuzzy matcher choice; architecture.md contradicts itself (lines 428, 594 versus 459; fuzzy.rs:3-4).
3. Crates listed but not used: tui-input, tui-tree-widget, nucleo-matcher (architecture.md:420-421, 428).
4. Frecency versus palette-only recency (terminal-ux.md:167).
5. List share of 40 percent versus 0.545 (terminal-ux.md:253; layout.rs:39).
6. Reader header block hidden under 20 rows: not built (terminal-ux.md:256).
7. Density meanings (terminal-ux.md:276).
8. Older list dates as `2024-10-03` versus `Oct'24` (terminal-ux.md:277; dates.rs:84-85).
9. `@` is a prefix, not a single key (terminal-ux.md:563); the prefix list omits `@` and `ctrl-o` (terminal-ux.md:160).
10. Glyph width claim versus box drawing (theme.rs:147-149, 188).
11. "Every key remappable" versus hard-coded overlay and input keys.
12. Hint text literals that do not follow the keymap.
13. Lockout only in the key editor.
14. `UnknownPreset` message omits gmail (keymap/config.rs:75).
15. Budgets claimed as enforced in CI and in release benchmarks; neither exists for the UI.
16. Snapshot sizes and themes (architecture.md:485).
17. Disk reads on the UI thread (terminal-ux.md:12 versus data/mod.rs:3-7; architecture.md:601).
18. "Every call bounded" versus whole-collection methods.
19. Accessible mode only partly built (terminal-ux.md:464).
20. Mask disable: three code paths behave three ways (architecture.md:37, 570).
21. Discard draft is confirmed, not undoable (terminal-ux.md:576).
22. `q` and Esc unwind to different depths on Masked and Rules.
23. Compose `Ctrl-o p` markdown preview: not built (terminal-ux.md:574).
24. Settings screen scope (terminal-ux.md:498).

### Conflicts and mismatches

- **update signature and where the keymap runs.** The docs say keys go through the keymap into an Action before update, and that update has the shape update(&mut State, Action) -> Vec<Cmd>. The code instead feeds raw Events into update, which resolves keys itself with Keymap::feed. update also takes a MailData read handle. Evidence: docs: architecture.md:144-145, 170-171; terminal-ux.md:384, 395. code: crates/mejl-ui/src/update.rs:54, 194-262; lib.rs:14 (verified)
- **update described as pure.** terminal-ux.md says update is pure state change plus command values. The code mutates &mut AppState, reads the store through &dyn MailData and an injected Clock, and runs a refresh pass after every event. render can also read a page from MailData when the cache does not cover the window. Evidence: terminal-ux.md:395; update.rs:74, 1673-1740; state.rs:779; view/panes.rs:161-187; view/mod.rs:1037-1064 (verified)
- **Fuzzy matcher choice.** architecture.md picks nucleo-matcher 0.3.1 in the crate table and marks disagreement 16 as resolved in its favour. The same doc's budget row says the palette uses mejl-ui's own matcher because nucleo-matcher is MPL-2.0. The code is an in-house scorer, and its comment says nucleo was rejected for licence reasons. No Cargo.toml references nucleo. Evidence: architecture.md:428, 459, 594; terminal-ux.md:166; crates/mejl-ui/src/fuzzy.rs:3-7; grep of all Cargo.toml for nucleo found nothing (verified)
- **Crates listed but not used.** The crate table lists tui-input 0.15.5 for text editing and tui-tree-widget 0.24.1 for the sidebar tree. Neither is a dependency anywhere. LineInput is in-house, and the sidebar tree is built by hand in state.rs. Evidence: architecture.md:420-421; crates/mejl-ui/Cargo.toml:10-24; input.rs:1-31; state.rs:653-751; grep found no tui-input or tui-tree-widget (verified)
- **Palette ranking: frecency.** The doc says the palette ranks by fuzzy score plus frecency of the user's own command use. The code keeps only the last 8 actions chosen from the palette, in recency order, with no frequency. Keys pressed directly do not count. With a filter, the palette ranks by fuzzy score and breaks ties by that order. Evidence: terminal-ux.md:167; update/overlays.rs:15-16, 72-85, 644 (the only caller of remember); picker.rs:269-271 (verified)
- **List and reader split.** The doc recommends the list take about 40 percent of the width. The code's automatic split gives the list 0.545 of the shared space, chosen to match the mock-up's 73/61 columns at 160. Evidence: terminal-ux.md:253; layout.rs:37-39; layout.rs:305-314 (verified)
- **Short terminals.** The doc says that under 20 rows the reader hides its header block and the hint bar becomes optional. The code drops only the hint bar. A grep found no height rule in reader.rs or the view code. Evidence: terminal-ux.md:256; layout.rs:25-26, 160; grep of reader.rs, view/panes.rs, view/mod.rs (verified for the hint bar; the absence is from grep)
- **Density meanings.** The doc defines compact as one line with no preview, comfortable as one line with a preview, and relaxed as two lines (sender and date, then subject and preview). The code's compact drops date headings but still shows the inline preview in three-pane layout. Comfortable adds date headings. Relaxed is the normal row plus a preview line underneath. Evidence: terminal-ux.md:276; state.rs:93-103; update.rs:92-127; view/panes.rs:219-224; settings.rs:76 (verified)
- **List date for older years.** The doc proposes 2024-10-03 for older dates. The list column is a fixed 6 cells and shows Oct'24. Full ISO dates appear only in the reader formats. Evidence: terminal-ux.md:277; dates.rs:7-9, 15-16, 73-87, 101-102 (verified)
- **@ is a prefix, not a single key.** The keymap spec lists @ as a single key for Masked Email actions, and the prefix list names only g . * b z and Space. In the code @ is a prefix in every preset (@ x, @ e, @ d, @ r, @ m, @ y, @ n, @ @), and ctrl-o is a compose prefix. vim also makes r a prefix. Evidence: terminal-ux.md:160, 563; architecture.md:588; preset.rs:88-117, 211-218 (verified)
- **Glyph width claim.** theme.rs says the Unicode glyph set uses only the Neutral East Asian width class. The scrollbar thumb is U+2503 and the border set uses box drawing characters, which terminal-ux.md lists as Ambiguous width. The test checks only unicode-width == 1, not the width class. Evidence: theme.rs:147-149, 188, 224-232, 812-831; terminal-ux.md:94 (verified for the code; that U+2503 is Ambiguous is inferred from the doc's statement that all box drawing characters are)
- **Every key remappable.** The architecture principle says every key is remappable. Picker keys (Esc, Enter, arrows, Ctrl-p, Ctrl-n, PgUp, PgDn, Tab, Ctrl-y, Space), prompt keys, the confirm y key, the capture Esc, the perf any-key close and LineInput's editing keys are hard-coded. Pickers and prompts have no keymap context. Evidence: architecture.md:38; terminal-ux.md:200; update/overlays.rs:584-624, 785-804; update.rs:203-221; update/keys.rs:382; input.rs:112-184; keymap/mod.rs:58-85 (verified)
- **Hint text not taken from the keymap.** Help and the hint bar are said to read from the same keymap. Most mail hints do, through keys_for. Overlay hints, the pending-prefix hints (Esc, ? all keys), picker hints and screen hints such as 1-4 and Left/Right are literal strings, so they would be wrong after a rebind. The Masked hint says 1-4 while 1-6 are bound. Evidence: view/mod.rs:799-900; view/picker.rs:19-44; view/screens.rs:1508-1514, 1551-1557; preset.rs:539-544 (verified)
- **Lockout enforced only in the key editor.** The editor keeps the last key of Back, Help, Palette, QuitNow, GoSettings and its own actions. Keymap::build fails only on prefix conflicts, so a hand-written [keys] table can unbind every Back or Help key and still load. The doc describes the rule only for the editor, so this is a gap, not a contradiction. Evidence: keys_screen.rs:29-37, 277-280, 529-538, 559-575; keymap/mod.rs:343-380; keymap/config.rs:209-210; terminal-ux.md:183 (verified)
- **Lockout covers quit-now, not quit.** The workflow task described the lockout as covering quit. The code and terminal-ux.md both protect QuitNow (Q). Quit (q) can lose its last key in the editor. Evidence: keys_screen.rs:31-37; terminal-ux.md:183 (verified)
- **UnknownPreset error text.** The error text tells the user to use fastmail or vim and leaves out gmail, which is a built-in preset. Evidence: keymap/config.rs:74-76; preset.rs:20-35 (verified)
- **Latency budgets claimed as enforced.** architecture.md says budgets are enforced in CI and lists a TestBackend benchmark for 100k scrolling with p99 under 8 ms and a palette perf test under 1 ms. Test comments say the real budget is checked in release benchmarks. No mejl-ui benches exist; only crates/mejl-store/benches/store.rs, which benches store reads. CI runs cargo test in debug, and only on release tags. The tests assert debug bounds of 100, 60 and 30 ms and a growth ratio. The 8 ms and 1 ms budgets are asserted nowhere. Evidence: architecture.md:41, 454-455, 459, 471; tests/scale.rs:63-66; tests/perf_screens.rs:23-25; tests/compose_editor.rs:83-93; crates/mejl-store/benches/store.rs:68-105; .github/workflows/release.yml:42-44 (verified)
- **Snapshot coverage.** architecture.md promises every screen at 80, 120 and 200 columns in light and mono themes. The snapshots are at 80x24, 120x36 and 160x40. There is one ascii-mono snapshot, a dark-theme style check and no light-theme snapshot. Evidence: architecture.md:485; tests/snapshots listing; tests/screens.rs:96-117, 156-170 (verified)
- **Disk reads on the UI thread.** terminal-ux.md says the UI thread never waits on the network or disk. MailData is a synchronous store read on the UI thread with a 2 ms p99 budget. architecture.md records this as provisionally resolved, pending a benchmark. Evidence: terminal-ux.md:12, 398-400; architecture.md:32, 163, 601; data/mod.rs:1-7 (verified)
- **MailData boundedness.** data/mod.rs says every call is a bounded, indexed read. masked_emails, rules, drafts and saved_searches return whole collections, and opening the palette reads all of them plus 200 contacts. Evidence: data/mod.rs:3-7, 574-580, 610-644; update/overlays.rs:152-207 (verified)
- **Accessible mode only partly built.** The doc's accessible mode has no spinners, no animations, no decorative box drawing, a text marker on the focused row, and announcements in the status line. The code parks the cursor on the focused row and parks it on the status line when a toast arrives. Nothing else depends on config.accessible, and box drawing is still drawn. Evidence: terminal-ux.md:463-465; view/mod.rs:90-99; update.rs:164; grep for accessible found only those plus view/screens.rs:1432 (verified)
- **Mask disable: confirm and undo.** The architecture says reversible actions use undo instead of confirmation, and lists mask disable mitigation as Confirm plus undo. Three code paths behave three ways. On the Masked screen, x disables with no confirm and shows an undo toast. From a message, @ x asks a confirm. From the @ @ menu, the disable row runs at once with no confirm and no undo hint. Evidence: architecture.md:37, 570; update/masked.rs:228-246, 373-376; update/overlays.rs:531-538, 767-779 (verified)
- **Discard draft.** The compose key table says Ctrl-o then d discards the draft and is undoable for a short time. The code asks a y/N confirm instead. Evidence: terminal-ux.md:576; update/compose.rs:285-288; tests/compose.rs:368 (verified)
- **q and Esc unwind to different depths.** On Masked and Rules, the screen handles Back step by step: Masked ends filtering, then clears the filter, then leaves; Rules goes from the form to the list, then leaves. Quit is not handled there. It falls through to back(), which sends the screen straight to Mail. In the key editor, Back and Quit behave the same. Evidence: update/masked.rs:281-290; update/rules.rs:717-723; update/screens.rs:22-29; update.rs:402-406, 776-779; update/keys.rs:119-126 (verified)
- **u as back.** The doc's global table says Esc and u back out one level. In the fastmail and gmail key editors, u is bound to key-reset, which shadows the global back. In the vim preset, u is undo everywhere. Evidence: terminal-ux.md:526; preset.rs:141, 270, 373, 604 (verified)
- **Help reachability.** The doc's layering makes ? help available everywhere. In the search screen only the Search context is active and it binds no help or palette key, so ? and : type into the query. In help itself q types into the filter. Compose fields reach help only through Ctrl-o ?. Evidence: terminal-ux.md:171-177; state.rs:1120-1132; preset.rs:488-529; tests/screens.rs:59-64; tests/keys.rs:316 (verified)
- **Hit test model.** The doc's inferred design stores the list area plus the offset with no per-row regions, and gives regions an explicit z. The code records one region per drawn row plus a marker region, and uses draw order as the z order. Evidence: terminal-ux.md:232-235; view/panes.rs:255, 279-283; hit.rs:100-115 (verified)
- **Compose markdown preview key.** The doc lists Ctrl-o then p to toggle a markdown preview. The compose table has no such binding. It binds Ctrl-o s, a, m, e, d, f, b, : and ?. Evidence: terminal-ux.md:574; preset.rs:499-513 (verified)
- **Settings screen scope.** The UX spec describes settings as a read-mostly view of the effective config and the detected terminal capabilities, with the config file path and a probe-again action. The built screen has 8 rows that change value and 3 read-only rows. It shows no capabilities, no path and no probe action. Evidence: terminal-ux.md:498; settings.rs:39-49; view/screens.rs:1354-1470 (verified)
- **Hint bar and the spela visible-key rule (tension).** mejl's hint bar drops non-important hints and shortens labels to fit the width, so some bound keys are not shown at 80 columns. Spela's review contract sends only keys displayed in the current frame. Evidence: view/mod.rs:956-991; spela/AGENTS.md:79; spela/docs/tui-testing.md:128-129 (verified)
- **Key model collides with spela's command keys (tension).** mejl's commands are letters and punctuation with prefixes (g . * @ b ctrl-o :, ?). Spela's TUI contract says navigation and commands need only numbers, arrows, Tab, Enter and Space, that punctuation is ordinary text only inside an input, and that Help and Quit are Actions rows. Evidence: preset.rs:120-219; spela/AGENTS.md:79-81; spela/docs/tui-testing.md:127-131, 150 (verified)
- **Layout breakpoints collide with spela (tension).** mejl uses three panes from 160 columns, two from 100, and one below that with a full-screen reader, where Tab does nothing; its floor is 30x8. Spela requires that at 80x24 Tab reveals the other pane, that 120x40 shows both panes, and that a resize prompt replaces the workspace below 80x24. Evidence: layout.rs:21-24, 142-144; update.rs:818-820, 1078-1081; spela/docs/tui-testing.md:201-203; spela/AGENTS.md:95-96 (verified)
- **Confirmation style collides with spela (tension).** mejl's confirm is a status-line question where y confirms and any other key cancels, and undo replaces confirmation for reversible changes. Spela's confirmations start on Cancel, need an explicit selection, can show scrollable details, and spela has a Confirm destructive actions setting. Evidence: update.rs:203-212; view/mod.rs:652-664; architecture.md:37; spela/docs/tui-testing.md:176-180 (verified)
- **Digits as keys (tension).** mejl uses digits as vim counts and as Masked screen filters. Spela uses digits as destination commands (text:0 opens Actions). Evidence: keymap/mod.rs:525-540; preset.rs:539-544; spela/AGENTS.md:84-85 (verified)

### Open questions

- Should the spela rewrite keep spela's own key contract, where commands use only numbers, arrows, Tab, Enter and Space, punctuation is text only inside inputs, and Help and Quit are Actions rows (spela/AGENTS.md:79-81; spela/docs/tui-testing.md:127-131, 150)? Or should it adopt mejl's letter and punctuation keymap with a : palette, ? help and q/Q quit? This one decision decides how much of mejl's keymap and presets carries over.
- Which layout rules win: spela's 80x24 with Tab to reveal the other pane, 120x40 with both panes, and a resize prompt below 80x24, or mejl's 100/160 breakpoints, stack layout and 30x8 floor?
- For spela's destructive and privileged operations (DLL swap, restore and install, GPU clocks and power limits applied through pkexec), should undo replace confirmation as in mejl, or should spela keep confirmations that start on Cancel and its Confirm destructive actions setting? Many spela operations are probably not cheaply undoable (inferred).
- Is the in-app key editor with custom presets stored as diffs (about 4,600 lines in mejl) wanted for full parity, or is it YAGNI? Does the current Go TUI have anything like it? (Unverified here; the Go code is outside this scope.)
- Is the macro system wanted at all? mejl's macros only batch mail changes and do not map onto any spela concept I know of (inferred).
- Should mouse capture be on by default in the rewrite? Spela's review procedure is keyboard-only through termctrl, and capture takes the terminal's text selection away.
- Should the rewrite enforce the lockout rule (Back, Help, Palette, QuitNow, Settings always keep a key) when it loads a hand-written key config too? mejl enforces it only in the editor.
- Should spela adopt mejl's hint bar fitting? It drops and shortens hints by width, which conflicts with spela's rule of sending only keys visible in the frame.
- Should the 8 ms frame budget become a real release-mode benchmark in the rewrite? mejl's UI tests only check loose debug-build bounds, and its comments refer to benchmarks that do not exist.
- Live test: does typing q inside the help overlay (it filters instead of closing) or inside search (it types) surprise users enough to change the rule that text fields own typing?
- Spela stores config in YAML (spela/AGENTS.md:39-40). Does the rewrite keep YAML, and if so, which Rust library can edit it in place while keeping comments, the role toml_edit plays in mejl? (Unverified.)
- Does spela need accessible mode with a cursor parked on the focused row? mejl built only that part of its accessibility plan.

## mejl binary crate, terminal layer, tests and infrastructure

Scope: `crates/mejl/src/**`, `crates/mejl/tests/**`, every `Cargo.toml`, the `justfile`, `.github/workflows/*` and `.github/codeql/*`. It also covers the transferable patterns of mejl-jmap, mejl-fake, mejl-store, mejl-sync and mejl-render. Comparison targets are `docs/architecture.md` and `docs/terminal-ux.md`.

Labels:
- **(verified: path:line)** means read in the code or doc.
- **(inferred)** means reasoned from code I read.
- **(unverified)** means not checked.

All paths are relative to `/home/jgabor/git/mejl`. "The doc says" and "the code does" are kept apart. Mismatches are listed in the conflicts output and summarised at the end.

### 1. Crate shape

- `main.rs` only calls `mejl::cli::main()` (verified: crates/mejl/src/main.rs:3-5). Everything else is a library, so it can be tested without a terminal, network or real directories (verified: crates/mejl/src/lib.rs:1-5).
- The module map, with the thread summary in the crate docs (verified: lib.rs:7-32):
  - `cli`, `paths`, `config`, `secrets`, `login`, `oauth_login`, `oauth`
  - `lock`, `terminal`, `events`, `input`, `timer`, `render_pool`
  - `demo`, `logging`, `doctor`, `keys`, `app`, `tui`
- `#![warn(missing_docs)]` is on in the binary crate (verified: lib.rs:34).
- The crate has a `[lib]` and a `[[bin]]`, both named `mejl` (verified: crates/mejl/Cargo.toml:10-16).

### 2. Process and thread model

#### 2.1 Thread inventory during a TUI run

| Thread | Created at | Role | How it reaches the UI thread |
|---|---|---|---|
| main (UI thread) | `cli::main` then `tui::run_inside` | Owns `Terminal<CrosstermBackend<Stdout>>`, `App`, `AppState` and the store `Reader`. Runs update, exec and draw. | Receives on `LoopReceiver` |
| `mejl-input` | input.rs:344-346 | `futures::executor::block_on` over crossterm `EventStream`, selected against a control channel | `LoopEvent::Input`, `LoopEvent::InputClosed` |
| crossterm's reader (unnamed) | Started by `EventStream` (verified comment: input.rs:12-16) | Polls the tty for the stream | Wakes the input thread's waker |
| `mejl-signals` | terminal/signals.rs:107-126 | signal-hook iterator for TERM, INT, HUP, CONT | `LoopEvent::Signal`. On a second stop signal it restores the terminal and exits itself (signals.rs:117-120) |
| `mejl-timer` | timer.rs:87-89 | Sleeps on a condvar until the earliest deadline | `LoopEvent::Tick`, `LoopEvent::Wake(Instant)` |
| `mejl-render-0`, `mejl-render-1` | render_pool.rs:322-328. Count is `min(2, cores)` (render_pool.rs:292-294) | Body rendering under `catch_unwind` | `LoopEvent::Rendered` |
| `mejl-io` (tokio, 1 worker) | app/mod.rs:441-446 | Attachment downloads, login-retry sleeps | `LoopEvent::Msg(AppEvent::Downloaded / RetryLogin)` (app/mod.rs:1507-1516, 1604-1607) |
| `mejl-sync` (engine driver) | mejl-sync/src/engine.rs:110-119 | `runtime.block_on(main)` and command dispatch | Sink closure |
| `mejl-sync-worker` x2 | engine.rs:83-87 | Network tasks: sync loop, push, flush, drafts, bodies, masked | Sink closure, which the binary turns into `Msg(AppEvent::Sync)` (tui.rs:146-149) |
| `mejl-store` | mejl-sync/src/store_thread.rs:274-275 | Owns the `Store` writer connections and runs jobs | Events through the sink, oneshot replies to async callers |
| tracing-appender worker | logging.rs:271 | Writes log files | None (inferred: `non_blocking` runs a writer thread) |
| `mejl-demo` (demo only) | demo.rs:77-83 | axum fake server and the mail simulator | None, it is reached over loopback HTTP |

#### 2.2 Channels and shared state

| Path | Mechanism | Evidence |
|---|---|---|
| Everything to the UI thread | Unbounded `std::sync::mpsc` of `LoopEvent<M>`. `M` is a type parameter, so the module knows no UI or sync types | events.rs:1-11, 46-62, 138-142 |
| UI thread to the input thread | futures unbounded mpsc of `Control { Pause(ack), Resume, Stop }`. The ack is a std mpsc with a 2 s timeout | input.rs:301-305, 343, 355-364; `PAUSE_TIMEOUT` input.rs:46 |
| UI thread to the timer | `Mutex<State>` plus `Condvar`. Wakes are a `BTreeSet<Instant>` | timer.rs:30-42, 106-127 |
| UI thread to the render workers | `Mutex<PoolState>` plus `Condvar` over a bounded queue where the newest request wins | render_pool.rs:88-180, 262-271, 355-359 |
| UI thread to the engine | `SyncHandle::send(Command)` over a tokio unbounded mpsc. It never blocks and returns false once the engine has stopped | mejl-sync/src/engine.rs:95, 161-164 |
| Engine to the UI thread | `sink: impl Fn(SyncEvent) + Send + Sync`, called "from several threads". It must return quickly. Data changes are coalesced on a task first | engine.rs:41-43, 210-215 |
| Engine to the store thread | std mpsc of boxed `FnOnce(&mut StoreCtx)` jobs, run one at a time in order. Async callers get a oneshot reply | store_thread.rs:1-6, 193-199, 286-292 |
| Body fetches | tokio unbounded mpsc of `BodyMsg` | engine.rs:96 |
| Engine shutdown | std mpsc done signals. `shutdown()` waits up to twice the timeout | engine.rs:108, 141-146, 178-201 |
| UI reads | The UI thread opens its own read connection from `ReaderFactory` and does small indexed reads synchronously | app/mod.rs:422-427; app/data.rs:1-6; mejl-store/src/lib.rs:16-24 |

#### 2.3 Who owns the terminal

- **Frames and side writes.** Only the UI thread draws frames. `TerminalHost` writes the sync markers and OSC 52 through separate `io::stdout()` handles on the same thread (verified: app/host.rs:71-93, 103-106).
- **Mode state.** Terminal mode state is a global `static ACTIVE: Mutex<Option<TerminalOptions>>`, so restore is idempotent and callable from any thread (verified: terminal/mod.rs:104, 166-172).
- **Restore from other threads.** Two other threads can restore the terminal: the signal thread on a second stop signal (signals.rs:117-120), and any thread through the panic hook (terminal/mod.rs:315-331).
- **Raw mode during the probe.** The probe opens `/dev/tty` and switches raw mode on and off itself before the TUI exists (verified: probe.rs:671-691).

#### 2.4 Doc comparison

- architecture.md:138-157 draws four boxes: input thread, UI thread, one tokio runtime, store thread.
  - The code has three tokio runtimes: engine, `mejl-io` and demo (verified: engine.rs:83-87; app/mod.rs:441-446; demo.rs:77-83).
  - It also has a dedicated render pool, a timer thread and a signal thread.
- The doc says rendering runs on tokio `spawn_blocking` (architecture.md:152, 290). The code uses std threads with no timeout (render_pool.rs:1-5, 382-402).
- The doc says the input thread uses a blocking `crossterm::event::read` and that the `event-stream` feature is not needed (architecture.md:159, 164). The code needs `event-stream` (crates/mejl/Cargo.toml:31) and uses `EventStream` so a pause can stop reading at once (input.rs:7-10).
- "UI thread never awaits and never blocks on the network" (mejl AGENTS.md:35) holds. The UI thread does block in three places (app/mod.rs:22-27):
  - the external editor
  - Ctrl-z suspend
  - engine shutdown on quit

  It also writes the config file with `sync_all` from the settings screen (app/mod.rs:1034, 1050; settings.rs:656-671).

### 3. Startup and shutdown order

tui.rs:1-14 documents the order, and the code follows it (verified: tui.rs:114-187, 205-260):

1. **Probe.** `probe::detect_with(env, Some(150 ms), caps_cache)` runs before anything else reads the terminal (tui.rs:118).
2. **UI settings.** Key override errors stop here with exit code 3, before the terminal changes (tui.rs:120-134).
3. **Store, channel and engine.** `Store::open`, then the main channel, then the OAuth save-error hook, then `Engine::start` with a sink into the channel (tui.rs:135-150).
4. **Terminal mode.** `terminal::enter(TerminalOptions { mouse: config.ui.mouse, kitty_keyboard: caps == Yes, theme_reports: false })` (tui.rs:152-165). If entering fails, the engine is shut down and an error returned.
5. **Threads and the app.** `InputThread::spawn`, `SignalWatcher::spawn`, `TerminalHost`, `Terminal::new(CrosstermBackend::new(io::stdout()))`, then `App::new` (Reader, Timer, RenderPool, `mejl-io` runtime) (tui.rs:218-246; app/mod.rs:421-492).
6. **First frame.** `app.start` draws it, and the time since process start is logged (tui.rs:247-250; `mark_start` is called first thing in cli.rs:263).
7. **Run and close.** `app.run(rx)`, then `app.close()` (tui.rs:251-257).
8. **Teardown.** The input thread and signal watcher drop, then the terminal guard drops (restore), then `handle.shutdown()`. A slow engine shutdown therefore never leaves a raw terminal (tui.rs:166-186).
9. **Exit report.** `print_report` writes held sends and exit notes to stdout after restore (cli.rs:385-401, 467).

### 4. The event loop

#### 4.1 One turn

`App::run` loops on `rx.recv_batch()` until quit or until every sender is gone (verified: app/mod.rs:531-540). `recv_batch` blocks for one event, then takes everything already waiting with `try_iter` (events.rs:114-124).

`handle_batch` (app/mod.rs:544-592) works like this:

1. Feed the previous frame's `FrameSample` to the UI (545-547).
2. `coalesce_batch` merges runs of input events (392-415). The input thread already coalesced once.
3. Handle each item:
   - **Input:** `mejl_ui::Event::from_crossterm`, then `feed`. `feed` pushes to a local `VecDeque` and drains it, calling `ui::update(&mut state, &data, ev)`, which returns `Vec<Cmd>`, and runs `exec` for each (661-679).
   - **Rendered:** becomes `Msg::BodyRendered` or `Msg::BodyFailed`.
   - **Tick or Wake:** becomes `Msg::Tick`.
   - **Signal:** a stop signal sets quit; Continue calls `repaint_all`.
   - **Msg(Sync):** goes to `sync_event` (1635-1980).
   - **Downloaded, LoginNotSaved, RetryLogin:** handled in place.
4. After each item, drain again. If quit is set, return at once without drawing (585-588).
5. `after_turn` (711-740):
   - quit-after-sends deadline
   - load-more requests
   - one toast per read error
   - held draft copies
   - `timer.set_ticking(state.shows_clock())`
   - `timer.wake_at_timestamp(state.next_wakeup())`
6. `draw(first_input)` once.

Commands are values. `exec` (786 onward) maps each `Cmd` to one of:
- an engine `Command`
- a `Host` call: open a URL, open a path, copy, edit, suspend
- a timer wake
- a local action

Async results are matched by a `u64` token kept in a `HashMap<u64, Waiting>` (143-180, 692-696).

#### 4.2 Drawing and synchronized output

`draw` (app/mod.rs:742-756) runs these steps:
1. `host.frame_begin()` queues `BeginSynchronizedUpdate` (CSI ? 2026 h) when the probe found sync output (host.rs:79-85; tui.rs:228).
2. `terminal.draw(|f| ui::draw(state, data, f))`.
3. `host.frame_end()` queues the end marker and flushes (host.rs:87-93).
4. The frame count goes up.

There is no dirty check: every batch draws (verified). The ratatui diff keeps the bytes small (inferred), but the frame is rebuilt every time. There is no `BufWriter`: the backend wraps `io::stdout()` directly (tui.rs:230). A search of mejl and mejl-ui finds no `BufWriter`.

#### 4.3 Latency measurement

- **What is measured.** `first_input` is the oldest `TimedEvent.at` in the batch (app/mod.rs:548-553). The input thread stamped it at read time (input.rs:401-412). After the draw, `FrameSample { input_to_flush: at.elapsed() }` is stored and fed at the start of the next turn (750-754, 545-547). It lands in mejl-ui's perf ring with buckets (mejl-ui/src/perf.rs:54; view/overlays.rs:402).
- **Granularity.** One sample per frame, not per event.
- **Doc comparison.** The doc says "Record `input_to_flush` for every input event handled in this frame" (architecture.md:174).

#### 4.4 Timers, wake-ups and idle

- **The timer.** It sends Tick only while ticking is on. Ticks land just after each wall-clock second (5 ms slack), and ticks missed after a stall are skipped. One-off wakes are kept in a `BTreeSet<Instant>`, and a past deadline fires at once (verified: timer.rs:1-12, 52-63, 104-139, 170-226).
- **The input thread.** It waits on the stream and the control channel together. The module doc says an idle mejl makes no timer wake-ups (input.rs:7-9). Whether crossterm's internal reader thread is wake-free is unverified.
- **Idle in practice.** `after_turn` turns ticking on whenever `shows_clock()` is true. That is the Mail screen at `TWO_PANE_MIN` width or more, with no prefix pending and no picker open, or compose with a saved time shown (verified: app/mod.rs:736; mejl-ui/src/state.rs:1089-1104). The default view therefore gets one Tick, one update and one full draw per second at rest. This contradicts "An idle mejl does no work" (timer.rs:3; architecture.md:176) and the budget "No timer wake-ups when nothing changes" (architecture.md:461).
- **Other wakes.** `next_wakeup` covers toast expiry, sending-toast seconds, compose autosave, the mark-read delay and the undo-send countdown (mejl-ui/src/state.rs:1010-1037).
- **Duplicate wakes (inferred).** `wake_at_timestamp` converts a wall-clock time to `Instant::now() + delay` on every turn (timer.rs:131-139). A deadline asked for on many turns becomes many slightly different instants, so the "same instant gives one wake-up" dedupe (timer.rs:122-123) does not apply. This is harmless but wasteful during busy periods.

#### 4.5 Render pool (render_pool.rs)

- **Queue rules.** Bounded queue, default 32, with 2 workers (render_pool.rs:289-294). A new request for the same email replaces the waiting one in place. A full queue drops the oldest. A result whose request was replaced while running is dropped (88-180, 404-418).
- **Prepared-body cache.** An LRU of 16 prepared bodies avoids re-parsing HTML on a resize rewrap. The cache lock is never held while preparing (185-260).
- **Panic safety.** Each worker marks its thread as panic-catching and runs `render` under `catch_unwind` (382-402).
- **Doc comparison.** mejl-render's own docs tell callers to use `spawn_blocking` plus `tokio::time::timeout` (mejl-render/src/lib.rs:25-29). The pool does neither. The 1 MiB input cap (mejl-render/src/lib.rs:259-278) is the real bound.

### 5. Terminal setup and teardown (terminal/mod.rs)

#### 5.1 Modes

- `TerminalOptions` defaults: mouse on, bracketed paste on, focus events on, kitty off, theme reports off (terminal/mod.rs:45-72).
- `switch_on` order: raw mode, alternate screen (1049), bracketed paste (2004), focus (1004), mouse, kitty push, theme reports (2031). Each step goes through `execute!`, then a final flush (141-161).
- Mouse is enabled by hand: `EnableMouseButtons` writes `CSI ?1000h CSI ?1002h CSI ?1006h`, with no 1003 or 1015 (85-102). This matches terminal-ux.md:225 and :591.
- Kitty flags are `DISAMBIGUATE_ESCAPE_CODES | REPORT_ALTERNATE_KEYS`, written as `CSI > 5 u`. There are no release events (77-79, test at terminal_pty.rs:263).
- `enter` refuses a second activation. If any step fails, it undoes what was already switched on (127-139).

#### 5.2 Restore

- **Order.** `switch_off` runs in reverse: 2031 off, pop kitty (`CSI < 1 u`), crossterm `DisableMouseCapture`, focus off, paste off, leave the alternate screen and show the cursor, flush, disable raw mode. Every step runs even if an earlier one fails, and the first error is returned (194-223).
- **Proof.** The pty test checks the exact enter and restore sequence and the termios state (terminal_pty.rs:251-287).
- **Mouse detail.** The module doc says restore "undoes exactly what was turned on" (7-9). The mouse step uses crossterm's `DisableMouseCapture`, which turns off more modes than were enabled (inferred from crossterm's mouse capture set, documented at terminal-ux.md:225). This is harmless.
- **Exit paths covered:**
  - guard drop on a normal quit and on error returns (tui.rs:166-181)
  - the panic hook
  - a second stop signal
  - the editor and Ctrl-z, through `suspend_for`
- **Gaps (inferred):**
  - an external SIGTSTP or SIGSTOP leaves the terminal raw on the alternate screen
  - SIGTERM before the signal watcher starts (tui.rs:118-221) gets the default action, and raw mode may be on during the probe

#### 5.3 Panic hook

- **What it does.** `install_panic_hook` logs every panic. It restores the terminal and calls the previous hook only when `panic_is_fatal` (315-331). It is installed in `cli::main` for every command (cli.rs:278).
- **Who counts as non-fatal.** Threads marked with `mark_thread_catches_panics`: render workers, the `mejl-io` and `mejl-demo` runtimes. Also threads named with prefixes `tokio-runtime-worker`, `mejl-sync` or `mejl-store` (268-311). The engine's runtime is built inside mejl-sync, so it cannot be marked; hence the prefix list (282-293).
- **The panic path never blocks.** `restore_after_panic` uses `try_lock`. If the lock is held elsewhere, it restores every mode (176-192).
- **Flaw (inferred).** A panic on a spawned, unmarked thread (`mejl-input`, `mejl-timer`, `mejl-signals`, unnamed) counts as fatal (tests: terminal/mod.rs:343-346). It does not end the process: no `panic = "abort"` is set (Cargo.toml:84-87). The terminal would be restored while the UI thread keeps drawing. If the input thread dies this way, it sends no `InputClosed`, so the app keeps running with no input.

### 6. Signals, suspend and the editor

- **Watched signals.** `SignalWatcher` watches SIGTERM, SIGINT, SIGHUP and SIGCONT (signals.rs:84-96).
  - The first stop signal is forwarded. The loop sets quit, so the outbox and drafts persist and the terminal restores on the way out (app/mod.rs:563-569).
  - A second stop signal restores the terminal and exits with 128 plus the signal number, from the signal thread itself (signals.rs:65-73, 115-120).
  - SIGCONT is forwarded, and the loop calls `repaint_all`: `terminal.clear()`, then a synthetic `Event::Resize` at the current size (app/mod.rs:1282-1290).
- **SIGINT and Ctrl-C.** Raw mode turns Ctrl+C into a key, so SIGINT only comes from `kill` (signals.rs:3-4; terminal/mod.rs:15-16). No preset binds `ctrl-c`: the q and Q quit keys are at mejl-ui keymap/preset.rs:143-144, 272-273, 370-371, and a search finds no ctrl-c binding.
- **SIGWINCH.** Not watched by mejl. Resizes arrive as crossterm `Event::Resize` and are coalesced (input.rs:75-81).
- **SIGTSTP and Ctrl-z.** Ctrl-z is a key bound to `Action::Suspend` in all three presets (mejl-ui keymap/preset.rs:145, 274, 372). It is handled before overlays and text inputs (mejl-ui update.rs:195-199) and produces `Cmd::Suspend`. The path is `TerminalHost::suspend`: pause the input thread, then `terminal::suspend_process()`, which is `suspend_for(|| libc::raise(SIGTSTP))` with the default action, then resume input, then `repaint_all` (host.rs:129-145; terminal/mod.rs:253-266; app/mod.rs:853-858).
  - The pty test proves the child really stops with a cooked tty and comes back in TUI mode after SIGCONT (terminal_pty.rs:314-358).
  - The headless app test covers Ctrl-z from every context and a redraw at a new size (tests/app.rs:1596-1658).
- **$EDITOR.** `EditorCommand::from_env`: `VISUAL`, then `EDITOR`, then `vi`. Values are split like shell words, and an unclosed quote is skipped (editor.rs:23-103).
  - `edit_text` writes a 0600 tempfile with a suffix (`.md` from the host), runs the editor in the foreground with inherited stdio, and reads the result back by path, because editors often replace the file (editor.rs:128-155).
  - A non-zero exit means the text is unchanged. Non-UTF-8 text is an error.
  - `TerminalHost::edit` pauses input, which also waits up to 500 ms for crossterm's reader to release its waker so the editor gets every key. It then runs `suspend_for`, then resumes (host.rs:108-127; input.rs:200-299, 437-449).
  - The app then clears the terminal and feeds `EditorDone` (app/mod.rs:864-872).
  - The pty test proves the editor receives every key (terminal_pty.rs:360-372, 451-472).

### 7. Capability probe and cache

#### 7.1 Environment and query burst

- **Environment (probe.rs:175-204).**
  - Truecolor from `COLORTERM` or a `TERM` ending in `direct` or `truecolor`.
  - `NO_COLOR` when non-empty.
  - `TERM`, `TERM_PROGRAM`.
  - Multiplexer: TMUX, then ZELLIJ, then STY.
  - `SSH_TTY`.
- **One write** (probe.rs:216-224), in this order:
  - `CSI ? u` (kitty flags)
  - `CSI ? 2026 $ p` (DECRQM sync)
  - `CSI ? 996 n` (theme mode)
  - `OSC 11 ; ?` (background)
  - `CSI > q` (XTVERSION)
  - `CSI c` (DA1) last, as the sentinel

  DECRQM 2027 is not included, although terminal-ux.md:75 lists it.
- **Parsing.** A small parser for CSI, OSC and DCS handles both BEL and ST terminators, skips noise and stops at a cut-off sequence (246-380). X11 colour specs are scaled to 8 bits (385-420). Light or dark comes from luma below 128 (207-214).
- **Answers.** Once DA1 has arrived, a missing answer means No. Without DA1 it means Unknown. A theme answer outranks the background colour (150-173).

#### 7.2 Timing

- **Defaults.** Timeout 150 ms (probe.rs:422-424). The quirks table can skip the probe or lengthen it (449-453). It runs only when stdin and stdout are both ttys (454-456, 662-666).
- **/dev/tty.** `probe_tty` opens `/dev/tty`, turns raw mode on if it is off, writes, and reads with `libc::poll` (670-733).
- **Read loop.** `run_probe_with` reads until DA1 or the timeout, with an 8192-byte cap (480-551).
- **Drain.** After a timeout, `drain_late_replies` keeps reading. It stops at 50 ms of quiet unless a reply is half read (`ReplyScanner` state machine), with a 500 ms cap. A DA1 found while draining still marks the probe complete (483-497, 553-660). Keys typed meanwhile are lost by design (probe.rs:21-28; terminal-ux.md:429).
- **Proof.** The pty test sends half an OSC 11 reply before the timeout and the rest after it. Only the key typed after the probe reaches the reader (terminal_pty.rs:374-402).
- **A second line of defence.** `input::drop_leaked_replies` removes whole OSC and DCS replies that crossterm turned into Alt-key runs within one batch (input.rs:92-166). CSI replies are not covered.

#### 7.3 caps.toml (terminal/cache.rs)

- **Location.** `<cache>/caps.toml`, version 1, newest first, at most 16 entries (cache.rs:30-47).
- **Contents.** Only kitty keyboard, sync output, theme reports and the XTVERSION name. Never the background or size (cache.rs:8-10, 49-63).
- **Key.** `term=..;program=..;version=$TERM_PROGRAM_VERSION;mux=..;ssh=..`, built from the environment only (94-108). XTVERSION is not part of the key, unlike terminal-ux.md:77. A differing XTVERSION rejects and forgets the entry (191-194, 227-233), which matches architecture.md:186.
- **Writes.** Only a complete probe with all three answers definite is written, and only when it changed (138-163). The write is atomic: tempfile and rename, without fsync (174-185).
- **Use.** The cache fills only Unknown answers after a timed-out probe (191-210, 216-240). Demo mode passes no cache path (tui.rs:106-117). `mejl doctor` uses no cache (cli.rs:824-826), so doctor may show Unknown where the TUI would use cached answers (inferred).

#### 7.4 Quirks (terminal/quirks.rs)

- Keyed by `TERM` or `TERM_PROGRAM`. The first match wins (quirks.rs:44-73).
- `TERM=dumb` and `TERM=eterm*` skip the probe. `TERM_PROGRAM=vscode` gets 400 ms.
- Every entry documents why it exists and whether that is verified (quirks.rs:9-13).
- There is no XTVERSION liar table, which terminal-ux.md:78 also says.

#### 7.5 OSC 52, OSC 8 and notifications (terminal/osc.rs)

- **OSC 52.** Written by hand, because "the OSC 52 crates on crates.io are copyleft" (osc.rs:3-4).
  - Own base64 encoder, `ESC ] 52 ; c ; <b64> ESC \`, capped at 74,990 bytes so the sequence stays under 100,000. Larger copies fail loudly (15-74).
  - The app first strips control and bidi characters (app/mod.rs:325-333) and writes through the host (host.rs:103-106).
  - The crossterm `osc52` feature is enabled in the workspace (Cargo.toml:25) but never used.
- **OSC 8.** `hyperlink` and `link_target` allow only http, https and mailto, percent-encode everything outside printable ASCII and cap URLs at 2048 (79-117).
  - Nothing outside tests calls them (verified by search).
  - Doctor still prints `links (OSC 8): on` unless GNU screen is detected (doctor.rs:305-313; probe.rs:132-136).
- **Notifications.** OSC 9, 99 and 777 are not implemented anywhere in the workspace (verified by search). terminal-ux.md:43 and :108 recommend them.

#### 7.6 Theme detection (terminal/theme.rs)

- Mode 2031 enable and disable commands and a report parser exist (theme.rs:25-94). `events::TerminalReport` is defined but nothing produces it (events.rs:30-42).
- The mode is never switched on. crossterm 0.29 holds and then drops keys after `CSI ? 997 ; 1 n` (theme.rs:9-16; tui.rs:155-157).
- The probe's `background`, `background_rgb` and `theme_reports` are used only by doctor and the cache (verified by search).
- The config accepts `auto:<name>` themes (config.rs:16, 205, 702-705, 813). The runtime passes the full name to `mejl_ui::Theme::named`, which has no `auto:` arm (mejl-ui/src/theme.rs:307-317). It falls back to the default theme with a warning (settings.rs:136-150).

### 8. Input path (input.rs, keys.rs)

- **Event sources.** The `EventSource` trait has `open()` returning a stream, and `drain_ready(max)`. `TerminalSource` uses crossterm `EventStream` and `poll(0)` plus `read()` (input.rs:168-198). `FakeSource` exists for tests (470-573).
- **Per event read:**
  1. Stamp `Instant::now()`.
  2. Drain up to 1024 waiting events (`MAX_BATCH`, so a huge paste cannot starve the loop) (40-42, 400-412).
  3. Run `drop_leaked_replies`, then `coalesce`: runs of Moved with the same modifiers, Drag with the same button and Resize keep the first timestamp and the last event. Wheel, paste and focus pass through (56-90, 414).
  4. Send `LoopEvent::Input`.
  5. On a read error, events already read are delivered first, then `InputClosed` (419-436).
- **Pause.** A pause drops the stream and waits until crossterm's reader thread has released the cloned waker. A `Tracked` stream wrapper counts `Arc` clones of a shared `WakerTrack` to know this. Only then is the pause acknowledged (200-299, 426-449). A lagging-reader test proves no key is stolen (793-919).
- **Into the UI.** `mejl_ui::Event::from_crossterm` maps Key (dropping Release), Mouse, Paste, Resize and Focus (mejl-ui/src/event.rs:58-69). `asked_by_user` marks keys, pastes and clicks for draft-copy timing (app/mod.rs:205-215). Focus lost flushes held draft copies (app/mod.rs:668-677).
- **`mejl keys --check` (keys.rs).**
  1. Parse `[keys]`, every `[presets.<name>]` and `[macros]` the same way the app does (`settings::key_setup`).
  2. Build the active preset, then every other custom preset, with `Keymap::build`.
  3. Merge `keymap.warnings()` and `keymap.check()`. Severity Error, or any parse error, sets `has_errors`.
  4. Print a summary line plus `preset <name>:` prefixed lines.

  Errors exit 3 (keys.rs:1-112; cli.rs:845-864). Without `--check` it prints a hint and exits 0 (cli.rs:851-854).

### 9. CLI (cli.rs)

#### 9.1 Structure

The surface (cli.rs:1-17, 71-155):

| Form | Flags | Notes |
|---|---|---|
| `mejl` | none | Runs the TUI |
| `mejl --demo` | none | Conflicts with subcommands (`args_conflicts_with_subcommands = true`) |
| `mejl login` | `--oauth`, `--token` (mutually exclusive), `--port <1..>` (implies OAuth, conflicts with `--token`) | |
| `mejl logout` | `--yes` | |
| `mejl doctor` | `--print-config`, `--no-probe` | |
| `mejl keys` | `--check` | |

- Global flags work before or after the subcommand (tested at cli.rs:927-929): `--config <PATH>`, `--account <NAME>`, `--log-level off|error|warn|info|debug|trace`.
- `--version` comes from Cargo, and `--help` carries an `after_help` block listing exit codes and the env vars `MEJL_TOKEN`, `MEJL_LOG`, `NO_COLOR` (cli.rs:57-79).
- There is no machine-readable output mode. A `--json` CLI is listed as later work (architecture.md:545).

#### 9.2 Exit codes

| Code | Meaning | Evidence |
|---|---|---|
| 0 | OK | cli.rs:42-55 |
| 1 | Failure | cli.rs:42-55 |
| 2 | Usage (clap's own code) | cli.rs:42-55 |
| 3 | Config error | cli.rs:42-55 |
| 4 | Not logged in or token refused | cli.rs:42-55 |
| 5 | Account open in another mejl | cli.rs:42-55 |

- `CliError { code, message }` with `From` impls for `ConfigError`, `LockError`, `SecretError`, `LoginError` and `io::Error` (158-218).
- `Cli::parse()` lets clap exit by itself for usage (2), help and version (0) (264). Tested in binary.rs:73-92 and cli.rs:932-955.

#### 9.3 Process entry and output conventions

`cli::main` (262-303) runs in this order:
1. `mark_start`, then parse arguments.
2. `Env::from_process` and `Paths::from_system`. A failure prints `mejl: ...` and exits 1.
3. Apply `--config`.
4. Start logging. A failure prints `mejl: ...; logging is off` and continues.
5. Install the panic hook.
6. Build a `Context` with the real keyring (lazy: `KeyringStore::new` only stores the service name, secrets.rs:88-99), the locked stdin, tty flags, `open::that_detached` and the OAuth timeout.
7. `run(cli, ctx, stdout, stderr)`.
8. Errors are logged, printed as `mejl: <message>` on stderr, and turned into an `ExitCode`.

Output conventions:
- `run(cli, ctx, out, err)` (325-353) is the testable seam: results go to `out`, warnings and prompts to `err`, and prompts read the injected stdin.
- Config warnings print as `mejl: warning in <path>: line L, column C: ...` (355-366).
- The test harness builds `Context` over `Paths::under(tempdir)` and `MemoryStore`, and captures both writers (cli.rs:963-1021).

#### 9.4 Commands

- **`run_tui`** (403-468) runs these steps in order:
  1. Load the config.
  2. Select the account.
  3. Create the account dir with mode 0700.
  4. Take the instance lock (exit 5).
  5. Resolve the login (exit 4 with a hint).
  6. Require a terminal (exit 1, "mejl needs an interactive terminal").

  The terminal check comes last on purpose, so config and auth errors read the same with or without a tty (423-425).
- **`run_demo`** (477-537): copies the user's config into the demo's temp dir so settings changes never touch the real file. It seeds local-only data, uses a real `HttpTransport` against loopback, forces push on, uses no caps cache, and hides held sends in the report.
- **`login`** (539-688): takes the instance lock. On a tty it asks "1 browser / 2 token". Without one it reads a token from stdin. The token prompt reads raw-mode key events without echo (login.rs:398-526).
- **`logout`** (690-810): takes the lock if the dir exists, revokes OAuth, deletes keyring entries, and removes `cache.db` with its `-wal`, `-shm` and `-journal` files. It asks before removing `state.db` unless `--yes`.
- **`doctor`** (812-843; doctor.rs):
  - Sections: Paths, Config, Account, other accounts, stored accounts not in the config, and Terminal (size, TERM, program, XTVERSION, truecolor, colour, background, theme reports, kitty, sync, OSC 8, clipboard note, multiplexer, SSH, probe status).
  - Rows are `  {label:<15} {value}` (doctor.rs:114-116).
  - It never prints a token, never runs `token_command` and never refreshes. Config errors exit 3. It probes only when both stdin and stdout are ttys and `--no-probe` is not given.

### 10. Config (config.rs) and write-back (app/settings.rs)

#### 10.1 Schema and defaults

Every key is optional, and a missing file means defaults (config.rs:1-36).

| Key | Default | Notes |
|---|---|---|
| `default_account` | none | |
| `accounts.<name>.session_url` | Fastmail URL | |
| `accounts.<name>.username` | none | |
| `accounts.<name>.token_command` | none | |
| `accounts.<name>.oauth_client_id` | none | |
| `ui.theme` | `terminal` | |
| `ui.preset` | `fastmail` | |
| `ui.density` | `comfortable` | |
| `ui.prefer_html` | false | |
| `ui.undo_send_seconds` | 10 | Range 0-300 |
| `ui.mark_read_delay_ms` | 0 | Range 0-60000 |
| `ui.mark_read_on_archive` | false | |
| `ui.remote_images` | `off` | |
| `ui.mouse` | true | |
| `keys` | empty | Raw table passed to the UI |
| `presets.<name>.base` | `fastmail` | |
| `presets.<name>.keys` | empty | |
| `macros.<name>` | none | Raw table |
| `sync.push` | true | |
| `sync.poll_interval_secs` | 300 | Range 30-86400 |
| `sync.prefetch` | 3 | Range 0-50 |
| `sync.body_cache_mb` | 256 | Range 16-65536 |

Structs use `serde(default)` and kebab-case enums (55-278). There are no terminal capability keys (`keyboard.kitty`, `render.sync`, `links.osc8`, `clipboard.osc52` and so on), unlike terminal-ux.md:79.

#### 10.2 Validation

- **Two passes.** `parse` reads the text twice: `toml::de::DeTable` for spans and `toml::from_str` for typed values (381-404).
- **Unknown keys** are warnings, not errors, so a newer config still loads. They carry line and column and a did-you-mean based on edit distance of at most max(2, len/4) (442-539).
- **Errors** carry line and column, are sorted by position, and are all reported at once (558-700):
  - type errors
  - out-of-range values
  - bad preset names
  - unknown `ui.preset`
  - bad account names
  - session URLs that are not https, except http on loopback (710-744)
  - an empty `token_command`
  - a bad `oauth_client_id`
  - a `default_account` with no section
- **Warnings** include an unknown theme and an `oauth_client_id` overridden by `token_command`.
- **Messages** are plain sentences, for example "`ui.undo_send_seconds` is 400, but must be between 0 and 300" (565-577).
- **Account selection** (747-778): an explicit name, else `default_account`, else the only account, else the implicit `default`. A typo never lands on a fresh empty account.
- **Example printer.** `example_config()` prints every key commented out with its default (786-874). A test checks it parses and matches the defaults (config.rs:1156).

#### 10.3 toml_edit write-back

- **Single settings.** `edit_text` parses into `DocumentMut`, creates `[ui]` if it is missing (inline tables included), and replaces a value while keeping its decor (comment and spacing) (settings.rs:317-344).
- **Key editor changes.** Custom presets and macros go through `edit_keys_text` (346-650).
- **Write safety.** The new text is re-parsed with `config::parse` before replacing. `write_atomic` creates the dir private if missing, writes a tempfile in the same dir, calls `sync_all`, copies the old file's permissions, then renames (255-266, 654-671).
- **Failure handling.** If writing fails, a plain setting still applies in memory with a toast (200-226). Key editor changes apply only when the file is written (236-253).
- **Tests** cover keeping comments, adding sections and refusing broken files (673-1041).

### 11. Paths, lock, logging, env, demo

- **paths.rs.** `etcetera::choose_native_strategy("org", "mejl", "mejl")`, which is XDG on Linux and honours `XDG_*_HOME`. The state dir falls back to `<data>/state` (paths.rs:57-77).
  - Layout: `<config>/config.toml`, `<data>/accounts/<name>/` (mode 0700: cache.db, state.db, mejl.lock), `<cache>/caps.toml`, `<state>/log/` (9-16).
  - `Paths::under(base)` serves tests and the demo (79-90).
  - Account names are 1-64 ASCII characters `[A-Za-z0-9-_.]`, with no leading dot (127-148).
  - `create_private_dir` also tightens an existing dir to 0700 (150-172).
- **lock.rs.** One mejl per account.
  - Advisory `flock(LOCK_EX|LOCK_NB)` on `<account>/mejl.lock`, mode 0600. The kernel releases it on exit, so a stale lock is impossible. `O_CLOEXEC` keeps child editors from inheriting it (lock.rs:1-12, 137-169).
  - The pid is written only for the error message (99-103).
  - A held lock is retried for 100 ms, to cover descriptors inherited across a fork (22, 69-92).
  - Non-unix falls back to `create_new` plus remove on drop (176-200).
  - `std::fs::File::lock` was avoided because it needs Rust 1.89 and the MSRV is 1.88 (3-6).
- **logging.rs.**
  - File only, never the terminal: daily rolling `mejl.log.<date>`, 7 kept, no ANSI, target and thread names on (1-16, 31-36, 257-281).
  - Level precedence: `--log-level`, then `MEJL_LOG` (EnvFilter syntax), then `info` (220-236).
  - A second filter caps hyper, hyper_util, h2, reqwest, rustls, zbus and keyring at info. It can only remove lines (40-50, 244-254).
  - Redaction runs on each event through `RedactingMakeWriter`. It covers registered secrets of at least 8 characters (longest first), anything after `Bearer `, and Fastmail token shapes like `fmu1-...` (52-194).
  - It is initialized for every command (cli.rs:277), even `doctor --print-config` (tested at binary.rs:94-103).
- **env.rs.**
  - A snapshot of 13 named variables taken once. Non-UTF-8 values are dropped. `Debug` prints names only, because `MEJL_TOKEN` is a credential (env.rs:8-83).
  - The rule is that nothing below the CLI calls `std::env::var` (3-6). `app::expand_home` reads `HOME` directly (app/mod.rs:377-384).
- **demo.rs.**
  - `DemoBackend` builds its own 1-worker `mejl-demo` runtime and marks its threads as panic-catching. It serves `FakeServer::demo(now)` on `127.0.0.1:0` with axum, keeps stores in a 0700 temp dir that is removed on drop, and runs a simulator that delivers a message every 45 s (demo.rs:1-180).
  - `seed_local` fills local-only data: rules, saved searches and an older rule block (187 onward).
  - The real engine talks to it over HTTP exactly as it would talk to Fastmail.

### 12. Tests and sandboxing

#### 12.1 tests/terminal_pty.rs

- **pty setup.** No pty crate. It calls `libc::openpty` directly at 80x24 (terminal_pty.rs:38-67).
- **Child process.** Each test re-runs the current test binary with `--exact pty_child --nocapture --test-threads=1` and `MEJL_PTY_SCENARIO=<name>`, with stdio on the pty slave (161-178). `pty_child` is a no-op test in normal runs (406-411).
- **Reading.** A reader thread polls the master and accumulates bytes. `wait_for(needle)` polls every 5 ms with a 10 s limit (69-135). The screen is never rendered: assertions are ordered byte needles (`in_order`) plus termios raw-mode checks via `tcgetattr` (190-198, 234-243).
- **Scenarios:**
  - `enter_restore`: exact mode sequences, no 1003 (251-287)
  - `suspend_for`: cooked terminal inside, raw after (289-312)
  - `suspend_process`: `setpgid`, `waitpid(WUNTRACED)` sees SIGTSTP, SIGCONT resumes (314-358)
  - `input_pause`: the editor gets `xyz` and no ghost keys after resume (360-372)
  - `probe`: `setsid` plus `TIOCSCTTY` in `pre_exec` so `/dev/tty` is the pty; sleeps of 40 ms and 260 ms simulate a slow terminal (374-402)
- **Limits.** The developer's terminal is never touched (1-12). The real `mejl` binary is never run in a pty, and nothing reads a rendered screen or measures startup.

#### 12.2 tests/binary.rs

- Runs `CARGO_BIN_EXE_mejl` with `env_clear()`, then PATH, HOME and `XDG_{CONFIG,DATA,CACHE,STATE}_HOME` pointed into a tempdir, with stdin null (binary.rs:14-55). Unix only.
- Keyring access is avoided by setting `MEJL_TOKEN` (1-7).
- Covers: version and help, usage exit 2, the print-config log dir staying in the sandbox, config error exit 3 with "line 2", doctor never printing the token (stdout, stderr and log files), the run without a tty exiting 1 after creating a 0700 account dir, demo without a tty, and login with empty piped stdin (73-175).

#### 12.3 tests/app.rs and tests/macros.rs

- **Headless full app.** The real `App` loop, real `Engine` and `Store` in a tempdir, `mejl-fake` in process, ratatui `TestBackend`, `RecordingHost` and `MemoryStore` (app.rs:1-6, 213-305).
- **Transport wrapper.** A `Counting` transport counts requests and can add delay (71-98).
- **Fast engine config.** Ping 1 s, debounce 10 ms and similar (54-67). OAuth tests serve the fake on loopback (144-210).
- **Reading the screen.** Concatenate `TestBackend` buffer cell symbols with a newline per width (322-333).
- **Driving the loop.** `wait(pred)` runs `recv_batch_until(50 ms)` plus `handle_batch` until the predicate holds or 20 s pass (336-354). `key(spec)` is one batch per key, so "same frame" can be asserted (371-381).
- **Coverage, about 47 tests:**
  - archive and undo in the same frame
  - send and undo-send
  - drafts and autosave
  - push
  - masks and rules
  - narrow layout
  - load-more
  - hostile control bytes
  - Ctrl-z from every context
  - the quit-while-sending prompt
  - key editor rebinds without a restart
  - OAuth refresh, lock and revoke
- **Latency test.** 200 keys at 160x40, reporting p50 to max and asserting p99 under 250 ms. Release numbers are reported, not asserted (app.rs:1234-1252).
- **macros.rs** is a trimmed copy of the same harness for macro behaviour (macros.rs:1-6, 407-548).

#### 12.4 tests/data.rs and inline tests

- data.rs tests the read model and pending overlay against a real store: hidden rows shift pages, select-all exceptions, refused moves, mark-read on archive (data.rs:1-3, 101-333).
- Inline `#[cfg(test)] mod tests` sit last in every source file (AGENTS.md:37). They use `Env::from_pairs`, `Paths::under`, `MemoryStore` and `FakeSource`.
- Sandboxing rule: tests never touch the real OS keyring or real XDG dirs (AGENTS.md:32; secrets.rs:22-24; cli.rs:977-992).
- `crates/mejl/Cargo.toml` has an empty `[dev-dependencies]` (line 49). Tests rely on `tempfile`, which is a normal dependency (line 43), and on `libc` for unix (45-47).

### 13. Infrastructure

#### 13.1 Workspace

- **Root manifest.** `resolver = "3"`, `members = ["crates/*"]`, `default-members = ["crates/mejl"]` (Cargo.toml:1-4).
- **Shared package fields.** version 0.1.0, edition 2024, rust-version 1.88, license MIT OR Apache-2.0 (6-11). Every member inherits them with `.workspace = true` (all member manifests).
- **Dependency inheritance.** One `[workspace.dependencies]` table (13-82). Members opt into features locally: `crossterm` with `event-stream` in the binary (crates/mejl/Cargo.toml:31), `tokio` with `test-util` in the mejl-fake dev-deps.
- **Declared but unused.** `sse-stream`, `secrecy`, `anyhow`, `mail-parser`, `mail-builder` and `pulldown-cmark` are declared in the workspace but in no member manifest (verified by search).
- **Crate graph:**
  - mejl depends on every internal crate, including mejl-fake as a normal dependency for demo mode (crates/mejl/Cargo.toml:18-25).
  - mejl-ui depends on mejl-render, mejl-sieve, ratatui and crossterm, with ratatui-textarea only as a reference dev-dependency.
  - mejl-sync depends on mejl-jmap and mejl-store.
  - mejl-store depends on mejl-jmap and rusqlite (bundled).
- **Lint config.** None: no `[workspace.lints]`, `clippy.toml`, `rustfmt.toml` or `rust-toolchain` (verified by listing). `#![warn(missing_docs)]` is set in mejl, mejl-fake, mejl-store, mejl-sync, mejl-ui and mejl-sieve, but not in mejl-jmap or mejl-render.

#### 13.2 Profiles

- Release: `lto = "thin"`, `codegen-units = 1`, `strip = true`, no `panic` setting, so unwind (Cargo.toml:84-87).
- Dev: dependencies at `opt-level = 1` (Cargo.toml:89-90).

#### 13.3 justfile

| Recipe | Command |
|---|---|
| `default` | `just --list` |
| `build` | `cargo build` |
| `release-build` | `cargo build --release --locked -p mejl` |
| `run *args` | |
| `demo` | `cargo run -- --demo` |
| `test` | `cargo test --workspace` |
| `fmt` | |
| `lint` | `cargo clippy --workspace --all-targets -- -D warnings` |
| `msrv` | `cargo +1.88 check --workspace --all-targets` |
| `check` | fmt check, lint, test |
| `keys` | `cargo run -- keys --check` |
| `release <version>` | Depends on `check`, verifies the `Cargo.toml` version and a clean tree, then makes an annotated tag and pushes it |

Evidence: justfile:1-64. AGENTS.md:25 gives the MSRV check without `--all-targets`.

#### 13.4 CI and release

- **codeql.yml.** Runs on push and pull request to main and weekly. The language matrix is `[rust, actions]` with `build-mode: none`. Analysis runs with `upload: never`, then `filter-tests.py` drops results in test code, then the SARIF is uploaded (codeql.yml:1-45).
- **codeql-config.yml.** `paths-ignore` lists `crates/*/tests/**` and `crates/*/benches/**` (codeql-config.yml:5-8).
- **filter-tests.py.** Drops results under tests or benches. In `.rs` files it drops results at or after the first `#[cfg(test)]` that opens a `mod ... {` (filter-tests.py:17-51). This depends on the inline-tests-last rule (AGENTS.md:37).
- **release.yml.** Triggers on tags `v[0-9]+.[0-9]+.[0-9]+*` or manual dispatch with a tag (release.yml:6-13).
  1. The check job verifies the Cargo version equals the tag, then runs `fmt --check`, `clippy --workspace --all-targets --locked -D warnings` and `test --workspace --locked` on stable (22-44).
  2. The build matrix covers x86_64-unknown-linux-gnu, aarch64-unknown-linux-gnu (ubuntu-24.04-arm) and aarch64-apple-darwin. Each produces a tar.gz with README and licenses plus a sha256 file (46-88).
  3. `gh release create --generate-notes --verify-tag` publishes (90-107).
- **Gaps.** No workflow runs tests on push or pull request. MSRV and benches never run in CI.

### 14. Transferable patterns in the other crates

- **Transport trait, with the fake as its second implementation.**
  - `mejl_jmap::transport::Transport: Send + Sync` has `send(HttpRequest) -> BoxFuture<HttpResponse>` and `stream(HttpRequest) -> BoxFuture<StreamingResponse>` over raw HTTP-shaped values. JSON and EventSource parsing therefore run the same against both implementations (mejl-jmap/src/transport.rs:1-121).
  - `HttpRequest`'s `Debug` redacts credential headers (45-56, 130-150).
  - mejl-fake provides `FakeServer::transport()` in process. Tests wrap it (the `Counting` and `Slow` transports in app.rs and macros.rs).
- **Fake HTTP server and demo data.**
  - `FakeServer` is cheap to clone. It is written from the RFCs, not from mejl's client, so it can catch client bugs (mejl-fake/src/lib.rs:1-5).
  - `serve_http` runs the same handler over loopback axum, "faults and quirks included" (serve.rs:1-4).
  - Runtime `Quirks` switches inject reported Fastmail deviations and faults (config.rs:116-214; lib.rs:213-216). A deterministic clock option exists (config.rs:136-140).
  - Fictional demo data, all under example domains, is time-shifted to `now` (demo.rs:1-7).
- **cache.db versus state.db.**
  - `cache.db` holds everything fetched. It is deletable and rebuilt on schema change, with `synchronous = NORMAL`.
  - `state.db` holds user intent: outbox, undo, drafts, rules, UI state. It is migrated in place and never dropped, with `synchronous = FULL`.
  - Both use WAL (mejl-store/src/lib.rs:1-14; schema.rs:30, 266-370, 390-396).
  - `Store::open` migrates, rebuilds a stale cache, returns `in_flight` outbox entries to pending, and re-derives overlays (store.rs:140-159).
- **The store thread owns the writers.**
  - Exactly one `Store` per account, moved into the `mejl-store` thread. Jobs are closures run in send order under `catch_unwind`, with oneshot replies (store_thread.rs:1-6, 193-199, 257-292).
  - Readers come from a `Send + Sync` `ReaderFactory` (mejl-store/src/lib.rs:71-83).
  - "Flush generations" drop responses that are older than a recorded flush (store_thread.rs:211-256).
- **Sync engine channels.**
  - `Engine::start(config, client, store, sink)` owns its runtime on a driver thread.
  - Commands arrive on an unbounded channel and are dispatched with `tokio::select!` against a cancellation token.
  - Long-lived loops are spawned once: sync, push, flush, drafts, bodies, masked.
  - Events leave through the sink, with data changes coalesced.
  - `shutdown()` is bounded and lets the store finish queued jobs (engine.rs:29-322).
- **Rendering on a worker.**
  - mejl-render is synchronous and CPU bound.
  - `PreparedBody` separates width-independent work from `render(width)`.
  - A 1 MiB input cap, nesting limits against stack overflow and `catch_unwind` around html2text make hostile input safe (mejl-render/src/lib.rs:23-48, 259-278; html.rs:35, 100-106).
  - The binary's render pool supplies the worker and the newest-wins queue (section 4.5).

### 15. Classification

| Part | Class | Why |
|---|---|---|
| events.rs | Reusable as is | Generic over the message type |
| timer.rs | Reusable as is | Generic. The 1 Hz clock policy lives in the app, not here. Consider deduping wakes by deadline |
| input.rs | Reusable as is | Generic EventSource, coalescing, pause/resume and leaked-reply filter. Needs crossterm with event-stream |
| terminal/mod.rs | Reusable, with fixes | Mode manager and restore are generic. Fix the panic classification of spawned threads, and the `CATCHING_THREAD_PREFIXES` names are mejl's |
| terminal/signals.rs | Reusable as is for a TUI | Wrapper-mode signal forwarding is a different problem |
| terminal/editor.rs | Reusable as is | Generic |
| terminal/probe.rs, cache.rs, quirks.rs | Reusable as is | Generic terminal knowledge |
| terminal/osc.rs | Reusable as is | Generic. OSC 8 is unused in mejl |
| terminal/theme.rs | Reusable, but dormant | Blocked by crossterm 0.29 |
| env.rs | Adaptable | Variable list is app-specific |
| lock.rs | Reusable as is | Point it at any directory |
| logging.rs | Adaptable | Fastmail token shapes and capped targets are mejl's |
| paths.rs | Adaptable | etcetera strategy is generic, the layout is mejl's |
| app/host.rs | Adaptable | Trait plus recording fake. Methods are generic |
| render_pool.rs | Adaptable | Newest-wins worker pool, typed to mejl-render |
| tui.rs | Adaptable | Startup and teardown order is the pattern |
| cli.rs | Adaptable | Context injection, exit-code module, out/err writers, require_terminal |
| config.rs | Adaptable | Span diagnostics, unknown-key warnings, example printer. Schema is mejl's. TOML-specific |
| app/settings.rs | Adaptable | toml_edit comment-preserving write-back. TOML-specific |
| doctor.rs | Adaptable | Report structure. Content is mejl's |
| keys.rs | Adaptable | Depends on mejl-ui's keymap |
| demo.rs | Adaptable pattern | Fake backend in process, temp dirs. Content is email |
| app/mod.rs loop skeleton | Adaptable | handle_batch, drain, exec, after_turn, draw, token map |
| app/mod.rs remainder, data.rs, convert.rs, mutate.rs, overlay.rs | Email-specific | Optimistic overlay is the only transferable idea |
| secrets.rs, login.rs, oauth.rs, oauth_login.rs | Email-specific | SecretStore trait plus MemoryStore is a pattern |
| tests/terminal_pty.rs | Reusable as is | Pty harness for mode sequences |
| tests/binary.rs | Reusable pattern | XDG and HOME sandbox |
| tests/app.rs, macros.rs | Adaptable pattern | TestBackend plus RecordingHost plus real loop. Content is email |
| tests/data.rs | Email-specific | |
| Cargo workspace, profiles, justfile, release.yml | Reusable with renames | Release targets may differ |
| CodeQL workflow and filter | Reusable as is | Needs the inline-tests-last rule |
| mejl-jmap Transport plus mejl-fake | Pattern adaptable | A trait seam with a fake as second implementation, and a fake that doubles as demo backend |
| mejl-store two-file durability and store thread | Pattern adaptable | |
| mejl-sync command and event channels | Pattern adaptable | |
| mejl-render on a worker | Pattern adaptable | |

### 16. Mismatches in brief

Each item is in the conflicts output with evidence.

- **Rendering and idle:**
  - every batch draws (no dirty flag)
  - 1 Hz ticks on the default screen
  - no BufWriter
  - one latency sample per frame, not per event
- **Input:** EventStream instead of a blocking read.
- **Runtimes:** three tokio runtimes, and the render pool does not use `spawn_blocking`.
- **Terminal features:**
  - `auto:` themes accepted but not implemented
  - probe background and theme answers unused
  - OSC 8 unused while doctor reports it on
  - no notifications
  - no capability overrides
  - no DECRQM 2027
  - cache key without XTVERSION
- **Dependencies:** unused workspace deps and crate picks, and the crossterm osc52 feature unused.
- **CI:** no test CI on push or pull request, no MSRV or bench CI.
- **Tests:** pty tests check bytes, not a vt100 screen, and the latency assertion is 250 ms, not 8 ms.
- **Robustness:**
  - panic classification of spawned threads
  - external SIGTSTP unhandled
  - signal watcher starts late
  - the `HOME` read breaks the env rule
  - settings fsync on the UI thread
  - a plain-text token gets a warning, not a refusal

### 17. Implications for a spela rewrite (inferred)

- **What to lift.** The terminal layer, event loop, input thread, timer, probe and cache, lock, env, logging redaction pattern, CLI Context and exit-code pattern, and the pty and binary test harnesses carry over with little change.
- **Background work.** spela's NVML metrics, Steam scans, DLL downloads and pkexec `apply-profile` round-trips map naturally onto engine Commands and LoopEvent messages. It is open whether spela needs a tokio runtime and a store thread, or only plain worker threads.
- **Live metrics.** A metrics view needs periodic ticks, so mejl's "idle does no work" goal changes meaning there. Make the tick policy explicit.
- **Wrapper and root paths.** Wrapper mode, which forwards signals to a game child, and the root `apply-profile` path must not inherit mejl's TUI signal policy or its unconditional per-user log setup.
- **Config format.** spela's YAML configs and profiles do not fit mejl's toml_edit write-back. The rewrite must choose a format, or find a YAML equivalent for comment-preserving edits.

### Conflicts and mismatches

- **Render only when dirty.** The docs say to render once only if state is dirty. The code draws after every batch with no dirty check, including batches that only carry a Tick. Evidence: docs/architecture.md:146,173; docs/terminal-ux.md:397 vs crates/mejl/src/app/mod.rs:590-591,742-756
- **Idle does no work.** The docs promise no timer wake-ups when nothing changes. after_turn turns 1 Hz ticking on whenever shows_clock() is true. That is the Mail screen at two-pane width with no prefix pending and no picker open, which is the default view. The default screen therefore wakes, updates and redraws every second at rest. Evidence: crates/mejl/src/timer.rs:3; docs/architecture.md:176,461 vs crates/mejl/src/app/mod.rs:736; crates/mejl-ui/src/state.rs:1089-1104
- **BufWriter for frames.** The docs say to flush a BufWriter. The code builds CrosstermBackend over plain io::stdout(), and no BufWriter exists anywhere in mejl or mejl-ui. Evidence: docs/architecture.md:173; docs/terminal-ux.md:400 vs crates/mejl/src/tui.rs:230; search for BufWriter in crates/mejl/src and crates/mejl-ui/src found none
- **Input thread design and the event-stream feature.** The docs describe a blocking crossterm::event::read and say the event-stream feature is not needed. The code runs crossterm EventStream under futures::executor::block_on, selected against a control channel so it can pause at once, and enables event-stream. Evidence: docs/architecture.md:159,164 vs crates/mejl/src/input.rs:7-10,184-189,346,394-398; crates/mejl/Cargo.toml:31
- **input_to_flush granularity.** The doc says to record input_to_flush for every input event in the frame. The code records one sample per frame, measured from the oldest input in the batch, and feeds it to the UI on the next turn. Evidence: docs/architecture.md:174; docs/terminal-ux.md:452 vs crates/mejl/src/app/mod.rs:545-553,750-754
- **Body render worker model.** The architecture doc puts html2text on tokio spawn_blocking with a time limit. mejl-render's docs tell callers to use spawn_blocking plus tokio::time::timeout. The binary uses its own std thread pool with catch_unwind and no timeout. Only the 1 MiB input cap bounds the work. Evidence: docs/architecture.md:152,290; crates/mejl-render/src/lib.rs:25-29 vs crates/mejl/src/render_pool.rs:1-5,322-328,382-402
- **Number of tokio runtimes.** The process diagram shows one tokio runtime. The code builds three: the engine's, the binary's mejl-io for downloads and login retries, and mejl-demo in demo mode. It also runs separate timer, signal and render threads. Evidence: docs/architecture.md:138-157 vs crates/mejl-sync/src/engine.rs:83-87,110-119; crates/mejl/src/app/mod.rs:441-446; crates/mejl/src/demo.rs:77-83
- **Declared but unused workspace dependencies.** sse-stream, secrecy, anyhow, mail-parser, mail-builder and pulldown-cmark are declared in the workspace but in no member manifest. The architecture doc names sse-stream for push and secrecy::SecretString for tokens. The code uses its own SSE parser and mejl_jmap::Secret. Evidence: Cargo.toml:35,43,44,47,61,66; docs/architecture.md:27,408; crates/mejl-jmap/src/sse.rs exists; search of crates/*/Cargo.toml found no member using these
- **Crate picks not in the build.** The architecture doc lists nucleo-matcher, tui-tree-widget, tui-input, crokey and vt100 as picks. None appears in any manifest. mejl-ui has its own fuzzy.rs, and the doc's budget table half-acknowledges the own matcher. Evidence: docs/architecture.md:27,420-421,428-429,435,459 vs Cargo.toml:13-82 and crates/*/Cargo.toml; crates/mejl-ui/src/fuzzy.rs exists
- **crossterm osc52 feature unused.** The workspace enables crossterm's osc52 feature, but mejl writes OSC 52 by hand, saying the crates are copyleft. Nothing uses crossterm's clipboard API. Evidence: Cargo.toml:25 vs crates/mejl/src/terminal/osc.rs:3-4,56-74; crates/mejl/src/app/host.rs:103-106; search found no crossterm clipboard use
- **auto:<name> themes accepted but not implemented.** The config docs, the example config and validation accept auto:<name> as picking a light or dark variant, and a config test asserts no warning for theme = "auto:dark". At runtime Settings::theme passes the full name to mejl_ui::Theme::named, which has no auto arm. The result is an UnknownTheme error, the default theme and a warning toast. The probe's background answer is never consulted. Evidence: crates/mejl/src/config.rs:16,205,702-705,813,910,930; docs/terminal-ux.md:599 vs crates/mejl/src/app/settings.rs:136-150; crates/mejl-ui/src/theme.rs:307-317
- **Probe answers unused by the UI.** background, background_rgb and theme_reports are probed, cached and shown by doctor, but nothing in the app reads them. Mode 2031 is never enabled because crossterm 0.29 swallows keys after a CSI ? 997 n report. TerminalReport is defined but never produced. The docs plan live theme switching via 2031 and OSC 11. Evidence: crates/mejl/src/terminal/probe.rs:107-112,150-173; crates/mejl/src/tui.rs:155-157; crates/mejl/src/terminal/theme.rs:9-16; crates/mejl/src/events.rs:30-42 vs docs/terminal-ux.md:479,599
- **OSC 8 links reported but never emitted.** osc::hyperlink and link_target are only called from their own tests. Doctor still prints 'links (OSC 8): on' unless GNU screen is detected. Evidence: crates/mejl/src/terminal/osc.rs:85-117; crates/mejl/src/doctor.rs:305-313; crates/mejl/src/terminal/probe.rs:132-136; search found no other caller
- **Terminal notifications absent.** terminal-ux.md recommends OSC 9, 99 and 777 notifications, preferred over SSH, with a config switch. No notification code exists in any crate. osc.rs covers only OSC 52 and OSC 8. Evidence: docs/terminal-ux.md:43,79,108 vs crates/mejl/src/terminal/osc.rs:1-117; workspace search for OSC 9, 99 and 777 found nothing
- **Capability overrides.** terminal-ux.md says config always wins, with an auto|on|off setting per capability: keyboard.kitty, render.sync, links.osc8, clipboard.osc52, images, notifications. The config schema has only ui.mouse. Kitty and sync output follow the probe alone. Evidence: docs/terminal-ux.md:79 vs crates/mejl/src/config.rs:442-469; crates/mejl/src/tui.rs:152-158,228
- **Probe burst contents.** The detection strategy lists CSI ? 2027 $ p (grapheme clustering) in the burst. QUERIES omits it. Evidence: docs/terminal-ux.md:75 vs crates/mejl/src/terminal/probe.rs:216-224
- **caps.toml cache key.** terminal-ux.md says to key the cache on the XTVERSION string plus TERM, TERM_PROGRAM and multiplexer. The code keys on TERM, TERM_PROGRAM, TERM_PROGRAM_VERSION, multiplexer and SSH, and uses XTVERSION only to reject an entry. architecture.md:186 matches the code, so the two docs disagree. Evidence: docs/terminal-ux.md:77 vs crates/mejl/src/terminal/cache.rs:94-108,191-194; docs/architecture.md:186
- **Probe timing at startup (doc internal tension).** terminal-ux.md recommends rendering the first frame from cached capabilities and applying the probe asynchronously. The same doc's as-built note, and the code, run a blocking probe before anything else, because crossterm would read the replies as keys. Keys typed during the drain are dropped. Evidence: docs/terminal-ux.md:76,429; crates/mejl/src/terminal/probe.rs:16-28; crates/mejl/src/tui.rs:118
- **CI enforcement.** The docs say latency budgets are enforced in CI and benchmarks fail CI on p99 regressions. No workflow runs tests, clippy or benches on push or pull request. fmt, clippy and tests run only in release.yml on a version tag. CodeQL runs on push and pull request but is not a test gate. Evidence: docs/architecture.md:41,471,490 vs .github/workflows/codeql.yml:5-11; .github/workflows/release.yml:6-13,22-44
- **MSRV check.** MSRV 1.88 is checked only by a local just recipe. CI builds on stable. AGENTS.md gives the check as 'cargo +1.88 check --workspace' while the justfile adds --all-targets. Evidence: Cargo.toml:9; justfile:35-37; AGENTS.md:25; .github/workflows/release.yml:30-32,64-66
- **End-to-end pty testing scope.** The docs say to run the real binary in a pty, read the screen with vt100, and measure startup to first flush. terminal_pty.rs uses raw libc::openpty and re-runs the test binary as a child scenario. It asserts escape-byte order and termios state, never a rendered screen, never the mejl binary, and never startup time. Evidence: docs/architecture.md:451,487 vs crates/mejl/tests/terminal_pty.rs:1-12,38-67,161-178,234-243
- **Latency assertion versus budget.** The budget is p99 under 8 ms for local actions. The only latency test asserts p99 under 250 ms on TestBackend, and only reports release numbers. Evidence: docs/architecture.md:454 vs crates/mejl/tests/app.rs:1234-1252
- **Panic hook model for spawned threads (inferred).** panic_is_fatal treats panics on mejl-input, mejl-timer, mejl-signals and unnamed threads as process-ending, so the hook restores the terminal. No panic = "abort" is set, so a panic on a spawned thread ends only that thread while the UI thread keeps drawing on a restored screen. A dead input thread sends no InputClosed. Evidence: crates/mejl/src/terminal/mod.rs:282-331,343-346; Cargo.toml:84-87; crates/mejl/src/input.rs:426-436
- **External SIGTSTP and early signals (inferred).** Only self-raised SIGTSTP (Ctrl-z) restores the terminal. An external SIGTSTP or SIGSTOP stops mejl in raw mode on the alternate screen. The signal watcher starts after the probe toggles raw mode and after terminal::enter, so a SIGTERM in that window gets the default action without a restore. Evidence: crates/mejl/src/terminal/signals.rs:84-96; crates/mejl/src/terminal/mod.rs:253-266; crates/mejl/src/tui.rs:118,152,221; crates/mejl/src/terminal/probe.rs:674-689
- **Env access rule.** env.rs says code below the CLI never calls std::env::var and reads an Env snapshot instead. app::expand_home reads HOME with std::env::var_os. Evidence: crates/mejl/src/env.rs:3-6 vs crates/mejl/src/app/mod.rs:377-384
- **UI thread blocking on disk.** Principles say there is no bulk disk I/O on the render path and the UI thread never blocks. Settings and key-editor changes write the config on the UI thread with tempfile, sync_all and rename. The editor, suspend and engine shutdown also block, which app/mod.rs acknowledges. Evidence: docs/architecture.md:32; AGENTS.md:35 vs crates/mejl/src/app/mod.rs:22-27,1034,1050; crates/mejl/src/app/settings.rs:223,249,656-671
- **Plain-text token in config (inferred).** architecture.md says that if a user puts a token in the config, mejl refuses and points to token_command. The code has no token key, so a 'token' entry only produces an unknown-key warning and is ignored. Evidence: docs/architecture.md:410 vs crates/mejl/src/config.rs:452-457,496-514
- **Workspace layout.** The architecture doc's layout lists five crates. The workspace has eight: mejl-fake, mejl-sync and mejl-render are extra. AGENTS.md lists all eight. Evidence: docs/architecture.md:441 vs Cargo.toml:3,15-21; AGENTS.md:9-18
- **Timer wake dedupe (inferred).** Timer::wake_at says asking twice for the same instant gives one wake-up. after_turn calls wake_at_timestamp on every turn, and that computes a fresh Instant::now() + delay. Repeated requests for one deadline therefore become several slightly different instants and several wakes. Evidence: crates/mejl/src/timer.rs:122-139; crates/mejl/src/app/mod.rs:737-739
- **Restore 'exactly what was turned on'.** The module doc says restore undoes exactly what was turned on. Mouse enable writes only 1000, 1002 and 1006 by hand, but restore uses crossterm's DisableMouseCapture, which turns off a wider set. This is harmless. The pty test checks only ?1000l. Evidence: crates/mejl/src/terminal/mod.rs:7-9,85-102,210-212; docs/terminal-ux.md:225; crates/mejl/tests/terminal_pty.rs:276-283
- **Doctor versus TUI capability view.** The TUI fills unknown probe answers from caps.toml. mejl doctor probes without the cache, so it can show unknown where the TUI would use cached support. Evidence: crates/mejl/src/tui.rs:118; crates/mejl/src/cli.rs:824-826; crates/mejl/src/terminal/probe.rs:430-432
- **Fake server ships in the release binary.** mejl-fake, with axum, is a normal dependency of the binary crate for --demo, so test infrastructure is compiled into release builds. This is a tension rather than a mismatch, since AGENTS.md says the fake serves the demo. Evidence: crates/mejl/Cargo.toml:20; crates/mejl-fake/Cargo.toml:17; crates/mejl/src/demo.rs:1-12; AGENTS.md:12

### Open questions

- spela's config and per-game profiles are YAML. mejl's comment-preserving write-back depends on toml_edit, and its diagnostics depend on toml's span API. Keep YAML for parity, needing a YAML crate that can preserve comments and give spans, or move to TOML with a migration?
- Does the rewrite need mejl's engine weight: a tokio runtime, a dedicated store thread, a SQLite cache and state? Or only the event loop, terminal layer and plain worker threads for NVML polling, Steam scans, DLL downloads and pkexec round-trips?
- spela runs as a game wrapper, forwarding signals to a child, and as a root apply-profile helper under pkexec. Should mejl's 'first stop signal quits, second exits 128+n' policy and its unconditional logger and panic-hook setup in cli::main apply only to TUI mode? Where should a root process log?
- spela's TUI likely shows live GPU metrics, which need periodic ticks. Should the rewrite keep mejl's on-demand timer with an explicit tick policy, or add a dirty flag so frames are skipped when nothing changed (mejl never skips)?
- Ctrl-C: mejl's raw-mode TUI ignores Ctrl-C (no preset binds it) and quits on q or Q. What should spela's rewrite do? The current Bubbletea TUI behaviour was not checked here.
- TUI testing: spela's AGENTS.md requires a termctrl visual review of the compiled binary. Should the rewrite also adopt mejl's headless TestBackend plus RecordingHost loop harness and the libc::openpty mode-sequence tests?
- Mouse capture is on by default in mejl, with modes 1000, 1002 and 1006, plus kitty disambiguate and alternate keys when probed. Should spela's TUI capture the mouse by default, and does it need capability overrides that mejl lacks?
- CI: mejl has no test or lint workflow on push or pull request, only on release tags, and builds Linux x86_64 and aarch64 plus macOS arm64. Does the user want a per-push gate for the rewrite, and which release targets? spela is Linux and NVIDIA only.
- MSRV: mejl pins 1.88 and works around the missing std File::lock (1.89) with libc flock. What MSRV should the rewrite promise?
- Are mejl's unbuilt terminal-ux features (auto themes, OSC 8 links, notifications, capability overrides, live 2031 theme switching) part of 'mejl's terminal UX' to adopt, or should the rewrite copy only what mejl actually ships?
- Live checks only a running system can settle: the real idle wake-up rate and per-frame cost of mejl's loop, and whether crossterm 0.29's EventStream reader thread is wake-free when idle (unverified here).
