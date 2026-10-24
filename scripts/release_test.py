"""Hermetic release regressions. No builds, downloads, tags, or pushes."""

import os
import re
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cpumon-release-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        for name in ("scripts", "aur", "bin", "dist", "published"):
            (self.root / name).mkdir()
        for name in (
            "Makefile",
            "main.go",
            "scripts/release.sh",
            "aur/PKGBUILD",
            "aur/.SRCINFO",
            "aur/cpumon.install",
        ):
            shutil.copyfile(ROOT / name, self.root / name)
        self.env = dict(
            os.environ, PATH=f"{self.root / 'bin'}:{os.environ['PATH']}"
        )
        self.stub(
            "git",
            'printf "%s\\n" "$*" >> git-calls\n'
            'case "$*" in *--short*) echo abc123;; esac\n',
        )
        self.stub("go", "exit 0\n")
        self.stub("gh", "exit 0\n")
        self.stub(
            "makepkg",
            """case "$1" in
  -g) printf "sha256sums=('%064d')\\n" 1;;
  --printsrcinfo) cat PKGBUILD;;
  *) exit 99;;
esac
""",
        )
        pkg_path = self.root / "aur/PKGBUILD"
        pkg = re.sub(
            r"^pkgver=.*$", "pkgver=0.2.10", pkg_path.read_text(), flags=re.M
        )
        pkg = re.sub(r"^pkgrel=.*$", "pkgrel=1", pkg, flags=re.M)
        pkg_path.write_text(pkg)
        main = self.root / "main.go"
        main.write_text(
            re.sub(
                r'const releaseVersion = ".*"',
                'const releaseVersion = "0.0.1"',
                main.read_text(),
            )
        )
        (self.root / "published/PKGBUILD").write_text(
            pkg.replace("pkgver=0.2.10", "pkgver=0.2.9").replace(
                "pkgrel=1", "pkgrel=7"
            )
        )
        (self.root / "published/.SRCINFO").write_text("existing metadata\n")

    def stub(self, name, body):
        path = self.root / "bin" / name
        path.write_text("#!/bin/sh\n" + body)
        path.chmod(0o755)

    def run_command(self, *args, input=""):
        return subprocess.run(
            args,
            cwd=self.root,
            env=self.env,
            input=input,
            text=True,
            capture_output=True,
            timeout=15,
        )

    def release_function(self, command):
        # Load existing functions without invoking the publishing entry point.
        script = (self.root / "scripts/release.sh").read_text()
        script = script.removesuffix("main\n")
        return self.run_command("bash", "-c", script + "\n" + command)

    def aur_update(self, input=""):
        return self.run_command(
            "make",
            "aur-update",
            f"AUR_DIR={self.root / 'published'}",
            "RELEASE_VERSION=v0.2.10",
            input=input,
        )

    def test_manifest_hashes_only_binaries_on_repeated_build(self):
        for name in (
            "cpumon-linux-amd64",
            "cpumon-linux-arm64",
            "checksums.txt",
            "unrelated",
        ):
            (self.root / "dist" / name).write_text(name)
        result = self.release_function("build_binaries; build_binaries")
        self.assertEqual(result.returncode, 0, result.stderr)
        manifest = (self.root / "dist/checksums.txt").read_text().splitlines()
        self.assertEqual(len(manifest), 2, manifest)
        result = self.run_command(
            "bash", "-c", "cd dist && sha256sum -c checksums.txt"
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_preflight_rejects_untracked_inputs(self):
        self.stub(
            "git",
            'case "$1" in status) echo "?? unexpected.go";; '
            "rev-parse) echo main;; esac\n",
        )
        result = self.release_function("preflight_checks")
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn("uncommitted", result.stderr)

    def test_release_rejects_ambiguous_version_variable(self):
        result = self.run_command("make", "-n", "release", "VERSION=v9.8.7")
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn("RELEASE_VERSION", result.stderr)

    def test_preflight_stops_if_cleanliness_check_fails(self):
        self.stub(
            "git",
            'case "$1" in status) exit 42;; rev-parse) echo main;; esac\n',
        )
        result = self.release_function("preflight_checks")
        self.assertEqual(result.returncode, 42, result.stdout + result.stderr)

    def test_resolved_version_is_forwarded_to_aur(self):
        self.stub("make", 'printf "%s\\n" "$@" > make-args\n')
        result = self.release_function("VERSION=v9.8.7; publish_aur")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(
            "RELEASE_VERSION=v9.8.7", (self.root / "make-args").read_text()
        )

    def test_new_version_does_not_prompt_for_pkgrel(self):
        result = self.aur_update()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn("Increment pkgrel", result.stdout)
        self.assertIn("Version changed: 0.2.9 -> 0.2.10", result.stdout)
        pkg = (self.root / "published/PKGBUILD").read_text()
        self.assertIn("pkgrel=1\n", pkg)
        self.assertIn("sha256sums=('" + "0" * 63 + "1')", pkg)
        for name in ("PKGBUILD", ".SRCINFO"):
            self.assertEqual(
                (self.root / "aur" / name).read_text(),
                (self.root / "published" / name).read_text(),
            )

    def test_same_version_preserves_or_increments_existing_pkgrel(self):
        for answer, expected in (("n", 7), ("y", 8)):
            with self.subTest(answer=answer):
                path = self.root / "published/PKGBUILD"
                path.write_text(
                    (self.root / "aur/PKGBUILD")
                    .read_text()
                    .replace("pkgrel=1", "pkgrel=7")
                )
                result = self.aur_update(input=answer + "\n")
                self.assertEqual(
                    result.returncode, 0, result.stdout + result.stderr
                )
                self.assertIn(f"pkgrel={expected}\n", path.read_text())

    def test_checksum_failure_never_overwrites_canonical_metadata(self):
        for body in (
            "exit 42\n",
            "echo \"sha256sums=('')\"\n",
            "echo \"sha256sums=('not-a-checksum')\"\n",
        ):
            with self.subTest(body=body):
                before = {
                    name: (self.root / "aur" / name).read_bytes()
                    for name in ("PKGBUILD", ".SRCINFO")
                }
                self.stub(
                    "makepkg",
                    'case "$1" in\n-g) '
                    + body
                    + ";;\n--printsrcinfo) cat PKGBUILD;;\nesac\n",
                )
                result = self.run_command(
                    "make",
                    "aur-publish",
                    f"AUR_DIR={self.root / 'published'}",
                    "RELEASE_VERSION=v0.2.10",
                    input="n\n",
                )
                self.assertNotEqual(result.returncode, 0, result.stdout)
                calls = (self.root / "published/git-calls").read_text()
                self.assertNotIn("push", calls)
                self.assertNotIn("commit", calls)
                for name, data in before.items():
                    self.assertEqual(
                        (self.root / "aur" / name).read_bytes(), data
                    )

    def test_srcinfo_failure_does_not_publish_partial_metadata(self):
        self.stub(
            "makepkg",
            """case "$1" in
  -g) printf "sha256sums=('%064d')\\n" 1;;
  --printsrcinfo) echo partial; exit 42;;
esac
""",
        )
        before = (self.root / "aur/.SRCINFO").read_bytes()
        result = self.aur_update()
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertEqual((self.root / "aur/.SRCINFO").read_bytes(), before)
        self.assertEqual(
            (self.root / "published/.SRCINFO").read_text(),
            "existing metadata\n",
        )
        self.assertFalse((self.root / "published/.SRCINFO.tmp").exists())


if __name__ == "__main__":
    unittest.main()
