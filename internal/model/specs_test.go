package model

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	now := int64(1700000000)
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"6d", 6 * 86400, false},
		{"20h", 20 * 3600, false},
		{"1s", 1, false},
		{"5m", 300, false},
		{"3d4h5m", 3*86400 + 4*3600 + 5*60, false},
		{"3d4h5m1s", 3*86400 + 4*3600 + 5*60 + 1, false},
		{"1d1d", 0, true}, // repeated unit
		{"0d", 0, false},  // zero allowed ("expire now")
		{"d", 0, true},    // missing number
		{"3", 0, true},    // missing unit
		{"3y", 0, true},   // no years on purpose
		{"3mo", 0, true},  // no months on purpose
		{"-3d", 0, true},  // sign not allowed here
	}
	for _, c := range cases {
		got, err := ParseDuration(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseDuration(%q) = %d, want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseDuration(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseDuration(%q) = %d, want %d", c.in, got, c.want)
		}
	}
	_ = now
}

func TestParseTimeArg(t *testing.T) {
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.Local).Unix()
	day := int64(86400)

	// exact relative
	spec, err := ParseTimeArg("3d4h5m", now)
	if err != nil || !spec.Exact || *spec.Lo != now-(3*day+4*3600+300) {
		t.Fatalf("exact relative: %+v err=%v", spec, err)
	}
	// newer than
	spec, _ = ParseTimeArg("+3d5m7s", now)
	if spec.Lo == nil || *spec.Lo != now-(3*day+5*60+7) || spec.Hi != nil {
		t.Errorf("+3d5m7s: %+v", spec)
	}
	// older than
	spec, _ = ParseTimeArg("-121d", now)
	if spec.Hi == nil || *spec.Hi != now-121*day || spec.Lo != nil {
		t.Errorf("-121d: %+v", spec)
	}
	// absolute datetime, all separators, optional seconds
	want := time.Date(2027, 1, 1, 3, 45, 1, 0, time.Local).Unix()
	for _, s := range []string{"2027-01-01 03:45:01", "2027/01/01 03:45:01", "2027_01_01 03:45:01"} {
		spec, err := ParseTimeArg(s, now)
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		if !spec.Exact || *spec.Lo != want || *spec.Hi != want {
			t.Errorf("%q: got %d want %d", s, *spec.Lo, want)
		}
	}
	// date only, no time
	want = time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local).Unix()
	for _, s := range []string{"2027/01/01", "2027-1-1", "2027_1_1", "20270101"} {
		spec, err := ParseTimeArg(s, now)
		if err != nil || !spec.Exact || *spec.Lo != want {
			t.Errorf("%q: %+v err=%v", s, spec, err)
		}
	}
	// signed absolute
	spec, _ = ParseTimeArg("+2027/1/1 3:45", now)
	if spec.Lo == nil || *spec.Lo != time.Date(2027, 1, 1, 3, 45, 0, 0, time.Local).Unix() || spec.Hi != nil {
		t.Errorf("+2027/1/1 3:45: %+v", spec)
	}
	spec, _ = ParseTimeArg("-2027_01_01", now)
	if spec.Hi == nil || *spec.Hi != want || spec.Lo != nil {
		t.Errorf("-2027_01_01: %+v", spec)
	}
	// errors
	for _, s := range []string{"", "hello", "2027-13-01", "2027-02-30"} {
		if _, err := ParseTimeArg(s, now); err == nil {
			t.Errorf("ParseTimeArg(%q) should fail", s)
		}
	}
}

func TestTimeSpecMatch(t *testing.T) {
	lo, hi := int64(100), int64(200)
	s := &TimeSpec{Lo: &lo, Hi: &hi}
	for _, v := range []int64{100, 150, 200} {
		if !s.Match(v) {
			t.Errorf("Match(%d) = false, want true", v)
		}
	}
	for _, v := range []int64{99, 201, 0, 1000} {
		if s.Match(v) {
			t.Errorf("Match(%d) = true, want false", v)
		}
	}
	s = &TimeSpec{Lo: &lo}
	if !s.Match(1000) || s.Match(99) {
		t.Error("lower bound mismatch")
	}
	var nilSpec *TimeSpec
	if !nilSpec.Match(42) {
		t.Error("nil spec must match everything")
	}
}

func TestParseSizeArg(t *testing.T) {
	cases := []struct {
		in        string
		wantLo    int64
		wantHi    int64
		wantExact bool
		wantErr   bool
	}{
		{"10t20g30m40k50b", 10*(1<<40) + 20*(1<<30) + 30*(1<<20) + 40*1024 + 50, 10*(1<<40) + 20*(1<<30) + 30*(1<<20) + 40*1024 + 50, true, false},
		{"+30m50b", 30*(1<<20) + 50, 0, false, false},
		{"-1g", 0, 1 << 30, false, false},
		{"1k", 1024, 1024, true, false},
		{"0b", 0, 0, true, false},
		{"", 0, 0, false, true},
		{"10", 0, 0, false, true},
		{"1z", 0, 0, false, true},
		{"g", 0, 0, false, true},
	}
	for _, c := range cases {
		got, err := ParseSizeArg(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseSizeArg(%q) should fail", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSizeArg(%q): %v", c.in, err)
			continue
		}
		if got.Exact != c.wantExact {
			t.Errorf("ParseSizeArg(%q).Exact = %v, want %v", c.in, got.Exact, c.wantExact)
		}
		if c.wantExact {
			if got.Lo == nil || *got.Lo != c.wantLo {
				t.Errorf("ParseSizeArg(%q).Lo = %v, want %d", c.in, got.Lo, c.wantLo)
			}
			continue
		}
		if c.wantLo > 0 && (got.Lo == nil || *got.Lo != c.wantLo) {
			t.Errorf("ParseSizeArg(%q).Lo = %v, want %d", c.in, got.Lo, c.wantLo)
		}
		if c.wantHi > 0 && (got.Hi == nil || *got.Hi != c.wantHi) {
			t.Errorf("ParseSizeArg(%q).Hi = %v, want %d", c.in, got.Hi, c.wantHi)
		}
	}
}

func TestFormatSizeASCII(t *testing.T) {
	cases := map[int64]string{
		0: "0 B", 512: "512 B", 1024: "1.0K", 1536: "1.5K",
		1 << 20: "1.0M", 1 << 30: "1.0G", 1 << 40: "1.0T",
	}
	for in, want := range cases {
		if got := FormatSize(in); got != want {
			t.Errorf("FormatSize(%d) = %q, want %q", in, got, want)
		}
	}
	for _, s := range []string{FormatSize(0), FormatSize(1 << 40)} {
		for _, r := range s {
			if r > 127 {
				t.Errorf("FormatSize produced non-ASCII output %q", s)
			}
		}
	}
}
