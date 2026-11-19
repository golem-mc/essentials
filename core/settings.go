package core

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Settings are typed readers of the config.yml keys more than one command
// group uses. A key only one group reads is read there with Config's
// getters. Essentials: ISettings
type Settings struct{ c *Config }

func (s *Settings) Config() *Config { return s.c }

func (s *Settings) Locale() string {
	l := s.c.String("locale", "")
	if l == "" || l == "<nil>" {
		return "en"
	}
	return l
}
func (s *Settings) PerPlayerLocale() bool { return s.c.Bool("per-player-locale", false) }
func (s *Settings) Debug() bool           { return s.c.Bool("debug", false) }

// Nicknames.
func (s *Settings) NicknamePrefix() string  { return s.c.String("nickname-prefix", "~") }
func (s *Settings) MaxNickLength() int      { return s.c.Int("max-nick-length", 16) }
func (s *Settings) ChangeDisplayName() bool { return s.c.Bool("change-displayname", true) }
func (s *Settings) OpsNameColor() string    { return s.c.String("ops-name-color", "4") }
func (s *Settings) RealNamesOnList() bool   { return s.c.Bool("real-names-on-list", false) }
func (s *Settings) HideDisplayNameInVanish() bool {
	return s.c.Bool("hide-displayname-in-vanish", true)
}

// Teleports.
func (s *Settings) TeleportSafety() bool { return s.c.Bool("teleport-safety", true) }
func (s *Settings) ForceDisableTeleportSafety() bool {
	return s.c.Bool("force-disable-teleport-safety", false)
}
func (s *Settings) AlwaysTeleportSafety() bool {
	return s.c.Bool("force-safe-teleport-location", false)
}
func (s *Settings) ConsiderWorldHeight() bool {
	return s.c.Bool("consider-world-height-for-teleport-safety", true)
}
func (s *Settings) WaterSafe() bool                  { return s.c.Bool("is-water-safe", false) }
func (s *Settings) TeleportCooldown() time.Duration  { return s.c.Seconds("teleport-cooldown", 0) }
func (s *Settings) TeleportDelay() time.Duration     { return s.c.Seconds("teleport-delay", 0) }
func (s *Settings) TeleportInvulnerability() float64 { return s.c.Float("teleport-invulnerability", 4) }
func (s *Settings) TeleportToCenter() bool           { return s.c.Bool("teleport-to-center", true) }
func (s *Settings) RegisterBackInListener() bool     { return s.c.Bool("register-back-in-listener", false) }
func (s *Settings) WorldTeleportPermissions() bool {
	return s.c.Bool("world-teleport-permissions", false)
}
func (s *Settings) TpaAcceptCancellation() time.Duration {
	return s.c.Seconds("tpa-accept-cancellation", 120)
}
func (s *Settings) TpaMaxRequests() int { return s.c.Int("tpa-max-requests", 5) }

// AFK.
func (s *Settings) AutoAfk() time.Duration     { return s.c.Seconds("auto-afk", 300) }
func (s *Settings) AutoAfkKick() time.Duration { return s.c.Seconds("auto-afk-timeout", -1) }
func (s *Settings) FreezeAfkPlayers() bool     { return s.c.Bool("freeze-afk-players", false) }
func (s *Settings) CancelAfkOnMove() bool      { return s.c.Bool("cancel-afk-on-move", true) }
func (s *Settings) CancelAfkOnInteract() bool  { return s.c.Bool("cancel-afk-on-interact", true) }
func (s *Settings) CancelAfkOnChat() bool      { return s.c.Bool("cancel-afk-on-chat", true) }
func (s *Settings) BroadcastAfkMessage() bool  { return s.c.Bool("broadcast-afk-message", true) }
func (s *Settings) AfkListName() string        { return s.c.String("afk-list-name", "none") }
func (s *Settings) DisableItemPickupAfk() bool {
	return s.c.Bool("disable-item-pickup-while-afk", false)
}
func (s *Settings) SleepIgnoresAfkPlayers() bool { return s.c.Bool("sleep-ignores-afk-players", true) }
func (s *Settings) RemoveGodOnDisconnect() bool  { return s.c.Bool("remove-god-on-disconnect", false) }
func (s *Settings) LastMessageReplyRecipient() bool {
	return s.c.Bool("last-message-reply-recipient", true)
}

// Commands.
func (s *Settings) DisabledCommands() []string { return s.c.StringList("disabled-commands") }
func (s *Settings) VerboseCommandUsages() bool { return s.c.Bool("verbose-command-usages", true) }
func (s *Settings) SocialSpyCommands() []string {
	return s.c.StringList("socialspy-commands")
}
func (s *Settings) MuteCommands() []string { return s.c.StringList("mute-commands") }

// CommandCost answers what running a command costs, from command-costs, or
// zero.
func (s *Settings) CommandCost(name string) decimal.Decimal {
	v := s.c.Float("command-costs."+strings.ToLower(name), 0)
	return decimal.NewFromFloat(v)
}

// Mutes and bans.
func (s *Settings) MaxMuteTime() time.Duration    { return s.c.Seconds("max-mute-time", -1) }
func (s *Settings) MaxTempbanTime() time.Duration { return s.c.Seconds("max-tempban-time", -1) }

// Homes.
func (s *Settings) SpawnIfNoHome() bool        { return s.c.Bool("spawn-if-no-home", true) }
func (s *Settings) WorldHomePermissions() bool { return s.c.Bool("world-home-permissions", false) }
func (s *Settings) ConfirmHomeOverwrite() bool { return s.c.Bool("confirm-home-overwrite", false) }

// Items.
func (s *Settings) DefaultStackSize() int   { return s.c.Int("default-stack-size", -1) }
func (s *Settings) OversizedStackSize() int { return s.c.Int("oversized-stacksize", 64) }
func (s *Settings) PermissionBasedItemSpawn() bool {
	return s.c.Bool("permission-based-item-spawn", false)
}
func (s *Settings) ItemSpawnBlacklist() []string {
	return splitList(s.c.String("item-spawn-blacklist", ""))
}
func (s *Settings) DropItemsIfFull() bool    { return s.c.Bool("drop-items-if-full", false) }
func (s *Settings) UnsafeEnchantments() bool { return s.c.Bool("unsafe-enchantments", false) }

// Economy.
func (s *Settings) CurrencySymbol() string { return s.c.String("currency-symbol", "$") }
func (s *Settings) CurrencySymbolSuffix() bool {
	return s.c.Bool("currency-symbol-suffix", false)
}
func (s *Settings) StartingBalance() decimal.Decimal {
	return decimal.NewFromFloat(s.c.Float("starting-balance", 0))
}
func (s *Settings) MaxMoney() decimal.Decimal {
	return decimal.NewFromFloat(s.c.Float("max-money", 10000000000000))
}
func (s *Settings) MinMoney() decimal.Decimal {
	return decimal.NewFromFloat(s.c.Float("min-money", -10000))
}
func (s *Settings) MinimumPayAmount() decimal.Decimal {
	return decimal.NewFromFloat(s.c.Float("minimum-pay-amount", 0.001))
}
func (s *Settings) EcoLogEnabled() bool { return s.c.Bool("economy-log-enabled", false) }

// Misc.
func (s *Settings) NearRadius() int             { return s.c.Int("near-radius", 200) }
func (s *Settings) HealCooldown() time.Duration { return s.c.Seconds("heal-cooldown", 60) }
func (s *Settings) MaxWalkSpeed() float64       { return s.c.Float("max-walk-speed", 0.8) }
func (s *Settings) MaxFlySpeed() float64        { return s.c.Float("max-fly-speed", 0.8) }
func (s *Settings) NoGodWorlds() []string       { return s.c.StringList("no-god-in-worlds") }

// splitList splits a comma separated config value.
func splitList(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
