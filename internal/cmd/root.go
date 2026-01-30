// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourorg/arc-sdk/output"
	"github.com/yourorg/arc-sdk/store"
	"github.com/yourorg/arc-sdk/utils"
)

// NewRootCmd creates the root command for arc-sessions.
func NewRootCmd(db *sql.DB) *cobra.Command {
	sessStore := store.NewSessionsStore(db)

	root := &cobra.Command{
		Use:   "arc-sessions",
		Short: "Manage agent sessions",
		Long: `Browse, search, and manage AI agent sessions.

Sessions are indexed conversations with Claude, Codex, or other agents.
Use this to find previous sessions, resume conversations, or archive old ones.`,
	}

	root.AddCommand(newListCmd(sessStore))
	root.AddCommand(newFindCmd())
	root.AddCommand(newShowCmd(sessStore))
	root.AddCommand(newArchiveCmd(sessStore))

	return root
}

func newListCmd(sessStore *store.SessionsStore) *cobra.Command {
	var out output.OutputOptions
	var limit int
	var agent string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List recent sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			ctx := context.Background()
			sessions, err := sessStore.FindRecent(ctx, limit)
			if err != nil {
				return err
			}

			// Filter by agent if specified
			if agent != "" {
				var filtered []store.Session
				for _, s := range sessions {
					if strings.EqualFold(s.Agent, agent) {
						filtered = append(filtered, s)
					}
				}
				sessions = filtered
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(sessions)
			}

			if out.Is(output.OutputQuiet) {
				for _, s := range sessions {
					fmt.Println(s.ID)
				}
				return nil
			}

			table := output.NewTable("ID", "Agent", "Project", "Lines", "Last Modified")
			for _, s := range sessions {
				id := s.ID
				if len(id) > 12 {
					id = id[:12]
				}
				table.AddRow(
					id,
					s.Agent,
					s.Project,
					fmt.Sprintf("%d", s.Lines),
					utils.HumanizeTime(s.ModTS),
				)
			}
			table.Render()

			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum sessions to show")
	cmd.Flags().StringVar(&agent, "agent", "", "Filter by agent (claude, codex, etc.)")

	return cmd
}

func newFindCmd() *cobra.Command {
	var pattern string
	var limit int
	var out output.OutputOptions

	cmd := &cobra.Command{
		Use:   "find [pattern]",
		Short: "Search sessions using ripgrep",
		Long: `Search session content using ripgrep.

This searches the actual session files (JSONL transcripts) for matching content.
Useful for finding sessions where you discussed a specific topic.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			if len(args) > 0 {
				pattern = args[0]
			}
			if pattern == "" {
				return fmt.Errorf("search pattern required")
			}

			// Find session directories
			sessionDirs := findSessionDirs()
			if len(sessionDirs) == 0 {
				fmt.Println("No session directories found.")
				return nil
			}

			// Use ripgrep to search
			rgArgs := []string{
				"-l", // Files with matches only
				"--json",
				pattern,
			}
			rgArgs = append(rgArgs, sessionDirs...)

			rgCmd := exec.Command("rg", rgArgs...)
			output, err := rgCmd.Output()
			if err != nil {
				// rg returns exit code 1 if no matches
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
					fmt.Println("No matching sessions found.")
					return nil
				}
				return fmt.Errorf("ripgrep failed: %w", err)
			}

			// Parse ripgrep JSON output
			var matches []string
			scanner := bufio.NewScanner(strings.NewReader(string(output)))
			for scanner.Scan() {
				var msg struct {
					Type string `json:"type"`
					Data struct {
						Path struct {
							Text string `json:"text"`
						} `json:"path"`
					} `json:"data"`
				}
				if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
					continue
				}
				if msg.Type == "match" && msg.Data.Path.Text != "" {
					matches = append(matches, msg.Data.Path.Text)
				}
			}

			if len(matches) == 0 {
				fmt.Println("No matching sessions found.")
				return nil
			}

			// Deduplicate by directory
			seen := make(map[string]bool)
			var uniqueDirs []string
			for _, m := range matches {
				dir := filepath.Dir(m)
				if !seen[dir] {
					seen[dir] = true
					uniqueDirs = append(uniqueDirs, dir)
				}
			}

			if limit > 0 && len(uniqueDirs) > limit {
				uniqueDirs = uniqueDirs[:limit]
			}

			fmt.Printf("Found %d sessions matching '%s':\n", len(uniqueDirs), pattern)
			for _, dir := range uniqueDirs {
				fmt.Printf("  %s\n", dir)
			}

			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)
	cmd.Flags().StringVarP(&pattern, "pattern", "p", "", "Search pattern")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results")

	return cmd
}

func newShowCmd(sessStore *store.SessionsStore) *cobra.Command {
	var out output.OutputOptions

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show session details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.Resolve(); err != nil {
				return err
			}

			id := args[0]
			ctx := context.Background()

			session, err := sessStore.Get(ctx, id)
			if err != nil {
				return fmt.Errorf("session not found: %s", id)
			}

			if out.Is(output.OutputJSON) {
				return output.JSON(session)
			}

			fmt.Printf("ID:       %s\n", session.ID)
			fmt.Printf("Agent:    %s\n", session.Agent)
			fmt.Printf("Project:  %s\n", session.Project)
			fmt.Printf("CWD:      %s\n", session.CWD)
			fmt.Printf("Branch:   %s\n", session.Branch)
			fmt.Printf("Path:     %s\n", session.Path)
			fmt.Printf("Lines:    %d\n", session.Lines)
			fmt.Printf("Created:  %s\n", utils.FormatTimestamp(session.CreateTS))
			fmt.Printf("Modified: %s\n", utils.FormatTimestamp(session.ModTS))
			if session.Archived {
				fmt.Println("Status:   archived")
			}

			return nil
		},
	}

	out.AddOutputFlags(cmd, output.OutputTable)

	return cmd
}

func newArchiveCmd(sessStore *store.SessionsStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <id>",
		Short: "Archive a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			ctx := context.Background()

			if err := sessStore.Archive(ctx, id); err != nil {
				return err
			}

			fmt.Printf("Archived session: %s\n", id)
			return nil
		},
	}

	return cmd
}

// findSessionDirs returns paths where session files might be stored.
func findSessionDirs() []string {
	var dirs []string

	// Check common session locations
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".claude", "projects"),
		filepath.Join(home, ".codex", "sessions"),
		filepath.Join(home, ".local", "share", "arc", "sessions"),
	}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			dirs = append(dirs, dir)
		}
	}

	return dirs
}
