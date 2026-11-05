package main

import (
	"errors"
	"time"

	"source/internal/exit"
	"source/internal/lock"
	"source/internal/store"
)

// leaseTTL is how long a lease lasts after each acquire or renew (5.8).
const leaseTTL = 12 * time.Hour

// requireLease renews lease name in <run>/locks for holder for leaseTTL.
// Each command that changes state calls it first. An empty holder is exit 64,
// and another holder or no lease is exit 4.
func requireLease(c *Ctx, name, holder string) error {
	if holder == "" {
		return exit.New(exit.Usage, "--holder <session> is required: the %s lease holder", name)
	}
	return heldExit(lock.Renew(store.Dir{Root: c.Run}.Locks(), name, holder, leaseTTL, c.Now()))
}

// heldExit maps lock.ErrHeld to exit 4 and returns any other error unchanged.
func heldExit(err error) error {
	if errors.Is(err, lock.ErrHeld) {
		return exit.New(exit.Held, "%v", err)
	}
	return err
}
