#!/usr/bin/env bash
# build.sh — convert docs/content/**.md to docs/pages/**.html using pandoc
#
# Usage:
#   ./build.sh             # build everything
#   ./build.sh --clean     # remove pages/ before building
#
# Sources: content/**.md + canonical files sync'd from repo root
# Output:  pages/**.html using template.html
#
# The generated pages/ directory is committed so GitHub Pages can serve
# docs/ directly without a build step at deploy time.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$ROOT/.." && pwd)"
CONTENT="$ROOT/content"
PAGES="$ROOT/pages"
TEMPLATE="$ROOT/template.html"

if ! command -v pandoc >/dev/null; then
  echo "error: pandoc not installed" >&2
  exit 1
fi

if [[ "${1:-}" == "--clean" ]]; then
  rm -rf "$PAGES"
fi

mkdir -p "$PAGES"

# Sync canonical sources from repo root into content/ with stable names.
# These files are auto-generated — edits belong in the repo root, not here.
sync_canonical() {
  local src="$1" dest="$2" title="$3"
  if [[ -f "$REPO/$src" ]]; then
    {
      echo "---"
      echo "title: $title"
      echo "canonical: true"
      echo "---"
      echo ""
      cat "$REPO/$src"
    } > "$CONTENT/$dest"
  fi
}

sync_canonical "GRAMMAR.md"    "reference/grammar.md" "Grammar reference"
sync_canonical "PRINCIPLES.md" "principles.md"        "Design principles"
sync_canonical "CHANGELOG.md"  "changelog.md"         "Changelog"

# Convert a markdown file to HTML via pandoc.
build_page() {
  local md="$1"
  local rel="${md#$CONTENT/}"                    # e.g. guides/models.md
  local out="$PAGES/${rel%.md}.html"             # pages/guides/models.html
  local depth
  depth="$(tr -cd '/' <<<"$rel" | wc -c)"
  local root=""
  for ((i=0; i<depth; i++)); do root+="../"; done

  # Extract title from first # heading (fallback to filename stem)
  local title
  title="$(grep -m1 '^# ' "$md" 2>/dev/null | sed 's/^# //' || true)"
  if [[ -z "$title" ]]; then
    title="$(basename "${rel%.md}" | tr '-' ' ')"
  fi

  mkdir -p "$(dirname "$out")"
  pandoc "$md" \
    --from=gfm \
    --to=html5 \
    --standalone \
    --template="$TEMPLATE" \
    --metadata=title:"$title" \
    --metadata=description:"$title — Kilnx documentation" \
    --metadata=canonical:"${rel%.md}.html" \
    --variable=root:"$root" \
    --output="$out"
}

# Find all markdown files under content/ and build each.
find "$CONTENT" -name '*.md' -print0 | while IFS= read -r -d '' md; do
  echo "  building ${md#$ROOT/}"
  build_page "$md"
done

# Build pages/index.html as the site landing (content/index.md)
if [[ -f "$CONTENT/index.md" ]]; then
  pandoc "$CONTENT/index.md" \
    --from=gfm \
    --to=html5 \
    --standalone \
    --template="$TEMPLATE" \
    --metadata=title:"Documentation" \
    --metadata=description:"Kilnx — declarative backend language. Official documentation." \
    --metadata=canonical:"" \
    --variable=root:"" \
    --output="$PAGES/index.html"
  echo "  building pages/index.html"
fi

# Copy static assets into pages/ so the built site is self-contained
cp -R "$ROOT/assets" "$PAGES/assets"

# CNAME must live at the site root for GitHub Pages custom domains
if [[ -f "$ROOT/CNAME" ]]; then
  cp "$ROOT/CNAME" "$PAGES/CNAME"
fi

# 404 fallback (copy of index for client-side nav)
cp "$PAGES/index.html" "$PAGES/404.html"

echo "done → $PAGES"
