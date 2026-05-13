from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("repo_tasks.py").resolve()


class RepoTasksTest(unittest.TestCase):
    def setUp(self) -> None:
        self.git_exe = shutil.which("git")
        if not self.git_exe:
            self.skipTest("未找到 git，跳过仓库任务测试。")
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def run_script(
        self,
        *args: str,
        cwd: Path | None = None,
        user_input: str | None = None,
    ) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, str(SCRIPT), *args],
            cwd=str(cwd or self.root),
            input=user_input,
            text=True,
            encoding="utf-8",
            errors="replace",
            capture_output=True,
            check=False,
        )

    def git(self, *args: str, cwd: Path | None = None) -> str:
        result = subprocess.run(
            [self.git_exe, *args],
            cwd=str(cwd or self.root),
            text=True,
            encoding="utf-8",
            errors="replace",
            capture_output=True,
            check=False,
        )
        if result.returncode != 0:
            self.fail(result.stderr or result.stdout)
        return result.stdout.strip()

    def init_repo(self) -> None:
        self.git("init")
        self.git("branch", "-M", "main")
        self.git("config", "user.name", "Codex Test")
        self.git("config", "user.email", "codex-test@example.com")
        (self.root / "README.md").write_text("# test\n", encoding="utf-8")
        self.git("add", "README.md")
        self.git("commit", "-m", "初始化测试仓库")

    def test_status_fails_outside_git_repo(self) -> None:
        result = self.run_script("status")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("当前目录不是 Git 仓库", result.stderr)

    def test_switch_requires_confirmation_without_yes_in_noninteractive_mode(self) -> None:
        self.init_repo()
        (self.root / "note.txt").write_text("changed\n", encoding="utf-8")
        result = self.run_script("switch", "--branch", "feature", user_input="")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("无法确认自动提交", result.stderr)
        self.assertEqual(self.git("branch", "--show-current"), "main")

    def test_switch_yes_auto_commits_before_switching(self) -> None:
        self.init_repo()
        (self.root / "note.txt").write_text("changed\n", encoding="utf-8")
        result = self.run_script("switch", "--branch", "feature", "--yes")
        self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
        self.assertEqual(self.git("branch", "--show-current"), "feature")
        self.assertIn("切换分支为feature前的自动提交", self.git("log", "-1", "--format=%s"))

    def test_push_without_configured_remote_fails_cleanly(self) -> None:
        self.init_repo()
        result = self.run_script("push")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("未设置远程源", result.stderr)

    def test_restore_node_enter_cancels(self) -> None:
        self.init_repo()
        (self.root / "note.txt").write_text("second\n", encoding="utf-8")
        self.git("add", "note.txt")
        self.git("commit", "-m", "第二次提交")
        result = self.run_script("restore-node", user_input="\n")
        self.assertEqual(result.returncode, 0, result.stderr or result.stdout)
        self.assertIn("已取消还原节点任务", result.stdout)


if __name__ == "__main__":
    unittest.main()
