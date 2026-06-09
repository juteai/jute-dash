# Gemini A2A Agent

A cloud model-backed assistant agent using `Google Gemini` API exposed over A2A 1.0.

## Prerequisites

Set your Gemini API key in your environment:

```sh
export GEMINI_API_KEY="your-api-key-here"
```

## Running the Agent

To run the agent standalone:

```sh
make run
```

This runs the HTTP server at `http://127.0.0.1:9898`.

You can customize the model using the `GEMINI_MODEL` environment variable (defaults to `gemini-2.5-flash`).

To run the full stack (Jute Hub, web dashboard, and this agent), see [examples/config/local/README.md](../../config/local/README.md).
