package chat_test

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"

	_ "github.com/golem-mc/essentials/chat"
	"github.com/golem-mc/essentials/internal/esstest"
	"github.com/golem-mc/golem/plugin"
)

func init() { esstest.RegisterCore() }

// TestChatFormat checks the default and a configured format, and a shout
// with local chat on.
func TestChatFormat(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	b := env.Join("Bob", false)
	env.Drain(a)

	b.Chat("hello &cthere")
	env.Expect(env.Messages(a, 5), "<Bob> hello &cthere")

	env.Ess.Config.Set("chat.format", "&7[{GROUP}|{WORLDNAME}]&r {DISPLAYNAME}: {MESSAGE}")
	b.Chat("hi")
	env.Expect(env.Messages(a, 5), "[default|world] Bob: hi")

	env.Ess.Config.Set("chat.radius", 10)
	a.Chat("!loud")
	env.Expect(env.Messages(b, 5), "[Shout] [default|world] Alice: loud")
	b.Chat("near")
	env.Expect(env.Messages(a, 5), "[L] [default|world] Bob: near")
}

// TestToggleshout toggles shout mode and checks it applies to chat.
func TestToggleshout(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	env.Expect(env.Run(a, "toggleshout"), "Shout mode enabled.")
	env.Ess.Config.Set("chat.radius", 10)
	a.Chat("all")
	env.Expect(env.Messages(a, 5), "[Shout] <Alice> all")
	env.Expect(env.Run(a, "toggleshout off"), "Shout mode disabled.")
}

// perms is a permission provider deciding the nodes it holds.
type perms map[string]bool

func (p perms) Has(_ plugin.CommandSender, node string) (has, decided bool) {
	has, decided = p[node]
	return has, decided
}

// TestPerColourPermission checks that a colour node a permission provider
// sets explicitly adds or removes that colour alone.
func TestPerColourPermission(t *testing.T) {
	env := esstest.Start(t)
	a := env.Join("Alice", true)
	b := env.Join("Bob", false)
	env.Drain(a)
	plugin.Provide[plugin.PermissionProvider](env.Ess.Server, perms{"essentials.chat.red": true}, 0)
	b.Chat("&chi &aho")
	env.Expect(env.Messages(a, 5), "<Bob> hi &aho")

	plugin.Provide[plugin.PermissionProvider](env.Ess.Server, perms{"essentials.chat.color": true, "essentials.chat.red": false}, 1)
	b.Chat("&chi &aho")
	env.Expect(env.Messages(a, 5), "<Bob> &chi ho")
}

// logBuffer is a log sink safe for the server's goroutines.
type logBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// TestChatLogsFormattedLine checks that the console logs the line the chat
// format renders, as Bukkit logs the formatted message.
func TestChatLogsFormattedLine(t *testing.T) {
	logs := &logBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	env := esstest.Start(t)
	b := env.Join("Bob", false)
	env.Ess.Config.Set("chat.format", "&7[{GROUP}|{WORLDNAME}]&r {DISPLAYNAME}: {MESSAGE}")
	b.Chat("logged line")
	env.Drain(b)
	if out := logs.String(); !strings.Contains(out, "[default|world] Bob: logged line") {
		t.Fatalf("the console log lacks the formatted line:\n%s", out)
	}
}
