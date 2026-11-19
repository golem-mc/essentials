package chat

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/golem-mc/essentials/core"
)

// chatRadius reads chat.radius (or the old chat-radius).
// Essentials: Settings._getChatRadius
func chatRadius(c *core.Config) int {
	if v, ok := c.Get("chat.radius"); ok && v != nil {
		return c.Int("chat.radius", 0)
	}
	return c.Int("chat-radius", 0)
}

// Essentials: Settings._getChatShout
func shoutChar(c *core.Config) byte { return firstChar(c.String("chat.shout", "!"), '!') }

// Essentials: Settings._getChatQuestion
func questionChar(c *core.Config) byte { return firstChar(c.String("chat.question", "?"), '?') }

func firstChar(s string, def byte) byte {
	if s == "" {
		return def
	}
	return s[0]
}

// stringAt answers the string at path, false when it is missing or a
// section, as Configurate's getString does.
func stringAt(c *core.Config, path string) (string, bool) {
	v, ok := c.Get(path)
	if !ok || v == nil {
		return "", false
	}
	switch v.(type) {
	case map[string]any, map[any]any, []any:
		return "", false
	}
	return fmt.Sprint(v), true
}

// chatFormat answers the format for a group and chat type, the first of
// chat.group-formats.<group>.<type>, chat.group-formats.<group>,
// chat.format.<type> and chat.format, with its placeholders turned into
// %1$s (display name), %2$s (message) and {0}..{9}.
// Essentials: Settings.ChatFormatConfigSupplier
func chatFormat(c *core.Config, group string, t chatType) string {
	groupPath := "chat.group-formats." + group
	format, ok := stringAt(c, groupPath+"."+t.key())
	if !ok {
		format, ok = stringAt(c, groupPath)
	}
	if !ok {
		format, ok = stringAt(c, "chat.format."+t.key())
	}
	if !ok {
		format, ok = stringAt(c, "chat.format")
	}
	if !ok {
		format = "&7[{GROUP}]&r {DISPLAYNAME}&7:&r {MESSAGE}"
	}
	return strings.NewReplacer(
		"{DISPLAYNAME}", "%1$s",
		"{MESSAGE}", "%2$s",
		"{GROUP}", "{0}",
		"{WORLD}", "{1}",
		"{WORLDNAME}", "{1}",
		"{SHORTWORLDNAME}", "{2}",
		"{TEAMPREFIX}", "{3}",
		"{TEAMSUFFIX}", "{4}",
		"{TEAMNAME}", "{5}",
		"{PREFIX}", "{6}",
		"{SUFFIX}", "{7}",
		"{USERNAME}", "{8}",
		"{NICKNAME}", "{9}",
	).Replace(format)
}

// worldAlias answers chat.world-aliases' name for world, else world.
// Essentials: Settings.getWorldAlias
func worldAlias(c *core.Config, world string) string {
	for k, v := range c.Map("chat.world-aliases") {
		if strings.EqualFold(k, world) && v != nil {
			return fmt.Sprint(v)
		}
	}
	return world
}

// The legacy codes by permission: essentials.chat.color, .format and
// .magic. Essentials: FormatUtil.COLORS, FORMATS, MAGIC
const (
	colorCodes  = "0123456789abcdef"
	formatCodes = "lmnor"
	magicCodes  = "k"
)

// codeNames are the codes' names for the per-code permissions
// (essentials.chat.red); k is obfuscated, as vanilla names it.
var codeNames = map[byte]string{
	'0': "black", '1': "dark_blue", '2': "dark_green", '3': "dark_aqua", '4': "dark_red", '5': "dark_purple",
	'6': "gold", '7': "gray", '8': "dark_gray", '9': "blue", 'a': "green", 'b': "aqua", 'c': "red",
	'd': "light_purple", 'e': "yellow", 'f': "white", 'k': "obfuscated", 'l': "bold", 'm': "strikethrough",
	'n': "underline", 'o': "italic", 'r': "reset",
}

// supportedCodes answers the legacy codes the user may use.
// Essentials: FormatUtil.getSupported
func supportedCodes(u *core.User, permBase string) map[byte]bool {
	out := map[byte]bool{}
	add := func(codes string) {
		for i := 0; i < len(codes); i++ {
			out[codes[i]] = true
		}
	}
	if u.HasPermission(permBase + ".color") {
		add(colorCodes)
	}
	if u.HasPermission(permBase + ".format") {
		add(formatCodes)
	}
	if u.HasPermission(permBase + ".magic") {
		add(magicCodes)
	}
	if p, ok := u.Player(); ok {
		for code, name := range codeNames {
			node := permBase + "." + name
			if !p.PermissionSet(node) {
				continue
			}
			out[code] = p.HasPermission(node)
		}
	}
	return out
}

var (
	// legacyCode is an & code, possibly escaped by a second &.
	// Essentials: FormatUtil.REPLACE_ALL_PATTERN
	legacyCode = regexp.MustCompile(`(&)?&([0-9a-fk-orA-FK-OR])`)
	// legacyHex is an &#rrggbb colour. Essentials: REPLACE_ALL_RGB_PATTERN
	legacyHex = regexp.MustCompile(`(&)?&#([0-9a-fA-F]{6})`)
	// sectionCode is a § code. Essentials: FormatUtil.STRIP_ALL_PATTERN
	sectionCode = regexp.MustCompile(`§+([0-9a-fk-orA-FK-OR])`)
	// urlPattern is what looks like a link. Essentials:
	// AbstractChatEvent.URL_PATTERN
	urlPattern = regexp.MustCompile(`((?:(?:https?)://)?[\w\-_.]{2,})\.([a-zA-Z]{2,}(?:/\S+)?)`)
	sectionHex = regexp.MustCompile(`§#[0-9a-fA-F]{6}`)
)

// formatMessage turns the & codes the user may use into § codes, strips
// the § codes they may not, and breaks links without permBase.url.
// Essentials: FormatUtil.formatMessage
func formatMessage(u *core.User, permBase, msg string) string {
	supported := supportedCodes(u, permBase)
	rgb := u.HasPermission(permBase + ".rgb")
	msg = sectionCode.ReplaceAllStringFunc(msg, func(m string) string {
		c := strings.ToLower(m[len(m)-1:])[0]
		if supported[c] {
			return m
		}
		return ""
	})
	if len(supported) > 0 || rgb {
		msg = legacyCode.ReplaceAllStringFunc(msg, func(m string) string {
			sub := legacyCode.FindStringSubmatch(m)
			c := strings.ToLower(sub[2])[0]
			if sub[1] == "" && supported[c] {
				return "§" + sub[2]
			}
			return "&" + sub[2]
		})
		if rgb {
			msg = legacyHex.ReplaceAllStringFunc(msg, func(m string) string {
				sub := legacyHex.FindStringSubmatch(m)
				if sub[1] == "" {
					return "§#" + sub[2]
				}
				return "&#" + sub[2]
			})
		}
	}
	if !u.HasPermission(permBase + ".url") {
		for urlPattern.MatchString(msg) {
			next := urlPattern.ReplaceAllString(msg, "$1 $2")
			if next == msg {
				break
			}
			msg = next
		}
	}
	return msg
}

// stripCodes removes § codes. Essentials: ChatColor.stripColor
func stripCodes(s string) string {
	return sectionHex.ReplaceAllString(sectionCode.ReplaceAllString(s, ""), "")
}

// messageMini answers a processed message as MiniMessage: its § codes as
// tags, everything else (an & the user may not use, a <) as typed.
func messageMini(msg string) string {
	const amp = "\x00"
	s := core.EscapeTags(strings.ReplaceAll(msg, "&", amp))
	return strings.ReplaceAll(core.LegacyToMini(s), amp, "&")
}
