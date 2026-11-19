package items

import (
	"math"
	"strconv"
	"strings"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
)

func registerEditsign(e *core.Essentials) {
	e.RegisterCommand(core.Cmd{Name: "editsign", PlayerOnly: true, Run: runEditsign, Complete: func(ctx *core.Ctx) []string {
		switch len(ctx.Args) {
		case 1:
			return []string{"set", "clear", "copy", "paste"}
		case 2:
			return []string{"1", "2", "3", "4"}
		case 3:
			n, err := strconv.Atoi(ctx.Args[1])
			if !strings.EqualFold(ctx.Args[0], "set") || err != nil || n < 1 || n > 4 {
				return nil
			}
			if _, sign, front, ok := targetSign(ctx.Player); ok {
				return []string{unformatText(ctx.Ess, ctx.User, "essentials.editsign", sign.Lines(front)[n-1])}
			}
		}
		return nil
	}})
}

// targetSign answers the sign the player looks at and the side facing them.
// Essentials: Commandeditsign.wrapSign
func targetSign(p plugin.Player) (plugin.Block, plugin.Sign, bool, bool) {
	b, ok := p.TargetBlock(5)
	if !ok {
		return b, nil, false, false
	}
	be, ok := b.World.BlockEntity(b.Pos)
	if !ok {
		return b, nil, false, false
	}
	sign, ok := be.(plugin.Sign)
	if !ok {
		return b, nil, false, false
	}
	st := b.State()
	var dir [2]float64 // x, z of the front's normal
	if v, ok := st.Prop(registry.PropFacing); ok {
		dir = map[string][2]float64{"north": {0, -1}, "south": {0, 1}, "west": {-1, 0}, "east": {1, 0}}[v]
	} else if rot, ok := registry.PropNamed("rotation"); ok {
		if v, ok := st.Prop(rot); ok {
			n, _ := strconv.Atoi(v)
			a := float64(n) * math.Pi / 8
			dir = [2]float64{-math.Sin(a), math.Cos(a)}
		}
	}
	loc := p.Location()
	c := b.Pos.Center()
	ex, ez := loc.Pos.X-c.X, loc.Pos.Z-c.Z
	return b, sign, ex*dir[0]+ez*dir[1] > 0, true
}

// Essentials: Commandeditsign
func runEditsign(ctx *core.Ctx) error {
	e := ctx.Ess
	args := ctx.Args
	if len(args) == 0 || (len(args) > 1 && !core.IsInt(args[1])) {
		return core.ErrNotEnoughArgs
	}
	block, sign, front, ok := targetSign(ctx.Player)
	if !ok {
		return core.Tlf("editsignCommandTarget")
	}
	lineArg := func() (int, error) {
		n, _ := strconv.Atoi(args[1])
		if n < 1 || n > 4 {
			return 0, core.Tlf("editsignCommandNoLine")
		}
		return n - 1, nil
	}
	lines := sign.Lines(front)
	copied, _ := core.SessionValue[[4]string](ctx.User, "items.signCopy")
	switch strings.ToLower(args[0]) {
	case "set":
		if len(args) <= 2 {
			return core.ErrNotEnoughArgs
		}
		line, err := lineArg()
		if err != nil {
			return err
		}
		text := strings.TrimSpace(formatString(e, ctx.User, "essentials.editsign", ctx.FinalArg(2)))
		if len([]rune(stripColor(text))) > 15 && !ctx.Has("essentials.editsign.unlimited") {
			return core.Tlf("editsignCommandLimit")
		}
		lines[line] = legacyText(e, text)
		if !writeSign(ctx, block, sign, front, lines) {
			return nil
		}
		ctx.Send("editsignCommandSetSuccess", line+1, text)
	case "clear":
		if len(args) == 1 {
			lines = [4]plugin.Text{}
			if !writeSign(ctx, block, sign, front, lines) {
				return nil
			}
			ctx.Send("editsignCommandClear")
			return nil
		}
		line, err := lineArg()
		if err != nil {
			return err
		}
		lines[line] = plugin.Text{}
		if !writeSign(ctx, block, sign, front, lines) {
			return nil
		}
		ctx.Send("editsignCommandClearLine", line+1)
	case "copy":
		if len(args) == 1 {
			for i := range 4 {
				copied[i] = unformatText(e, ctx.User, "essentials.editsign", lines[i])
			}
			ctx.User.Session["items.signCopy"] = copied
			ctx.Send("editsignCopy", ctx.Label)
			return nil
		}
		line, err := lineArg()
		if err != nil {
			return err
		}
		copied[line] = unformatText(e, ctx.User, "essentials.editsign", lines[line])
		ctx.User.Session["items.signCopy"] = copied
		ctx.Send("editsignCopyLine", line+1, ctx.Label)
	case "paste":
		if len(args) == 1 {
			for i := range 4 {
				lines[i] = legacyText(e, formatString(e, ctx.User, "essentials.editsign", copied[i]))
			}
			if !writeSign(ctx, block, sign, front, lines) {
				return nil
			}
			ctx.Send("editsignPaste", ctx.Label)
			return nil
		}
		line, err := lineArg()
		if err != nil {
			return err
		}
		lines[line] = legacyText(e, formatString(e, ctx.User, "essentials.editsign", copied[line]))
		if !writeSign(ctx, block, sign, front, lines) {
			return nil
		}
		ctx.Send("editsignPasteLine", line+1, ctx.Label)
	default:
		return core.ErrNotEnoughArgs
	}
	return nil
}

// writeSign fires SignChangeEvent for the new lines, so protection plugins
// can veto them, and writes what the listeners leave unless the sign is
// waxed; false when it was not written.
// Essentials: Commandeditsign.callSignEvent
func writeSign(ctx *core.Ctx, b plugin.Block, sign plugin.Sign, front bool, lines [4]plugin.Text) bool {
	if sign.Waxed() && !ctx.Has("essentials.editsign.waxed.exempt") {
		return false
	}
	ev := &plugin.SignChangeEvent{Player: ctx.Player, Block: b, Front: front, Lines: lines}
	ctx.Ess.Server.Events().Fire(ev)
	if ev.Cancelled() {
		if ctx.Ess.Settings.Debug() {
			ctx.Ess.Log.Info("SignChangeEvent canceled for /editsign execution by " + ctx.Player.Name())
		}
		return false
	}
	sign.SetLines(front, ev.Lines)
	return true
}
