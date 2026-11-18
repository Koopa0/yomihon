package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/vaults"
	"github.com/koopa0/yomihon/internal/vault"
)

// vaultsCommand lists the vaults the reader has opened, and removes one. Adding
// one is `yomihon <dir>`: a folder joins the list by being read, and nothing
// else makes yomihon read a path.
const vaultsCommand = "vaults"

const removeVerb = "remove"

// vaultsLine is one vault as the command prints it as JSON.
type vaultsLine struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Address string `json:"address"`
}

// runVaults is the command. It works with or without a running yomihon, since
// both read the same list.
func runVaults(_ context.Context, args []string, cfg config, stdout, stderr io.Writer, stdoutIsTerminal bool) int {
	removing := len(args) > 0 && args[0] == removeVerb
	if removing {
		args = args[1:]
	}
	parsed, err := parseCommandArgs(args)
	if err != nil {
		return commandError(stderr, err)
	}
	if err = checkVaultsArgs(&parsed, removing); err != nil {
		return commandError(stderr, err)
	}
	if cfg.configDir == "" {
		return commandError(stderr, fmt.Errorf("no configuration directory, so no list of vaults is kept: %s", cfg.noConfigDir))
	}
	store, err := vaults.Open(cfg.configDir, nil, vaults.Report(func(line string) {
		_, _ = fmt.Fprintln(stderr, line) //nolint:errcheck // a line about the list that cannot be written has nowhere else to go
	}))
	if err != nil {
		return commandError(stderr, fmt.Errorf("open the list of vaults: %w", err))
	}
	if removing {
		return removeVault(store, parsed.positionals[0], stdout, stderr)
	}
	return listVaults(store, judge.ResolveFormat(parsed.format, stdoutIsTerminal), stdout, stderr)
}

func checkVaultsArgs(parsed *commandArgs, removing bool) error {
	switch {
	case parsed.all, len(parsed.deny) > 0, parsed.baseline != "", parsed.rootCount > 0:
		return errors.New("usage: yomihon vaults [--format json|human] — or yomihon vaults remove <name|path>")
	case removing && (parsed.format != nil || len(parsed.positionals) != 1):
		return errors.New("usage: yomihon vaults remove <name|path>")
	case !removing && len(parsed.positionals) > 0:
		return errors.New("usage: yomihon vaults [--format json|human]")
	}
	return nil
}

func listVaults(store *vaults.Store, format judge.Format, stdout, stderr io.Writer) int {
	list, err := store.List()
	if err != nil {
		return commandError(stderr, fmt.Errorf("read the list of vaults: %w", err))
	}
	lines := make([]vaultsLine, 0, len(list))
	for i := range list {
		lines = append(lines, vaultsLine{Name: list[i].Name, Path: list[i].Path, Address: addressOf(list[i].Port)})
	}
	if format == judge.FormatJSON {
		payload, marshalErr := json.Marshal(lines)
		if marshalErr != nil {
			return commandError(stderr, fmt.Errorf("encode the list of vaults: %w", marshalErr))
		}
		if _, err = fmt.Fprintf(stdout, "%s\n", payload); err != nil {
			return commandError(stderr, fmt.Errorf("write output: %w", err))
		}
		return 0
	}
	table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	for i := range lines {
		if _, err = fmt.Fprintf(table, "%s\t%s\t%s\n", lines[i].Name, vault.Tilde(lines[i].Path), lines[i].Address); err != nil {
			return commandError(stderr, fmt.Errorf("write output: %w", err))
		}
	}
	if err = table.Flush(); err != nil {
		return commandError(stderr, fmt.Errorf("write output: %w", err))
	}
	return 0
}

// takeOutOfList takes the vault out of the list the reader keeps, through the
// store that keeps it. It is the one place yomihon's commands change that list
// by taking an entry out, and the write guard names it.
func takeOutOfList(store *vaults.Store, id string) error {
	_, err := store.Remove(id)
	return err
}

// removeVault takes one vault out of the list. Only the list entry goes: its
// marks stay where they are, and are there when the folder is read again.
func removeVault(store *vaults.Store, which string, stdout, stderr io.Writer) int {
	list, err := store.List()
	if err != nil {
		return commandError(stderr, fmt.Errorf("read the list of vaults: %w", err))
	}
	chosen, err := pickVault(list, which)
	if err != nil {
		return commandError(stderr, err)
	}
	if err = takeOutOfList(store, chosen.ID); err != nil {
		return commandError(stderr, fmt.Errorf("remove the vault: %w", err))
	}
	if _, err = fmt.Fprintf(stdout, "removed %s  %s\n", chosen.Name, vault.Tilde(chosen.Path)); err != nil {
		return commandError(stderr, fmt.Errorf("write output: %w", err))
	}
	return 0
}

// pickVault finds the vault a reader means by a name or a path. A folder that
// exists is spelled the way the list spells it before it is compared, so a
// relative path or one through a link finds its vault; a name that two vaults
// share is refused, since the path tells them apart.
func pickVault(list []vaults.Entry, which string) (vaults.Entry, error) {
	byPath := ""
	if _, resolved, err := vault.ResolveRoot(which); err == nil {
		byPath = resolved
	}
	var found []vaults.Entry
	for i := range list {
		if list[i].Path == which || (byPath != "" && list[i].ID == confdir.VaultID(byPath)) {
			return list[i], nil
		}
		if list[i].Name == which {
			found = append(found, list[i])
		}
	}
	switch len(found) {
	case 0:
		return vaults.Entry{}, fmt.Errorf("no vault in the list is called or is at %q", which)
	case 1:
		return found[0], nil
	default:
		paths := make([]string, 0, len(found))
		for i := range found {
			paths = append(paths, found[i].Path)
		}
		slices.Sort(paths)
		return vaults.Entry{}, fmt.Errorf("%d vaults are called %q; give the path of one: %s", len(found), which, strings.Join(paths, ", "))
	}
}

func addressOf(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/", port)
}
