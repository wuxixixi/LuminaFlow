# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LuminaFlow is a Go application that batch converts images to videos using the DMXAPI (MiniMax Hailuo 2.3) AI video generation API. It supports both CLI and GUI modes.

## Build Commands

```bash
# CLI mode (default)
go build -o LuminaFlow.exe .

# GUI mode (requires CGO and graphics libraries)
go build -tags gui -o LuminaFlow.exe .

# Using Makefile
make build-windows    # CLI build
make deps             # Download dependencies
make fmt              # Format code
make vet              # Run static analysis
```

## Running

```bash
# CLI mode - requires DMXAPI_API_KEY in .env or environment
./LuminaFlow.exe

# GUI mode
./LuminaFlow.exe -cli  # Forces CLI mode from main.go
```

## Architecture

**Entry Points:**
- `cli_main.go` - CLI-only entry (build tag `!gui`, default)
- `gui_main.go` - GUI entry (build tag `gui`, requires Fyne/CGO)
- `main.go` - Mode selector (used when both builds are present)

**Core Components:**
- `config.go` - Configuration: APIKey, OutputDir, Concurrency, Prompt, Duration, Resolution, Model
- `api.go` - DMXAPI client with 3-step workflow: SubmitTask → PollTask → RetrieveFile → DownloadVideo
- `processor.go` - Worker pool for batch processing with task state machine (Pending → Encoding → Submitting → Processing → Downloading → Done/Failed)
- `imaging.go` - Image scanning, validation (min 300px short side, aspect ratio 0.4-2.5, max 20MB), and base64 encoding
- `cli.go` - CLI interface implementation
- `ui.go` - Fyne GUI (build tag `gui`)

**API Details:**
- Base URL: `https://www.dmxapi.cn`
- Endpoints: `/v1/video_generation`, `/v1/query/video_generation`, `/v1/files/retrieve`
- Model: `MiniMax-Hailuo-2.3`
- Poll timeout: 10 minutes with exponential backoff

## Configuration

Create `.env` file:
```
DMXAPI_API_KEY=your-api-key-here
```

Default settings (config.go):
- Output: `./output/`
- Concurrency: 2 (max 4)
- Duration: 6 seconds
- Resolution: 768P
- Prompt: Chinese text for natural motion effects

## Input/Output

- Input: Scans `./data/`, `./input/`, `./images/` directories
- Supported formats: `.jpg`, `.jpeg`, `.png`, `.webp`
- Output: MP4 files in `./output/` directory
