#!/bin/sh
set -eu

usage() {
  cat <<'EOF'
Usage: ./install-local-voice.sh [--check]

Installs local voice command tools into .jute/local-voice-tools:
  - openWakeWord Python package plus a Jute-compatible CLI wrapper
  - Piper TTS Python package plus a default Amy voice model
  - faster-whisper Python package plus a Jute-compatible CLI wrapper

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
OPENWAKEWORD_MODEL_VERSION="${JUTE_OPENWAKEWORD_MODEL_VERSION:-v0.5.1}"

case "${1:-}" in
  --help|-h)
    usage
    exit 0
    ;;
  --check)
    ok=1
    for bin in openwakeword jute-faster-whisper piper; do
      if [ -x "$BIN_DIR/$bin" ]; then
        path="$BIN_DIR/$bin"
      elif command -v "$bin" >/dev/null 2>&1; then
        path="$(command -v "$bin")"
      else
        printf '%s: missing\n' "$bin"
        ok=0
        continue
      fi
      case "$bin" in
        openwakeword)
          if "$path" --model hey_jarvis --threshold 0.35 --check >/dev/null 2>&1; then
            printf '%s: %s\n' "$bin" "$path"
          else
            printf '%s: broken (%s --model hey_jarvis --threshold 0.35 --check failed)\n' "$bin" "$path"
            ok=0
          fi
          ;;
        jute-faster-whisper)
          if "$path" --check >/dev/null 2>&1; then
            printf '%s: %s\n' "$bin" "$path"
          else
            printf '%s: broken (%s --check failed)\n' "$bin" "$path"
            ok=0
          fi
          ;;
        *)
          if "$path" --help >/dev/null 2>&1; then
            printf '%s: %s\n' "$bin" "$path"
          else
            printf '%s: broken (%s --help failed)\n' "$bin" "$path"
            ok=0
          fi
          ;;
      esac
    done
    if [ -f "$PIPER_MODEL" ] && [ -f "$PIPER_CONFIG" ]; then
      printf 'piper-model: %s\n' "$PIPER_MODEL"
    else
      printf 'piper-model: missing (%s and %s)\n' "$PIPER_MODEL" "$PIPER_CONFIG"
      ok=0
    fi
    if [ "$ok" -eq 1 ]; then
      smoke_wav=$(mktemp -t jute-voice-check.XXXXXX.wav)
      trap 'rm -f "$smoke_wav"' EXIT
      if printf '%s' 'turn on the kitchen lights' |
        JUTE_PIPER_MODEL="$PIPER_MODEL" "$BIN_DIR/piper" --model "$PIPER_MODEL" --output_file "$smoke_wav" >/dev/null 2>&1 &&
        JUTE_FASTER_WHISPER_MODEL_DIR="$MODEL_DIR/whisper" "$BIN_DIR/jute-faster-whisper" --model tiny.en --input "$smoke_wav" --language en |
          grep -qi 'kitchen lights'; then
        printf 'stt-smoke: ok\n'
      else
        printf 'stt-smoke: failed\n'
        ok=0
      fi
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

need curl
need python3

if [ ! -x "$VENV_DIR/bin/python" ]; then
  python3 -m venv "$VENV_DIR"
fi

"$VENV_DIR/bin/python" -m pip install --upgrade pip
"$VENV_DIR/bin/python" -m pip install openwakeword onnxruntime piper-tts faster-whisper
OPENWAKEWORD_MODEL_DIR=$("$VENV_DIR/bin/python" - <<'PY'
import os
import openwakeword
print(os.path.join(os.path.dirname(os.path.abspath(openwakeword.__file__)), "resources", "models"))
PY
)
mkdir -p "$OPENWAKEWORD_MODEL_DIR"
for model in embedding_model.onnx melspectrogram.onnx hey_jarvis_v0.1.onnx; do
  download "https://github.com/dscripka/openWakeWord/releases/download/$OPENWAKEWORD_MODEL_VERSION/$model" \
    "$OPENWAKEWORD_MODEL_DIR/$model"
done

{
printf '#!%s\n' "$VENV_DIR/bin/python"
cat <<'PY'
import argparse
import json
from openwakeword.model import Model

parser = argparse.ArgumentParser()
parser.add_argument("--model", required=True)
parser.add_argument("--input", default="")
parser.add_argument("--threshold", type=float, default=0.5)
parser.add_argument("--check", action="store_true")
parser.add_argument("--output-format", default="json")
args = parser.parse_args()

model_arg = args.model
wakeword_models = [model_arg] if model_arg.endswith((".onnx", ".tflite")) else [model_arg.replace("_", " ")]
model = Model(wakeword_models=wakeword_models, inference_framework="onnx")
if args.check:
    print(json.dumps({"ok": True}))
    raise SystemExit(0)
if not args.input:
    raise SystemExit("--input is required unless --check is used")
predictions = model.predict_clip(args.input)
if isinstance(predictions, dict):
    scores = predictions
else:
    scores = {}
    for frame in predictions:
        if isinstance(frame, dict):
            for key, value in frame.items():
                scores[key] = max(float(value), scores.get(key, 0.0))
confidence = max([float(v) for v in scores.values()] or [0.0])
print(json.dumps({"detected": confidence >= args.threshold, "confidence": confidence}))
PY
} > "$BIN_DIR/openwakeword"
chmod +x "$BIN_DIR/openwakeword"

{
printf '#!%s\n' "$VENV_DIR/bin/python"
cat <<'PY'
import argparse
import json
import os
from faster_whisper import WhisperModel

parser = argparse.ArgumentParser()
parser.add_argument("--model", default="tiny.en")
parser.add_argument("--input", default="")
parser.add_argument("--language", default="en")
parser.add_argument("--check", action="store_true")
args = parser.parse_args()

if args.check:
    print(json.dumps({"ok": True}))
    raise SystemExit(0)
if not args.input:
    raise SystemExit("--input is required unless --check is used")

download_root = os.environ.get("JUTE_FASTER_WHISPER_MODEL_DIR")
device = os.environ.get("JUTE_FASTER_WHISPER_DEVICE", "cpu")
compute_type = os.environ.get("JUTE_FASTER_WHISPER_COMPUTE_TYPE", "int8")
model = WhisperModel(
    args.model,
    device=device,
    compute_type=compute_type,
    download_root=download_root,
)
segments, info = model.transcribe(args.input, language=args.language, vad_filter=True)
text = " ".join(segment.text.strip() for segment in segments).strip()
print(json.dumps({
    "text": text,
    "providerId": "local-whisper-stt",
    "modelId": args.model,
    "language": info.language or args.language,
    "durationMs": int(float(info.duration or 0) * 1000),
}))
PY
} > "$BIN_DIR/jute-faster-whisper"
chmod +x "$BIN_DIR/jute-faster-whisper"

if [ ! -x "$BIN_DIR/piper" ] && [ -x "$VENV_DIR/bin/piper" ]; then
  ln -sf "$VENV_DIR/bin/piper" "$BIN_DIR/piper"
fi

download "$PIPER_BASE_URL/$PIPER_VOICE.onnx" "$PIPER_MODEL"
download "$PIPER_BASE_URL/$PIPER_VOICE.onnx.json" "$PIPER_CONFIG"

cat > "$ENV_FILE" <<EOF
export PATH="$BIN_DIR:\$PATH"
export JUTE_OPENWAKEWORD_BIN="$BIN_DIR/openwakeword"
export JUTE_OPENWAKEWORD_MODEL="\${JUTE_OPENWAKEWORD_MODEL:-$OPENWAKEWORD_MODEL_DIR/hey_jarvis_v0.1.onnx}"
export JUTE_OPENWAKEWORD_THRESHOLD="\${JUTE_OPENWAKEWORD_THRESHOLD:-0.35}"
export JUTE_FASTER_WHISPER_BIN="\${JUTE_FASTER_WHISPER_BIN:-$BIN_DIR/jute-faster-whisper}"
export JUTE_FASTER_WHISPER_MODEL_DIR="\${JUTE_FASTER_WHISPER_MODEL_DIR:-$MODEL_DIR/whisper}"
export JUTE_PIPER_BIN="$BIN_DIR/piper"
export JUTE_PIPER_MODEL="$PIPER_MODEL"
EOF

echo "Local voice tools installed under $TOOLS_DIR"
echo "Make targets in examples/config/local source $ENV_FILE automatically."
echo "Run: cd examples/config/local && make run-kronk"
