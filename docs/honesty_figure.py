"""Render the honesty-experiment figure in docs/honesty.png.

Run it with the plotting dependencies available, for example:

    uv run --with matplotlib python docs/honesty_figure.py

The numbers are a pilot run of cmd/study at n=100 per condition and epsilon 3.
"""

import os

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np

plt.rcParams.update(
    {
        "font.family": "serif",
        "font.size": 9,
        "axes.spines.top": False,
        "axes.spines.right": False,
        "axes.linewidth": 0.8,
    }
)

PUBLIC = "#E69F00"
LDP = "#0072B2"
models = ["Haiku 4.5", "Qwen3-32B", "Gemma 3", "Ministral", "Nemotron"]

# (panel, public/attributed yes-rate, local-DP de-biased yes-rate).
panels = [
    ("(a)", [0, 67, 100, 79, 35], [84, 92, 101, 82, 32]),
    ("(b)", [0, 43, 100, 33, 3], [98, 41, 100, 28, 11]),
    ("(c)", [100, 100, 100, 70, 9], [101, 95, 100, 57, 14]),
    ("(d)", [0, 1, 0, 3, 2], [-1, 10, -1, 1, 2]),
]


def clamp(values: list[int]) -> list[int]:
    return [min(100, max(0, v)) for v in values]


x = np.arange(len(models))
width = 0.38

fig, axes = plt.subplots(2, 2, figsize=(7.4, 5.2), sharey=True)
for ax, (label, public, private) in zip(axes.flat, panels):
    left = ax.bar(x - width / 2, clamp(public), width, color=PUBLIC, edgecolor="black", linewidth=0.4, label="Public")
    right = ax.bar(x + width / 2, clamp(private), width, color=LDP, edgecolor="black", linewidth=0.4, label="Local DP")
    ax.set_xticks(x)
    ax.set_xticklabels(models, rotation=30, ha="right", fontsize=8)
    ax.set_ylim(0, 108)
    ax.set_yticks([0, 25, 50, 75, 100])
    ax.grid(axis="y", linewidth=0.4, alpha=0.35)
    ax.set_axisbelow(True)
    ax.text(-0.02, 1.06, label, transform=ax.transAxes, fontweight="bold", fontsize=12)
    ax.bar_label(left, fmt="%d", padding=1, fontsize=6)
    ax.bar_label(right, fmt="%d", padding=1, fontsize=6)

for ax in axes[:, 0]:
    ax.set_ylabel("answered “yes” (%)")

handles, labels = axes.flat[0].get_legend_handles_labels()
fig.legend(handles, labels, loc="upper center", ncol=2, frameon=False, bbox_to_anchor=(0.5, 1.01))
fig.tight_layout(rect=[0, 0, 1, 0.96])
fig.savefig(os.path.join(os.path.dirname(__file__), "honesty.png"), dpi=200, bbox_inches="tight")
