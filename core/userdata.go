package core

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/shopspring/decimal"
)

// LazyLocation is a saved location: a world by key, resolved when used, so a
// world that is not loaded does not lose the location. Essentials keeps the
// world's UUID in world and its name in world-name; here both hold the
// dimension key. Essentials: LazyLocation
type LazyLocation struct {
	World     string  `yaml:"world"`
	WorldName string  `yaml:"world-name"`
	X         float64 `yaml:"x"`
	Y         float64 `yaml:"y"`
	Z         float64 `yaml:"z"`
	Yaw       float32 `yaml:"yaw"`
	Pitch     float32 `yaml:"pitch"`
}

// LazyOf saves loc.
func LazyOf(loc plugin.Location) LazyLocation {
	w := ""
	if loc.World != nil {
		w = loc.World.Key().String()
	}
	return LazyLocation{World: w, WorldName: w, X: loc.Pos.X, Y: loc.Pos.Y, Z: loc.Pos.Z, Yaw: loc.Yaw, Pitch: loc.Pitch}
}

// Resolve answers the location in its loaded world; false when the world is
// not loaded.
func (l LazyLocation) Resolve(s plugin.Server) (plugin.Location, bool) {
	for _, name := range []string{l.World, l.WorldName} {
		if name == "" {
			continue
		}
		if w, ok := WorldByName(s, name); ok {
			return plugin.Location{World: w, Pos: plugin.Vec3{X: l.X, Y: l.Y, Z: l.Z}, Yaw: l.Yaw, Pitch: l.Pitch}, true
		}
	}
	return plugin.Location{}, false
}

// MailMessage is a mail. Essentials: MailMessage
type MailMessage struct {
	Read       bool   `yaml:"read"`
	Legacy     bool   `yaml:"legacy"`
	SenderName string `yaml:"sender-name,omitempty"`
	SenderUUID string `yaml:"sender-uuid,omitempty"`
	// Timestamp and Expire are Unix milliseconds; Expire 0 never expires.
	Timestamp int64  `yaml:"timestamp,omitempty"`
	Expire    int64  `yaml:"expire,omitempty"`
	Message   string `yaml:"message"`
}

// CommandCooldown is a command-cooldowns entry: a pattern the command line
// matches and when the cooldown ends, Unix milliseconds.
type CommandCooldown struct {
	Pattern string `yaml:"pattern"`
	Value   int64  `yaml:"value"`
}

// Timestamps are a user's times, Unix milliseconds.
type Timestamps struct {
	LastTeleport     int64             `yaml:"lastteleport"`
	LastHeal         int64             `yaml:"lastheal"`
	Mute             int64             `yaml:"mute"`
	Jail             int64             `yaml:"jail"`
	OnlineJail       int64             `yaml:"onlinejail"`
	Logout           int64             `yaml:"logout"`
	Login            int64             `yaml:"login"`
	Kits             map[string]int64  `yaml:"kits,omitempty"`
	CommandCooldowns []CommandCooldown `yaml:"command-cooldowns,omitempty"`
}

// UserData is what is saved of a user, in userdata/<uuid>.yml, with the
// keys Essentials' UserConfigHolder writes, so existing files load.
type UserData struct {
	Money                     *decimal.Decimal        `yaml:"money,omitempty"`
	Homes                     map[string]LazyLocation `yaml:"homes,omitempty"`
	Nickname                  string                  `yaml:"nickname,omitempty"`
	Unlimited                 []string                `yaml:"unlimited,omitempty"`
	Powertools                map[string][]string     `yaml:"powertools,omitempty"`
	LastLocation              *LazyLocation           `yaml:"lastlocation,omitempty"`
	LogoutLocation            *LazyLocation           `yaml:"logoutlocation,omitempty"`
	Jail                      string                  `yaml:"jail,omitempty"`
	Mail                      []MailMessage           `yaml:"mail,omitempty"`
	TeleportEnabled           bool                    `yaml:"teleportenabled"`
	TeleportAuto              bool                    `yaml:"teleportauto"`
	Ignore                    []string                `yaml:"ignore,omitempty"`
	GodMode                   bool                    `yaml:"godmode"`
	FlyMode                   bool                    `yaml:"flymode"`
	Muted                     bool                    `yaml:"muted"`
	MuteReason                string                  `yaml:"mute-reason,omitempty"`
	Jailed                    bool                    `yaml:"jailed"`
	IPAddress                 string                  `yaml:"ip-address"`
	Afk                       bool                    `yaml:"afk"`
	Geolocation               string                  `yaml:"geolocation,omitempty"`
	SocialSpy                 bool                    `yaml:"socialspy"`
	NPC                       bool                    `yaml:"npc"`
	LastAccountName           string                  `yaml:"last-account-name,omitempty"`
	NPCName                   string                  `yaml:"npc-name,omitempty"`
	PowertoolsEnabled         bool                    `yaml:"powertoolsenabled"`
	AcceptingPay              bool                    `yaml:"accepting-pay"`
	ConfirmPay                *bool                   `yaml:"confirm-pay,omitempty"`
	ConfirmClear              *bool                   `yaml:"confirm-clear,omitempty"`
	LastMessageReplyRecipient *bool                   `yaml:"last-message-reply-recipient,omitempty"`
	BaltopExempt              bool                    `yaml:"baltop-exempt"`
	Shouting                  *bool                   `yaml:"shouting,omitempty"`
	PastUsernames             []string                `yaml:"past-usernames,omitempty"`
	Timestamps                Timestamps              `yaml:"timestamps"`
	// Extra holds the modules' own keys, saved under extra:, so a module can
	// keep data without changing this struct.
	Extra map[string]any `yaml:"extra,omitempty"`
}

// newUserData answers the defaults a user starts with.
func newUserData() *UserData {
	return &UserData{TeleportEnabled: true, PowertoolsEnabled: true, AcceptingPay: true}
}
