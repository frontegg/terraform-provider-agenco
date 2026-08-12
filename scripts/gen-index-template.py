#!/usr/bin/env python3
"""Build templates/index.md.tmpl from README.md.

The Terraform Registry renders the provider Overview page from docs/index.md, which tfplugindocs
generates from templates/index.md.tmpl — it never reads README.md. Keeping the prose in both by
hand means one of them goes stale, so the README is the single source and this derives the
template from it.

Everything from the first included heading up to EXCLUDE_FROM is carried over. Sections after that
are repo-facing (build instructions, local test scaffolding) and do not belong on the registry.
"""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
README = ROOT / "README.md"
TEMPLATE = ROOT / "templates" / "index.md.tmpl"

# The Overview starts here: everything before it is the title and the provider block, which the
# template renders itself from examples/provider/provider.tf.
INCLUDE_FROM = "## Authentication"

# Repo-facing from here on.
EXCLUDE_FROM = "## Development"

HEADER = """---
page_title: "{{ .ProviderShortName }} Provider"
description: |-
{{ .Description | plainmarkdown | trimspace | prefixlines "  " }}
---

<!-- Generated from README.md by scripts/gen-index-template.py. Edit the README, run `make docs`. -->

# {{ .ProviderShortName }} Provider

{{ .Description | trimspace }}

## Example Usage

{{ tffile .ExampleFile }}

"""

FOOTER = """
{{ .SchemaMarkdown | trimspace }}
"""


def main() -> int:
    readme = README.read_text()

    if "{{" in readme:
        print(
            "error: README.md contains '{{', which tfplugindocs would evaluate as a Go template. "
            "Escape it before generating.",
            file=sys.stderr,
        )
        return 1

    try:
        start = readme.index(INCLUDE_FROM)
        end = readme.index(EXCLUDE_FROM)
    except ValueError as missing:
        print(f"error: expected heading not found in README.md: {missing}", file=sys.stderr)
        return 1

    body = readme[start:end].rstrip()

    # The registry serves this page from the provider's own docs, so a self-referential anchor
    # link to the migration section still resolves, but links to the repo do not. Leave both
    # alone rather than guessing; flag bare relative links so they are noticed.
    for relative in re.findall(r"\]\((?!https?://|#)([^)]+)\)", body):
        print(f"warning: relative link '{relative}' will not resolve on the registry", file=sys.stderr)

    TEMPLATE.parent.mkdir(exist_ok=True)
    TEMPLATE.write_text(HEADER + body + "\n" + FOOTER)
    print(f"wrote {TEMPLATE.relative_to(ROOT)} ({len(body)} bytes of README content)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
