package fetcher

import "testing"

func TestFingerprintPrefersGUID(t *testing.T) {
	a := ParsedItem{GUID: "guid-1", Link: "https://x/1", Title: "A"}
	if Fingerprint(a) != "guid-1" {
		t.Errorf("fingerprint = %q, want guid-1", Fingerprint(a))
	}
}

func TestFingerprintFallback(t *testing.T) {
	a := ParsedItem{Link: "https://x/1", Title: "A"}
	b := ParsedItem{Link: "https://x/1", Title: "A"}
	c := ParsedItem{Link: "https://x/2", Title: "A"}

	fa, fb, fc := Fingerprint(a), Fingerprint(b), Fingerprint(c)
	if fa == "" || fa != fb {
		t.Errorf("identical items should share fingerprint: %q vs %q", fa, fb)
	}
	if fa == fc {
		t.Error("different link should produce different fingerprint")
	}
	if Fingerprint(ParsedItem{}) != "" {
		t.Error("empty item should yield empty fingerprint")
	}
}

func TestDedup(t *testing.T) {
	items := []ParsedItem{
		{GUID: "g1", Title: "one"},
		{GUID: "g2", Title: "two"},
		{GUID: "g1", Title: "one duplicate"}, // 重复 guid
		{Title: "no identity"},               // 无 link/guid -> 丢弃
		{Link: "https://x/3", Title: "three"},
		{Link: "https://x/3", Title: "three"}, // 重复指纹
	}
	out := Dedup(items)
	if len(out) != 3 {
		t.Fatalf("dedup len = %d, want 3 (%v)", len(out), out)
	}
	if out[0].GUID != "g1" || out[1].GUID != "g2" || out[2].Link != "https://x/3" {
		t.Errorf("dedup order/content wrong: %+v", out)
	}
}
