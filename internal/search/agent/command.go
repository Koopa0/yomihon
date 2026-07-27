package agent

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search/semantic"
	"github.com/koopa0/yomihon/internal/vault"
)

const maxCLIInputBytes = 4096

type rootConfig struct {
	rootEnv     string
	defaultRoot func() (string, error)
}

type semanticActionConfig struct {
	reader        *vault.Reader
	artifact      schema.ArtifactPolicy
	privacy       schema.PrivacyPolicy
	buildProgress func(embedded, total int) error
}

type vaultCapabilities struct {
	artifact schema.ArtifactPolicy
	privacy  schema.PrivacyPolicy
	reader   *vault.Reader
	// governed says whether this folder claimed authority at all. The refusal is
	// the same either way — permission is positive authority and silence is not
	// permission — but what a person is told about it is not.
	governed bool
}

type commandFailureKind uint8

const (
	commandFailureTool         commandFailureKind = 1
	commandFailureUnavailable  commandFailureKind = 2
	commandFailureUnanswerable commandFailureKind = 3
	commandFailureInternal     commandFailureKind = 4
)

type commandFailure struct {
	kind   commandFailureKind
	reason searchReason
}

type cliArgCursor struct {
	args         []string
	parsingFlags bool
}

func (c *cliArgCursor) next() (arg string, flagPosition, ok bool) {
	for len(c.args) > 0 {
		arg = c.args[0]
		c.args = c.args[1:]
		if c.parsingFlags && arg == "--" {
			c.parsingFlags = false
			continue
		}
		return arg, c.parsingFlags, true
	}
	return "", false, false
}

func takeFlagValue(name, inline string, hasInline bool, rest []string) (value string, remaining []string, err error) {
	if hasInline {
		if inline == "" {
			return "", rest, fmt.Errorf("flag %s needs a non-empty value", name)
		}
		return inline, rest, nil
	}
	if len(rest) == 0 {
		return "", rest, fmt.Errorf("flag %s needs a value", name)
	}
	if rest[0] == "" {
		return "", rest[1:], fmt.Errorf("flag %s needs a non-empty value", name)
	}
	return rest[0], rest[1:], nil
}

func validateCLIRoot(root string) error {
	if len(root) > maxCLIInputBytes {
		return fmt.Errorf("root exceeds %d bytes", maxCLIInputBytes)
	}
	if containsCLIControl(root) {
		return errors.New("root contains control characters")
	}
	return nil
}

func containsCLIControl(s string) bool {
	for _, r := range s {
		if r <= '\x1f' || r >= '\x7f' && r <= '\x9f' {
			return true
		}
	}
	return false
}

func validCLIArgs(args []string) bool {
	for _, arg := range args {
		if !utf8.ValidString(arg) {
			return false
		}
	}
	return true
}

func resolveRoot(explicit string, config rootConfig) (string, error) {
	var root string
	switch {
	case explicit != "":
		root = explicit
	case config.rootEnv != "":
		root = config.rootEnv
	default:
		if config.defaultRoot == nil {
			return "", errors.New("resolve vault root: home directory is unavailable")
		}
		resolved, err := config.defaultRoot()
		if err != nil {
			return "", fmt.Errorf("resolve vault root: %w", err)
		}
		root = resolved
		if root == "" {
			return "", errors.New("resolve vault root: default is empty")
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve vault root: %w", err)
	}
	return filepath.Clean(root), nil
}

func callBeforeCapabilityRecheck(hook func()) {
	if hook != nil {
		hook()
	}
}

func loadVaultCapabilities(ctx context.Context, root string) (vaultCapabilities, error) {
	reader, err := vault.Open(root)
	if err != nil {
		return vaultCapabilities{}, err
	}
	capabilities := vaultCapabilities{reader: reader}
	contract, err := schema.LoadReader(ctx, reader)
	if err != nil {
		// A missing or invalid contract makes the privacy/artifact capability
		// unavailable; it is an answerability state, not a local tool crash.
		capabilities.governed = !schema.ContractAbsent(err)
		return capabilities, nil
	}
	capabilities.governed = true
	capabilities.artifact = contract.ArtifactPolicy()
	capabilities.privacy = contract.PrivacyPolicy()
	return capabilities, nil
}

func releaseVaultReader(reader *vault.Reader) {
	if reader == nil {
		return
	}
	if closeErr := reader.Close(); closeErr != nil {
		// The action is complete and this capability is read-only; there is no
		// durable write whose result a close failure could change.
		return
	}
}

func classifyEmbedError(err *semantic.EmbedError) commandFailure {
	switch err.Kind {
	case semantic.EmbedFailureUnreachable:
		return unavailableFailure(searchReasonEmbedderUnreachable)
	case semantic.EmbedFailureRateLimited:
		return unavailableFailure(searchReasonRateLimited)
	case semantic.EmbedFailureRejected:
		return unavailableFailure(searchReasonEmbedderRejected)
	case semantic.EmbedFailureMalformedRequest, semantic.EmbedFailureInternal:
		return commandFailure{kind: commandFailureInternal}
	case semantic.EmbedFailureProvider:
		return unavailableFailure(searchReasonEmbedderFailed)
	default:
		return unavailableFailure(searchReasonEmbedderFailed)
	}
}

func unavailableFailure(reason searchReason) commandFailure {
	return commandFailure{kind: commandFailureUnavailable, reason: reason}
}

func matchesAnyError(err error, targets ...error) bool {
	for _, target := range targets {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
