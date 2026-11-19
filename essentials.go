// Package essentials is EssentialsX ported to golem: the core plugin,
// registered as "essentials". The modules (spawn, chat, protect, antibuild)
// are separate plugins in their own packages that depend on it.
package essentials

import (
	"github.com/golem-mc/essentials/commands/admin"
	"github.com/golem-mc/essentials/commands/economy"
	"github.com/golem-mc/essentials/commands/items"
	"github.com/golem-mc/essentials/commands/messaging"
	"github.com/golem-mc/essentials/commands/player"
	"github.com/golem-mc/essentials/commands/teleport"
	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// Essentials is the core plugin.
type Essentials struct{ ess *core.Essentials }

func init() { plugin.Register("essentials", &Essentials{}) }

// groups are the command groups, each a port of a set of Essentials'
// Command* classes and the listeners that go with them.
var groups = []func(*core.Essentials){
	admin.Register,
	economy.Register,
	items.Register,
	messaging.Register,
	player.Register,
	teleport.Register,
}

// Enable loads the data and registers every command group.
func (p *Essentials) Enable(s plugin.Server) error {
	e, err := core.Enable(s)
	if err != nil {
		return err
	}
	p.ess = e
	for _, g := range groups {
		g(e)
	}
	return nil
}

// Disable saves every user.
func (p *Essentials) Disable() {
	if p.ess != nil {
		p.ess.Disable()
	}
}
