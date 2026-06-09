# Ollama A2A Agent

A local model-backed assistant agent using `Ollama` exposed over A2A 1.0.

## Prerequisites

1. Install Ollama from [ollama.com](https://ollama.com).
2. Download your preferred model (defaults to `llama3.1`):
   ```sh
   ollama run llama3.1
   ```

## Running the Agent

To run the agent standalone:

```sh
make run
```

This runs the HTTP server at `http://127.0.0.1:9999`.

You can customize the Ollama API endpoint and model using environment variables:
- `OLLAMA_API_BASE`: Ollama API URL (defaults to `http://localhost:11434/v1`)
- `OLLAMA_MODEL`: Model ID to use (defaults to `llama3.1`)

To run the full stack (Jute Hub, web dashboard, and this agent), see [examples/config/local/README.md](../../config/local/README.md).
