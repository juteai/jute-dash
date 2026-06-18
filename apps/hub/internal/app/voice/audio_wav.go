package voice

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
	byteRateInt := sampleRate * channels * DefaultSampleWidth
	if len(pcm) > math.MaxUint32 || sampleRate > math.MaxUint32 || byteRateInt > math.MaxUint32 {
		return nil, errors.New("WAV is too large")
	}

	var buf bytes.Buffer
	dataSize := uint32(len(pcm)) //nolint:gosec // bounds checked above before writing the WAV header.
	sampleRate32 := uint32(sampleRate)
	byteRate := uint32(byteRateInt) //nolint:gosec // bounds checked above before writing the WAV header.
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36)+dataSize)
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&buf, binary.LittleEndian, sampleRate32)
	_ = binary.Write(&buf, binary.LittleEndian, byteRate)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(channels*DefaultSampleWidth))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, dataSize)
	buf.Write(pcm)
	return buf.Bytes(), nil
}

func DecodeWAV(raw []byte, startedAt time.Time) (CapturedUtterance, error) {
	reader := bytes.NewReader(raw)
	header := make([]byte, 12)
	if _, err := io.ReadFull(reader, header); err != nil {
		return CapturedUtterance{}, err
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return CapturedUtterance{}, errors.New("invalid WAV header")
	}

	var sampleRate int
	var channels int
	var pcm []byte
	for reader.Len() > 0 {
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(reader, chunkHeader); err != nil {
			return CapturedUtterance{}, err
		}
		chunkID := string(chunkHeader[0:4])
		chunkSize := int(binary.LittleEndian.Uint32(chunkHeader[4:8]))
		if chunkSize < 0 || chunkSize > reader.Len() {
			return CapturedUtterance{}, fmt.Errorf("invalid WAV chunk size for %s", chunkID)
		}
		chunk := make([]byte, chunkSize)
		if _, err := io.ReadFull(reader, chunk); err != nil {
			return CapturedUtterance{}, err
		}
		if chunkSize%2 == 1 && reader.Len() > 0 {
			if _, err := reader.ReadByte(); err != nil {
				return CapturedUtterance{}, err
			}
		}
		switch chunkID {
		case "fmt ":
			if len(chunk) < 16 {
				return CapturedUtterance{}, errors.New("invalid WAV fmt chunk")
			}
			audioFormat := binary.LittleEndian.Uint16(chunk[0:2])
			channels = int(binary.LittleEndian.Uint16(chunk[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(chunk[4:8]))
			bitsPerSample := binary.LittleEndian.Uint16(chunk[14:16])
			if audioFormat != 1 || channels != DefaultChannels || bitsPerSample != 16 {
				return CapturedUtterance{}, errors.New("WAV must be 16-bit mono PCM")
			}
		case "data":
			pcm = append([]byte(nil), chunk...)
		}
	}
	if len(pcm) == 0 || sampleRate <= 0 {
		return CapturedUtterance{}, errors.New("WAV missing fmt or data chunk")
	}
	return UtteranceFromPCM(pcm, sampleRate, channels, startedAt, 20*time.Millisecond), nil
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
