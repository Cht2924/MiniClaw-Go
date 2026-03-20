---
name: weather
description: 获取当前天气和天气预报，并优先验证地理位置匹配是否正确。适用于城市名模糊、中文地名、或需要结构化天气数据的场景。
homepage: https://wttr.in/:help
metadata: {"nanobot":{"emoji":"🌦️","requires":{"bins":["curl"]}}}
---

# 天气技能

先确认地点，再报告天气。对于中文城市名或非拉丁字符输入，优先使用 `wttr.in` 原始查询，因为它对本地名称的解析通常更稳；只有在位置确认后，再考虑用 Open-Meteo 拉结构化数据。

## 准确性规则

- 最终回答里要明确写出匹配到的地点、国家/地区和观测时间。
- 不要盲信第一条地理编码结果，要检查 `country`、`admin1`、`admin2`、`population`。
- 中文城市名不要直接默认交给 Open-Meteo 的 geocoding，除非结果明显正确。
- 如果仍然存在多个合理候选地点，要么追问用户，要么明确写出你的假设。
- 调 Open-Meteo 时用 `timezone=auto`，保证时间和地点一致。

## wttr.in

快速查询当前天气：

```bash
curl -s "https://wttr.in/London?format=%l:+%c+%t+%h+%w"
```

中文城市示例：

```bash
curl -s "https://wttr.in/%E6%88%90%E9%83%BD?format=%l:+%c+%t+%h+%w"
curl -s "https://wttr.in/%E4%B8%8A%E6%B5%B7?format=%l:+%c+%t+%h+%w"
```

需要更多结构化信息时：

```bash
curl -s "https://wttr.in/Chengdu?format=j1"
```

## Open-Meteo

先做地理编码并校验地点：

```bash
curl -s "https://geocoding-api.open-meteo.com/v1/search?name=Chengdu&count=3&language=en&format=json"
```

确认坐标后再拉天气：

```bash
curl -s "https://api.open-meteo.com/v1/forecast?latitude=30.66667&longitude=104.06667&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m&daily=weather_code,temperature_2m_max,temperature_2m_min&forecast_days=1&timezone=auto"
```

## 处理建议

- 若 geocoding 结果可疑，优先退回到 `wttr.in` 的原始城市名查询。
- 如果用户只需要一句话天气，没必要返回完整 JSON。
- 如果需要“今天”和“未来几天”并列展示，优先组织成结构化摘要。
