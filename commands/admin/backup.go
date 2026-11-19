package admin

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golem-mc/essentials/core"
	"github.com/golem-mc/golem/plugin"
)

// backup runs backup.command every backup.interval minutes while players
// are online (or always, with backup.always-run), with saving turned off
// around it. Its fields are used on the tick goroutine; done is closed by
// the goroutine that runs the command. Essentials: Backup
type backup struct {
	e      *core.Essentials
	task   plugin.Task
	active bool
	// done is closed when the running backup's command ends.
	done     chan struct{}
	mu       sync.Mutex
	shutdown bool
}

func registerBackup(e *core.Essentials) {
	b := &backup{e: e}
	if len(e.Server.Players()) > 0 || b.alwaysRun() {
		b.start()
	}
	plugin.Listen(e.Server, func(*plugin.PlayerJoinEvent) { b.start() }, plugin.ListenOptions{Priority: plugin.Monitor})
	e.OnDisable(func() {
		b.stop()
		b.mu.Lock()
		b.shutdown = true
		b.mu.Unlock()
		if b.active && b.done != nil {
			<-b.done
		}
	})
	e.RegisterCommand(core.Cmd{Name: "backup", Run: func(ctx *core.Ctx) error {
		cmd := b.command()
		if cmd == "" || strings.EqualFold(cmd, "save-all") {
			return core.Tlf("backupDisabled")
		}
		b.run()
		ctx.Send("backupStarted")
		return nil
	}})
}

func (b *backup) command() string { return b.e.Config.String("backup.command", "") }
func (b *backup) alwaysRun() bool { return b.e.Config.Bool("backup.always-run", false) }

// start schedules the backups unless they are scheduled or the interval is
// under a minute.
func (b *backup) start() {
	if b.task != nil {
		return
	}
	interval := b.e.Config.Int("backup.interval", 1440) * 1200
	if interval < 1200 {
		return
	}
	b.task = b.e.Server.Scheduler().RunTimer(interval, interval, b.run)
}

func (b *backup) stop() {
	if b.task != nil {
		b.task.Cancel()
		b.task = nil
	}
}

// run saves the worlds and runs the backup command in the server's
// directory, logging its output.
func (b *backup) run() {
	if b.active {
		return
	}
	e := b.e
	cmd := b.command()
	if cmd == "" {
		return
	}
	console := e.Server.Console()
	if strings.EqualFold(cmd, "save-all") {
		_, _ = e.Server.Dispatch(console, "save-all")
		return
	}
	b.active = true
	b.done = make(chan struct{})
	e.Log.Info(e.TlString("backupStarted"))
	_, _ = e.Server.Dispatch(console, "save-all")
	_, _ = e.Server.Dispatch(console, "save-off")
	dir := filepath.Dir(filepath.Dir(e.Server.DataDir()))
	if abs, err := filepath.Abs(e.Server.DataDir()); err == nil {
		dir = filepath.Dir(filepath.Dir(abs))
	}
	done := b.done
	e.Server.Scheduler().RunAsync(func() {
		defer close(done)
		b.exec(strings.Fields(cmd), dir)
		b.mu.Lock()
		shutdown := b.shutdown
		b.mu.Unlock()
		if shutdown {
			return
		}
		e.Server.Scheduler().RunTask(func() {
			_, _ = e.Server.Dispatch(console, "save-on")
			if !b.alwaysRun() && len(e.Server.Players()) == 0 {
				b.stop()
			}
			b.active = false
			e.Log.Info(e.TlString("backupFinished"))
		})
	})
}

// exec runs the command, logging its output.
func (b *backup) exec(args []string, dir string) {
	log := b.e.Log
	c := exec.Command(args[0], args[1:]...)
	c.Dir = dir
	out, err := c.StdoutPipe()
	if err != nil {
		log.Error("An error occurred while building the backup child process", "err", err)
		return
	}
	c.Stderr = c.Stdout
	if err := c.Start(); err != nil {
		log.Error("An error occurred while building the backup child process", "err", err)
		return
	}
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		log.Info(sc.Text())
	}
	if err := c.Wait(); err != nil {
		log.Error("The backup child process failed", "err", err)
	}
}
