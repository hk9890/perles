# Perles Session Viewer

A React-based viewer for Perles orchestration session files.

## Quick Start

The frontend is served by the Perles Go binary. To develop:

```bash
# 1. Start perles in dashboard mode (note the API port in output)
cd .. && ./perles
# Look for: "API server started port=XXXXX"

# 2. Install dependencies
npm install

# 3. Start the frontend dev server with the API port
VITE_API_PORT=XXXXX npm run dev
```

For production, the frontend is embedded in the Go binary and served
automatically when you run Perles in dashboard mode. Press `o` on a
workflow to open the session viewer in your browser.

## Features

- **Overview**: Session metadata, token usage, worker info, fabric activity stats
- **Fabric Events**: Channel creation, messages, replies, acks - with filtering
- **Coordinator**: View coordinator message log with tool calls highlighted
- **Workers**: Switch between workers and view their message logs
- **MCP Requests**: All MCP tool calls with decoded request/response JSON

## Architecture

- **Frontend**: React + TypeScript + Vite
- **Backend**: Go API server (embedded in Perles binary)

The Go server reads session files from disk and returns structured JSON.
In development, Vite proxies `/api` requests to the Go backend.

## Session File Structure

```
session-dir/
├── metadata.json       # Session metadata (status, workers, tokens)
├── fabric.jsonl        # Fabric messaging events
├── mcp_requests.jsonl  # MCP tool calls (base64 encoded)
├── messages.jsonl      # Inter-agent messages
├── coordinator/
│   ├── messages.jsonl  # Coordinator conversation log
│   └── raw.jsonl       # Raw API responses
└── workers/
    ├── worker-1/
    │   ├── messages.jsonl
    │   └── raw.jsonl
    └── ...
```
