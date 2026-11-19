package core

import (
	"time"

	"github.com/shopspring/decimal"
)

// startTimer runs Essentials' once-a-second checks (auto-AFK, AFK kick,
// mute and jail expiry) and saves changed users every five seconds.
// Essentials: EssentialsTimer
func (e *Essentials) startTimer() {
	e.Server.Scheduler().RunTimer(20, 20, func() {
		now := time.Now()
		for _, u := range e.OnlineUsers() {
			e.checkActivity(u, now)
			u.IsMuted()
			e.Jails.checkTimeout(u)
		}
	})
	e.Server.Scheduler().RunTimer(100, 100, func() { e.Users.SaveAll(false) })
}

// checkActivity sets a user AFK after auto-afk seconds without activity and
// kicks them (or runs afk-timeout-commands) after auto-afk-timeout.
// Essentials: User.checkActivity
func (e *Essentials) checkActivity(u *User, now time.Time) {
	p, ok := u.Player()
	if !ok {
		return
	}
	if u.LastActivity.IsZero() {
		u.LastActivity = now
	}
	idle := now.Sub(u.LastActivity)
	if kick := e.Settings.AutoAfkKick(); kick > 0 && idle > kick && !u.HasPermission("essentials.afk.kickexempt") {
		u.LastActivity = now
		cmds := e.Config.StringList("afk-timeout-commands")
		if len(cmds) == 0 {
			reason := e.TlString("autoAfkKickReason", FormatDouble(kick.Minutes()))
			p.Kick(e.Mini(reason))
			return
		}
		for _, c := range cmds {
			_, _ = e.Server.Dispatch(e.Server.Console(), e.ReplaceKeywords(c, u))
		}
		return
	}
	if auto := e.Settings.AutoAfk(); auto > 0 && !u.IsAfk() && idle > auto && u.HasPermission("essentials.afk.auto") {
		u.SetAfk(true, "")
	}
}

func decimalOf(n int) decimal.Decimal { return decimal.NewFromInt(int64(n)) }
