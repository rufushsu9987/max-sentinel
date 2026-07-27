#!/usr/bin/env python3
"""Validate non-Go project assets without requiring Docker or network access."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]


def load_yaml(path: Path, *, multi: bool = False) -> None:
    with path.open("r", encoding="utf-8") as handle:
        docs = list(yaml.safe_load_all(handle)) if multi else [yaml.safe_load(handle)]
    if not docs or all(doc is None for doc in docs):
        raise ValueError(f"{path.relative_to(ROOT)} has no YAML documents")


def load_json(path: Path) -> None:
    with path.open("r", encoding="utf-8") as handle:
        json.load(handle)


def validate_markdown_links() -> None:
    pattern = re.compile(r"\[[^\]]+\]\((?!https?://|mailto:|#)([^)]+)\)")
    for path in [ROOT / "README.md", *sorted((ROOT / "docs").glob("*.md"))]:
        text = path.read_text(encoding="utf-8")
        for target in pattern.findall(text):
            clean = target.split("#", 1)[0]
            if not clean:
                continue
            resolved = (path.parent / clean).resolve()
            if not resolved.exists():
                raise FileNotFoundError(
                    f"Broken relative link in {path.relative_to(ROOT)}: {target}"
                )


def main() -> int:
    load_yaml(ROOT / "deploy/docker-compose.yml")
    load_yaml(ROOT / "deploy/k8s/all.yaml", multi=True)
    load_yaml(ROOT / "deploy/prometheus/prometheus.yml")
    load_yaml(ROOT / "deploy/grafana/provisioning/datasources/prometheus.yml")
    load_yaml(ROOT / "deploy/grafana/provisioning/dashboards/default.yml")
    load_yaml(ROOT / "docs/openapi.yaml")
    load_json(ROOT / "deploy/grafana/dashboards/max-sentinel.json")
    validate_markdown_links()
    print("Validated YAML, JSON, and relative Markdown links.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # noqa: BLE001 - CLI should report every validation failure.
        print(f"validation failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
