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
	RelayDescription string `envconfig:"RELAY_DESCRIPTION" default:"experimental, do not use"`
	RelayIcon        string `envconfig:"RELAY_ICON" default:"http://icons.iconarchive.com/icons/paomedia/small-n-flat/512/bell-icon.png"`
	DatabaseURL      string `envconfig:"DATABASE_URL"`
}

var (
	s     Settings
	log   = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	relay = khatru.NewRelay()
)

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig")
		return
	}

	app := &cli.App{
		Name:  "jinglebells",
		Usage: "a personal relay",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "host",
				Usage: "address in which to listen for the server",
				Value: s.Host,
			},
			&cli.StringFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "port in which to listen for the server",
				Value:   s.Port,
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "relay name",
				Value: s.RelayName,
			},
			&cli.StringFlag{
				Name:  "description",
				Usage: "relay description",
				Value: s.RelayDescription,
			},
			&cli.StringFlag{
				Name:  "icon",
				Usage: "relay icon image",
				Value: s.RelayIcon,
			},
			&cli.StringFlag{
				Name:  "pubkey",
				Usage: "relay owner pubkey",
				Value: s.RelayPubkey,
			},
			&cli.StringFlag{
				Name:    "database",
				Aliases: []string{"db"},
				Usage:   "what database to use as a backend ('sqlite', 'lmdb' or 'badger')",
				Value:   "sqlite",
			},
			&cli.StringFlag{
				Name:    "database-url",
				Aliases: []string{"database-path", "dbpath"},
				Usage:   "path where to save the database files",
				Value:   "./data",
			},
		},
		ArgsUsage: "",
		Action: func(c *cli.Context) error {
			// check if scripts exist
			log.Info().Msg("checking for scripts under ./scripts/")
			os.MkdirAll("scripts", 0700)
			for _, requiredFile := range []scriptPath{
				REJECT_EVENT,
				REJECT_FILTER,
			} {
				if _, err := os.Stat(string(requiredFile)); err != nil {
					if os.IsNotExist(err) {
						// if they don't exist, create them
						err := os.WriteFile(string(requiredFile), []byte(defaultScripts[requiredFile]+"\n"), 0644)
						if err != nil {
							return fmt.Errorf("failed to write %s: %w", requiredFile, err)
						}
					} else {
						return fmt.Errorf("missing file %s: %w", requiredFile, err)
					}
				}
			}

			// relay metadata
			relay.Info.Name = c.String("name")
			relay.Info.PubKey = c.String("pubkey")
			relay.Info.Description = c.String("description")
			relay.Info.Icon = c.String("icon")

			// basic relay methods with custom stores
			dbpath := c.String("database-url")
			var db eventstore.Store
			switch c.String("database") {
			case "sqlite", "sqlite3":
				db = &sqlite3.SQLite3Backend{DatabaseURL: dbpath}
			case "lmdb":
				os.MkdirAll(dbpath, 0700)
				dbpath = filepath.Join(dbpath, "lmdb")
				db = &lmdb.LMDBBackend{Path: dbpath}
			case "badger":
				os.MkdirAll(dbpath, 0700)
				dbpath = filepath.Join(dbpath, "badger")
				db = &badger.BadgerBackend{Path: dbpath}
			default:
				return fmt.Errorf("unknown option '%s' for database", c.String("database"))
			}
			if err := db.Init(); err != nil {
				return fmt.Errorf("failed to initialize database: %w", err)
			}
			log.Info().Msgf("storing data with %s under ./%s", c.String("database"), dbpath)

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
