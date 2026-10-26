//! Drawing one frame.
//!
//! [`render`] draws the frame terminal-ux.md sets: the top bar with the
//! brand, destinations and live metrics; one outer box with the panes and
//! their titles in the border; the status line; the hint bar read from the
//! keymap; and the open popup over the dimmed workspace. Pane contents are
//! the screens' ([`crate::screens::render_pane`]). Every clickable label
//! records its region in the frame's [`HitMap`].
//!
//! It reads the state and writes only the hit map, so a frame drawn twice
//! from the same state is the same frame.

use ratatui::Frame;
use ratatui::layout::Rect;
use spela_core::alerts::{self, Severity};

use crate::hit::{HitMap, HitTarget};
use crate::keymap::Binding;
use crate::layout::{Layout, Mode, Pane};
use crate::paint::{Painter, Span};
use crate::screens::{self, Action, Dest, Popup};
use crate::state::{AppState, Level};
use crate::text::width;
use crate::theme::{Role, temperature_ratio, thermal_step};

/// Draws the whole frame and records its click targets in `state.hits`.
pub fn render(state: &mut AppState, frame: &mut Frame) {
    let mut hits = HitMap::new();
    draw(state, frame, &mut hits);
    state.hits = hits;
}

/// [`render`] without touching the state: the frame and the hit map.
pub fn draw(state: &AppState, frame: &mut Frame, hits: &mut HitMap) {
    let layout = state.layout();
    let area = frame.area();
    if layout.mode == Mode::TooSmall {
        resize_prompt(state, frame, area);
        return;
    }
    top_bar(state, frame, &layout, hits);
    panes(state, frame, &layout, hits);
    if state.popup.is_some() {
        let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
        p.dim_backdrop(layout.frame);
        screens::render_popup(state, frame, &layout, hits);
    }
    status_line(state, frame, &layout);
    hint_bar(state, frame, &layout, hits);
}

fn resize_prompt(state: &AppState, frame: &mut Frame, area: Rect) {
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    let lines = [
        ("Resize terminal".to_owned(), Role::Strong),
        ("spela needs at least 80x24".to_owned(), Role::Text),
        (
            format!("Current size: {}x{}", area.width, area.height),
            Role::Muted,
        ),
    ];
    let top = area.height.saturating_sub(3) / 2;
    for (i, (text, role)) in lines.iter().enumerate() {
        let y = top + i as u16;
        if y >= area.height {
            break;
        }
        let w = u16::try_from(width(text))
            .unwrap_or(u16::MAX)
            .min(area.width);
        p.text((area.width - w) / 2, y, area.width, text, *role);
    }
}

/// The destinations part of the top bar: where each one is and its text.
fn destinations(state: &AppState) -> Vec<(Dest, u16, String)> {
    let digits = state.browsing();
    let mut x = 9u16;
    let mut out = Vec::new();
    for (i, dest) in Dest::ALL.iter().enumerate() {
        if i > 0 {
            x += 2;
        }
        let mut label = String::new();
        if *dest == state.dest {
            label.push_str(state.glyphs.cursor);
            label.push(' ');
        }
        if digits {
            label.push(dest.digit());
            label.push(' ');
        }
        label.push_str(dest.name());
        let w = u16::try_from(width(&label)).unwrap_or(0);
        out.push((*dest, x, label));
        x += w;
    }
    out
}

fn top_bar(state: &AppState, frame: &mut Frame, layout: &Layout, hits: &mut HitMap) {
    let w = layout.area.width;
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    p.text(0, 0, w, " spela ", Role::Brand);
    let dests = destinations(state);
    let mut end = 9;
    for (dest, x, label) in &dests {
        let role = if *dest == state.dest {
            Role::Key
        } else {
            Role::Text
        };
        end = p.text(*x, 0, w.saturating_sub(*x), label, role);
        hits.push(
            Rect::new(*x, 0, end - x, 1),
            HitTarget::Action(Action::Go(*dest)),
        );
    }
    let room = usize::from(w.saturating_sub(1).saturating_sub(end + 2));
    let parts = metrics_parts(state, room);
    let spans: Vec<Span<'_>> = parts.iter().map(|(t, r)| (t.as_str(), *r)).collect();
    p.spans_right(w.saturating_sub(1), 0, &spans);
}

/// The metrics on the right of the top bar, cut to `room` cells in the
/// drop order terminal-ux.md sets: the sparkline first, then CPU, then
/// power, then use and the `GPU` label. The temperature and the alert word
/// are never dropped.
pub fn metrics_parts(state: &AppState, room: usize) -> Vec<(String, Role)> {
    let g = &state.glyphs;
    let m = &state.metrics;
    let alert = alerts::highest(&m.alerts).map(|a| {
        let (glyph, role) = match a.severity {
            Severity::Critical => (g.error, Role::Error),
            Severity::Warning => (g.warn, Role::Warn),
            Severity::Info => (g.warn, Role::Muted),
        };
        (format!("{glyph} {}", a.label()), role)
    });
    let mut temp = None;
    let mut util = None;
    let mut power = None;
    let mut spark: Vec<(String, Role)> = Vec::new();
    match &m.gpu {
        Some(Ok(gpu)) => {
            let step = thermal_step(temperature_ratio(gpu.temperature_c));
            temp = Some((
                format!("{}{}", gpu.temperature_c, g.degree),
                Role::Thermal(step),
            ));
            util = Some(format!(" {}%", gpu.utilization_percent));
            power = Some(format!(" {}W", gpu.power_draw_w));
            for t in m.temperature.last(5) {
                let r = temperature_ratio(t);
                spark.push((g.spark_for(r).to_owned(), Role::Thermal(thermal_step(r))));
            }
        }
        Some(Err(_)) => temp = Some(("N/A".to_owned(), Role::Muted)),
        None => {}
    }
    let cpu = match &m.cpu {
        Some(Ok(c)) => c.utilization_percent.map(|u| format!("{u}%")),
        Some(Err(_)) => Some("N/A".to_owned()),
        None => None,
    };
    // Try the full form, then drop parts in order until it fits.
    for level in 0..5 {
        let mut parts: Vec<(String, Role)> = Vec::new();
        if let Some(t) = &temp {
            if level < 4 {
                parts.push(("GPU ".into(), Role::Muted));
            }
            parts.push(t.clone());
            if level < 4
                && let Some(u) = &util
            {
                parts.push((u.clone(), Role::Text));
            }
            if level < 3
                && let Some(pw) = &power
            {
                parts.push((pw.clone(), Role::Text));
            }
            if level < 1 && !spark.is_empty() {
                parts.push((" ".into(), Role::Text));
                parts.extend(spark.iter().cloned());
            }
        }
        if level < 2
            && let Some(c) = &cpu
        {
            parts.push((
                if temp.is_some() { " CPU " } else { "CPU " }.into(),
                Role::Muted,
            ));
            parts.push((c.clone(), Role::Text));
        }
        if let Some(a) = &alert {
            // The last form keeps the temperature and the alert word with
            // one space between, as the 80-column mockup does.
            let sep = match (parts.is_empty(), level) {
                (true, _) => "",
                (false, 4) => " ",
                (false, _) => "  ",
            };
            parts.push((sep.into(), Role::Text));
            parts.push(a.clone());
        }
        let total: usize = parts.iter().map(|(t, _)| width(t)).sum();
        if total <= room || level == 4 {
            if total > room {
                // Even the last form does not fit: keep the alert alone.
                return alert.into_iter().collect();
            }
            return parts;
        }
    }
    Vec::new()
}

fn panes(state: &AppState, frame: &mut Frame, layout: &Layout, hits: &mut HitMap) {
    let theme = state.theme;
    {
        let mut p = Painter::new(frame.buffer_mut(), theme, state.glyphs);
        p.frame(layout.frame, "", Role::Border, Role::Muted);
        if layout.two_panes() {
            let x = layout.list.border_x;
            let style = p.style(Role::Border);
            let f = layout.frame;
            let b = state.glyphs.border;
            for y in f.top() + 1..f.bottom() - 1 {
                p.buf.set_string(x, y, b.vertical_left, style);
            }
            p.buf.set_string(x, f.top(), state.glyphs.joins[2], style);
            p.buf
                .set_string(x, f.bottom() - 1, state.glyphs.joins[3], style);
        }
    }
    let shown: Vec<Pane> = if layout.two_panes() {
        vec![Pane::List, Pane::Detail]
    } else {
        vec![state.focus]
    };
    for pane in shown {
        let area = layout.pane(pane);
        hits.push(area.inner, HitTarget::Pane(pane));
        screens::render_pane(state, frame, area, hits);
        let mut p = Painter::new(frame.buffer_mut(), theme, state.glyphs);
        let focused = state.focus == pane;
        let (title, role) = if layout.two_panes() {
            let role = if focused { Role::Key } else { Role::Muted };
            (screens::title(state, pane), role)
        } else {
            let word = match pane {
                Pane::List => "List",
                Pane::Detail => "Detail",
            };
            (format!("{} {word}", state.glyphs.cursor), Role::Key)
        };
        let (row, col) = area.title_at;
        let max = area.inner.width.saturating_sub(2);
        p.title(col, row, max, &title, role);
        if let Some(text) = screens::footer(state, pane) {
            let bottom = layout.frame.bottom() - 1;
            p.title(col, bottom, max, &text, Role::Muted);
        }
    }
}

/// The status line's work state, on the right: running work first.
pub fn work_state(state: &AppState) -> Option<(String, Role)> {
    if state.jobs.hardware.is_some() {
        return Some(("Applying...".into(), Role::Warn));
    }
    if !state.saves.is_empty() {
        return Some(("Saving...".into(), Role::Warn));
    }
    if state.jobs.scan.is_some() {
        return Some(("Scanning games...".into(), Role::Warn));
    }
    if let Some(notice) = &state.release {
        return Some((notice.clone(), Role::Warn));
    }
    state.saved_once.then(|| ("Saved".into(), Role::Muted))
}

fn status_line(state: &AppState, frame: &mut Frame, layout: &Layout) {
    let w = layout.area.width;
    let y = layout.status.y;
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    p.clear(layout.status);
    let right = work_state(state);
    let right_w = right
        .as_ref()
        .map_or(0, |(t, _)| u16::try_from(width(t)).unwrap_or(0));
    let left_max = w.saturating_sub(right_w + 4);
    let popup_text = match &state.popup {
        Some(Popup::Palette(pal)) => Some(screens::palette::status_text(pal)),
        _ => None,
    };
    if let Some(text) = popup_text {
        p.text(1, y, left_max, &text, Role::Muted);
    } else if state.popup.is_none()
        && let Some(m) = &state.status
    {
        let (text, role) = match m.level {
            Level::Info => (m.text.clone(), Role::Text),
            Level::Ok => (format!("{} {}", state.glyphs.ok, m.text), Role::Ok),
            Level::Error => (format!("{} {}", state.glyphs.error, m.text), Role::Error),
        };
        p.text(1, y, left_max, &text, role);
    }
    if let Some((text, role)) = right {
        p.spans_right(w.saturating_sub(1), y, &[(text.as_str(), role)]);
    }
}

/// The hints that fit in `room` cells: the hinted, available bindings in
/// order, dropping the lowest priority first. `0 Actions` and the other
/// hints marked always stay.
pub fn fitted_hints(bindings: &[Binding], room: usize) -> Vec<&Binding> {
    let mut shown: Vec<&Binding> = Vec::new();
    for b in bindings {
        if b.hint.is_none() || !b.available() {
            continue;
        }
        let text = b.key_text();
        // A key hinted twice shows once, the first time.
        if shown.iter().any(|s| s.key_text() == text) {
            continue;
        }
        shown.push(b);
    }
    let cost = |s: &[&Binding]| -> usize {
        s.iter()
            .map(|b| width(&b.key_text()) + 1 + width(&b.label))
            .sum::<usize>()
            + 2 * s.len().saturating_sub(1)
    };
    while cost(&shown) > room {
        let drop = shown
            .iter()
            .enumerate()
            .filter(|(_, b)| {
                b.hint
                    .is_some_and(|h| h.priority != crate::keymap::Hint::ALWAYS)
            })
            .min_by_key(|(i, b)| (b.hint.map_or(0, |h| h.priority), usize::MAX - i))
            .map(|(i, _)| i);
        match drop {
            Some(i) => {
                shown.remove(i);
            }
            None => break,
        }
    }
    shown
}

fn hint_bar(state: &AppState, frame: &mut Frame, layout: &Layout, hits: &mut HitMap) {
    let w = layout.area.width;
    let y = layout.hints.y;
    let bindings = screens::bindings(state);
    let room = usize::from(w.saturating_sub(2));
    let shown = fitted_hints(&bindings, room);
    let mut p = Painter::new(frame.buffer_mut(), state.theme, state.glyphs);
    p.clear(layout.hints);
    let mut x = 1u16;
    let end = w.saturating_sub(1);
    for (i, b) in shown.iter().enumerate() {
        if i > 0 {
            x += 2;
        }
        if x >= end {
            break;
        }
        let start = x;
        x = p.text(x, y, end - x, &b.key_text(), Role::Key);
        x = p.text(
            x,
            y,
            end.saturating_sub(x),
            &format!(" {}", b.label),
            Role::Muted,
        );
        hits.push(
            Rect::new(start, y, x - start, 1),
            HitTarget::Action(b.action.clone()),
        );
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::keymap::Key;

    #[test]
    fn hints_drop_the_lowest_priority_first_and_keep_always() {
        let b = vec![
            Binding::new(Key::char('0'), Action::Actions, "Actions")
                .hint(crate::keymap::Hint::ALWAYS),
            Binding::new(Key::UP, Action::Help, "Field").hint_as("Up/Down", 100),
            Binding::new(Key::LEFT, Action::Help, "Section").hint_as("Left/Right", 40),
            Binding::new(Key::ESC, Action::Back, "List").hint(10),
            Binding::new(Key::TAB, Action::NextPane, "List").hint(crate::keymap::Hint::ALWAYS),
            Binding::new(Key::char('?'), Action::Help, "Help")
                .hint(20)
                .unavailable("x"),
        ];
        let all = fitted_hints(&b, 200);
        let labels: Vec<&str> = all.iter().map(|b| b.label.as_str()).collect();
        assert_eq!(labels, ["Actions", "Field", "Section", "List", "List"]);
        let narrow = fitted_hints(&b, 36);
        let labels: Vec<&str> = narrow.iter().map(|b| b.label.as_str()).collect();
        assert_eq!(labels, ["Actions", "Field", "List"]);
        let none = fitted_hints(&b, 0);
        assert_eq!(none.len(), 2, "the always hints stay");
    }
}
