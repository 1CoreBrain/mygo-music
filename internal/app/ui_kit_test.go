package app

import (
	"testing"
	"time"

	"mygo-music/internal/i18n"
)

// Durations read as minutes and seconds, and counts shortly, as people read
// them in the listener's language.
func TestClockAndCount(t *testing.T) {
	defer i18n.SetLanguage(i18n.English)
	if s := clock(185 * time.Second); s != "3:05" {
		t.Error(s)
	}
	for _, c := range []struct {
		language string
		n        int
		want     string
	}{
		{i18n.English, 9999, "9999"},
		{i18n.English, 56789, "56.8K"},
		{i18n.English, 1234567, "1.2M"},
		{i18n.Chinese, 56789, "5.7万"},
		{i18n.Chinese, 123456789, "1.2亿"},
	} {
		i18n.SetLanguage(c.language)
		if s := count(c.n); s != c.want {
			t.Errorf("%s: count(%d) = %q, want %q", c.language, c.n, s, c.want)
		}
	}
}
