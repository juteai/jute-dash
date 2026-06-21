#!/bin/sh
set -eu

model=""
input=""

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
output=$("$bin" --model "$model_arg" --input "$input" --output-format json)
case "$output" in
  *'"detected":true'*|*true*) printf '{"detected":true,"providerId":"local-openwakeword","modelId":"%s","confidence":1}\n' "$model" ;;
  *) printf '{"detected":false,"providerId":"local-openwakeword","modelId":"%s","confidence":0}\n' "$model" ;;
esac
