package main

import (
	"fmt"
	"net/http"
	"os"

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
	RelayName        string `envconfig:"RELAY_NAME"`
	RelayPubkey      string `envconfig:"RELAY_PUBKEY"`
	RelayDescription string `envconfig:"RELAY_DESCRIPTION"`
	RelayContact     string `envconfig:"RELAY_CONTACT"`
	RelayIcon        string `envconfig:"RELAY_ICON"`
	DatabasePath     string `envconfig:"DATABASE_PATH" default:"./db"`
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
		},
		ArgsUsage: "",
		Action: func(c *cli.Context) error {
			// check if scripts exist
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

			// use cli arguments
			if port := c.String("port"); port != "" {
				s.Port = port
			}
			if host := c.String("host"); host != "" {
				s.Host = host
			}
			if name := c.String("name"); name != "" {
				s.RelayName = name
			}
			if description := c.String("description"); description != "" {
				s.RelayDescription = description
			}
			if icon := c.String("icon"); icon != "" {
				s.RelayIcon = icon
			}
			if pubkey := c.String("pubkey"); pubkey != "" {
				s.RelayPubkey = pubkey
			}

			// relay metadata
			relay.Info.Name = s.RelayName
			relay.Info.PubKey = s.RelayPubkey
			relay.Info.Description = s.RelayDescription
			relay.Info.Contact = s.RelayContact
			relay.Info.Icon = s.RelayIcon

			// basic relay methods
			db := sqlite3.SQLite3Backend{DatabaseURL: s.DatabasePath}
			if err := db.Init(); err != nil {
				panic(err)
			}
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
