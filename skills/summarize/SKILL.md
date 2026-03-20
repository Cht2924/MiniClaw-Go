---
name: summarize
description: 对 URL、本地文件、播客或 YouTube 链接做摘要，或尽力提取文本/字幕。适用于“总结这篇文章”“概括这个链接”“提取这个视频内容”之类的请求。
homepage: https://summarize.sh
metadata: {"nanobot":{"emoji":"🧾","requires":{"bins":["summarize"]},"install":[{"id":"brew","kind":"brew","formula":"steipete/tap/summarize","bins":["summarize"],"label":"安装 summarize（brew）"}]}}
---

# 摘要技能

这是一个快速摘要 URL、本地文件和 YouTube 链接的 CLI 工具。

## 何时使用

当用户提出下面这类请求时，优先考虑这个技能：

- “用 summarize.sh”
- “这个链接 / 视频讲了什么？”
- “帮我总结这篇文章”
- “帮我转写这个 YouTube / 视频”

## 快速开始

```bash
summarize "https://example.com" --model google/gemini-3-flash-preview
summarize "/path/to/file.pdf" --model google/gemini-3-flash-preview
summarize "https://youtu.be/dQw4w9WgXcQ" --youtube auto
```

## YouTube：摘要与字幕

尽力提取字幕：

```bash
summarize "https://youtu.be/dQw4w9WgXcQ" --youtube auto --extract-only
```

如果用户要完整转写，但内容特别长，先给一版紧凑摘要，再询问要展开的片段或时间范围。

## 模型与密钥

根据你选择的提供商设置环境变量：

- OpenAI：`OPENAI_API_KEY`
- Anthropic：`ANTHROPIC_API_KEY`
- xAI：`XAI_API_KEY`
- Google：`GEMINI_API_KEY`

如果没有显式指定模型，默认使用 `google/gemini-3-flash-preview`。

## 常用参数

- `--length short|medium|long|xl|xxl|<chars>`
- `--max-output-tokens <count>`
- `--extract-only`
- `--json`
- `--firecrawl auto|off|always`
- `--youtube auto`

## 配置文件

可选配置文件：`~/.summarize/config.json`

```json
{ "model": "openai/gpt-5.4" }
```

## 补充说明

- `FIRECRAWL_API_KEY` 可用于被限制抓取的网站。
- `APIFY_API_TOKEN` 可作为 YouTube 的备用提取方式。
