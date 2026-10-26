# spela tasks. Run `just` to list them.

# List the recipes.
default:
    @just --list

# Build the debug binary.
build:
    cargo build

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
