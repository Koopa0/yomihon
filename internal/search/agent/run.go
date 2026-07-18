package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/koopa0/yomihon/internal/search/semantic"
	"github.com/koopa0/yomihon/internal/search/semantic/observer"
)

const productionSemanticDimension = 1536

// CLI owns the process capabilities used by agent-facing search commands.
// The embedding credential remains dormant until an explicit semantic action
// reaches the provider gate.
type CLI struct {
	// Root is the default vault root when the command has no --root flag.
	Root string
	// DefaultRoot is called only when neither --root nor Root selected a vault.
	// Command composition owns the operator-specific default and its environment.
	DefaultRoot func() (string, error)
	// Stdout receives the frozen result envelope or human-readable result.
	Stdout io.Writer
	// Stderr receives diagnostics and build progress, never query text.
	Stderr io.Writer
	// ReadEmbeddingKey is called only after all local semantic gates pass.
	ReadEmbeddingKey func() string
}

type indexerConstructor func(*semantic.IndexerConfig, func() string) (*semantic.Indexer, error)

// Run executes name with args and returns the frozen process exit code.
func (c CLI) Run(ctx context.Context, name string, args []string) int {
	return runCommand(ctx, name, args, c, semantic.NewIndexer)
}

func runCommand(ctx context.Context, name string, args []string, cli CLI, newIndexer indexerConstructor) int {
	config := rootConfig{rootEnv: cli.Root, defaultRoot: cli.DefaultRoot}
	openIndexer := func(action semanticActionConfig) (*semantic.Indexer, error) {
		return newProductionIndexer(action, cli.ReadEmbeddingKey, newIndexer)
	}
	switch name {
	case "search":
		return runSearch(ctx, args, config, searchDeps{
			stdout: cli.Stdout,
			stderr: cli.Stderr,
			openSemantic: func(action semanticActionConfig) (reconcileSearch, error) {
				indexer, err := openIndexer(action)
				if err != nil {
					return nil, err
				}
				return func(ctx context.Context, corpus semantic.Corpus) (SemanticSearch, error) {
					return indexer.Reconcile(ctx, corpus)
				}, nil
			},
		})
	case "search-index":
		return runSearchIndex(ctx, args, config, buildCommandDeps{
			stdout: cli.Stdout,
			stderr: cli.Stderr,
			openBuilder: func(action semanticActionConfig) (generationBuilder, error) {
				return openIndexer(action)
			},
		})
	default:
		if err := writeSearchText(cli.Stderr, fmt.Sprintf("yomihon: unknown command %q; use search or search-index\n", name)); err != nil {
			return 2
		}
		return 2
	}
}

func newProductionIndexer(
	action semanticActionConfig,
	readAPIKey func() string,
	newIndexer indexerConstructor,
) (*semantic.Indexer, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("resolve semantic cache directory: %w", err)
	}
	compatibility, err := semantic.QueryVectorCompatibilityIdentity(productionSemanticDimension)
	if err != nil {
		return nil, fmt.Errorf("derive semantic query identity: %w", err)
	}
	workload, err := observer.Committed(compatibility)
	if err != nil {
		return nil, fmt.Errorf("load semantic observer workload: %w", err)
	}
	if workload.Dimension() != productionSemanticDimension {
		return nil, errors.New("semantic observer workload does not match the production protocol")
	}
	latencyVectors := workload.QueryVectors()
	if len(latencyVectors) != 40 {
		return nil, errors.New("semantic observer requires exactly 40 query vectors")
	}
	if readAPIKey == nil {
		readAPIKey = func() string { return "" }
	}
	if newIndexer == nil {
		return nil, errors.New("semantic indexer constructor is unavailable")
	}
	return newIndexer(&semantic.IndexerConfig{
		StorePath:      filepath.Join(cacheDir, "yomihon", "semantic", "generation.sqlite"),
		Vault:          action.reader,
		Dimension:      productionSemanticDimension,
		ChunkTokenCap:  semantic.ChunkTokenCap,
		ArtifactPolicy: action.artifact,
		PrivacyPolicy:  action.privacy,
		LatencyVectors: latencyVectors,
		BuildProgress:  action.buildProgress,
	}, readAPIKey)
}
