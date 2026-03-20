---
name: tmux
description: 通过发送按键、抓取 pane 输出等方式远程控制 tmux 会话。适用于需要交互式 TTY、持续观察命令输出或并行跑多个代理任务的场景。
metadata: {"nanobot":{"emoji":"🧵","os":["darwin","linux"],"requires":{"bins":["tmux"]}}}
---

# tmux 技能

只有在确实需要交互式 TTY 时再使用 tmux。对于长时间运行但不需要交互的任务，优先考虑普通后台执行模式。

## 快速开始

```bash
SOCKET_DIR="${NANOBOT_TMUX_SOCKET_DIR:-${TMPDIR:-/tmp}/nanobot-tmux-sockets}"
mkdir -p "$SOCKET_DIR"
SOCKET="$SOCKET_DIR/nanobot.sock"
SESSION=nanobot-python

tmux -S "$SOCKET" new -d -s "$SESSION" -n shell
tmux -S "$SOCKET" send-keys -t "$SESSION":0.0 -- 'PYTHON_BASIC_REPL=1 python3 -q' Enter
tmux -S "$SOCKET" capture-pane -p -J -t "$SESSION":0.0 -S -200
```

启动后，记得把监控命令打印出来：

```text
用于观察：
  tmux -S "$SOCKET" attach -t "$SESSION"
  tmux -S "$SOCKET" capture-pane -p -J -t "$SESSION":0.0 -S -200
```

## Socket 约定

- 使用环境变量 `NANOBOT_TMUX_SOCKET_DIR`
- 默认 socket 路径是 `"$NANOBOT_TMUX_SOCKET_DIR/nanobot.sock"`

## Pane 定位与命名

- 目标格式：`session:window.pane`
- 名称尽量短，不要带空格
- 可用以下命令排查：

```bash
tmux -S "$SOCKET" list-sessions
tmux -S "$SOCKET" list-panes -a
```

## 查找会话

- 查看当前 socket 上的会话：`{baseDir}/scripts/find-sessions.sh -S "$SOCKET"`
- 扫描所有 socket：`{baseDir}/scripts/find-sessions.sh --all`

## 安全发送输入

- 优先用字面发送：

```bash
tmux -S "$SOCKET" send-keys -t target -l -- "$cmd"
```

- 发送控制键：

```bash
tmux -S "$SOCKET" send-keys -t target C-c
```

## 观察输出

- 抓取最近输出：

```bash
tmux -S "$SOCKET" capture-pane -p -J -t target -S -200
```

- 等待提示词或关键文本：

```bash
{baseDir}/scripts/wait-for-text.sh -t session:0.0 -p 'pattern'
```

## Python REPL

如果要驱动 Python REPL，请设置 `PYTHON_BASIC_REPL=1`，否则某些高级 REPL 会让 `send-keys` 流程变得不稳定。

## Windows / WSL

- tmux 主要支持 macOS / Linux
- 在 Windows 上请通过 WSL 使用，并在 WSL 内安装 tmux

## 并行代理场景

tmux 很适合同时跑多个编码代理：

```bash
SOCKET="${TMPDIR:-/tmp}/codex-army.sock"

for i in 1 2 3; do
  tmux -S "$SOCKET" new-session -d -s "agent-$i"
done

tmux -S "$SOCKET" send-keys -t agent-1 "cd /tmp/project1 && codex --yolo '修复 bug X'" Enter
tmux -S "$SOCKET" send-keys -t agent-2 "cd /tmp/project2 && codex --yolo '修复 bug Y'" Enter
```

## 清理

- 关闭单个会话：`tmux -S "$SOCKET" kill-session -t "$SESSION"`
- 关闭整个 socket：`tmux -S "$SOCKET" kill-server`
