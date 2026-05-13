from __future__ import annotations

import argparse
import re
import shutil
import sys
from pathlib import Path


SLUG_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
COPY_ITEMS = [
    "AGENTS.md",
    "README.md",
    ".gitignore",
    ".gitattributes",
    ".codex",
    ".vscode/tasks.json",
    "docs",
    "scripts",
]


def repository_root() -> Path:
    return Path(__file__).resolve().parents[2]


def validate_slug(value: str, label: str) -> None:
    if value and not SLUG_PATTERN.fullmatch(value):
        raise ValueError(f"{label} 只能使用小写字母、数字和连字符：{value}")


def copy_item(source: Path, target: Path, *, overwrite: bool) -> None:
    if not source.exists():
        return

    if target.exists():
        if not overwrite:
            raise FileExistsError(f"目标已存在：{target}")
        if target.is_dir():
            shutil.rmtree(target)
        else:
            target.unlink()

    if source.is_dir():
        ignore = shutil.ignore_patterns(
            "__pycache__",
            "*.pyc",
            ".pytest_cache",
            "temp",
            "Bin",
        )
        shutil.copytree(source, target, ignore=ignore)
    else:
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, target)


def write_project_readme(target_dir: Path, project_name: str) -> None:
    readme = target_dir / "README.md"
    text = (
        f"# {project_name}\n\n"
        "本项目由基础 AI 规则与 Codex skill 框架初始化。\n\n"
        "## 开发规则\n\n"
        "- `AGENTS.md`：始终生效的项目级约束。\n"
        "- `.codex/skills/`：按项目、功能或任务触发的开发规范。\n"
        "- `docs/`：项目说明、设计和使用文档。\n"
    )
    readme.write_text(text, encoding="utf-8")


def write_project_skill(target_dir: Path, project_slug: str, project_name: str) -> None:
    skill_dir = target_dir / ".codex/skills" / f"{project_slug}-project"
    skill_dir.mkdir(parents=True, exist_ok=True)
    skill_file = skill_dir / "SKILL.md"
    if skill_file.exists():
        return

    text = f"""---
name: {project_slug}-project
description: 维护 {project_name} 的 Codex 项目上下文。用于修改项目结构、添加模块、运行验证、更新文档、处理构建/测试/发布流程，以及需要理解该项目目录和长期约束的任务。
---

# {project_name} Project

## 技术栈

- 当前状态：待确认。
- 确认技术栈后，记录主要语言、框架、包管理器、运行命令、构建命令和测试命令。

## 入口

- 项目说明：`README.md`。
- 项目文档：`docs/`。
- 项目脚本：`scripts/`。
- 临时输出：`temp/`。
- 最终产物：`Bin/{project_slug}/`。

## 工作流

1. 先读取 `AGENTS.md`、`README.md` 和本 skill。
2. 按实际技术栈读取源码、配置、脚本和测试入口。
3. 按项目既有风格修改，保持范围最小。
4. 执行最小必要验证。
5. 新增稳定功能或复杂工作流时，同步创建或更新功能 skill 或任务 skill。

## 约束

- 派生项目的 `README.md` 可自由改写；开发规范以 `AGENTS.md` 和 `.codex/skills/` 为准。
- 新增子项目时，创建或更新对应项目 skill。
"""
    skill_file.write_text(text, encoding="utf-8")


def ensure_output_dirs(target_dir: Path) -> None:
    for relative in ("Bin", "temp", "docs/tech-stack"):
        path = target_dir / relative
        path.mkdir(parents=True, exist_ok=True)
        keep = path / ".gitkeep"
        if not keep.exists():
            keep.write_text("", encoding="utf-8")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="从当前仓库初始化一个 Codex 规则基础项目。")
    parser.add_argument("target", help="目标项目目录。")
    parser.add_argument("--project-name", default="", help="项目显示名称。")
    parser.add_argument("--project-slug", default="", help="项目 slug，小写字母、数字和连字符。")
    parser.add_argument(
        "--overwrite",
        action="store_true",
        help="覆盖目标目录中同名框架文件或目录。",
    )
    return parser


def main() -> int:
    args = build_parser().parse_args()
    source_root = repository_root()
    target_dir = Path(args.target).resolve()
    project_slug = args.project_slug.strip()
    project_name = args.project_name.strip() or project_slug or target_dir.name

    try:
        validate_slug(project_slug, "项目 slug")
        target_dir.mkdir(parents=True, exist_ok=True)
        for item in COPY_ITEMS:
            copy_item(source_root / item, target_dir / item, overwrite=args.overwrite)
        ensure_output_dirs(target_dir)
        if args.project_name.strip():
            write_project_readme(target_dir, project_name)
        if project_slug:
            write_project_skill(target_dir, project_slug, project_name)
    except (OSError, ValueError) as error:
        print(f"[FAIL] {error}", file=sys.stderr)
        return 1

    print(f"[OK] 已初始化项目框架：{target_dir}")
    if project_slug:
        print(f"[OK] 已准备项目 skill：.codex/skills/{project_slug}-project/SKILL.md")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
