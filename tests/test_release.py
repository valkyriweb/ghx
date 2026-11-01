"""Exercise scripts/release offline; requires Python 3, bash, git, tar and shasum."""

import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]


class ReleaseTest(unittest.TestCase):
    def test_release_commits_formula_and_snapshot_together(self):
        with tempfile.TemporaryDirectory(prefix="ghx-release-") as directory:
            work = Path(directory)
            source, tap, commands = (work / name for name in ("source", "tap", "bin"))
            for path in (source / "scripts", tap / "tests", commands):
                path.mkdir(parents=True)
            shutil.copy2(ROOT / "scripts" / "release", source / "scripts" / "release")
            verify = source / "scripts" / "verify"
            verify.write_text("#!/bin/sh\nexit 0\n")
            verify.chmod(0o755)
            other = {"repository": "example/other", "version": "2.3.4", "assets": {"other.tgz": "a" * 64}}
            (tap / "tests" / "releases.json").write_text(json.dumps({
                "other": other,
                "ghx": {"repository": "valkyriweb/ghx", "version": "0.0.1", "assets": {}},
            }))
            (source / "CHANGELOG.md").write_text("## Unreleased\n\n## v9.8.7 — 2099-01-01\n")
            real_git = shutil.which("git")
            env = {
                **os.environ,
                "GIT_CONFIG_NOSYSTEM": "1",
                "GIT_CONFIG_GLOBAL": os.devnull,
                "GIT_AUTHOR_NAME": "Release fixture",
                "GIT_AUTHOR_EMAIL": "fixture@example.invalid",
                "GIT_COMMITTER_NAME": "Release fixture",
                "GIT_COMMITTER_EMAIL": "fixture@example.invalid",
                "REAL_GIT": real_git,
                "FIXTURE_TAP": str(tap),
                "FIXTURE_LOG": str(work / "commands.log"),
                "FIXTURE_CLONE": str(work / "clone-path"),
            }

            def git(repo, *args):
                return subprocess.check_output([real_git, "-C", str(repo), *args], env=env, text=True).strip()

            for repo in (source, tap):
                git(repo, "init", "-q")
                git(repo, "add", ".")
                git(repo, "commit", "-qm", "fixture")

            # One dispatcher rejects unknown external operations. Only local fixture
            # Git operations reach real git; tags, pushes and gh never do.
            dispatcher = commands / "dispatch"
            dispatcher.write_text("#!/usr/bin/env python3\n" + '''
import os
from pathlib import Path
import subprocess
import sys

name, args = Path(sys.argv[0]).name, sys.argv[1:]
with open(os.environ["FIXTURE_LOG"], "a") as log:
    log.write(name + " " + " ".join(args) + "\\n")
if name == "go" and args[0] == "build":
    Path(args[args.index("-o") + 1]).write_text(os.environ["GOOS"] + "/" + os.environ["GOARCH"] + args[-1])
elif name == "gh" and args[:2] == ["release", "create"]:
    pass
elif name == "gh" and args[:3] == ["repo", "clone", "valkyriweb/homebrew-tap"]:
    subprocess.run([os.environ["REAL_GIT"], "clone", "-q", os.environ["FIXTURE_TAP"], args[3]], check=True)
    Path(os.environ["FIXTURE_CLONE"]).write_text(args[3])
elif name == "git":
    operation = args[2:] if args[:1] == ["-C"] else args
    if operation[0] in ("tag", "push"):
        pass
    elif operation[0] in ("status", "rev-parse", "add", "commit"):
        sys.exit(subprocess.run([os.environ["REAL_GIT"], *args]).returncode)
    else:
        sys.exit("unexpected git operation: " + repr(args))
else:
    sys.exit("unexpected external command: " + name + repr(args))
''')
            dispatcher.chmod(0o755)
            for name in ("git", "gh", "go"):
                (commands / name).symlink_to(dispatcher)
            env["PATH"] = str(commands) + os.pathsep + env["PATH"]
            # Confine the script's own mktemp clone to this fixture's lifetime.
            env["TMPDIR"] = str(work)
            missing = subprocess.run(["bash", str(source / "scripts" / "release"), "v9.8.8"],
                                     env=env, text=True, capture_output=True)
            self.assertNotEqual(missing.returncode, 0)
            self.assertIn("CHANGELOG.md missing release heading for v9.8.8", missing.stderr)
            self.assertFalse((work / "commands.log").exists(), "release ran external commands before rejecting missing notes")
            result = subprocess.run(["bash", str(source / "scripts" / "release"), "v9.8.7"],
                                    env=env, text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            clone = Path((work / "clone-path").read_text())
            snapshot = json.loads(git(clone, "show", "HEAD:tests/releases.json"))
            self.assertEqual(snapshot["other"], other)
            release = snapshot["ghx"]
            self.assertEqual(release["repository"], "valkyriweb/ghx")
            self.assertEqual(release["version"], "9.8.7")
            archives = {f"ghx-{platform}.tar.gz" for platform in (
                "darwin-arm64", "darwin-amd64", "linux-arm64", "linux-amd64")}
            self.assertEqual(set(release["assets"]), archives)
            expected = {name: hashlib.sha256((source / "dist" / name).read_bytes()).hexdigest()
                        for name in archives}
            self.assertEqual(release["assets"], expected)
            formula = git(clone, "show", "HEAD:Formula/ghx.rb")
            self.assertIn('version "9.8.7"', formula)
            pairs = re.findall(r'url "([^"]+)"\s+sha256 "([0-9a-f]{64})"', formula)
            self.assertEqual(dict(pairs), {
                f"https://github.com/valkyriweb/ghx/releases/download/v9.8.7/{name}": digest
                for name, digest in expected.items()
            })
            changed = git(clone, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD").splitlines()
            self.assertEqual(set(changed), {"Formula/ghx.rb", "tests/releases.json"})
            self.assertEqual(git(clone, "status", "--porcelain"), "")
            self.assertIn("gh release create v9.8.7 ", (work / "commands.log").read_text())


if __name__ == "__main__":
    unittest.main()
