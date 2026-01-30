// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	"github.com/yourorg/arc-sessions/internal/cmd"
	"github.com/yourorg/arc-sdk/db"
)

func main() {
	database, err := db.Open(db.DefaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "arc-sessions: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	root := cmd.NewRootCmd(database)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
