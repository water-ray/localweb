# Codex 规则与 Skill 说明

本项目把规则分为两层：

- `AGENTS.md`：始终生效的项目级硬约束。
- `.codex/skills/`：按场景读取的可迁移工作流。

## 已内置的 Skills

- `codex-project-foundation`：用于初始化和维护 Codex 基础项目框架。
- `codex-skill-generator`：用于生成项目 skill、功能 skill 和任务 skill。
- `repo-git-workflow`：用于执行 Git/GitHub 提交、推送、拉取和远程配置任务。

## 新项目迁移

1. 复制 `AGENTS.md`、`.codex/skills/`、`docs/`、`scripts/`、`.vscode/tasks.json` 和 `.gitignore`。
2. 按新项目技术栈添加源码、配置和测试目录。
3. 使用 `codex-skill-generator` 为新项目生成项目 skill。
4. 对稳定功能域继续生成独立功能 skill。
5. 如果需要跨项目或跨设备全局触发，把对应 skill 目录同步到 `$CODEX_HOME/skills/`。

## 添加子项目或功能

- 子项目有独立构建、运行或测试流程时，创建项目 skill。
- 功能有稳定修改路径、专门业务规则或重复验证步骤时，创建功能 skill。
- 一次性临时需求不写入 skill；只沉淀可复用流程。

## 验证

修改 skill 后优先运行：

```powershell
py -3 "$env:USERPROFILE\.codex\skills\.system\skill-creator\scripts\quick_validate.py" ".codex\skills\<skill-name>"
```
