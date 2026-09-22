#!/usr/bin/env python3
"""CI-only watched-red checks against the composed declared-source HTTP path.

Run with: python3 tools/check-source-location-mutations.py --output-dir PATH
This edits one production site at a time, compiles the real note test binary,
and requires that binary to fail at the named HTTP assertion. Build failures,
timeouts, dead needles, and unrelated failures never count as caught mutations.
Every edited file is restored byte-for-byte in finally, including on SIGTERM.
The output directory retains command transcripts, not compiled binaries.
"""

import argparse
from dataclasses import dataclass
import os
from pathlib import Path
import re
import shlex
import signal
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[1]
TEST = "TestDeclaredSourceLocationsReachTheReadingPage"
PACKAGE = "./internal/note"


@dataclass(frozen=True)
class Mutation:
    name: str
    path: str
    needle: str
    replacement: str
    assertion: str


# Each needle names exactly one production edit site. Each assertion is an
# existing HTTP-boundary observation, not a source-text or build-error oracle.
MUTATIONS = (
    Mutation(
        "drop-heading-fragment",
        "internal/render/location.go",
        "\t\tlocation.Fragment = id\n",
        "\t\tif link.Block != \"\" { location.Fragment = id }\n",
        r'declared source block missing "href=\"/notes/Source.md#methods\""',
    ),
    Mutation(
        "drop-block-fragment",
        "internal/render/location.go",
        "\t\tlocation.Fragment = id\n",
        "\t\tif link.Block == \"\" { location.Fragment = id }\n",
        r'declared source block missing "href=\"/notes/Source.md#%5Equote-1\""',
    ),
    Mutation(
        "keep-identical-location-twice",
        "internal/snapshot/basedon.go",
        "\t\tif seen[locationKey] {\n",
        "\t\tif false && seen[locationKey] {\n",
        "identical location was not collapsed:",
    ),
    Mutation(
        "reverse-location-order",
        "internal/snapshot/basedon.go",
        "\t\tgroups[at].Locations = append(groups[at].Locations, location)\n",
        "\t\tgroups[at].Locations = append([]render.SourceLocation{location}, groups[at].Locations...)\n",
        "locations lost authored order:",
    ),
    Mutation(
        "reverse-file-group-order",
        "internal/snapshot/basedon.go",
        "\treturn groups, diagnostics\n",
        "\tslices.Reverse(groups)\n\treturn groups, diagnostics\n",
        "file grouping lost authored order:",
    ),
    Mutation(
        "duplicate-source-group-count",
        "internal/snapshot/basedon.go",
        "\treturn groups, diagnostics\n",
        "\tif len(groups) > 0 { groups = append(groups, groups[0]) }\n\treturn groups, diagnostics\n",
        r'declared source block missing "ui-navitem__count\">2</span>"',
    ),
    Mutation(
        "keep-file-row-for-single-location",
        "internal/snapshot/basedon.go",
        "\t\tgroups[at].Locations = append(groups[at].Locations, location)\n",
        "\t\tgroups[at].Locations = append(groups[at].Locations, location)\n\t\tgroups[at].WholeFile = true\n",
        "single location did not collapse:",
    ),
    Mutation(
        "drop-explicit-whole-file-declaration",
        "internal/snapshot/basedon.go",
        "\t\t\tgroups[at].WholeFile = true\n",
        "\t\t\tgroups[at].WholeFile = false\n",
        "bare source disappeared beside its one location:",
    ),
    Mutation(
        "discard-authored-alias",
        "internal/render/location.go",
        "\tif display != \"\" {\n",
        "\tif false && display != \"\" {\n",
        'declared source block missing "Method evidence"',
    ),
    Mutation(
        "discard-resolved-heading-words",
        "internal/render/location.go",
        "\t\t\t\tlabel, found = headingInnerText(heading[2]), true\n",
        "\t\t\t\tlabel, found = link.Heading, true\n",
        'declared source block missing "Other › Observation"',
    ),
    Mutation(
        "invent-missing-location-fragment",
        "internal/render/location.go",
        "\tlocation.Diagnostic = diag\n",
        "\tlocation.Fragment = id\n\tlocation.Diagnostic = diag\n",
        "missing location retained a false destination:",
    ),
    Mutation(
        "drop-source-diagnostics-at-handler",
        "internal/note/handler.go",
        "\tresult.Diagnostics = append(result.Diagnostics, sourceDiagnostics...)\n",
        "\t_ = sourceDiagnostics\n",
        "note health omits all source-location diagnostics",
    ),
    Mutation(
        "drop-fragment-at-href-boundary",
        "internal/ui/pages/basedon.go",
        "\tif fragment != \"\" {\n",
        "\tif false && fragment != \"\" {\n",
        r'declared source block missing "href=\"/notes/Source.md#methods\""',
    ),
)


class Failure(Exception):
    pass


class NotApplied(Failure):
    pass


def interrupted(signum, _frame):
    raise KeyboardInterrupt(f"received signal {signum}")


def run(command, label, output_dir, env, cwd=ROOT):
    """Save even interrupted output; stop only this command's process group."""
    print(f"COMMAND {label} (cwd={cwd}): {shlex.join(command)}", flush=True)
    process = subprocess.Popen(
        command, cwd=cwd, env=env, stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT, text=True, encoding="utf-8",
        errors="replace", start_new_session=True,
    )
    output = ""
    try:
        output, _ = process.communicate(timeout=300)
    except BaseException:
        if process.poll() is None:
            os.killpg(process.pid, signal.SIGTERM)
        try:
            output, _ = process.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            output, _ = process.communicate()
        raise
    finally:
        transcript = (
            f"cwd={cwd}\n$ {shlex.join(command)}\n{output}\n"
            f"exit_status={process.returncode}\n"
        )
        (output_dir / f"{label}.log").write_text(transcript, encoding="utf-8")
        print(transcript, flush=True)
    return process.returncode, output


def compiled_test(label, output_dir, build_dir, env):
    binary = build_dir / "note.test"
    binary.unlink(missing_ok=True)
    code, _ = run(
        ["go", "test", "-c", "-o", str(binary), PACKAGE],
        f"{label}-compile", output_dir, env,
    )
    if code != 0 or not binary.is_file():
        raise Failure(f"{label}: compilation failed; this is not a caught mutation")
    print(f"COMPILED {label}", flush=True)
    return run(
        [str(binary), f"-test.run=^{TEST}$", "-test.count=1", "-test.v"],
        f"{label}-test", output_dir, env, cwd=ROOT / "internal/note",
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", required=True, type=Path)
    args = parser.parse_args()
    if os.environ.get("GITHUB_ACTIONS") != "true":
        raise Failure("this mutation harness may run only in GitHub Actions")
    output_dir = args.output_dir.resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env["GOMAXPROCS"] = "2"
    env["GOFLAGS"] = "-p=2"
    signal.signal(signal.SIGTERM, interrupted)

    code, _ = run(["git", "rev-parse", "HEAD"], "commit", output_dir, env)
    if code != 0:
        raise Failure("cannot identify the checked-out commit")

    paths = sorted({mutation.path for mutation in MUTATIONS})
    code, _ = run(["git", "diff", "--exit-code", "HEAD", "--", *paths],
                  "clean-production", output_dir, env)
    if code != 0:
        raise Failure("production mutation targets must match the checked-out commit")
    originals = {path: (ROOT / path).read_bytes() for path in paths}
    names = [mutation.name for mutation in MUTATIONS]
    if len(set(names)) != len(names):
        raise Failure("mutation names must be unique")
    # Validate the entire table before starting, so an obsolete later needle
    # cannot leave a partial run that looks like complete evidence.
    for mutation in MUTATIONS:
        count = originals[mutation.path].count(mutation.needle.encode("utf-8"))
        if count != 1 or mutation.needle == mutation.replacement:
            raise NotApplied(f"{mutation.name}: needle matches {count} sites; want exactly one changed site")

    with tempfile.TemporaryDirectory(prefix="yomihon-source-mutations-") as temporary:
        build_dir = Path(temporary)
        code, output = compiled_test("baseline", output_dir, build_dir, env)
        if code != 0 or f"--- PASS: {TEST} (" not in output:
            raise Failure("the precise composed HTTP test must pass before mutations")
        caught = []
        for mutation in MUTATIONS:
            path = ROOT / mutation.path
            original = originals[mutation.path]
            mutated = original.replace(mutation.needle.encode("utf-8"),
                                       mutation.replacement.encode("utf-8"))
            try:
                path.write_bytes(mutated)
                if path.read_bytes() != mutated:
                    raise NotApplied(f"{mutation.name}: production edit was not written")
                code, output = compiled_test(mutation.name, output_dir, build_dir, env)
                # Tie the observed failure to this test and its named assertion.
                # A panic, missing binary, timeout or generic nonzero is no proof.
                assertion = re.compile(
                    r"^\s+\w+_test\.go:\d+: " + re.escape(mutation.assertion), re.MULTILINE,
                )
                failed_tests = re.findall(r"^--- FAIL: (\S+) \(", output, re.MULTILINE)
                if (code != 1 or failed_tests != [TEST] or not assertion.search(output)
                        or re.search(r"^(?:panic:|fatal error:)", output, re.MULTILINE)):
                    raise Failure(f"{mutation.name}: expected assertion was not the observed test failure")
                print(f"MUTATE-RESULT: caught {mutation.name}", flush=True)
                caught.append(mutation.name)
            finally:
                path.write_bytes(original)
                if path.read_bytes() != original:
                    raise Failure(f"{mutation.name}: production file restoration failed")
        if caught != names:
            raise Failure("the complete mutation table was not caught")
        code, output = compiled_test("restored", output_dir, build_dir, env)
        if code != 0 or f"--- PASS: {TEST} (" not in output:
            raise Failure("the restored composed HTTP test did not pass")
    for path, original in originals.items():
        if (ROOT / path).read_bytes() != original:
            raise Failure(f"production file was not restored: {path}")
    print(f"PASS source-location-mutations: all {len(names)} compiled mutations caught; sources restored", flush=True)


if __name__ == "__main__":
    try:
        main()
    except NotApplied as error:
        print(f"NOT-APPLIED source-location-mutations: {error}", file=sys.stderr)
        sys.exit(2)
    except (Failure, OSError, subprocess.SubprocessError) as error:
        print(f"FAIL source-location-mutations: {error}", file=sys.stderr)
        sys.exit(1)
    except KeyboardInterrupt as error:
        print(f"STOPPED source-location-mutations: {error}", file=sys.stderr)
        sys.exit(130)
