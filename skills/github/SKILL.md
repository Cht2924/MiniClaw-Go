---
name: github
description: "使用 `gh` CLI 与 GitHub 交互。适用于查看仓库、Issue、Pull Request、CI 运行记录，以及需要 `gh api` 做高级查询的场景。"
metadata: {"nanobot":{"emoji":"🐙","requires":{"bins":["gh"]},"install":[{"id":"brew","kind":"brew","formula":"gh","bins":["gh"],"label":"安装 GitHub CLI（brew）"},{"id":"apt","kind":"apt","package":"gh","bins":["gh"],"label":"安装 GitHub CLI（apt）"}]}}
---

# GitHub 技能

使用 `gh` CLI 与 GitHub 交互。如果当前目录不是一个 Git 仓库，请始终显式传入 `--repo owner/repo`，或者直接使用仓库 URL。

## 常用 Pull Request 操作

查看某个 PR 的 CI 状态：

```bash
gh pr checks 55 --repo owner/repo
```

拉取最近 3 个 PR：

```bash
gh pr list --repo owner/repo --limit 3
```

查看最近 3 个 Issue：

```bash
gh issue list --repo owner/repo --limit 3
```

查看最近工作流运行记录：

```bash
gh run list --repo owner/repo --limit 10
```

查看某次运行并定位失败步骤：

```bash
gh run view <run-id> --repo owner/repo
```

只看失败步骤日志：

```bash
gh run view <run-id> --repo owner/repo --log-failed
```

## 用 `gh api` 做高级查询

`gh api` 适合获取子命令没有直接暴露的数据。

查询指定 PR 的关键字段：

```bash
gh api repos/owner/repo/pulls/55 --jq '.title, .state, .user.login'
```

只取前 3 条仓库数据：

```bash
gh api users/owner/repos --jq '.[0:3] | .[] | {name, stargazers_count, html_url}'
```

## JSON 输出

大多数 `gh` 子命令都支持 `--json`，可以配合 `--jq` 做结构化筛选：

```bash
gh issue list --repo owner/repo --json number,title --jq '.[] | "\(.number): \(.title)"'
```

## 使用建议

- 需要稳定脚本输出时，优先加上 `--json` 或 `--jq`。
- 只需要少量结果时，用 `--limit 3`、`--limit 5` 之类的限制数量，避免拉太多无关信息。
- 如果用户没有给仓库名，不要猜，先根据当前目录或明确上下文确认目标仓库。
