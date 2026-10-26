//! The stored-profile writes of `dlss`, `gpu`, `cpu` and `proton`: `set`,
//! and `reset` (`profile-reset` for `gpu` and `cpu`).
//!
//! The flags come from the field table, so each section's `set` has one
//! flag per field and the help is the field's own text. They change the
//! stored profile, never live hardware: `gpu reset`, `cpu governor` and `cpu
//! smt` do that.
//!
//! Every flag takes spela's one bool vocabulary, a number or a word, and
//! `default`, which makes the field follow All games (with `--all-games`,
//! the system default). `--sr-preset` and `--rr-preset` are the exceptions:
//! there `default` is a stored preset, and `reset <game> sr_preset` follows.
//! A bad value is a usage error before anything is read (exit 2), so
//! `--rr-preset auto` fails at parse time. Every flag of one run is one
//! write. Go's fixes are kept:
//!
//! - `--fan-speed 0` stores 0, which means no fan change at launch, and
//!   `--fan-speed default` follows. Go's help said "0 to clear".
//! - `--shader-cache-path default` follows instead of storing an empty path.
//! - `--vrr` and `--sr-override` exist, as in the TUI (decision 20).
//!   `--power-mizer` and `--sr-model-preset` are gone (decision 10).
//!
//! As in Go, a stored `--sr-mode` or `--sr-preset` also turns
//! `dlss.sr_override` on, `--rr-mode` and `--rr-preset` turn
//! `dlss.rr_override` on, and `--fg` and `--multi-frame` turn
//! `dlss.fg_override` on, unless the same run sets that override itself.
//!
//! An empty value, as in `--governor ""`, counts as a flag that was not
//! given, as Go's string flags did, so a script that passes an unset
//! variable changes nothing. The number flags and `--indicator` and
//! `--fg-indicator` were typed flags in Go, so an empty value is an error
//! there, as it was.
//!
//! `cpu set --governor <g>` checks `<g>` against the governors sysfs lists,
//! through the CPU backend, after the game is found and before anything is
//! written, with Go's texts: an unlisted one is a usage error, and a list
//! that cannot be read fails.
//!
//! `reset <game> [field]...` makes the named fields follow, or every field
//! of the section when none is named.

use std::io::Write;

use clap::{Arg, ArgAction, ArgMatches, Args, Command, FromArgMatches};
use spela_core::backend::Cpu;
use spela_core::profile::field::Rule;
use spela_core::profile::store;
use spela_core::profile::{Field, FieldKey, Kind, SMT_UNCHANGED, Section, Value, parse_bool};

use crate::cli::Context;
use crate::commands::target::{clear_all_message, follow_message, line, profile_error, target};
use crate::error::{CliError, exit};

/// `proton` as a section number for [`SetArgs`] and [`ResetArgs`].
pub const PROTON: u8 = 0;
/// `dlss`.
pub const DLSS: u8 = 1;
/// `gpu`.
pub const GPU: u8 = 2;
/// `cpu`.
pub const CPU: u8 = 3;

/// `proton set` arguments.
pub type ProtonSetArgs = SetArgs<PROTON>;
/// `dlss set` arguments.
pub type DlssSetArgs = SetArgs<DLSS>;
/// `gpu set` arguments.
pub type GpuSetArgs = SetArgs<GPU>;
/// `cpu set` arguments.
pub type CpuSetArgs = SetArgs<CPU>;
/// `proton reset` arguments.
pub type ProtonResetArgs = ResetArgs<PROTON>;
/// `dlss reset` arguments.
pub type DlssResetArgs = ResetArgs<DLSS>;
/// `gpu profile-reset` arguments.
pub type GpuResetArgs = ResetArgs<GPU>;
/// `cpu profile-reset` arguments.
pub type CpuResetArgs = ResetArgs<CPU>;

const NO_CHANGES: &str = "No changes specified. Use --help to see available options.";

const DEFAULT_HELP: &str = "\
Every flag also takes `default`, which makes the setting follow All games
(with --all-games, the system default).";

/// `dlss set`'s note: its two preset flags read `default` as a preset.
const DLSS_HELP: &str = "\
Every flag also takes `default`, which makes the setting follow All games
(with --all-games, the system default). For --sr-preset and --rr-preset,
`default` is a preset; use reset to follow All games.";

fn section(number: u8) -> Section {
    Section::ALL[usize::from(number)]
}

/// What one flag asks for.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Setting {
    /// `default`: follow All games, or the system default on All games.
    Follow,
    /// An own value.
    Store(Value),
    /// An empty value: the flag counts as not given, as in Go.
    Absent,
}

/// Whether an empty value means "not given" for a field's flag: Go's string
/// flags. Go's number flags and its two plain bool flags refused an empty
/// value.
fn empty_is_absent(key: FieldKey) -> bool {
    key.field().kind != Kind::Int
        && !matches!(key, FieldKey::DlssIndicator | FieldKey::DlssFgIndicator)
}

/// Reads one flag's text for a field: empty is no flag at all, `default`
/// follows, except for the two presets, where it is a stored value; anything
/// else is the field's value, checked. The texts are Go's, such as
/// `invalid bool "x" (use true/false)`.
pub fn parse_setting(key: FieldKey, raw: &str) -> Result<Setting, String> {
    if raw.is_empty() && empty_is_absent(key) {
        return Ok(Setting::Absent);
    }
    let preset = matches!(key, FieldKey::DlssSrPreset | FieldKey::DlssRrPreset);
    if !preset && raw.trim().eq_ignore_ascii_case("default") {
        return Ok(Setting::Follow);
    }
    if key == FieldKey::CpuSmt {
        // `unchanged` leaves SMT as it is at launch, even when All games
        // sets it: the stored form of decision 65.
        if raw.trim().eq_ignore_ascii_case(SMT_UNCHANGED) {
            return Ok(Setting::Store(Value::OptionalBool(None)));
        }
        return parse_bool(raw)
            .map(|b| Setting::Store(Value::OptionalBool(Some(b))))
            .ok_or_else(|| {
                format!("invalid SMT value: {raw} (use on, off, unchanged, or default)")
            });
    }
    key.field()
        .parse(raw)
        .map(Setting::Store)
        .map_err(|e| e.to_string())
}

fn value_name(field: &Field) -> &'static str {
    match (field.kind, field.rule) {
        (Kind::Bool | Kind::OptionalBool, _) => "BOOL",
        (Kind::Int, _) => "N",
        (_, Rule::AbsolutePathOrEmpty) => "PATH",
        (_, Rule::SystemList) => "GOVERNOR",
        _ => "VALUE",
    }
}

/// The help heading over a section's flags, so they never interleave with
/// the global flags.
fn heading(section: Section) -> &'static str {
    match section {
        Section::Proton => "Proton settings",
        Section::Dlss => "DLSS settings",
        Section::Gpu => "GPU settings",
        Section::Cpu => "CPU settings",
    }
}

fn flag_help(field: &Field) -> String {
    let mut help = field.description.to_owned();
    let choices: Vec<&str> = field
        .choices()
        .iter()
        .copied()
        .filter(|c| !c.is_empty())
        .collect();
    if field.kind != Kind::OptionalBool && !choices.is_empty() {
        help.push_str(&format!(" Values: {}.", choices.join(", ")));
    }
    if field.kind == Kind::OptionalBool {
        help.push_str(
            " Values: on, off, unchanged (leave SMT as it is, even when All games sets it).",
        );
    }
    help
}

/// `<section> set <game> [flags]` or `<section> set --all-games [flags]`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SetArgs<const S: u8> {
    /// The game, by AppID or exact name.
    pub game: Option<String>,
    /// Change All games instead of a game.
    pub all_games: bool,
    /// Each flag given, in table order.
    pub settings: Vec<(FieldKey, Setting)>,
}

fn game_arg() -> Arg {
    Arg::new("game")
        .value_name("GAME")
        .help("The game: its Steam AppID or its exact name")
        .required_unless_present("all_games")
}

fn all_games_arg(what: &str) -> Arg {
    Arg::new("all_games")
        .long("all-games")
        .action(ArgAction::SetTrue)
        .help(format!("{what} All games, which every game follows"))
}

impl<const S: u8> Args for SetArgs<S> {
    fn augment_args(cmd: Command) -> Command {
        let name = cmd.get_name().to_owned();
        let usage = format!(
            "spela {sub} {name} <GAME> [OPTIONS]\n       spela {sub} {name} --all-games [OPTIONS]",
            sub = section(S).name()
        );
        let mut cmd = cmd
            .override_usage(usage)
            .arg(game_arg())
            .arg(all_games_arg("Change").conflicts_with("game"))
            .after_help(if matches!(section(S), Section::Dlss) {
                DLSS_HELP
            } else {
                DEFAULT_HELP
            });
        for field in section(S).fields() {
            let Some(flag) = field.flag else { continue };
            let key = field.id;
            let mut arg = Arg::new(field.key)
                .long(flag)
                .value_name(value_name(field))
                .help_heading(heading(section(S)))
                .help(flag_help(field))
                .value_parser(move |raw: &str| parse_setting(key, raw));
            if field.kind == Kind::Int {
                arg = arg.allow_negative_numbers(true);
            }
            if matches!(key, FieldKey::DlssIndicator | FieldKey::DlssFgIndicator) {
                // Go's plain bool flags: `--indicator` alone turns it on.
                arg = arg
                    .num_args(0..=1)
                    .require_equals(true)
                    .default_missing_value("true");
            }
            cmd = cmd.arg(arg);
        }
        cmd
    }

    fn augment_args_for_update(cmd: Command) -> Command {
        Self::augment_args(cmd)
    }
}

impl<const S: u8> FromArgMatches for SetArgs<S> {
    fn from_arg_matches(matches: &ArgMatches) -> Result<Self, clap::Error> {
        let settings = section(S)
            .fields()
            .filter(|f| f.flag.is_some())
            .filter_map(|f| matches.get_one::<Setting>(f.key).map(|s| (f.id, s.clone())))
            .filter(|(_, s)| *s != Setting::Absent)
            .collect();
        Ok(SetArgs {
            game: matches.get_one::<String>("game").cloned(),
            all_games: matches.get_flag("all_games"),
            settings,
        })
    }

    fn update_from_arg_matches(&mut self, matches: &ArgMatches) -> Result<(), clap::Error> {
        *self = Self::from_arg_matches(matches)?;
        Ok(())
    }
}

/// The override a stored DLSS value turns on, as Go's `dlss set` does.
fn implied_override(key: FieldKey) -> Option<FieldKey> {
    match key {
        FieldKey::DlssSrMode | FieldKey::DlssSrPreset => Some(FieldKey::DlssSrOverride),
        FieldKey::DlssRrMode | FieldKey::DlssRrPreset => Some(FieldKey::DlssRrOverride),
        FieldKey::DlssFgEnabled | FieldKey::DlssMultiFrame => Some(FieldKey::DlssFgOverride),
        _ => None,
    }
}

/// The values to store and the fields to follow for one run's flags, with
/// the overrides they imply.
pub fn changes(settings: &[(FieldKey, Setting)]) -> (Vec<(FieldKey, Value)>, Vec<FieldKey>) {
    let mut values = Vec::new();
    let mut follow = Vec::new();
    for (key, setting) in settings {
        match setting {
            Setting::Follow => follow.push(*key),
            Setting::Store(value) => values.push((*key, value.clone())),
            Setting::Absent => {}
        }
    }
    for (key, setting) in settings {
        let Setting::Store(_) = setting else { continue };
        let Some(implied) = implied_override(*key) else {
            continue;
        };
        let given = settings.iter().any(|(k, _)| *k == implied);
        let added = values.iter().any(|(k, _)| *k == implied);
        if !given && !added {
            values.push((implied, Value::Bool(true)));
        }
    }
    (values, follow)
}

fn updated_message(section: Section, name: &str) -> String {
    match section {
        Section::Dlss => format!("Updated DLSS configuration for {name}"),
        other => format!("Updated {} profile for {name}", other.title()),
    }
}

/// Runs `<section> set`.
pub fn set<const S: u8>(
    args: &SetArgs<S>,
    ctx: &mut Context,
    out: &mut dyn Write,
) -> Result<(), CliError> {
    let target = target(ctx, args.game.as_deref(), args.all_games)?;
    if args.settings.is_empty() {
        return line(out, NO_CHANGES);
    }
    for (key, setting) in &args.settings {
        if let (FieldKey::CpuGovernor, Setting::Store(Value::Text(governor))) = (key, setting) {
            check_governor(ctx.backends.cpu.as_mut(), governor)?;
        }
    }
    let (values, follow) = changes(&args.settings);
    store::edit(&ctx.paths, target.scope, &target.name, &values, &follow).map_err(profile_error)?;
    line(out, &updated_message(section(S), target.label()))
}

/// Go's `validateCPUGovernorFlag`: a governor is valid when sysfs lists it
/// (architecture, CPU). An empty one leaves the governor unchanged and is
/// not checked.
fn check_governor(cpu: &mut dyn Cpu, governor: &str) -> Result<(), CliError> {
    if governor.is_empty() {
        return Ok(());
    }
    let available = cpu.available_governors().map_err(|e| {
        CliError::new(
            exit::FAILURE,
            format!("--governor: read available CPU governors: {e}"),
        )
    })?;
    if available.iter().any(|g| g == governor) {
        return Ok(());
    }
    let listed = if available.is_empty() {
        "none".to_owned()
    } else {
        available.join(", ")
    };
    Err(CliError::new(
        exit::USAGE,
        format!("--governor: CPU governor {governor:?} is not available (available: {listed})"),
    ))
}

/// `<section> reset <game> [field]...` or `--all-games [field]...`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ResetArgs<const S: u8> {
    /// Change All games instead of a game.
    pub all_games: bool,
    /// The game, then the fields; only fields with `--all-games`.
    pub words: Vec<String>,
}

/// The names a section's reset help lists: Go's short names for Proton,
/// the field names elsewhere, as Go's help texts list them.
fn reset_names(section: Section) -> Vec<&'static str> {
    section
        .fields()
        .map(|f| match (section, f.aliases.first()) {
            (Section::Proton, Some(alias)) => *alias,
            _ => f.name(),
        })
        .collect()
}

impl<const S: u8> Args for ResetArgs<S> {
    fn augment_args(cmd: Command) -> Command {
        let s = section(S);
        let name = cmd.get_name().to_owned();
        let usage = format!(
            "spela {sub} {name} <GAME> [FIELD]...\n       spela {sub} {name} --all-games [FIELD]...",
            sub = s.name()
        );
        cmd.override_usage(usage)
            .arg(
                Arg::new("words")
                    .value_name("GAME | FIELD")
                    .num_args(0..)
                    .required_unless_present("all_games")
                    .help(format!(
                        "The game, then the fields to reset. Without a field, every {} setting",
                        s.title()
                    )),
            )
            .arg(all_games_arg("Reset"))
            .after_help(format!("Fields: {}.", reset_names(s).join(", ")))
    }

    fn augment_args_for_update(cmd: Command) -> Command {
        Self::augment_args(cmd)
    }
}

impl<const S: u8> FromArgMatches for ResetArgs<S> {
    fn from_arg_matches(matches: &ArgMatches) -> Result<Self, clap::Error> {
        Ok(ResetArgs {
            all_games: matches.get_flag("all_games"),
            words: matches
                .get_many::<String>("words")
                .map(|w| w.cloned().collect())
                .unwrap_or_default(),
        })
    }

    fn update_from_arg_matches(&mut self, matches: &ArgMatches) -> Result<(), clap::Error> {
        *self = Self::from_arg_matches(matches)?;
        Ok(())
    }
}

/// Go's `unknown <SUB> field "<f>"`, with Proton's list of names.
fn unknown_field(section: Section, input: &str) -> CliError {
    let (sub, suffix) = match section {
        Section::Proton => (
            "proton".to_owned(),
            format!(" (valid: {})", reset_names(section).join(", ")),
        ),
        other => (other.title().to_owned(), String::new()),
    };
    CliError::new(
        exit::USAGE,
        format!("unknown {sub} field \"{input}\"{suffix}"),
    )
}

/// Runs `<section> reset`.
pub fn reset<const S: u8>(
    args: &ResetArgs<S>,
    ctx: &mut Context,
    out: &mut dyn Write,
) -> Result<(), CliError> {
    let s = section(S);
    let (game, fields) = match (args.all_games, args.words.split_first()) {
        (true, _) => (None, &args.words[..]),
        (false, Some((game, fields))) => (Some(game.as_str()), fields),
        (false, None) => (None, &[][..]),
    };
    let target = target(ctx, game, args.all_games)?;
    let mut keys: Vec<FieldKey> = Vec::new();
    for input in fields {
        let key = s
            .lookup(input)
            .map(|f| f.id)
            .ok_or_else(|| unknown_field(s, input))?;
        if !keys.contains(&key) {
            keys.push(key);
        }
    }
    if keys.is_empty() {
        let all: Vec<FieldKey> = s.fields().map(|f| f.id).collect();
        let removed = store::follow(&ctx.paths, target.scope, &all).map_err(profile_error)?;
        let what = format!("all {} settings", s.title());
        return line(
            out,
            &clear_all_message(&target, &what, removed.map(|r| r.len())),
        );
    }
    let removed = store::follow(&ctx.paths, target.scope, &keys).map_err(profile_error)?;
    for key in keys {
        let cleared = removed
            .as_ref()
            .is_some_and(|r| r.iter().any(|(k, _)| *k == key));
        let text = follow_message(&target, key.field().label, removed.is_some(), cleared);
        line(out, &text)?;
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parse<A: Args + FromArgMatches>(args: &[&str]) -> Result<A, clap::Error> {
        let cmd = A::augment_args(Command::new("set").no_binary_name(true));
        let matches = cmd.try_get_matches_from(args)?;
        A::from_arg_matches(&matches)
    }

    fn store(v: Value) -> Setting {
        Setting::Store(v)
    }

    #[test]
    fn each_section_gets_one_flag_per_field_with_a_flag() {
        let cmd = GpuSetArgs::augment_args(Command::new("set"));
        let longs: Vec<&str> = cmd.get_arguments().filter_map(Arg::get_long).collect();
        assert_eq!(
            longs,
            [
                "all-games",
                "clock-offset",
                "vrr",
                "memory-offset",
                "power-limit",
                "fan-speed",
                "shader-cache",
                "shader-cache-path",
                "threaded-opt",
            ]
        );
        let dlss = DlssSetArgs::augment_args(Command::new("set"));
        assert!(
            dlss.get_arguments()
                .any(|a| a.get_long() == Some("sr-override"))
        );
        assert!(
            !dlss
                .get_arguments()
                .any(|a| a.get_long() == Some("sr-model-preset"))
        );
        let fan = cmd
            .get_arguments()
            .find(|a| a.get_long() == Some("fan-speed"))
            .unwrap();
        let help = fan.get_help().unwrap().to_string();
        assert!(help.contains("0 means no fan change at launch"), "{help}");
        let sr = dlss
            .get_arguments()
            .find(|a| a.get_long() == Some("sr-preset"))
            .unwrap();
        let help = sr.get_help().unwrap().to_string();
        assert!(
            help.contains("Values: default, auto, A, B, C, D, E, F, J, K, L, M."),
            "{help}"
        );
    }

    #[test]
    fn flags_parse_values_default_and_negative_numbers() {
        let a: GpuSetArgs =
            parse(&["x", "--clock-offset", "-100", "--fan-speed", "default"]).unwrap();
        assert_eq!(a.game.as_deref(), Some("x"));
        assert_eq!(
            a.settings,
            [
                (FieldKey::GpuClockOffset, store(Value::Int(-100))),
                (FieldKey::GpuFanSpeed, Setting::Follow),
            ]
        );
        let a: GpuSetArgs = parse(&["--all-games", "--fan-speed", "0"]).unwrap();
        assert!(a.all_games && a.game.is_none());
        assert_eq!(a.settings, [(FieldKey::GpuFanSpeed, store(Value::Int(0)))]);
        assert!(parse::<GpuSetArgs>(&["x", "--all-games"]).is_err());
        assert!(parse::<GpuSetArgs>(&[]).is_err(), "a game or --all-games");
        let a: DlssSetArgs = parse(&["x", "--sr-preset", "default", "--indicator"]).unwrap();
        assert_eq!(
            a.settings,
            [
                (FieldKey::DlssSrPreset, store(Value::Text("default".into()))),
                (FieldKey::DlssIndicator, store(Value::Bool(true))),
            ]
        );
        let a: DlssSetArgs = parse(&["x", "--fg-indicator=off"]).unwrap();
        assert_eq!(
            a.settings,
            [(FieldKey::DlssFgIndicator, store(Value::Bool(false)))]
        );
        let e = parse::<DlssSetArgs>(&["x", "--rr-preset", "auto"]).unwrap_err();
        assert_eq!(e.exit_code(), 2);
        assert!(
            e.to_string()
                .contains("dlss.rr_preset: unsupported value \"auto\""),
            "{e}"
        );
        let e = parse::<ProtonSetArgs>(&["x", "--hdr", "maybe"]).unwrap_err();
        assert!(
            e.to_string()
                .contains("invalid bool \"maybe\" (use true/false)"),
            "{e}"
        );
        let e = parse::<CpuSetArgs>(&["x", "--smt", "sometimes"]).unwrap_err();
        assert!(
            e.to_string()
                .contains("invalid SMT value: sometimes (use on, off, unchanged, or default)"),
            "{e}"
        );
        let a: CpuSetArgs = parse(&["x", "--smt", "yes", "--governor", "default"]).unwrap();
        assert_eq!(
            a.settings,
            [
                (FieldKey::CpuGovernor, Setting::Follow),
                (FieldKey::CpuSmt, store(Value::OptionalBool(Some(true)))),
            ]
        );
        // Decision 65: SMT left unchanged even when All games sets it.
        for word in ["unchanged", "Unchanged"] {
            let a: CpuSetArgs = parse(&["x", "--smt", word]).unwrap();
            assert_eq!(
                a.settings,
                [(FieldKey::CpuSmt, store(Value::OptionalBool(None)))]
            );
        }
        let e = parse::<GpuSetArgs>(&["x", "--shader-cache-path", "rel"]).unwrap_err();
        assert!(e.to_string().contains("must be an absolute path"), "{e}");
        let a: GpuSetArgs = parse(&["x", "--shader-cache-path", "default"]).unwrap();
        assert_eq!(
            a.settings,
            [(FieldKey::GpuShaderCachePath, Setting::Follow)]
        );
        let a: CpuSetArgs = parse(&["x"]).unwrap();
        assert!(a.settings.is_empty());
        // Go's string flags took an empty value as not given; its typed
        // flags refused it.
        let a: CpuSetArgs = parse(&["x", "--governor", "", "--smt", ""]).unwrap();
        assert!(a.settings.is_empty());
        let a: GpuSetArgs = parse(&["x", "--shader-cache-path", "", "--vrr", ""]).unwrap();
        assert!(a.settings.is_empty());
        assert!(parse::<GpuSetArgs>(&["x", "--fan-speed", ""]).is_err());
        assert!(parse::<DlssSetArgs>(&["x", "--indicator="]).is_err());
    }

    #[test]
    fn stored_dlss_values_turn_their_override_on_unless_given() {
        let (values, follow) = changes(&[
            (FieldKey::DlssSrMode, store(Value::Text("dlaa".into()))),
            (FieldKey::DlssRrOverride, store(Value::Bool(false))),
            (FieldKey::DlssRrPreset, store(Value::Text("K".into()))),
            (FieldKey::DlssMultiFrame, store(Value::Int(2))),
            (FieldKey::DlssFgEnabled, Setting::Follow),
        ]);
        assert_eq!(
            values,
            [
                (FieldKey::DlssSrMode, Value::Text("dlaa".into())),
                (FieldKey::DlssRrOverride, Value::Bool(false)),
                (FieldKey::DlssRrPreset, Value::Text("K".into())),
                (FieldKey::DlssMultiFrame, Value::Int(2)),
                (FieldKey::DlssSrOverride, Value::Bool(true)),
                (FieldKey::DlssFgOverride, Value::Bool(true)),
            ]
        );
        assert_eq!(follow, [FieldKey::DlssFgEnabled]);
        let (values, _) = changes(&[(FieldKey::DlssSrMode, Setting::Follow)]);
        assert!(values.is_empty(), "following turns nothing on");
    }

    #[test]
    fn reset_takes_a_game_then_fields_or_only_fields_with_all_games() {
        let a: DlssResetArgs = parse(&["x"]).unwrap();
        assert_eq!(a.words, ["x"]);
        let a: DlssResetArgs = parse(&["--all-games", "sr_mode", "fg"]).unwrap();
        assert!(a.all_games);
        assert_eq!(a.words, ["sr_mode", "fg"]);
        let a: DlssResetArgs = parse(&["--all-games"]).unwrap();
        assert!(a.words.is_empty());
        assert!(parse::<DlssResetArgs>(&[]).is_err());
        assert_eq!(
            reset_names(Section::Proton),
            ["hdr", "wayland", "ngx_updater", "vkd3d_heap"]
        );
        assert_eq!(
            unknown_field(Section::Proton, "x").message,
            "unknown proton field \"x\" (valid: hdr, wayland, ngx_updater, vkd3d_heap)"
        );
        assert_eq!(
            unknown_field(Section::Dlss, "power_mizer").message,
            "unknown DLSS field \"power_mizer\""
        );
        assert_eq!(
            updated_message(Section::Dlss, "X"),
            "Updated DLSS configuration for X"
        );
        assert_eq!(
            updated_message(Section::Gpu, "X"),
            "Updated GPU profile for X"
        );
        assert_eq!(
            updated_message(Section::Proton, "X"),
            "Updated Proton profile for X"
        );
    }
}
