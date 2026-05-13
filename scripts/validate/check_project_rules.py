from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


SKILL_NAME_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


class CheckResult:
    def __init__(self) -> None:
        self.errors: list[str] = []
        self.warnings: list[str] = []

    def error(self, message: str) -> None:
        self.errors.append(message)

    def warn(self, message: str) -> None:
        self.warnings.append(message)

    def print(self) -> None:
        for message in self.errors:
            print(f"[FAIL] {message}")
        for message in self.warnings:
            print(f"[WARN] {message}")
        if not self.errors:
            print("[OK] 项目规则框架检查通过。")

    @property
    def exit_code(self) -> int:
        return 1 if self.errors else 0


def read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def parse_frontmatter(path: Path, result: CheckResult) -> dict[str, str]:
    text = read_text(path)
    if not text.startswith("---"):
        result.error(f"{path} 缺少 YAML frontmatter。")
        return {}

    end = text.find("\n---", 3)
    if end == -1:
        result.error(f"{path} frontmatter 未正确闭合。")
        return {}

    fields: dict[str, str] = {}
    for raw_line in text[3:end].splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if ":" not in line:
            result.error(f"{path} frontmatter 包含无法解析的行：{raw_line}")
            continue
        key, value = line.split(":", 1)
        fields[key.strip()] = value.strip().strip('"').strip("'")
    return fields


def check_required_paths(root: Path, result: CheckResult) -> None:
    required_files = [
        "AGENTS.md",
        "README.md",
        ".gitignore",
        ".gitattributes",
        ".vscode/tasks.json",
        "scripts/git/repo_tasks.py",
    ]
    required_dirs = [
        ".codex/skills",
        "docs",
        "scripts",
    ]

    for relative in required_files:
        path = root / relative
        if not path.is_file():
            result.error(f"缺少必要文件：{relative}")

    for relative in required_dirs:
        path = root / relative
        if not path.is_dir():
            result.error(f"缺少必要目录：{relative}")

    for relative in ("Bin", "temp"):
        path = root / relative
        if not path.exists():
            result.warn(f"输出目录不存在：{relative}。如需保留空目录，可添加 .gitkeep。")


def check_tasks_json(root: Path, result: CheckResult) -> None:
    path = root / ".vscode/tasks.json"
    if not path.exists():
        return
    try:
        data = json.loads(read_text(path))
    except json.JSONDecodeError as error:
        result.error(f".vscode/tasks.json 不是合法 JSON：{error}")
        return

    labels = [
        task.get("label")
        for task in data.get("tasks", [])
        if isinstance(task, dict) and task.get("label")
    ]
    if len(labels) != len(set(labels)):
        result.error(".vscode/tasks.json 存在重复任务 label。")
    if not labels:
        result.warn(".vscode/tasks.json 未定义任务。")


def check_skill(root: Path, skill_dir: Path, result: CheckResult) -> None:
    skill_file = skill_dir / "SKILL.md"
    if not skill_file.is_file():
        result.error(f"{skill_dir.relative_to(root)} 缺少 SKILL.md。")
        return

    fields = parse_frontmatter(skill_file, result)
    name = fields.get("name", "")
    description = fields.get("description", "")
    expected_name = skill_dir.name

    if not name:
        result.error(f"{skill_file.relative_to(root)} 缺少 name。")
    elif name != expected_name:
        result.error(
            f"{skill_file.relative_to(root)} 的 name={name} 与目录名 {expected_name} 不一致。"
        )
    elif not SKILL_NAME_PATTERN.fullmatch(name):
        result.error(f"{skill_file.relative_to(root)} 的 name 不符合小写连字符命名：{name}")

    if not description:
        result.error(f"{skill_file.relative_to(root)} 缺少 description。")

    agent_file = skill_dir / "agents/openai.yaml"
    if agent_file.exists() and name:
        agent_text = read_text(agent_file)
        if f"${name}" not in agent_text:
            result.error(f"{agent_file.relative_to(root)} 的 default_prompt 未包含 ${name}。")


def check_skills(root: Path, result: CheckResult) -> None:
    skills_root = root / ".codex/skills"
    if not skills_root.exists():
        return

    skill_dirs = [
        path
        for path in skills_root.iterdir()
        if path.is_dir() and not path.name.startswith(".")
    ]
    if not skill_dirs:
        result.warn(".codex/skills 下没有 skill 目录。")
        return

    for skill_dir in sorted(skill_dirs):
        check_skill(root, skill_dir, result)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="检查 Codex 项目规则与 skill 框架。")
    parser.add_argument(
        "--root",
        default=".",
        help="要检查的项目根目录，默认当前目录。",
    )
    return parser


def main() -> int:
    args = build_parser().parse_args()
    root = Path(args.root).resolve()
    result = CheckResult()

    if not root.exists():
        result.error(f"项目根目录不存在：{root}")
        result.print()
        return result.exit_code

    check_required_paths(root, result)
    check_tasks_json(root, result)
    check_skills(root, result)
    result.print()
    return result.exit_code


if __name__ == "__main__":
    raise SystemExit(main())
