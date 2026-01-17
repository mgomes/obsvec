# obsvec

A vector search CLI tool for Markdown directories, such as Obsidian vaults. Uses Cohere embeddings and sqlite-vec to provide semantic search across your markdown notes.

## Features

- Semantic search using Cohere embed-v4 and rerank-v3.5
- Local SQLite database with sqlite-vec for fast vector search
- Incremental indexing (only re-indexes changed files)
- Watch mode for automatic re-indexing on file changes
- Opens search results directly in Obsidian

## Installation

Requires Go 1.21+ and a C compiler (for CGO/sqlite).

```bash
# Clone and build
git clone https://github.com/mgomes/obsvec
cd obsvec
make build

# Or install to $GOPATH/bin
make install
```

On macOS, you may want Homebrew's SQLite to avoid deprecation warnings:

```bash
brew install sqlite
```

The Makefile automatically uses Homebrew's SQLite if available.

## Setup

On first run, you'll be prompted to enter:

1. Your Cohere API key (get one at https://dashboard.cohere.com/api-keys)
2. The path to your Obsidian vault

```bash
./ofind -setup
```

Configuration is stored in `~/.config/obsvec/config.json`.

## Usage

### Index your vault

```bash
# Incremental index (only new/changed files)
ofind -index

# Full reindex
ofind -index -full
```

### Search

```bash
ofind -q "your search query"
```

Use arrow keys to navigate results, Enter to open in Obsidian, q to quit.

### Watch mode

Automatically re-index files as they change:

```bash
ofind -watch
```

## How it works

1. Markdown files are chunked by headers and size (roughly 500 tokens per chunk)
2. Chunks are embedded using Cohere's embed-v4 model (1024 dimensions)
3. Embeddings are stored in SQLite using sqlite-vec
4. Queries are embedded and matched against stored vectors
5. Top candidates are reranked using Cohere's rerank-v3.5 for better relevance

## Database

The SQLite database is stored at `~/.config/obsvec/obsvec.db`. Delete this file to force a complete reindex.

## License

MIT
