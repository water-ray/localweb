# 脚本目录说明

本目录用于统一存放项目脚本。

## 组织方式

- 默认使用 Python 编写脚本。
- 按项目类型、用途或阶段再分一级目录，例如：
  - `scripts/git/`
  - `scripts/build/`
  - `scripts/dev/`
  - `scripts/release/`
  - `scripts/tools/`
  - `scripts/validate/`
  - `scripts/project/`

## 约定

- 不要把脚本散落在仓库根目录或源码目录。
- 脚本运行产生的临时文件写入 `temp/`。
- 构建、打包后的最终产物输出到 `Bin/`。

## 当前脚本能力

- `scripts/build/build_localweb.py`
  - 创建 `Bin/localweb/` 输出目录，并调用 `go build` 交叉编译 Ubuntu/Linux amd64 版本 LocalWeb。
  - 输出 `Bin/localweb/localweb`。
  - 同步复制根目录 `config.example.json` 到 `Bin/localweb/config.example.json`。
- `scripts/git/repo_tasks.py`
  - 作为 `.vscode/tasks.json` 中仓库任务的统一入口。
  - 提供分支查看、切换/创建、提交、推送、拉取远程分支和远程源设置等功能。
  - 推送和拉取任务支持时间显示、实时输出，以及必要时的交互确认。
- `scripts/git/test_repo_tasks.py`
  - 使用临时 Git 仓库验证仓库任务脚本的关键路径。
- `scripts/validate/check_project_rules.py`
  - 检查基础文件、任务 JSON、skill frontmatter、skill 命名和 agent prompt 引用。
- `scripts/project/init_project.py`
  - 从当前仓库复制基础规则框架到目标目录，并可生成项目 skill 骨架。
