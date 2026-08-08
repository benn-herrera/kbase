package tokens

import "testing"

func TestEstimate(t *testing.T) {
	for _, tc := range []struct {
		name string
		est  Estimator
		in   string
		want int
	}{
		{"empty string", Estimator{}, "", 0},
		{"zero value uses default ratio", Estimator{}, "abcdefgh", 2},
		{"partial token rounds up", Estimator{}, "abcde", 2},
		{"single byte is one token", Estimator{}, "a", 1},
		{"explicit default matches zero value", Estimator{CharsPerToken: DefaultCharsPerToken}, "abcdefgh", 2},
		{"negative ratio falls back to default", Estimator{CharsPerToken: -3}, "abcdefgh", 2},
		{"calibrated ratio is honored", Estimator{CharsPerToken: 2}, "abcdefgh", 4},
		{"fractional ratio is honored", Estimator{CharsPerToken: 3.5}, "abcdefgh", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.est.Estimate(tc.in); got != tc.want {
				t.Errorf("Estimate(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestEstimateCountsBytes pins the documented bytes-not-runes choice (§8):
// the ratio is calibrated against byte lengths, so a multi-byte rune must
// cost more than an ASCII one.
func TestEstimateCountsBytes(t *testing.T) {
	const multibyte = "日本語" // 3 runes, 9 bytes
	var e Estimator
	if got, want := e.Estimate(multibyte), 3; got != want {
		t.Errorf("Estimate(%q) = %d, want %d (9 bytes / 4)", multibyte, got, want)
	}
	if e.Estimate(multibyte) <= e.Estimate("abc") {
		t.Error("multi-byte text must not estimate at or below the same rune count of ASCII")
	}
}
