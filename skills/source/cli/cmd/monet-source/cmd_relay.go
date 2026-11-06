package main

import (
	"fmt"

	"source/internal/exit"
	"source/internal/relay"
)

func init() { commands["relay"] = subcommands("relay", relaySubs) }

// relaySubs maps each relay subcommand to its handler.
var relaySubs = map[string]func(c *Ctx, args []string) error{"mute": relayMute}

const relayMuteUsage = "usage: monet-source relay mute"

// relayMute is relay mute. It sets muted in the hook state of this session
// and prints the state path. It takes no argument and no lease.
func relayMute(c *Ctx, args []string) error {
	if len(args) != 0 {
		return exit.New(exit.Usage, "%s", relayMuteUsage)
	}
	p, err := relay.Mute()
	if err != nil {
		return exit.New(exit.Refused, "%v", err)
	}
	_, err = fmt.Fprintf(c.Stdout, "relay: muted %s\n", p)
	return err
}
