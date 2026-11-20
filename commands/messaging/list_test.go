package messaging

import (
	"strings"
	"testing"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/essentials/internal/esstest"
)

// TestListHeadingColour checks that /list's default group heading is a
// coloured heading, not the text of its tags.
func TestListHeadingColour(t *testing.T) {
	env := esstest.Start(t)
	g := &group{e: env.Ess}
	line := g.outputTag("", core.Raw(env.Ess.I18n.Format("", "connectedPlayers")), "Bob")
	txt := core.ParseMini(line, env.Ess.Theme())
	plain := txt.PlainString()
	if strings.Contains(plain, "<primary>") || strings.Contains(plain, "<reset>") {
		t.Fatalf("heading shows tags: %q", plain)
	}
	if !strings.HasPrefix(plain, "Connected players: ") {
		t.Fatalf("heading = %q", plain)
	}
	if len(txt.Children) == 0 || txt.Children[0].Style.Color != core.DefaultTheme.Primary {
		t.Fatalf("heading colour = %+v, want %s", txt.Children, core.DefaultTheme.Primary)
	}
}
