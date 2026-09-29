#!/usr/bin/env python3
"""CI-only production regressions for the composed open-thought shelf."""

import importlib.util
from pathlib import Path
import sys

spec = importlib.util.spec_from_file_location(
    "thought_mutations", Path(__file__).with_name("thought-mutations.py")
)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)

# The shared runner requires one production edit site, a passing baseline,
# and the designated assertion's normal Go test failure. It restores bytes
# after every mode, including timeout, signal, and failed evidence checks.
runner.MODES = {
    "visible-source-location": {
        "file": "internal/note/openthoughts.go",
        "before": "Language: reading.Language, Wrap: true",
        "after": "Language: reading.Language, Wrap: false",
        "test": "TestOpenThoughtsComposesBothSourcesAndReachesOlderRows",
        "assertion": "open desk rows preserving their source location =",
    },
    "literal-initial": {
        "file": "internal/schema/declaredinitial.go",
        "before": "c != nil && c.initialDeclared && c.StartsAt(noteType, status)",
        "after": "c != nil && c.StartsAt(noteType, status)",
        "test": "TestOpenThoughtsDoesNotTreatInferredInitialAsDeclared",
        "assertion": "inferred initial status acquired an undeclared open-thought role",
    },
    "initial-membership": {
        "file": "internal/note/openthoughts.go",
        "before": "if !h.sources.Contract.DeclaresInitial(role, status) {",
        "after": 'if status == "" {',
        "test": "TestOpenThoughtsComposesBothSourcesAndReachesOlderRows",
        "assertion": "complete open shelf rows =",
    },
    "live-status": {
        "file": "internal/note/openthoughts.go",
        "before": "status, err := h.sources.ObservedStatus(ctx, reading.RelPath)",
        "after": "status, err := reading.Status, error(nil)",
        "test": "TestOpenThoughtsRechecksStatusBeforeTheNextScan",
        "assertion": "open shelf after settled has thought = true",
    },
    "corrupt-unavailable": {
        "file": "internal/note/openthoughts.go",
        "before": "fault = statedOnce(fault, wording.UncertaintyUnavailable.In(lang))",
        "after": 'fault = ""',
        "test": "TestOpenThoughtsKeepsMarksWithoutGovernanceAndReportsCorruption",
        "assertion": "corrupt marks were presented as an empty shelf rather than unavailable",
    },
    "mixed-newest-first": {
        "file": "internal/note/openthoughts.go",
        "before": "b.at.Compare(a.at)",
        "after": "a.at.Compare(b.at)",
        "test": "TestOpenThoughtsComposesBothSourcesAndReachesOlderRows",
        "assertion": "marks and notes are not interleaved newest first",
    },
    "authored-source-location": {
        "file": "internal/note/openthoughts.go",
        "before": 'text += " — " + strings.Join(declarations, "; ")',
        "after": 'text += ""',
        "test": "TestOpenThoughtsComposesBothSourcesAndReachesOlderRows",
        "assertion": 'complete open shelf is missing "[[Source#Chapter]]"',
    },
}
for mode in runner.MODES.values():
    mode["package"] = "cmd/yomihon"

if __name__ == "__main__":
    sys.exit(runner.main())
