//go:generate go run -mod=vendor github.com/99designs/gqlgen
package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"

	"github.com/stashapp/stash/internal/api"
	"github.com/stashapp/stash/internal/build"
	"github.com/stashapp/stash/internal/desktop"
	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/ui"

	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	exitCode := 0
	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	defer recoverPanic()

	helpFlag := false
	pflag.BoolVarP(&helpFlag, "help", "h", false, "show this help text and exit")

	versionFlag := false
	pflag.BoolVarP(&versionFlag, "version", "v", false, "show version number and exit")

	pflag.Parse()

	if helpFlag {
		pflag.Usage()
		return
	}

	if versionFlag {
		fmt.Printf(build.VersionString() + "\n")
		return
	}

	mgr, err := manager.Initialize()
	if err != nil {
		displayError(fmt.Errorf("initialization error: %w", err))
		exitCode = 1
		return
	}
	defer mgr.Shutdown()

	server, err := api.Initialize()
	if err != nil {
		displayError(fmt.Errorf("initialization error: %w", err))
		exitCode = 1
		return
	}
	defer server.Close()

	exit := make(chan int)

	go func() {
		err := server.Start()
		if !errors.Is(err, http.ErrServerClosed) {
			displayError(fmt.Errorf("http server error: %w", err))
			exit <- 1
		}
	}()

	go handleSignals(exit)
	desktop.Start(exit, &ui.FaviconProvider)

	exitCode = <-exit
}

func recoverPanic() {
	if desktop.IsDesktop() {
		if p := recover(); p != nil {
			desktop.FatalError(fmt.Errorf("Panic: %v", p))
			os.Exit(1)
		}
	}
}

func displayError(err error) {
	if desktop.IsDesktop() {
		desktop.FatalError(err)
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
}

func handleSignals(exit chan<- int) {
	// handle signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	<-signals
	exit <- 0
}
