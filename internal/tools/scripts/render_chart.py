import argparse
import json
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt


def setup_style():
    plt.rcParams["font.sans-serif"] = [
        "Microsoft YaHei",
        "SimHei",
        "Arial Unicode MS",
        "DejaVu Sans",
    ]
    plt.rcParams["axes.unicode_minus"] = False


def render_bar(payload):
    labels = payload["labels"]
    values = payload["values"]
    title = payload["title"]
    output = Path(payload["output_path"])
    x_label = payload.get("x_label", "")
    y_label = payload.get("y_label", "")

    fig, ax = plt.subplots(figsize=(10, 6), dpi=160)
    colors = ["#c46b1a", "#0f766e", "#2563eb", "#7c3aed", "#dc2626", "#0ea5e9"]
    bars = ax.bar(labels, values, color=colors[: len(labels)], width=0.58)
    ax.set_title(title, fontsize=18, pad=16)
    if x_label:
        ax.set_xlabel(x_label)
    if y_label:
        ax.set_ylabel(y_label)
    ax.grid(axis="y", linestyle="--", alpha=0.25)
    ax.set_axisbelow(True)

    max_value = max(values) if values else 0
    offset = max_value * 0.01 if max_value > 0 else 0.05
    for bar, value in zip(bars, values):
        ax.text(
            bar.get_x() + bar.get_width() / 2,
            value + offset,
            f"{value:,.2f}".rstrip("0").rstrip("."),
            ha="center",
            va="bottom",
            fontsize=11,
        )

    fig.tight_layout()
    output.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output, bbox_inches="tight")
    plt.close(fig)


def render_pie(payload):
    labels = payload["labels"]
    values = payload["values"]
    title = payload["title"]
    output = Path(payload["output_path"])

    fig, ax = plt.subplots(figsize=(8.6, 6.8), dpi=160)
    colors = ["#c46b1a", "#0f766e", "#2563eb", "#7c3aed", "#dc2626", "#0ea5e9"]
    wedges, _, _ = ax.pie(
        values,
        labels=labels,
        autopct=lambda pct: f"{pct:.1f}%",
        startangle=90,
        colors=colors[: len(labels)],
        wedgeprops={"linewidth": 1.2, "edgecolor": "white"},
        textprops={"fontsize": 11},
    )
    ax.set_title(title, fontsize=18, pad=16)
    legend_labels = []
    for label, value in zip(labels, values):
        legend_labels.append(f"{label}: {value:,.2f}".rstrip("0").rstrip("."))
    ax.legend(
        wedges,
        legend_labels,
        title="数据项",
        loc="center left",
        bbox_to_anchor=(1.02, 0.5),
        frameon=False,
    )
    fig.tight_layout()
    output.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output, bbox_inches="tight")
    plt.close(fig)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    args = parser.parse_args()

    payload = json.loads(Path(args.input).read_text(encoding="utf-8"))
    setup_style()

    chart_type = payload["chart_type"].strip().lower()
    if chart_type == "bar":
        render_bar(payload)
    elif chart_type == "pie":
        render_pie(payload)
    else:
        raise ValueError(f"unsupported chart type: {chart_type}")

    print(f"rendered {chart_type} chart to {payload['output_path']}")


if __name__ == "__main__":
    main()
