package main

// planCommands holds the "plan" subcommands. Each cmd_plan_<name>.go file
// adds itself from init.
var planCommands = map[string]func(c *Ctx, args []string) error{}

func init() { commands["plan"] = subcommands("plan", planCommands) }
