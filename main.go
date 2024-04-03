package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/fiatjaf/eventstore"
	"github.com/fiatjaf/eventstore/badger"
	"github.com/fiatjaf/eventstore/lmdb"
	"github.com/fiatjaf/eventstore/sqlite3"
	"github.com/fiatjaf/khatru"
	"github.com/hoisie/mustache"
	"github.com/kelseyhightower/envconfig"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip11"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/urfave/cli/v2"
)

type Settings struct {
	Host             string `envconfig:"HOST" default:""`
	Port             string `envconfig:"PORT" default:"5577"`
	ServiceURL       string `envconfig:"SERVICE_URL"`
	RelayName        string `envconfig:"RELAY_NAME" default:"jinglebells"`
	RelayPubkey      string `envconfig:"RELAY_PUBKEY" default:"79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"`
	RelayDescription string `envconfig:"RELAY_DESCRIPTION" default:"an experimental relay"`
	DatabaseBackend  string `envconfig:"DATABASE" default:"sqlite"`
	DatabaseURL      string `envconfig:"DATABASE_URL"`
	CustomDirectory  string `envconfig:"DATA_DIRECTORY" default:"stuff"`
	DataDirectory    string `envconfig:"SCRIPTS_DIRECTORY" default:"data"`
}

var (
	s       Settings
	db      eventstore.Store
	log     = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	relay   = khatru.NewRelay()
	wrapper nostr.RelayStore
)

const (
	CATEGORY_COMMON   = "common things you should set\n   ============================"
	CATEGORY_UNCOMMON = "complex advanced stuff\n   ======================"
	CATEGORY_NETWORK  = "server network settings\n   ======================="
)

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig")
		return
	}

	app := &cli.App{
		Name:  "jingle",
		Usage: "a customizeable personal relay",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "host",
				Usage:       "address in which to listen for the server",
				Value:       s.Host,
				Destination: &s.Host,
				Category:    CATEGORY_NETWORK,
			},
			&cli.StringFlag{
				Name:        "port",
				Aliases:     []string{"p"},
				Usage:       "port in which to listen for the server",
				Value:       s.Port,
				Destination: &s.Port,
				Category:    CATEGORY_NETWORK,
			},
			&cli.StringFlag{
				Name:        "service-url",
				Usage:       "base url of the relay, with http(s):// prefix",
				Destination: &s.ServiceURL,
				Category:    CATEGORY_NETWORK,
			},
			&cli.StringFlag{
				Name:        "name",
				Usage:       "relay name",
				Value:       s.RelayName,
				Destination: &s.RelayName,
				Category:    CATEGORY_COMMON,
			},
			&cli.StringFlag{
				Name:        "description",
				Usage:       "relay description",
				Value:       s.RelayDescription,
				Destination: &s.RelayDescription,
				Category:    CATEGORY_COMMON,
			},
			&cli.StringFlag{
				Name:        "pubkey",
				Usage:       "relay owner pubkey",
				Value:       s.RelayPubkey,
				Destination: &s.RelayPubkey,
				Category:    CATEGORY_COMMON,
			},
			&cli.StringFlag{
				Name:        "db",
				Usage:       "what database to use as a backend ('sqlite', 'lmdb' or 'badger')",
				Value:       s.DatabaseBackend,
				Destination: &s.DatabaseBackend,
				Category:    CATEGORY_COMMON,
			},
			&cli.StringFlag{
				Name:        "database-uri",
				Usage:       "path or custom URI that will be given to the database driver, prefixed with --datadir",
				DefaultText: "the name of the database driver",
				Destination: &s.DatabaseURL,
				Category:    CATEGORY_UNCOMMON,
			},
			&cli.StringFlag{
				Name:        "datadir",
				Usage:       "base directory for putting databases in",
				Value:       s.DataDirectory,
				Destination: &s.DataDirectory,
				Category:    CATEGORY_UNCOMMON,
			},
			&cli.StringFlag{
				Name:        "scriptsdir",
				Usage:       "base directory for putting scripts in",
				Value:       s.CustomDirectory,
				Destination: &s.CustomDirectory,
				Category:    CATEGORY_UNCOMMON,
			},
		},
		ArgsUsage: "",
		Action: func(c *cli.Context) error {
			// ensure this directory exists
			os.MkdirAll(s.CustomDirectory, 0700)

			// check if scripts exist
			log.Info().Msgf("checking for scripts under ./%s/", s.CustomDirectory)
			for _, scriptName := range []scriptPath{
				REJECT_EVENT,
				REJECT_FILTER,
			} {
				scriptPath := filepath.Join(s.CustomDirectory, string(scriptName))
				if _, err := os.Stat(scriptPath); err != nil {
					if os.IsNotExist(err) {
						// if they don't exist, create them
						err := os.WriteFile(scriptPath, []byte(defaultScripts[scriptName]+"\n"), 0644)
						if err != nil {
							return fmt.Errorf("failed to write %s: %w", scriptName, err)
						}
					} else {
						return fmt.Errorf("missing file %s: %w", scriptName, err)
					}
				}
			}

			// relay metadata
			relay.Info.Name = s.RelayName
			relay.Info.PubKey = s.RelayPubkey
			relay.Info.Description = s.RelayDescription
			relay.OverwriteRelayInformation = append(relay.OverwriteRelayInformation,
				func(ctx context.Context, r *http.Request, info nip11.RelayInformationDocument) nip11.RelayInformationDocument {
					info.Icon = getIconURL(r)
					return info
				},
			)

			// basic relay methods with custom stores
			if err := os.MkdirAll(s.DataDirectory, 0700); err != nil {
				return fmt.Errorf("failed to create datadir '%s': %w", s.DataDirectory, err)
			}
			var dbpath string
			switch s.DatabaseBackend {
			case "sqlite", "sqlite3":
				uri := s.DatabaseURL
				if uri == "" {
					uri = "sqlite"
				}
				dbpath = filepath.Join(s.DataDirectory, uri)
				db = &sqlite3.SQLite3Backend{DatabaseURL: dbpath}
			case "lmdb":
				uri := s.DatabaseURL
				if uri == "" {
					uri = "lmdb"
				}
				dbpath = filepath.Join(s.DataDirectory, uri)
				db = &lmdb.LMDBBackend{Path: dbpath}
			case "badger":
				uri := s.DatabaseURL
				if uri == "" {
					uri = "badger"
				}
				dbpath = filepath.Join(s.DataDirectory, uri)
				db = &badger.BadgerBackend{Path: dbpath}
			default:
				return fmt.Errorf("unknown option '%s' for database", s.DatabaseBackend)
			}
			if err := db.Init(); err != nil {
				return fmt.Errorf("failed to initialize database: %w", err)
			}
			wrapper = eventstore.RelayWrapper{Store: db}
			defer db.Close()
			log.Info().Msgf("storing data with %s under ./%s", s.DatabaseBackend, dbpath)

			relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
			relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
			relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)

			// custom policies
			relay.RejectEvent = append(relay.RejectEvent,
				rejectEvent,
			)
			relay.RejectFilter = append(relay.RejectFilter,
				rejectFilter,
			)
			relay.OnDisconnect = append(relay.OnDisconnect,
				onDisconnect,
			)

			// other http handlers
			log.Info().Msgf("checking for html and assets under ./%s/", s.CustomDirectory)
			homePath := filepath.Join(s.CustomDirectory, "index.html")
			if _, err := os.Stat(homePath); err != nil {
				if os.IsNotExist(err) {
					os.WriteFile(homePath, []byte(`
<!doctype html>
<p>this is the <b>{{Name}}</b> nostr relay</p>
<img width="200" src="{{Icon}}">
<p>controlled by: <code>{{PubKey}}</code></p>
description:
<blockquote>{{Description}}</blockquote>
`), 0644)
				}
			}

			mux := relay.Router()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path[1:]
				if path == string(REJECT_EVENT) || path == string(REJECT_FILTER) {
					w.WriteHeader(403)
					return
				}

				if path == "" {
					path = "index.html"
				}
				filePath := filepath.Join(s.CustomDirectory, path)

				if filepath.Ext(filePath) == ".html" {
					w.Header().Set("content-type", "text/html")
					relay.Info.Icon = getIconURL(r)
					fmt.Fprint(w, mustache.RenderFile(filePath, relay.Info))
				} else {
					http.ServeFile(w, r, filePath)
				}
			})

			// start the server
			localhost := s.Host
			if localhost == "" {
				localhost = "0.0.0.0"
			}
			log.Info().Msg("running on http://" + localhost + ":" + s.Port)
			server := &http.Server{Addr: ":" + s.Port, Handler: relay}
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			g, ctx := errgroup.WithContext(ctx)
			g.Go(server.ListenAndServe)
			g.Go(func() error {
				<-ctx.Done()
				return server.Shutdown(context.Background())
			})
			if err := g.Wait(); err != nil {
				log.Debug().Err(err).Msg("exit reason")
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
