#!/bin/sh
set -eu

usage() {
  cat <<'EOF'
Usage: ./install-local-voice.sh [--check]

Installs local voice command tools into .jute/local-voice-tools:
  - openWakeWord Python package plus a Jute-compatible CLI wrapper
  - Piper TTS Python package plus a default Amy voice model
  - gowhisper CLI from the upstream release binary

Options:
  --check   Verify detected local voice tools and exit without installing.
  --help    Show this help.
EOF
}

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
TOOLS_DIR="${JUTE_LOCAL_VOICE_TOOLS_DIR:-"$ROOT/.jute/local-voice-tools"}"
BIN_DIR="$TOOLS_DIR/bin"
MODEL_DIR="$TOOLS_DIR/models"
VENV_DIR="$TOOLS_DIR/venv"
PIPER_VOICE="${JUTE_PIPER_VOICE:-en_US-amy-medium}"
PIPER_BASE_URL="${JUTE_PIPER_VOICE_BASE_URL:-https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/amy/medium}"
PIPER_MODEL="$MODEL_DIR/$PIPER_VOICE.onnx"
PIPER_CONFIG="$PIPER_MODEL.json"
ENV_FILE="$TOOLS_DIR/local-voice.env"
GOWHISPER_VERSION="${JUTE_GOWHISPER_VERSION:-v0.0.39}"

case "${1:-}" in
  --help|-h)
    usage
    exit 0
    ;;
  --check)
    ok=1
    for bin in openwakeword gowhisper piper; do
      if [ -x "$BIN_DIR/$bin" ]; then
        path="$BIN_DIR/$bin"
      elif command -v "$bin" >/dev/null 2>&1; then
        path="$(command -v "$bin")"
      else
        printf '%s: missing\n' "$bin"
        ok=0
        continue
      fi
      if "$path" --help >/dev/null 2>&1; then
        printf '%s: %s\n' "$bin" "$path"
      else
        printf '%s: broken (%s --help failed)\n' "$bin" "$path"
        ok=0
      fi
    done
    if [ -f "$PIPER_MODEL" ] && [ -f "$PIPER_CONFIG" ]; then
      printf 'piper-model: %s\n' "$PIPER_MODEL"
    else
      printf 'piper-model: missing (%s and %s)\n' "$PIPER_MODEL" "$PIPER_CONFIG"
      ok=0
    fi
    exit $((1 - ok))
    ;;
  "")
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac

mkdir -p "$BIN_DIR" "$MODEL_DIR"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 is required" >&2
    exit 1
  fi
}

download() {
  url="$1"
  out="$2"
  if [ -f "$out" ]; then
    return
  fi
  echo "Downloading $(basename "$out")..."
  curl -L --fail "$url" -o "$out"
}

gowhisper_asset() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)
  case "$os" in
    darwin) os=darwin ;;
    linux) os=linux ;;
    *) return 1 ;;
  esac
  case "$arch" in
    arm64|aarch64) arch=arm64 ;;
    x86_64|amd64) arch=amd64 ;;
    *) return 1 ;;
  esac
  printf 'gowhisper-%s-%s' "$os" "$arch"
}

install_gowhisper() {
  if [ -x "$BIN_DIR/gowhisper" ]; then
    return
  fi
  asset=$(gowhisper_asset) || {
    echo "No gowhisper release binary for this platform; set JUTE_GO_WHISPER_BIN" >&2
    exit 1
  }
  url="https://github.com/mutablelogic/go-whisper/releases/download/$GOWHISPER_VERSION/$asset"
  tmp="$BIN_DIR/gowhisper.tmp"
  echo "Downloading gowhisper $GOWHISPER_VERSION..."
  if curl -L --fail "$url" -o "$tmp"; then
    mv "$tmp" "$BIN_DIR/gowhisper"
    chmod +x "$BIN_DIR/gowhisper"
    if command -v xattr >/dev/null 2>&1; then
      xattr -d com.apple.quarantine "$BIN_DIR/gowhisper" >/dev/null 2>&1 || true
    fi
    return
  fi
  rm -f "$tmp"
  echo "gowhisper download failed; set JUTE_GO_WHISPER_BIN or install it manually" >&2
  exit 1
}

need curl
need python3

if [ ! -x "$VENV_DIR/bin/python" ]; then
  python3 -m venv "$VENV_DIR"
fi

"$VENV_DIR/bin/python" -m pip install --upgrade pip
"$VENV_DIR/bin/python" -m pip install openwakeword piper-tts

{
printf '#!%s\n' "$VENV_DIR/bin/python"
cat <<'PY'
import argparse
import json
from openwakeword.model import Model

parser = argparse.ArgumentParser()
parser.add_argument("--model", required=True)
parser.add_argument("--input", required=True)
parser.add_argument("--output-format", default="json")
args = parser.parse_args()

model_arg = args.model
wakeword_models = [model_arg] if model_arg.endswith((".onnx", ".tflite")) else [model_arg.replace("_", " ")]
model = Model(wakeword_models=wakeword_models)
predictions = model.predict_clip(args.input)
scores = predictions if isinstance(predictions, dict) else {}
confidence = max([float(v) for v in scores.values()] or [0.0])
print(json.dumps({"detected": confidence >= 0.5, "confidence": confidence}))
PY
} > "$BIN_DIR/openwakeword"
chmod +x "$BIN_DIR/openwakeword"

if [ ! -x "$BIN_DIR/piper" ] && [ -x "$VENV_DIR/bin/piper" ]; then
  ln -sf "$VENV_DIR/bin/piper" "$BIN_DIR/piper"
fi

install_gowhisper

download "$PIPER_BASE_URL/$PIPER_VOICE.onnx" "$PIPER_MODEL"
download "$PIPER_BASE_URL/$PIPER_VOICE.onnx.json" "$PIPER_CONFIG"

cat > "$ENV_FILE" <<EOF
export PATH="$BIN_DIR:\$PATH"
export JUTE_OPENWAKEWORD_BIN="$BIN_DIR/openwakeword"
export JUTE_OPENWAKEWORD_MODEL="\${JUTE_OPENWAKEWORD_MODEL:-hey jarvis}"
export JUTE_GO_WHISPER_BIN="\${JUTE_GO_WHISPER_BIN:-$BIN_DIR/gowhisper}"
export JUTE_PIPER_BIN="$BIN_DIR/piper"
export JUTE_PIPER_MODEL="$PIPER_MODEL"
EOF

echo "Local voice tools installed under $TOOLS_DIR"
echo "Make targets in examples/config/local source $ENV_FILE automatically."
echo "Run: cd examples/config/local && make run-kronk"
