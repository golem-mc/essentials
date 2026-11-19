package admin

import (
	"math"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// started is when the process started, near enough: the JVM start time
// Essentials shows as the uptime.
var started = time.Now()

func registerMisc(e *core.Essentials) {
	e.RegisterCommand(
		core.Cmd{Name: "gc", Run: runGC},
		core.Cmd{Name: "ext", Run: runExt, Complete: func(ctx *core.Ctx) []string {
			if len(ctx.Args) == 1 {
				return loopPlayerNames(ctx)
			}
			return nil
		}},
	)
}

// Essentials: Commandext
func runExt(ctx *core.Ctx) error {
	extOthers := func() error {
		return loopOnlinePlayers(ctx, ctx.Args[0], func(u *core.User) {
			if p, ok := u.Player(); ok {
				p.SetFireTicks(0)
			}
			ctx.Send("extinguishOthers", u.DisplayName())
		})
	}
	if ctx.User == nil {
		if len(ctx.Args) == 0 {
			return core.ErrNotEnoughArgs
		}
		return extOthers()
	}
	if len(ctx.Args) > 0 && ctx.Has("essentials.ext.others") {
		return extOthers()
	}
	ctx.Player.SetFireTicks(0)
	ctx.Send("extinguish")
	return nil
}

// Essentials: Commandgc
func runGC(ctx *core.Ctx) error {
	e := ctx.Ess
	tps := e.Server.Ticks().TPS()[0]
	color := "red"
	switch {
	case tps >= 18:
		color = "green"
	case tps >= 15:
		color = "yellow"
	}
	ctx.Send("uptime", core.FormatDateDiff(started))
	ctx.Send("tps", core.Raw("<"+color+">"+core.FormatDouble(tps)))

	// The JVM's max, total and free memory are, for Go, the memory limit
	// (else what the runtime holds), what the runtime holds from the OS,
	// and what of that holds no live objects.
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	const mb = 1024 * 1024
	limit := uint64(debug.SetMemoryLimit(-1))
	if limit == math.MaxInt64 {
		limit = ms.Sys
	}
	free := ms.HeapIdle - ms.HeapReleased + ms.HeapInuse - ms.HeapAlloc
	ctx.Send("gcmax", limit/mb)
	ctx.Send("gctotal", ms.Sys/mb)
	ctx.Send("gcfree", free/mb)

	for _, w := range e.Server.Worlds() {
		kind := "World"
		switch w.Dimension() {
		case plugin.MinecraftKey("the_nether"):
			kind = "Nether"
		case plugin.MinecraftKey("the_end"):
			kind = "The End"
		}
		chunks, tiles := loadedChunks(w)
		entities := 0
		for range w.Entities() {
			entities++
		}
		ctx.Send("gcWorld", kind, core.WorldName(w), chunks, entities, tiles)
	}
	return nil
}

// loadedChunks counts the world's loaded chunks and their block entities.
func loadedChunks(w plugin.World) (chunks, tiles int) {
	for _, c := range w.LoadedChunks() {
		chunks++
		tiles += len(c.BlockEntities())
	}
	return chunks, tiles
}
