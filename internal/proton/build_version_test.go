package proton

import "testing"

func TestParseCachyOSBuildTag(t *testing.T) {
	tag, ok := ParseCachyOSBuildTag("cachyos-11.0-20260521-slr")
	if !ok {
		t.Fatal("expected parse success")
	}
	if tag.Major != 11 || tag.Minor != 0 || tag.Date != 20260521 {
		t.Fatalf("tag = %+v", tag)
	}
	if _, ok := ParseCachyOSBuildTag("GE-Proton10-34"); ok {
		t.Fatal("expected GE-Proton name to not parse")
	}
}

func TestBuildTagMeetsMinimum(t *testing.T) {
	if !buildTagMeetsMinimum("cachyos-11.0-20260521-slr", MinProtonCachyOSIntegratedBuild) {
		t.Fatal("expected 11.0 integrated build to meet minimum")
	}
	if buildTagMeetsMinimum("cachyos-10.0-20260410-slr", MinProtonCachyOSIntegratedBuild) {
		t.Fatal("expected 10.x build to be below integrated minimum")
	}
}
