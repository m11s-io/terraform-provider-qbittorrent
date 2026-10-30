#!/usr/bin/env python3
"""Validate the provider subset against the official OpenAPI 3.2 schema."""

import json
import subprocess
import tempfile
import urllib.request
from pathlib import Path

import yaml
import jsonschema

SCHEMA_URL = "https://spec.openapis.org/oas/3.2/schema-base/2026-08-30"

spec = yaml.safe_load(Path("api/qbittorrent.yaml").read_text())
with tempfile.NamedTemporaryFile(suffix=".yaml") as bundled_file:
    subprocess.run(
        [
            "go",
            "run",
            "./scripts/bundle_openapi.go",
            "--input",
            "api/qbittorrent.yaml",
            "--output",
            bundled_file.name,
        ],
        check=True,
    )
    bundled = yaml.safe_load(Path(bundled_file.name).read_text())
with urllib.request.urlopen(SCHEMA_URL) as response:
    schema = json.load(response)
validator = jsonschema.validators.validator_for(schema)(schema)
errors = []
for document_name, document in (("source", spec), ("bundled", bundled)):
    for error in validator.iter_errors(document):
        error.add_note(f"in {document_name} OpenAPI document")
        errors.append(error)
errors.sort(key=lambda e: list(map(str, e.path)))
for error in errors:
    location = "/".join(map(str, error.path)) or "<root>"
    print(f"{location}: {error.message} ({'; '.join(error.__notes__)})")
raise SystemExit(bool(errors))
