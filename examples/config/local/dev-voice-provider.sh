#!/bin/sh
set -eu

mode="${1:-}"
case "$mode" in
  wake)
    printf '{"detected":true,"providerId":"local-dev-wake","modelId":"hey-jute","confidence":0.9}\n'
    ;;
  stt)
    printf '{"text":"turn on the kitchen lights","providerId":"local-dev-stt","modelId":"dev-transcript","language":"en","durationMs":500}\n'
    ;;
  tts)
    cat >/dev/null
    printf '{"providerId":"local-dev-tts","voiceId":"amy","locale":"en","contentType":"audio/wav","sampleRate":16000,"sampleWidth":2,"channels":1,"durationMs":500,"playbackKind":"metadata"}\n'
    ;;
  *)
    echo "usage: $0 wake|stt|tts" >&2
    exit 2
    ;;
esac
