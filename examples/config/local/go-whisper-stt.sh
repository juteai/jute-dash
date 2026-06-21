#!/bin/sh
set -eu

model=""
input=""
language="en"

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
    --language)
      language="${2:-en}"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [ -z "$model" ] || [ -z "$input" ]; then
  echo "usage: $0 --model MODEL --input WAV --language LANG" >&2
  exit 2
fi

bin="${JUTE_GO_WHISPER_BIN:-gowhisper}"
if ! command -v "$bin" >/dev/null 2>&1; then
  echo "gowhisper is required; set JUTE_GO_WHISPER_BIN or install gowhisper on PATH" >&2
  exit 1
fi

output=$("$bin" transcribe "$model" "$input" --format json --language "$language")
case "$output" in
  \{*) printf '%s\n' "$output" ;;
  *)
    escaped=$(printf '%s' "$output" | sed 's/\\/\\\\/g; s/"/\\"/g')
    printf '{"text":"%s","providerId":"local-whisper-stt","modelId":"%s","language":"%s"}\n' "$escaped" "$model" "$language"
    ;;
esac
