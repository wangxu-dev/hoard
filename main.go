package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/wangxu-dev/hoard/app"
	"github.com/wangxu-dev/hoard/internal/covers"
	"github.com/wangxu-dev/hoard/internal/relay"
	"github.com/wangxu-dev/hoard/internal/source"
	"github.com/wangxu-dev/hoard/internal/store"
)

func main() {
	var logFile *os.File
	defer func() {
		if logFile != nil {
			log.SetOutput(os.Stderr)
			_ = logFile.Close()
		}
	}()
	media, err := relay.New()
	if err != nil {
		log.Fatal(err)
	}
	service := &app.App{Media: media, Engine: source.NewEngine()}
	accounts := app.NewAccounts(service)
	syncer := app.NewSync(service)
	var explorationCache string
	exploration := app.NewExploration(func() (*store.Store, string) { return service.Store, explorationCache })
	mygo.Bind(service, accounts, syncer, exploration, app.NewLibrary(service), app.NewPlayer(service), app.NewUpdates(), app.NewActions(service))
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) { app.CloseAccounts(service) })
	mygo.App.OnQuit(func() {
		app.CloseExploration(exploration)
		if service.Store != nil {
			_ = service.Store.Close()
		}
		log.Print("[hoard:app] stopped")
	})
	mygo.App.SetName("Hoard")
	// The scheme is registered before any window; covers serve once the cache opens.
	if err := mygo.Protocol.HandleFunc(covers.Scheme, func(w http.ResponseWriter, r *http.Request) {
		if service.Covers == nil {
			http.NotFound(w, r)
			return
		}
		service.Covers.ServeHTTP(w, r)
	}); err != nil {
		log.Print("[hoard:cover] scheme registration failed")
	}
	mygo.App.WhenReady(func() {
		if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
			explorationCache = filepath.Join(dir, "analysis", "exploration-v2")
			// Preserve diagnostics across the old and new update processes.
			if err := os.MkdirAll(dir, 0700); err == nil {
				logPath := filepath.Join(dir, "hoard.log")
				if info, statErr := os.Stat(logPath); statErr == nil && info.Size() > 2<<20 {
					if rotateErr := os.Rename(logPath, logPath+".previous"); rotateErr != nil {
						log.Print("[hoard:app] log rotation failed")
					}
				}
				logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
				if err == nil {
					log.SetOutput(io.MultiWriter(logFile, os.Stderr))
					log.Printf("[hoard:app] started version=%s", mygo.App.Version())
				} else {
					log.Print("[hoard:app] log file open failed")
				}
			} else {
				log.Print("[hoard:app] data directory creation failed")
			}
			service.Store, err = store.Open(filepath.Join(dir, "library.db"))
			if err != nil {
				log.Print("[hoard:store] database open failed")
			} else if cache, err := covers.New(filepath.Join(dir, "covers"), service.Store.CoverURLs); err == nil {
				service.Covers = cache
			} else {
				log.Print("[hoard:cover] cache open failed")
			}
		}
		app.NewWindow()
		if service.Store != nil && os.Getenv("HOARD_INTEREST_DEV") != "1" {
			if err := app.RestoreAccounts(service); err != nil {
				log.Print("[hoard:accounts] account restore failed")
			}
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
