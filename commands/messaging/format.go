package messaging

import (
	"crypto/md5"
	"regexp"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/google/uuid"
)

// The legacy formatting codes by kind. Essentials: FormatUtil.COLORS,
// FORMATS, MAGIC
const (
	colorCodes  = "0123456789abcdef"
	formatCodes = "lmnor"
	magicCodes  = "k"
)

// codeNames are the names of the per-code permissions
// (essentials.chat.red, essentials.chat.obfuscated). Bukkit: ChatColor
var codeNames = map[byte]string{
	'0': "black", '1': "dark_blue", '2': "dark_green", '3': "dark_aqua", '4': "dark_red",
	'5': "dark_purple", '6': "gold", '7': "gray", '8': "dark_gray", '9': "blue", 'a': "green",
	'b': "aqua", 'c': "red", 'd': "light_purple", 'e': "yellow", 'f': "white",
	'k': "obfuscated", 'l': "bold", 'm': "strikethrough", 'n': "underline", 'o': "italic", 'r': "reset",
}

var (
	replaceAllPattern    = regexp.MustCompile(`(&)?&([0-9a-fk-orA-FK-OR])`)
	replaceAllRGBPattern = regexp.MustCompile(`(&)?&#([0-9a-fA-F]{6})`)
	stripAllPattern      = regexp.MustCompile(`§+([0-9a-fk-orA-FK-OR])`)
	stripRGBPattern      = regexp.MustCompile(`§x((?:§[0-9a-fA-F]){6})`)
	stripColorPattern    = regexp.MustCompile(`(?i)§[0-9a-fk-orx]`)
	// urlPattern is what /msg and /mail break up for players without
	// essentials.*.url. Essentials: AbstractChatEvent.URL_PATTERN
	urlPattern = regexp.MustCompile(`((?:(?:https?)://)?[\w\-.]{2,})\.([a-zA-Z]{2,}(?:/\S+)?)`)
	// linkPattern finds the links broadcasts make clickable. Adventure:
	// the default URL replacement
	linkPattern  = regexp.MustCompile(`(?:(https?)://)?([-\w_.]+\.\w{2,})(/\S*)?`)
	invalidChars = regexp.MustCompile("[^\t\n\r -~\u0085 -퟿-￼]")
	ipPattern    = regexp.MustCompile(`^([01]?\d\d?|2[0-4]\d|25[0-5])\.([01]?\d\d?|2[0-4]\d|25[0-5])\.([01]?\d\d?|2[0-4]\d|25[0-5])\.([01]?\d\d?|2[0-4]\d|25[0-5])$`)
)

// replaceSubmatch is regexp's ReplaceAllStringFunc with the submatches.
func replaceSubmatch(re *regexp.Regexp, s string, fn func(m []string) string) string {
	return re.ReplaceAllStringFunc(s, func(match string) string { return fn(re.FindStringSubmatch(match)) })
}

// replaceColor turns the & codes in supported into § codes, and &#rrggbb
// into Bukkit's §x hex form with rgb; &&c stays a literal &c.
// Essentials: FormatUtil.replaceColor
func replaceColor(s, supported string, rgb bool) string {
	s = replaceSubmatch(replaceAllPattern, s, func(m []string) string {
		if m[1] == "" && strings.Contains(supported, strings.ToLower(m[2])) {
			return "§" + m[2]
		}
		return "&" + m[2]
	})
	if !rgb {
		return s
	}
	return replaceSubmatch(replaceAllRGBPattern, s, func(m []string) string {
		if m[1] != "" {
			return "&#" + m[2]
		}
		var b strings.Builder
		b.WriteString("§x")
		for _, c := range m[2] {
			b.WriteString("§" + string(c))
		}
		return b.String()
	})
}

// replaceFormat turns every & code into a § code. Essentials:
// FormatUtil.replaceFormat
func replaceFormat(s string) string { return replaceColor(s, colorCodes+formatCodes+magicCodes, true) }

// stripCodes removes the § codes in strip. Essentials: FormatUtil.stripColor
func stripCodes(s, strip string) string {
	return replaceSubmatch(stripAllPattern, s, func(m []string) string {
		if strings.Contains(strip, strings.ToLower(m[1])) {
			return ""
		}
		return m[0]
	})
}

// stripFormat removes every § code. Bukkit: ChatColor.stripColor
func stripFormat(s string) string { return stripColorPattern.ReplaceAllString(s, "") }

// stripEssentialsFormat removes the & codes. Essentials:
// FormatUtil.stripEssentialsFormat
func stripEssentialsFormat(s string) string {
	return replaceAllRGBPattern.ReplaceAllString(replaceAllPattern.ReplaceAllString(s, ""), "")
}

// unformatString turns § codes back into & codes. Essentials:
// FormatUtil.unformatString
func unformatString(s string) string {
	s = replaceSubmatch(stripRGBPattern, s, func(m []string) string {
		return "&#" + strings.ReplaceAll(m[1], "§", "")
	})
	return replaceSubmatch(stripAllPattern, s, func(m []string) string { return "&" + strings.ToLower(m[1]) })
}

// supportedCodes answers the codes u may use under permBase: .color,
// .format, .magic, then each code's own node where it is set explicitly.
// Essentials: FormatUtil.getSupported
func (g *group) supportedCodes(u *core.User, permBase string) string {
	set := map[byte]bool{}
	add := func(codes string, on bool) {
		for i := 0; i < len(codes); i++ {
			set[codes[i]] = on
		}
	}
	if u.HasPermission(permBase + ".color") {
		add(colorCodes, true)
	}
	if u.HasPermission(permBase + ".format") {
		add(formatCodes, true)
	}
	if u.HasPermission(permBase + ".magic") {
		add(magicCodes, true)
	}
	if p, online := u.Player(); online {
		for code, name := range codeNames {
			if node := permBase + "." + name; p.PermissionSet(node) {
				set[code] = p.HasPermission(node)
			}
		}
	}
	var b strings.Builder
	for code, on := range set {
		if on {
			b.WriteByte(code)
		}
	}
	return b.String()
}

// formatString applies the codes u may use under permBase and strips the
// rest. Essentials: FormatUtil.formatString
func (g *group) formatString(u *core.User, permBase, msg string) string {
	supported := g.supportedCodes(u, permBase)
	var strip strings.Builder
	for _, c := range colorCodes + formatCodes + magicCodes {
		if !strings.ContainsRune(supported, c) {
			strip.WriteRune(c)
		}
	}
	rgb := u.HasPermission(permBase + ".rgb")
	if strip.Len() > 0 {
		msg = stripCodes(msg, strip.String())
	}
	if supported != "" || rgb {
		msg = replaceColor(msg, supported, rgb)
	}
	return msg
}

// formatMessage is formatString that also breaks up links unless u has
// permBase.url. Essentials: FormatUtil.formatMessage
func (g *group) formatMessage(u *core.User, permBase, msg string) string {
	msg = g.formatString(u, permBase, msg)
	if !u.HasPermission(permBase + ".url") {
		msg = blockURL(msg)
	}
	return msg
}

// blockURL puts a space before the top level domain of every link.
// Essentials: FormatUtil.blockURL
func blockURL(s string) string {
	for urlPattern.MatchString(s) {
		s = urlPattern.ReplaceAllString(s, "$1 $2")
	}
	return s
}

// sanitize removes characters that cannot be shown. Essentials:
// StringUtil.sanitizeString
func sanitize(s string) string { return invalidChars.ReplaceAllString(s, "") }

// sectionMini answers s, which may hold § codes, as MiniMessage that shows
// its text as written: tags escaped, & left alone, § codes as colours.
func sectionMini(s string) string {
	m := core.LegacyToMini(strings.ReplaceAll(core.EscapeTags(s), "&", "\x00"))
	return strings.ReplaceAll(m, "\x00", "&")
}

// legacyText answers a § formatted string as text, as Bukkit's
// sendMessage(String) shows it.
func (g *group) legacyText(s string) plugin.Text {
	return core.ParseMini(sectionMini(s), g.e.Theme())
}

// withLinks makes the links in a MiniMessage string clickable. Essentials:
// AdventureFacet.legacyToMiniWithUrls
func withLinks(s string) string {
	return linkPattern.ReplaceAllStringFunc(s, func(link string) string {
		url := link
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "https://" + url
		}
		return "<click:open_url:'" + strings.ReplaceAll(url, "'", "\\'") + "'>" + link + "</click>"
	})
}

// validIP reports whether s is an IPv4 address. Essentials: FormatUtil.validIP
func validIP(s string) bool { return ipPattern.MatchString(s) }

// parseUUID reads a UUID in its dashed form only, as Java's UUID.fromString
// is used for.
func parseUUID(s string) (plugin.UUID, error) {
	if len(s) != 36 {
		return plugin.UUID{}, errNotUUID
	}
	return uuid.Parse(s)
}

var errNotUUID = &core.TlError{Key: "playerNotFound", Plain: true}

// offlineUser answers the user of the player Bukkit's
// getOfflinePlayer(name) gives: a player who has played here, found in
// usercache.json, else the offline-mode UUID of the name, as Bukkit falls
// back to when no profile is found. A new user is saved with the name.
func (g *group) offlineUser(name string) *core.User {
	id := offlineUUID(name)
	if op, ok := g.e.Server.OfflinePlayerNamed(name); ok {
		id, name = op.UUID(), op.Name()
	}
	u := g.e.Users.Get(id)
	if u.Data.LastAccountName == "" {
		u.Data.LastAccountName = name
		g.e.Users.Rename(u.ID, name)
	}
	return u
}

// offlineUUID answers the UUID an offline-mode server gives name, as
// Bukkit's OfflinePlayer for an unknown name has. Java:
// UUID.nameUUIDFromBytes("OfflinePlayer:" + name)
func offlineUUID(name string) plugin.UUID {
	h := md5.Sum([]byte("OfflinePlayer:" + name))
	h[6] = h[6]&0x0f | 0x30
	h[8] = h[8]&0x3f | 0x80
	return plugin.UUID(h)
}
