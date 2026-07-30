#!/bin/sh
set -eu
cd "$(dirname "$0")"
CONFIG="${1:-config.local.json}"
[ -f "$CONFIG" ] || cp config.example.json "$CONFIG"
exec .venv/bin/python -m gateway.main --config "$CONFIG"
