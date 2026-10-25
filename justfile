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

# Run every test.
test:
    cargo test --workspace

# Format the code.
fmt:
    cargo fmt --all

# Lint with every warning as an error.
lint:
    cargo clippy --workspace --all-targets -- -D warnings

# The gate to pass before handing off: format, lint, test.
check:
    cargo fmt --all --check
    just lint
    just test

# Run `just check` before every commit in this checkout.
hooks:
    git config extensions.worktreeConfig true
    git config --worktree core.hooksPath .githooks
