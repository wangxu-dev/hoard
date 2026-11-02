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
	mygo.Bind(service, accounts, syncer, app.NewLibrary(service), app.NewPlayer(service), app.NewUpdates(), app.NewActions(service))
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) { app.CloseAccounts(service) })
	mygo.App.OnQuit(func() {
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
			// Keep the current launch's diagnostics available in GUI builds.
			if err := os.MkdirAll(dir, 0700); err == nil {
				logFile, err = os.OpenFile(filepath.Join(dir, "hoard.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
				if err == nil {
					log.SetOutput(io.MultiWriter(logFile, os.Stderr))
					log.Print("[hoard:app] started")
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
		if service.Store != nil {
			if err := app.RestoreAccounts(service); err != nil {
				log.Print("[hoard:accounts] account restore failed")
			}
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
