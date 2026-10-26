# spela tasks. Run `just` to list them.

# List the recipes.
default:
    @just --list

# Build the debug binary.
build:
    cargo build

# Build the release binary, target/release/spela.
build-release:
    cargo build --release --locked -p spela

# Install the release binary to /usr/bin/spela and the polkit policy, the
# paths the AUR package uses: Steam runs launch options with
# PATH=/usr/bin:/bin, and the policy names /usr/bin/spela. Run
# `just build-release` first, then `sudo just install`. `just install <dir>`
# stages the files under <dir> instead.
install destdir="":
    @test -x target/release/spela || { echo "target/release/spela is missing: run 'just build-release' first" >&2; exit 1; }
    install -Dm755 target/release/spela "{{destdir}}/usr/bin/spela"
    install -Dm644 data/polkit/io.github.spela.policy "{{destdir}}/usr/share/polkit-1/actions/io.github.spela.policy"

# Run spela with any arguments, for example `just run tui`.
run *args:
    cargo run -- {{args}}

# Try spela on built-in fictional data. Touches no GPU, Steam or polkit.
demo:
    cargo run -- --demo

# Run every test.
test:
    cargo test --workspace --locked

# Format the code.
fmt:
    cargo fmt --all

# Lint with every warning as an error.
lint:
    cargo clippy --workspace --all-targets --locked -- -D warnings

# Check the oldest supported Rust (1.89). Needs `rustup toolchain install 1.89`.
msrv:
    cargo +1.89 check --workspace --all-targets --locked

# Check licences, banned crates and sources. Needs `cargo install cargo-deny`.
# CI also checks advisories, which downloads the RustSec database.
deny:
    cargo deny --offline check licenses bans sources

# The gate to pass before handing off: format, lint, test, MSRV.
check:
    cargo fmt --all --check
    just lint
    just test
    just msrv

# Run `just check` before every commit in this checkout.
hooks:
    git config extensions.worktreeConfig true
    git config --worktree core.hooksPath .githooks
