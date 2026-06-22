package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"time"
)

const (
	DefaultSampleRate  = 16000
	DefaultChannels    = 1
	DefaultSampleWidth = 2
)

func EncodeWAV(utterance CapturedUtterance) ([]byte, error) {
	if len(utterance.Frames) == 0 {
		return nil, errors.New("utterance has no frames")
	}
	sampleRate := utterance.SampleRate
	if sampleRate == 0 {
		sampleRate = utterance.Frames[0].SampleRate
	}
	channels := utterance.Channels
	if channels == 0 {
		channels = utterance.Frames[0].Channels
	}
	if sampleRate <= 0 || channels != DefaultChannels {
		return nil, errors.New("WAV requires positive sample rate and mono audio")
	}
	pcm := flattenUtterancePCM(utterance)
	blockAlignInt := channels * DefaultSampleWidth
	byteRateInt := sampleRate * blockAlignInt
	if len(pcm) > math.MaxUint32 ||
		sampleRate > math.MaxUint32 ||
		channels > math.MaxUint16 ||
		blockAlignInt > math.MaxUint16 ||
		byteRateInt > math.MaxUint32 {
		return nil, errors.New("WAV is too large")
	}

	var buf bytes.Buffer
	dataSize := uint32(len(pcm)) //nolint:gosec // bounds checked above before writing the WAV header.
	sampleRate32 := uint32(sampleRate)
	channels16 := uint16(channels)
	blockAlign := uint16(blockAlignInt)
	byteRate := uint32(byteRateInt) //nolint:gosec // bounds checked above before writing the WAV header.
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36)+dataSize)
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, channels16)
	_ = binary.Write(&buf, binary.LittleEndian, sampleRate32)
	_ = binary.Write(&buf, binary.LittleEndian, byteRate)
	_ = binary.Write(&buf, binary.LittleEndian, blockAlign)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, dataSize)
	buf.Write(pcm)
	return buf.Bytes(), nil
}

func UtteranceFromPCM(
	pcm []byte,
	sampleRate int,
	channels int,
	startedAt time.Time,
	frameDuration time.Duration,
) CapturedUtterance {
	if sampleRate == 0 {
		sampleRate = DefaultSampleRate
	}
	if channels == 0 {
		channels = DefaultChannels
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	if frameDuration <= 0 {
		frameDuration = 20 * time.Millisecond
	}
	bytesPerFrame := int(float64(sampleRate*channels*DefaultSampleWidth) * frameDuration.Seconds())
	if bytesPerFrame <= 0 {
		bytesPerFrame = sampleRate * channels * DefaultSampleWidth / 50
	}
	var frames []AudioFrame
	for offset := 0; offset < len(pcm); offset += bytesPerFrame {
		end := offset + bytesPerFrame
		if end > len(pcm) {
			end = len(pcm)
		}
		duration := durationForPCM(end-offset, sampleRate, channels)
		frames = append(frames, AudioFrame{
			PCM:         append([]byte(nil), pcm[offset:end]...),
			SampleRate:  sampleRate,
			SampleWidth: DefaultSampleWidth,
			Channels:    channels,
			Timestamp:   startedAt.Add(durationForPCM(offset, sampleRate, channels)),
			Duration:    duration,
		})
	}
	return CapturedUtterance{
		Frames:     frames,
		StartedAt:  startedAt,
		EndedAt:    startedAt.Add(durationForPCM(len(pcm), sampleRate, channels)),
		SampleRate: sampleRate,
		Channels:   channels,
	}
}

func flattenUtterancePCM(utterance CapturedUtterance) []byte {
	var pcm []byte
	for _, frame := range utterance.Frames {
		pcm = append(pcm, frame.PCM...)
	}
	return pcm
}

func durationForPCM(byteCount int, sampleRate int, channels int) time.Duration {
	if sampleRate <= 0 || channels <= 0 || byteCount <= 0 {
		return 0
	}
	samples := float64(byteCount) / float64(DefaultSampleWidth*channels)
	return time.Duration(samples / float64(sampleRate) * float64(time.Second))
}
