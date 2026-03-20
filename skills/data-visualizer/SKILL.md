---
name: data-visualizer
description: 使用 Python 图表渲染工具生成柱状图或饼图，并把图片嵌入 Markdown 文档。适用于“把数据做成图表”“生成柱状图/饼图”“把统计结果可视化并写入报告”等任务。
---

# 数据可视化技能

当用户希望把结构化数据做成图表时，优先使用这个技能。

## 适用场景

- 把统计结果做成柱状图
- 把占比数据做成饼图
- 把图表嵌入 Markdown 报告
- 为测试报告、分析报告、演示文档补充可视化图片

## 工具偏好

优先使用：

- `visualize_chart`
- `write_file`
- `read_file`

必要时可配合：

- `search_web`
- `fetch_url`
- `mcp.github.*`

## 标准流程

1. 先整理出干净的数据：
   - `labels`
   - `values`
   - 图表标题
   - 输出路径
2. 根据数据关系选择图表类型：
   - 比较不同项的数值差异：用 `bar`
   - 展示整体占比：用 `pie`
3. 调用 `visualize_chart` 生成 PNG 文件，建议输出到 `workspace/charts/`
4. 在 Markdown 中使用图片语法嵌入：

```markdown
![图表说明](charts/example.png)
```

5. 如果报告里需要两张图，通常可以同时给：
   - 一张柱状图
   - 一张饼图

## 输出建议

- 图表文件命名清晰，例如：
  - `workspace/charts/github_top3_bar.png`
  - `workspace/charts/github_top3_pie.png`
- 报告里紧跟图片后写一句解释图表表达的含义
- 不要只给图，不写图表结论

## 示例

### 柱状图

```json
{
  "chart_type": "bar",
  "title": "GitHub Top 3 Stars",
  "labels": ["servers", "modelcontextprotocol", "python-sdk"],
  "values": [81568, 7555, 22224],
  "output_path": "workspace/charts/github_top3_bar.png",
  "x_label": "Repository",
  "y_label": "Stars"
}
```

### 饼图

```json
{
  "chart_type": "pie",
  "title": "GitHub Top 3 Stars Share",
  "labels": ["servers", "modelcontextprotocol", "python-sdk"],
  "values": [81568, 7555, 22224],
  "output_path": "workspace/charts/github_top3_pie.png"
}
```

## 注意事项

- `labels` 和 `values` 数量必须一致
- 饼图的 `values` 之和必须大于 0
- 图表输出路径必须在允许目录内
- 如果用户要的是“真实渲染图”，不要只写 Mermaid，优先生成 PNG
