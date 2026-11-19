package core

import (
	"strings"
	"testing"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/shopspring/decimal"
)

func TestParseMiniColorsAndUnknownTags(t *testing.T) {
	txt := ParseMini("<primary>Usage: <secondary>/home <player></secondary> <dark_red>done", DefaultTheme)
	if got := txt.PlainString(); got != "Usage: /home <player> done" {
		t.Fatalf("plain = %q", got)
	}
	var colors []string
	for _, c := range txt.Children {
		colors = append(colors, c.Style.Color)
	}
	want := []string{"#ffaa00", "#ff5555", "#ffaa00", "dark_red"}
	if strings.Join(colors, ",") != strings.Join(want, ",") {
		t.Fatalf("colors = %v, want %v", colors, want)
	}
}

func TestParseMiniClickHoverAndEscape(t *testing.T) {
	txt := ParseMini(`<click:run_command:/tpaccept><hover:show_text:'Accept'>[yes]</hover></click> \<b>`, DefaultTheme)
	if txt.PlainString() != "[yes] <b>" {
		t.Fatalf("plain = %q", txt.PlainString())
	}
	first := txt.Children[0]
	if first.Style.Click == nil || first.Style.Click.Action != plugin.RunCommand || first.Style.Click.Value != "/tpaccept" {
		t.Fatalf("click = %+v", first.Style.Click)
	}
	if first.Style.Hover == nil || first.Style.Hover.Text.PlainString() != "Accept" {
		t.Fatalf("hover = %+v", first.Style.Hover)
	}
}

func TestLegacyToMini(t *testing.T) {
	if got := LegacyToMini("&4Red&lBold &#00ff00Green §ex"); got != "<reset><dark_red>Red<bold>Bold <reset><#00ff00>Green <reset><yellow>x" {
		t.Fatalf("got %q", got)
	}
	if got := LegacyToMini("§x§f§f§0§0§a§aNick"); got != "<reset><#ff00aa>Nick" {
		t.Fatalf("bukkit hex: %q", got)
	}
	if got := sectionToMini("a & b §cred"); got != "a & b <reset><red>red" {
		t.Fatalf("section only: %q", got)
	}
}

func TestMessageFormatQuoting(t *testing.T) {
	cases := map[string]string{
		"{0} has {1}":          "A has B",
		"don''t {0}":           "don't A",
		"'{0}' is literal {1}": "{0} is literal B",
		"{0,number} and {2}":   "A and {2}",
	}
	for in, want := range cases {
		if got := MessageFormat(in, []string{"A", "B"}); got != want {
			t.Errorf("%q = %q, want %q", in, got, want)
		}
	}
}

func TestParsePropertiesEscapes(t *testing.T) {
	p := parseProperties(strings.NewReader("# c\nkey=a\\: b\\u00e9\nmulti=one \\\n  two\nsp ace\n"))
	if p["key"] != "a: bé" || p["multi"] != "one two" || p["sp"] != "ace" {
		t.Fatalf("parsed %q", p)
	}
}

func TestBundledMessagesFormat(t *testing.T) {
	i := newI18n("", "en")
	if got := i.Format("", "backOther", "Bob<x>"); got != "<primary>Returned<secondary> Bob\\<x><primary> to previous location." {
		t.Fatalf("got %q", got)
	}
	de := newI18n("", "de")
	if de.Format("", "pong") == i.Format("", "pong") && de.Format("", "playerNotFound") == i.Format("", "playerNotFound") {
		t.Fatal("German locale did not load")
	}
}

func TestParseDateDiff(t *testing.T) {
	now := time.Now()
	got, err := ParseDateDiff("1d2h30m", true)
	if err != nil {
		t.Fatal(err)
	}
	if d := got.Sub(now) - (26*time.Hour + 30*time.Minute); d < -time.Second || d > time.Second {
		t.Fatalf("off by %v", d)
	}
	if _, err := ParseDateDiff("soon", true); err == nil {
		t.Fatal("want illegalDate")
	}
	if s := formatDateDiff(now, now.Add(26*time.Hour+30*time.Minute+5*time.Second)); s != "1 day 2 hours 30 minutes" {
		t.Fatalf("format = %q", s)
	}
}

func TestFormatAmount(t *testing.T) {
	for in, want := range map[string]string{"1234567.891": "1,234,567.89", "100": "100", "-5.5": "-5.50", "0.001": "0"} {
		if got := FormatAmount(decimal.RequireFromString(in)); got != want {
			t.Errorf("%s = %q, want %q", in, got, want)
		}
	}
}

func TestExperienceCurve(t *testing.T) {
	if ExpAtLevel(30) != 1395 || ExpAtLevel(16) != 352 {
		t.Fatalf("levels: 30=%d 16=%d", ExpAtLevel(30), ExpAtLevel(16))
	}
}
