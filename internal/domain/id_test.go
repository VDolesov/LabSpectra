package domain

import "testing"

func TestFormatID(t *testing.T) {
	cases := []struct {
		year, seq int
		want      string
	}{
		{2026, 1, "IR-2026-001"},
		{2026, 42, "IR-2026-042"},
		{2026, 999, "IR-2026-999"},
		{2026, 1000, "IR-2026-1000"},
	}
	for _, c := range cases {
		if got := FormatID(c.year, c.seq); got != c.want {
			t.Errorf("FormatID(%d,%d)=%q, want %q", c.year, c.seq, got, c.want)
		}
	}
}

func TestValidID(t *testing.T) {
	valid := []string{"IR-2026-001", "IR-2026-1000", "IR-1999-123"}
	for _, id := range valid {
		if !ValidID(id) {
			t.Errorf("ValidID(%q)=false, want true", id)
		}
	}
	invalid := []string{"", "IR-2026-1", "ir-2026-001", "IR-26-001", "XX-2026-001", "IR-2026-001-x"}
	for _, id := range invalid {
		if ValidID(id) {
			t.Errorf("ValidID(%q)=true, want false", id)
		}
	}
}

func TestNextID(t *testing.T) {

	if got := NextID(2026, nil); got != "IR-2026-001" {
		t.Errorf("NextID empty = %q, want IR-2026-001", got)
	}

	existing := []string{"IR-2026-001", "IR-2026-003", "IR-2025-099", "broken"}
	if got := NextID(2026, existing); got != "IR-2026-004" {
		t.Errorf("NextID = %q, want IR-2026-004", got)
	}

	if got := NextID(2027, existing); got != "IR-2027-001" {
		t.Errorf("NextID new year = %q, want IR-2027-001", got)
	}
}

func TestParseID(t *testing.T) {
	y, s, ok := ParseID("IR-2026-042")
	if !ok || y != 2026 || s != 42 {
		t.Errorf("ParseID = (%d,%d,%v), want (2026,42,true)", y, s, ok)
	}
	if _, _, ok := ParseID("nope"); ok {
		t.Error("ParseID(nope) ok=true, want false")
	}
}
