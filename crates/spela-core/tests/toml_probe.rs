//! A probe for the config crates. The config and profile code that uses
//! toml and toml_edit arrives in Phases 2 and 3. Until then this test makes
//! `cargo +1.89 check --all-targets` build both at the MSRV, which the
//! architecture's Phase 1 asks to settle. Remove it when real code depends
//! on them.

#[test]
fn toml_reads_and_toml_edit_keeps_comments() {
    let text = "# spela settings\nversion = 1\n\n[ui]\n# The colours.\ntheme = \"mono\"\n";
    let value: toml::Table = toml::from_str(text).unwrap();
    assert_eq!(value["version"].as_integer(), Some(1));
    assert_eq!(value["ui"]["theme"].as_str(), Some("mono"));

    let mut doc: toml_edit::DocumentMut = text.parse().unwrap();
    doc["ui"]["theme"] = toml_edit::value("neon");
    let out = doc.to_string();
    assert!(out.starts_with("# spela settings\n"), "{out}");
    assert!(out.contains("# The colours.\ntheme = \"neon\"\n"), "{out}");
}
