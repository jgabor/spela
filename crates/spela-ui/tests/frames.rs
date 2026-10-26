//! Frames on ratatui's TestBackend, as insta snapshots at the two sizes of
//! spela's layout contract, 120x40 and 80x24, plus the resize prompt.
//!
//! Read a changed snapshot against `docs/mockups.html` before accepting
//! it: the List is 32 cells wide at 120 columns, titles sit at column 2 of
//! their border, the metrics end one cell before the right edge, and the
//! status and hint text start at column 1.

use spela_core::alerts::{Alert, AlertKind, Severity};
use spela_core::backend::GpuMetrics;
use spela_core::metrics::CpuMetrics;
use spela_ui::event::{Event, MetricsSample, Msg};
use spela_ui::testing::{key, render_text, state_with_games};
use spela_ui::update;

fn sample(temp: u32, alerts: Vec<Alert>) -> Event {
    Event::Msg(Msg::Metrics(Box::new(MetricsSample {
        gpu: Ok(GpuMetrics {
            temperature_c: temp,
            utilization_percent: 37,
            memory_used_mib: 2048,
            memory_total_mib: 16376,
            power_draw_w: 182,
            graphics_clock_mhz: 1800,
            memory_clock_mhz: 10501,
            fan_percent: Some(40),
            ..GpuMetrics::default()
        }),
        cpu: Ok(CpuMetrics {
            utilization_percent: Some(12),
            mhz: Some(3900),
            memory_used_mib: Some(8000),
            memory_total_mib: Some(32000),
            ..CpuMetrics::default()
        }),
        alerts,
    })))
}

fn power_alert() -> Alert {
    Alert {
        kind: AlertKind::PowerLimit,
        severity: Severity::Warning,
        message: "GPU power-limited at 318W/320W".into(),
        suggestion: "Increase power limit to 352W for potential FPS gain".into(),
    }
}

#[test]
fn library_120x40() {
    let mut s = state_with_games(120, 40, 12);
    for t in [38, 44, 49, 52, 54] {
        update(&mut s, sample(t, Vec::new()));
    }
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn library_80x24_with_an_alert() {
    let mut s = state_with_games(80, 24, 12);
    update(&mut s, sample(71, vec![power_alert()]));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn library_detail_80x24() {
    let mut s = state_with_games(80, 24, 12);
    update(&mut s, key("tab"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn actions_popup_120x40() {
    let mut s = state_with_games(120, 40, 12);
    update(&mut s, key("0"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn actions_popup_80x24() {
    let mut s = state_with_games(80, 24, 12);
    update(&mut s, key("0"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn help_120x40() {
    let mut s = state_with_games(120, 40, 12);
    update(&mut s, key("?"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn palette_120x40() {
    let mut s = state_with_games(120, 40, 12);
    update(&mut s, key(":"));
    for c in ["d", "l"] {
        update(&mut s, key(c));
    }
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn find_80x24() {
    let mut s = state_with_games(80, 24, 12);
    update(&mut s, key("/"));
    for c in ["e", "x"] {
        update(&mut s, key(c));
    }
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn empty_library_120x40() {
    let mut s = state_with_games(120, 40, 0);
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn settings_placeholder_120x40() {
    let mut s = state_with_games(120, 40, 3);
    update(&mut s, key("4"));
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn resize_prompt_72x20() {
    let mut s = state_with_games(72, 20, 3);
    insta::assert_snapshot!(render_text(&mut s));
}

#[test]
fn ascii_glyphs_80x24() {
    let mut s = state_with_games(80, 24, 6);
    let mut config = s.config.clone();
    config.ui.ascii_glyphs = true;
    s.set_config(config);
    let text = render_text(&mut s);
    assert!(text.is_ascii(), "{text}");
    insta::assert_snapshot!(text);
}

#[test]
fn mono_and_no_color_paint_no_colour() {
    use ratatui::Terminal;
    use ratatui::backend::TestBackend;
    use spela_core::config::Theme;
    for (theme, no_color) in [(Theme::Mono, false), (Theme::Neon, true)] {
        let mut s = state_with_games(120, 40, 6);
        s.no_color = no_color;
        let mut config = s.config.clone();
        config.ui.theme = theme;
        s.set_config(config);
        update(&mut s, sample(90, vec![power_alert()]));
        let mut t = Terminal::new(TestBackend::new(120, 40)).unwrap();
        t.draw(|f| spela_ui::render(&mut s, f)).unwrap();
        for cell in t.backend().buffer().content() {
            assert_eq!(cell.fg, ratatui::style::Color::Reset, "{theme:?}");
            assert_eq!(cell.bg, ratatui::style::Color::Reset, "{theme:?}");
        }
    }
}
