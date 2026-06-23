#!/bin/sh
set -eu

voice=""
language="en"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --voice)
      voice="${2:-}"
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

text=$(cat)
if [ -z "$text" ]; then
  echo "TTS text is required" >&2
  exit 2
fi

bin="${JUTE_PIPER_BIN:-piper}"
model="${JUTE_PIPER_MODEL:-$voice}"
if ! command -v "$bin" >/dev/null 2>&1; then
  echo "piper is required; set JUTE_PIPER_BIN or install piper on PATH" >&2
  exit 1
fi
if [ -z "$model" ]; then
  echo "JUTE_PIPER_MODEL is required for piper TTS" >&2
  exit 1
fi

audio=$(mktemp -t jute-piper.XXXXXX.wav)
trap 'rm -f "$audio"' EXIT
printf '%s' "$text" | "$bin" --model "$model" --output_file "$audio" >/dev/null
audio_b64=$(base64 < "$audio" | tr -d '\n')

printf '{"providerId":"local-piper-tts","voiceId":"%s","locale":"%s","contentType":"audio/wav","sampleRate":22050,"sampleWidth":2,"channels":1,"playbackKind":"browser","audioBase64":"%s"}\n' "$voice" "$language" "$audio_b64"
