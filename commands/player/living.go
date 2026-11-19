package player

import (
	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// damage hurts p by amount as damageType would. Bukkit: Damageable.damage
func damage(e *core.Essentials, p plugin.Player, amount float32, damageType string) {
	if t, ok := e.Server.Registries().DamageType(plugin.MinecraftKey(damageType)); ok {
		p.Damage(amount, plugin.DamageSource{Type: t})
	}
}
