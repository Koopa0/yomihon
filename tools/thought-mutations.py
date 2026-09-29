#!/usr/bin/env python3
"""Run source-linked thought regression proofs on GitHub CI only.

Each mode changes one uniquely matched production site, runs its named Go
test, and restores the original bytes in a finally block. A caught mutation
requires a normal test failure and that test's assertion text, never a build
failure, panic, timeout, or a different test failing. The aggregate command
exits zero only after every selected mode is caught.
"""

import argparse
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[1]
MODULE = "github.com/koopa0/yomihon"

# This table owns both --list and execution. Needles name source declarations
# and must match exactly once; tests and expected assertion text are not edited.
MODES = {
    "answer-role-default": {
        "file": "internal/schema/answer.go",
        "before": 'if c == nil || c.answerType == nil {\n\t\treturn ""\n\t}',
        "after": 'if c == nil || c.answerType == nil {\n\t\treturn "lesson"\n\t}',
        "package": "internal/schema",
        "test": "TestAnswerTypeRequiresExplicitEnumMember/absent",
        "assertion": 'AnswerType() = "lesson", want ""',
    },
    "answer-role-stale": {
        "file": "internal/schema/answer.go",
        "before": 'if !same {\n\t\tstate.stale.Store(true)\n\t\treturn ""\n\t}',
        "after": 'if !same {\n\t\tstate.stale.Store(true)\n\t\treturn state.noteType\n\t}',
        "package": "internal/schema",
        "test": "TestAnswerTypeRevocationClosesEveryContractCopy",
        "assertion": 'AnswerType() = "lesson" after revocation',
    },
    "thought-multiple-initials": {
        "file": "internal/note/thought.go",
        "before": "if len(initial) == 1 {",
        "after": "if len(initial) > 0 {",
        "package": "internal/note",
        "test": "TestThoughtMarkdownOmitsMultipleInitialStates",
        "assertion": "want status omitted with multiple initial states",
    },
    "thought-source-fragment": {
        "file": "internal/note/thought.go",
        "before": 'target += "#" + section',
        "after": 'target += ""',
        "package": "internal/note",
        "test": "TestThoughtMarkdownContainsOnlyDeterminedFrontmatterAndSource",
        "assertion": "thoughtMarkdown() =",
    },
    "reverse-multiple-locations": {
        "file": "internal/snapshot/declaredby.go",
        "before": "places = append(places, place)",
        "after": "places = []DeclaredPlace{place}",
        "package": "internal/snapshot",
        "test": "TestDeclaredByPreservesEachSourcesAuthoredLocations",
        "assertion": "DeclaredBy(Book) mismatch (-want +got)",
    },
    "uncertainty-corrupt-overwrite": {
        "file": "internal/mark/uncertainty.go",
        "before": "held, err := f.Uncertainties()\n\tif err != nil {\n\t\treturn false, err\n\t}",
        "after": "held, err := f.Uncertainties()\n\tif err != nil {\n\t\theld = []Uncertainty{}\n\t}",
        "package": "internal/mark",
        "test": "TestCorruptUncertaintiesAreNeverReplaced",
        "assertion": "toggle damaged file = <nil>; want unreadable",
    },
}


class EvidenceFailure(Exception):
    """An applied regression was not caught by its designated assertion."""


class NotApplied(Exception):
    """The production needle no longer identifies exactly one site."""


def stop_owned_process(process):
    """Stop this Go invocation and its descendants, not other CI processes."""
    if os.name == "posix":
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
    elif process.poll() is None:
        process.kill()


def run_go(arguments, output_file):
    environment = os.environ.copy()
    environment["GOMAXPROCS"] = "2"
    environment["GOFLAGS"] = "-p=2"
    command = ["go", "test", "-json", "-count=1", "-timeout=90s", *arguments]
    print("RUN " + " ".join(command), flush=True)
    process = subprocess.Popen(
        command,
        cwd=ROOT,
        env=environment,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        start_new_session=os.name == "posix",
    )
    try:
        output, _ = process.communicate(timeout=180)
    except BaseException:
        stop_owned_process(process)
        output, _ = process.communicate()
        output_file.write_text(output, encoding="utf-8")
        raise
    output_file.write_text(output, encoding="utf-8")
    events = []
    for line in output.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(event, dict):
            events.append(event)
    return process.returncode, output, events


def belongs(event, mode):
    test = event.get("Test", "")
    return event.get("Package") == MODULE + "/" + mode["package"] and (
        test == mode["test"] or test.startswith(mode["test"] + "/")
    )


def baseline(selected, output_dir):
    tests = sorted({mode["test"].split("/")[0] for _, mode in selected})
    packages = sorted({"./" + mode["package"] for _, mode in selected})
    expression = "^(" + "|".join(re.escape(test) for test in tests) + ")$"
    code, output, events = run_go(
        ["-run", expression, *packages], output_dir / "baseline.jsonl"
    )
    if code != 0:
        print(output, end="")
        raise EvidenceFailure("baseline tests failed; no mutation is evidence")
    for name, mode in selected:
        if not any(
            belongs(event, mode)
            and event.get("Test") == mode["test"]
            and event.get("Action") == "pass"
            for event in events
        ):
            raise EvidenceFailure(f"baseline did not run and pass {name}: {mode['test']}")
    print("BASELINE-RESULT: passed every selected test", flush=True)


def require_caught(name, mode, code, output, events):
    # A test's fail event alone can also accompany a panic. Require its own
    # assertion output and reject process/build failures independently.
    crashed = re.search(r"(?im)^(?:panic:|fatal error:|runtime:)", output)
    broken_build = "[build failed]" in output or any(
        event.get("Action") == "build-fail" for event in events
    )
    failed = any(
        belongs(event, mode)
        and event.get("Test") == mode["test"]
        and event.get("Action") == "fail"
        for event in events
    )
    assertion_events = [
        event["Output"]
        for event in events
        if belongs(event, mode)
        and event.get("Action") == "output"
        and mode["assertion"] in event.get("Output", "")
    ]
    if code != 1 or crashed or broken_build or not failed or not assertion_events:
        print(output, end="")
        raise EvidenceFailure(
            f"{name}: expected {mode['test']} assertion failure; "
            f"exit={code}, named-fail={failed}, assertion={bool(assertion_events)}, "
            f"crash={bool(crashed)}, build-failure={broken_build}"
        )
    for line in assertion_events:
        print(line, end="" if line.endswith("\n") else "\n")
    print(f"MUTATE-RESULT: caught {name}", flush=True)


def mutate(name, mode, output_dir):
    source = ROOT / mode["file"]
    original = source.read_bytes()
    before = mode["before"].encode("utf-8")
    after = mode["after"].encode("utf-8")
    count = original.count(before)
    if count != 1 or before == after:
        print(f"MUTATE-RESULT: not-applied {name}", flush=True)
        raise NotApplied(f"{name}: expected one edit site, found {count}")
    mutated = original.replace(before, after, 1)
    try:
        source.write_bytes(mutated)
        if source.read_bytes() != mutated:
            raise NotApplied(f"{name}: written mutation did not match intended bytes")
        print(f"MUTATE-APPLIED: {name} {mode['file']} (one site)", flush=True)
        expression = "/".join("^" + re.escape(part) + "$" for part in mode["test"].split("/"))
        code, output, events = run_go(
            ["-run", expression, "./" + mode["package"]],
            output_dir / (name + ".jsonl"),
        )
        require_caught(name, mode, code, output, events)
    finally:
        source.write_bytes(original)
        if source.read_bytes() != original:
            raise EvidenceFailure(f"{name}: production source was not restored")
        print(f"MUTATE-RESTORED: {name}", flush=True)


def interrupt(_signal, _frame):
    raise KeyboardInterrupt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--list", action="store_true", help="list the modes without running tests")
    parser.add_argument("--mode", choices=MODES, help="run just one mode after its baseline")
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=Path(os.environ.get("RUNNER_TEMP", "/tmp")) / "thought-mutations",
    )
    args = parser.parse_args()
    if args.list:
        print("\n".join(MODES))
        return 0
    if os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("CI") != "true":
        print("Refusing to run outside GitHub CI; local tests are paused.", file=sys.stderr)
        return 2
    selected = [(name, mode) for name, mode in MODES.items() if args.mode in (None, name)]
    args.output_dir.mkdir(parents=True, exist_ok=True)
    signal.signal(signal.SIGTERM, interrupt)
    try:
        baseline(selected, args.output_dir)
        for name, mode in selected:
            mutate(name, mode, args.output_dir)
    except NotApplied as error:
        print(f"FAIL {error}", file=sys.stderr)
        return 2
    except (EvidenceFailure, OSError, subprocess.TimeoutExpired) as error:
        print(f"FAIL {error}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("Interrupted; the active production mutation was restored.", file=sys.stderr)
        return 130
    print(f"MUTATION-SUITE: caught {len(selected)} of {len(selected)}", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
