#!/bin/sh
set -eu

mode="${1:-}"
case "$mode" in
  capture)
    if [ "${JUTE_DEV_VOICE_CAPTURE_ON_START:-}" = "1" ]; then
      dd if=/dev/zero bs=320 count=5 2>/dev/null
      i=0
      while [ "$i" -lt 10 ]; do
        printf '\377\177%.0s' $(seq 1 160)
        i=$((i + 1))
      done
      dd if=/dev/zero bs=320 count=5 2>/dev/null
      exit 0
    fi
    while :; do
      dd if=/dev/zero bs=320 count=1 2>/dev/null
      sleep 0.1
    done
    ;;
  wake)
    printf '{"detected":true,"providerId":"local-dev-wake","modelId":"hey-jute","confidence":0.9}\n'
    ;;
  stt)
    printf '{"text":"turn on the kitchen lights","providerId":"local-dev-stt","modelId":"dev-transcript","language":"en","durationMs":500}\n'
    ;;
  tts)
    text="$(cat)"
    if command -v say >/dev/null 2>&1 && [ -n "$text" ]; then
      say "$text" >/dev/null 2>&1 || true
      playback="local"
    else
      playback="metadata"
    fi
    printf '{"providerId":"local-dev-tts","voiceId":"amy","locale":"en","contentType":"audio/wav","sampleRate":16000,"sampleWidth":2,"channels":1,"durationMs":500,"playbackKind":"%s"}\n' "$playback"
    ;;
  *)
    echo "usage: $0 capture|wake|stt|tts" >&2
    exit 2
    ;;
esac
