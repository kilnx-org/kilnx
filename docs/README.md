# Kilnx documentation site

Source for [docs.kilnx.org](https://docs.kilnx.org). Static HTML generated from markdown via pandoc. Deployed to GitHub Pages.

## Structure

```
docs/
├── CNAME                  # docs.kilnx.org
├── index.html             # docs landing (generated from content/index.md)
├── template.html          # pandoc HTML template (shell)
├── build.sh               # markdown → html build script
├── assets/
│   └── style.css          # hand-written, no framework
├── content/               # source markdown (edit these)
│   ├── index.md
│   ├── getting-started.md
│   ├── guides/
│   ├── reference/         # includes synced copies of GRAMMAR.md etc.
│   └── tutorials/
└── pages/                 # generated html (gitignored; built by CI)
```

## Building locally

```bash
cd docs
./build.sh
```

Requires `pandoc` (macOS: `brew install pandoc`; Debian/Ubuntu: `apt install pandoc`).

Open `index.html` in a browser or serve with `python3 -m http.server`.

## Content rules

- **Edit `content/*.md`** — the source of truth for everything except `reference/grammar.md`, `principles.md`, `changelog.md`. Those three are synced from the repo root (`GRAMMAR.md`, `PRINCIPLES.md`, `CHANGELOG.md`) by `build.sh`. Edit them in the repo root, not here.
- **No frontmatter required.** The first `# heading` becomes the page title.
- **Keep examples runnable.** Code blocks should copy-paste cleanly into an `app.kilnx` file.
- **Prefer tables for reference material.** Prose for guides, tables for spec.

## Deployment

GitHub Actions (`.github/workflows/docs.yml`) builds on push to `main` when `docs/content/**` or canonical sources (`GRAMMAR.md`, `PRINCIPLES.md`, `CHANGELOG.md`) change, then deploys the built site to GitHub Pages.

The `pages/` directory is generated, not committed. GitHub Pages reads the generated artifact directly from the Action.
