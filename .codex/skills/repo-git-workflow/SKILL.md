---
name: repo-git-workflow
description: 使用仓库内 scripts/git/repo_tasks.py 和 .vscode/tasks.json 执行 Git/GitHub 仓库任务。用于查看分支和远程、切换或创建分支、提交当前改动、推送当前或指定分支、拉取当前分支、设置远程仓库源。
---

# Repo Git Workflow

## 入口

- 脚本：`scripts/git/repo_tasks.py`
- VS Code 任务：`.vscode/tasks.json`
- 缓存：`temp/git_task_cache.json`

## 执行原则

- 执行提交或推送前，先查看当前分支、远程和改动摘要。
- 提交说明缺失时先向用户确认，不自动编造含义不明的提交信息。
- 推送远程源缺失时，脚本会按输入、缓存、当前上游、`origin` 的顺序解析。
- 强推只通过脚本内交互确认执行；不要绕过确认直接运行 `git push --force`。
- 拉取当前分支使用普通 `git pull`，不执行覆盖式同步。

## 常用命令

查看当前分支、上游、远程和作者信息：

```powershell
py -3 scripts/git/repo_tasks.py status
```

切换或创建分支：

```powershell
py -3 scripts/git/repo_tasks.py switch --branch <branch>
```

提交当前全部改动：

```powershell
py -3 scripts/git/repo_tasks.py commit --message "<message>"
```

推送当前分支：

```powershell
py -3 scripts/git/repo_tasks.py push-current --remote <remote>
```

将当前 HEAD 推送到指定远程分支：

```powershell
py -3 scripts/git/repo_tasks.py push-branch --remote <remote> --branch <branch>
```

拉取当前分支：

```powershell
py -3 scripts/git/repo_tasks.py pull-current --remote <remote>
```

新增或更新远程源：

```powershell
py -3 scripts/git/repo_tasks.py set-remote --name <remote> --url <url>
```

## VS Code 任务

当用户明确说运行“任务”时，优先使用 `.vscode/tasks.json` 中的任务标签：

- `仓库：查看当前分支`
- `仓库：切换/创建分支`
- `仓库：提交当前改动`
- `仓库：推送当前分支`
- `仓库：推送指定分支`
- `仓库：拉取当前分支`
- `仓库：设置远程仓库源`

## 输出处理

- 脚本输出包含时间戳、开始时间、结束时间和总耗时。
- 最近远程源和目标分支写入 `temp/git_task_cache.json`，该文件属于临时缓存。
- 如果当前目录不是 Git 仓库，先说明无法执行仓库任务，不要尝试初始化仓库，除非用户明确要求。
