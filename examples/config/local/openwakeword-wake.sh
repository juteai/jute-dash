#!/bin/sh
set -eu

model=""
input=""
threshold="${JUTE_OPENWAKEWORD_THRESHOLD:-0.35}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --model)
      model="${2:-}"
      shift 2
      ;;
    --input)
      input="${2:-}"
      shift 2
      ;;
    --threshold)
      threshold="${2:-}"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [ -z "$model" ] || [ -z "$input" ]; then
  echo "usage: $0 --model MODEL --input WAV" >&2
  exit 2
fi

bin="${JUTE_OPENWAKEWORD_BIN:-openwakeword}"
if ! command -v "$bin" >/dev/null 2>&1; then
  echo "openwakeword is required; set JUTE_OPENWAKEWORD_BIN or install openwakeword on PATH" >&2
  exit 1
fi

model_arg="${JUTE_OPENWAKEWORD_MODEL:-$model}"
output=$("$bin" --model "$model_arg" --input "$input" --threshold "$threshold" --output-format json)
if printf '%s' "$output" | python3 -c 'import json,sys; raise SystemExit(0 if json.load(sys.stdin).get("detected") else 1)'; then
  printf '%s\n' "$output"
  exit 0
fi

if [ "${JUTE_WAKE_STT_FALLBACK:-true}" != "true" ]; then
  printf '%s\n' "$output"
  exit 0
fi

stt_bin="${JUTE_FASTER_WHISPER_BIN:-jute-faster-whisper}"
if ! command -v "$stt_bin" >/dev/null 2>&1; then
  printf '%s\n' "$output"
  exit 0
fi

transcript=$("$stt_bin" --model "${JUTE_WAKE_STT_MODEL:-tiny.en}" --input "$input" --language en 2>/dev/null || true)
fallback=$(
  python3 - "$output" "$transcript" "$model_arg" "$threshold" <<'PY'
import json
import re
import sys

original = json.loads(sys.argv[1])
try:
    stt = json.loads(sys.argv[2])
except Exception:
    print(json.dumps(original))
    raise SystemExit(0)

phrase = re.sub(r"[_-]+", " ", sys.argv[3].lower()).strip()
text = re.sub(r"[^a-z0-9 ]+", " ", str(stt.get("text", "")).lower())
text = re.sub(r"\s+", " ", text).strip()
detected = bool(phrase and phrase in text)
if detected:
    original["detected"] = True
    original["confidence"] = max(float(sys.argv[4]), float(original.get("confidence", 0)))
    original["transcript"] = stt.get("text", "")
print(json.dumps(original))
PY
)
printf '%s\n' "$fallback"
