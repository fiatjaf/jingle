package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/fiatjaf/eventstore"
	"github.com/fiatjaf/eventstore/badger"
	"github.com/fiatjaf/eventstore/lmdb"
	"github.com/fiatjaf/eventstore/sqlite3"
	"github.com/fiatjaf/khatru"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"

	"github.com/urfave/cli/v2"
)

type Settings struct {
	Host             string `envconfig:"HOST" default:""`
	Port             string `envconfig:"PORT" default:"5577"`
	Domain           string `envconfig:"DOMAIN"`
	RelayName        string `envconfig:"RELAY_NAME" default:"my custom relay"`
	RelayPubkey      string `envconfig:"RELAY_PUBKEY" default:"79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"`
	RelayDescription string `envconfig:"RELAY_DESCRIPTION" default:"this is an experimental relay"`
	RelayIcon        string `envconfig:"RELAY_ICON" default:"http://icons.iconarchive.com/icons/paomedia/small-n-flat/512/bell-icon.png"`
	DatabaseBackend  string `envconfig:"DATABASE" default:"sqlite"`
	DatabaseURL      string `envconfig:"DATABASE_URL"`
	ScriptsDirectory string `envconfig:"DATA_DIRECTORY" default:"./scripts"`
	DataDirectory    string `envconfig:"SCRIPTS_DIRECTORY" default:"./data"`
}

var (
	s     Settings
	log   = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	relay = khatru.NewRelay()
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
		Usage: "a personal relay",
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
				Name:        "icon",
				Usage:       "relay icon image",
				Value:       s.RelayIcon,
				Destination: &s.RelayIcon,
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
				Value:       s.ScriptsDirectory,
				Destination: &s.ScriptsDirectory,
				Category:    CATEGORY_UNCOMMON,
			},
		},
		ArgsUsage: "",
		Action: func(c *cli.Context) error {
			// check if scripts exist
			log.Info().Msg("checking for scripts under ./scripts/")
			os.MkdirAll(s.ScriptsDirectory, 0700)
			for _, scriptName := range []scriptPath{
				REJECT_EVENT,
				REJECT_FILTER,
			} {
				scriptPath := filepath.Join(s.ScriptsDirectory, string(scriptName))
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
			relay.Info.Icon = s.RelayIcon

			// basic relay methods with custom stores
			if err := os.MkdirAll(s.DataDirectory, 0700); err != nil {
				return fmt.Errorf("failed to create datadir '%s': %w", s.DataDirectory, err)
			}
			var db eventstore.Store
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

			// other http handlers
			mux := relay.Router()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("content-type", "text/html")
				fmt.Fprintf(w, `<b>welcome</b> to my relay!`)
			})

			// start the server
			localhost := s.Host
			if localhost == "" {
				localhost = "0.0.0.0"
			}
			log.Info().Msg("running on http://" + localhost + ":" + s.Port)
			if err := http.ListenAndServe(s.Host+":"+s.Port, relay); err != nil {
				return err
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
