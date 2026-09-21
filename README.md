<p align="center">
  <img src="./images/banner.png" alt="cmus-lyric banner" width="600" />
</p>

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go"></a>
  <a href="https://github.com/charmbracelet/bubbletea"><img src="https://img.shields.io/badge/Bubble_Tea-TUI-ff69b4?style=flat" alt="Bubble Tea"></a>
  <a href="https://github.com/charmbracelet/lipgloss"><img src="https://img.shields.io/badge/Lipgloss-Styling-7D56F4?style=flat" alt="Lipgloss"></a>
  <a href="https://github.com/charmbracelet/bubbles"><img src="https://img.shields.io/badge/Bubbles-Components-AD8EE6?style=flat" alt="Bubbles"></a>
  <a href="https://lrclib.net"><img src="https://img.shields.io/badge/LRCLIB-Lyrics_API-4CAF50?style=flat" alt="LRCLIB"></a>
  <a href="https://github.com/index-null/cmus-lyric/releases/latest"><img src="https://img.shields.io/github/v/release/index-null/cmus-lyric?style=flat&color=blue" alt="Release"></a>
  <a href="https://github.com/index-null/cmus-lyric/blob/master/LICENSE"><img src="https://img.shields.io/github/license/index-null/cmus-lyric?style=flat" alt="License"></a>
</p>

# cmus-lyric

English | [中文](README_zh.md)

A terminal-based synced lyrics viewer for [cmus](https://cmus.github.io/), built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

> Inspired by [pekrockstar/cmus-lyric](https://github.com/pekrockstar/cmus-lyric), rewritten from scratch with a modern Go stack.

<p align="center">
  <img src="./images/demo.png" alt="cmus-lyric demo" width="600" />
</p>

## Overview

`cmus-lyric` connects to your running cmus instance via Unix socket (with `cmus-remote` fallback), reads the current track, and displays time-synced lyrics in a beautiful TUI. It resolves lyrics from multiple sources — embedded audio tags, local `.lrc` files, a disk cache, and online APIs — all fetched asynchronously without blocking the UI.

**Features:**

- Real-time synced lyric scrolling with highlight
- Auto-fetch from four sources **in parallel**: LRCLIB, Netease Music, Kugou, lyrics.ovh (non-blocking)
- Lyric picker (`r`): search every source at once, preview each candidate and pick the right one
- Track info panel (`i`): bitrate, format, file size and tags when a track has no lyrics
- Instrumental / placeholder detection — `纯音乐，请欣赏` style placeholders are never shown as lyrics
- Translation lyrics support (`.t.lrc` / `.t.lyric` side-by-side)
- Embedded lyrics extraction from audio files (ID3/Vorbis Comment)
- Album cover display (`lyrics cover`)
- Lyrics caching keyed by artist + title + **duration**, so different versions of a song never share a cache entry (location determined by `os.UserCacheDir()`, e.g. `~/Library/Caches/cmus-lyric/` on macOS, `~/.cache/cmus-lyric/` on Linux)
- Duration-aware scoring (±30s tolerance) plus title/artist similarity ranking
- Full-width bracket (`［00:05.00］`) and GBK/UTF-8 normalization
- Unix socket IPC for low-overhead cmus communication
- GBK/UTF-8 auto-detection
- Progress bar and playback status
- Debug mode (`d` key) to inspect track metadata and lyric sources
- Minimal, distraction-free UI

## Install

### Homebrew (macOS / Linux)

```bash
brew install index-null/tap/lyrics
```

### Shell script

```bash
curl -fsSL https://raw.githubusercontent.com/index-null/cmus-lyric/master/install.sh | bash
```

Or install to a custom directory:

```bash
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/index-null/cmus-lyric/master/install.sh | bash
```

### Go

```bash
go install github.com/index-null/cmus-lyric/cmd/lyrics@latest
```

### Manual download

Download the binary for your platform from the [Releases](https://github.com/index-null/cmus-lyric/releases/latest) page, extract and move to your `PATH`:

```bash
tar xzf cmus-lyric_*_darwin_arm64.tar.gz
sudo install -m 755 lyrics /usr/local/bin/lyrics
```

### From source

```bash
git clone https://github.com/index-null/cmus-lyric.git
cd cmus-lyric
task install   # or: go build -o lyrics ./cmd/lyrics && sudo mv lyrics /usr/local/bin/
```

## Prerequisites

- [cmus](https://cmus.github.io/) music player (must be running)

## Usage

Start cmus and play a song, then in another terminal:

```bash
lyrics
```

| Key          | Action                                            |
| ------------ | ------------------------------------------------- |
| `q` `Ctrl+C` | Quit                                              |
| `?`          | Toggle help                                       |
| `i`          | Toggle track info (bitrate / format / tags)        |
| `d`          | Toggle debug                                      |
| `r`          | Search every lyric source and pick a candidate     |

Inside the picker:

| Key          | Action                                        |
| ------------ | --------------------------------------------- |
| `↑/↓` `j/k`  | Move through candidates (or scroll the preview) |
| `Tab`        | Switch focus between the list and the preview  |
| `Enter`      | Use the highlighted lyric                      |
| `s`          | Use it and save next to the audio file         |
| `Esc`        | Close the picker                               |

### How lyrics are resolved

1. Extract embedded lyrics from the audio file (ID3 USLT / Vorbis Comment)
2. Look for `<filename>.lrc` / `<filename>.lyric` (and `<title>.lrc`) next to the audio file
3. If a `.t.lrc` / `.t.lyric` file exists alongside, translation lines are shown below each lyric line
4. Check the local cache (keyed by artist + title + duration)
5. If nothing is found, query LRCLIB, Netease Music, Kugou and lyrics.ovh **in parallel** and use the best-scoring candidate
6. Press `r` at any time to run the same search manually and choose a candidate yourself

**Ranking**: candidates are scored by title similarity (60%), artist similarity (25%) and duration closeness (15%); synced (timestamped) lyrics always rank above plain text, and karaoke/remix/cover versions are penalised.

**No lyrics?** If every source comes back empty (or the track is flagged instrumental), the player shows a track info panel with format, bitrate, size, year, genre, track/disc numbers and embedded artwork info instead of a fake "纯音乐，请欣赏" line.

### Album cover

Display the album cover of the current track:

```bash
lyrics cover
```

Cover is fetched from:
1. Embedded album art in audio file
2. Local cache (location determined by `os.UserCacheDir()`, e.g. `~/Library/Caches/cmus-lyric/` on macOS, `~/.cache/cmus-lyric/` on Linux)
3. Netease Music API (auto-saved to cache)

## Project Structure

```
cmus-lyric/
├── cmd/lyrics/           # Application entry point
├── internal/
│   ├── cmus/             # cmus IPC (Unix socket + exec fallback)
│   ├── cover/            # Album cover display
│   ├── lyric/            # Lyric parsing, caching, sources (lrclib/netease/kugou/lyrics.ovh), audio probing
│   └── player/           # Bubble Tea model, view, lyric picker, info panel
├── .github/workflows/    # CI/CD (auto-release on tag)
├── Taskfile.yml          # Build tasks
├── .golangci.yml         # Linter config (v2)
├── lefthook.yml          # Git hooks (fmt + lint + build + test)
├── .goreleaser.yaml      # Release config
├── install.sh            # One-line install script
└── go.mod
```

## Development

```bash
# First time: install git hooks & verify toolchain (one-time)
task setup

task build          # Build binary to bin/
task run            # Build and run
task lint           # Run golangci-lint
task test           # Run tests
task check          # Full quality check (tidy + lint + test)
```

> [!NOTE]
> Linting requires [golangci-lint](https://golangci-lint.run/) v2.x. Install with `brew install golangci-lint` or [other methods](https://golangci-lint.run/welcome/install/).
>
> `task setup` installs [lefthook](https://github.com/evilmartians/lefthook) git hooks (pre-commit: fmt+lint+build, pre-push: lint+test). Hooks prevent pushing code that would fail CI — they mirror the same checks that run on GitHub.

## Release

Releases are fully automated via GitHub Actions. To create a new release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This triggers the workflow which:

1. Builds binaries for linux/darwin x amd64/arm64
2. Creates a GitHub Release with checksums
3. Updates the Homebrew tap formula

> [!TIP]
> To preview a release locally: `goreleaser release --snapshot --clean`
