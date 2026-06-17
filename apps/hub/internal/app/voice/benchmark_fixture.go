package voice

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	BenchmarkSampleRate  = 16000
	BenchmarkChannels    = 1
	BenchmarkSampleWidth = 2
)

type BenchmarkAudioSpec struct {
	Duration   time.Duration
	Frequency  float64
	Amplitude  float64
	SampleRate int
	Channels   int
	StartedAt  time.Time
}

type BenchmarkFixtureSetManifest struct {
	Issue    string                     `json:"issue"`
	Kind     string                     `json:"kind"`
	Fixtures []BenchmarkFixtureManifest `json:"fixtures"`
}

type BenchmarkFixtureManifest struct {
	ID                 string `json:"id"`
	Description        string `json:"description,omitempty"`
	Path               string `json:"path"`
	SHA256             string `json:"sha256,omitempty"`
	Source             string `json:"source,omitempty"`
	RecordedAt         string `json:"recordedAt,omitempty"`
	Consent            *bool  `json:"consent,omitempty"`
	ExpectWake         *bool  `json:"expectWake,omitempty"`
	ExpectedTranscript string `json:"expectedTranscript,omitempty"`
	Language           string `json:"language,omitempty"`
}

func NewBenchmarkToneFixture(
	id string,
	description string,
	spec BenchmarkAudioSpec,
) (BenchmarkFixture, error) {
	pcm, sampleRate, channels, err := DeterministicPCM16(spec)
	if err != nil {
		return BenchmarkFixture{}, err
	}
	return BenchmarkFixture{
		ID:          id,
		Description: sanitizeText(description),
		Utterance:   BenchmarkUtteranceFromPCM(pcm, sampleRate, channels, spec.StartedAt, 20*time.Millisecond),
	}, nil
}

func DecodeBenchmarkFixtureSetManifest(raw string) (BenchmarkFixtureSetManifest, error) {
	var manifest BenchmarkFixtureSetManifest
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return BenchmarkFixtureSetManifest{}, fmt.Errorf("decode benchmark fixture set: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return BenchmarkFixtureSetManifest{}, errors.New("decode benchmark fixture set: trailing JSON data")
	}
	return manifest, nil
}

func LoadBenchmarkFixtureSet(
	dir string,
	manifest BenchmarkFixtureSetManifest,
	startedAt time.Time,
) ([]BenchmarkFixture, []string) {
	var problems []string
	if strings.TrimSpace(manifest.Issue) == "" {
		problems = append(problems, "issue is required")
	}
	if strings.TrimSpace(manifest.Kind) == "" {
		problems = append(problems, "kind is required")
	}
	if len(manifest.Fixtures) == 0 {
		problems = append(problems, "fixtures must declare at least one fixture")
	}

	fixtures := make([]BenchmarkFixture, 0, len(manifest.Fixtures))
	for i, entry := range manifest.Fixtures {
		location := fmt.Sprintf("fixtures[%d]", i)
		fixture, entryProblems := loadBenchmarkFixture(dir, entry, startedAt, location)
		problems = append(problems, entryProblems...)
		if len(entryProblems) == 0 {
			fixtures = append(fixtures, fixture)
		}
	}
	return fixtures, problems
}

func loadBenchmarkFixture(
	dir string,
	entry BenchmarkFixtureManifest,
	startedAt time.Time,
	location string,
) (BenchmarkFixture, []string) {
	var problems []string
	if strings.TrimSpace(entry.ID) == "" {
		problems = append(problems, location+".id is required")
	}
	if unsafeModelPath(entry.Path) {
		problems = append(problems, location+".path must be a relative fixture asset path")
	}
	if len(problems) > 0 {
		return BenchmarkFixture{}, problems
	}

	raw, err := os.ReadFile(filepath.Join(dir, filepath.Clean(entry.Path)))
	if err != nil {
		return BenchmarkFixture{}, append(problems, location+".path could not be read")
	}
	if expected := strings.TrimSpace(entry.SHA256); expected != "" && BenchmarkBytesSHA256(raw) != expected {
		return BenchmarkFixture{}, append(problems, location+".sha256 does not match fixture bytes")
	}
	utterance, err := DecodeBenchmarkWAV(raw, startedAt)
	if err != nil {
		return BenchmarkFixture{}, append(problems, location+".path must point to a 16-bit mono PCM WAV fixture")
	}
	return BenchmarkFixture{
		ID:                 safeFixtureID(entry.ID),
		Description:        sanitizeText(entry.Description),
		Utterance:          utterance,
		ExpectWake:         entry.ExpectWake,
		ExpectedTranscript: sanitizeText(entry.ExpectedTranscript),
		Language:           safeIdentifier(entry.Language),
	}, nil
}

func DeterministicPCM16(spec BenchmarkAudioSpec) ([]byte, int, int, error) {
	sampleRate := spec.SampleRate
	if sampleRate == 0 {
		sampleRate = BenchmarkSampleRate
	}
	channels := spec.Channels
	if channels == 0 {
		channels = BenchmarkChannels
	}
	if sampleRate <= 0 {
		return nil, 0, 0, errors.New("sample rate must be positive")
	}
	if channels != BenchmarkChannels {
		return nil, 0, 0, errors.New("benchmark fixtures require mono audio")
	}
	if spec.Duration <= 0 {
		return nil, 0, 0, errors.New("duration must be positive")
	}
	amplitude := spec.Amplitude
	if amplitude == 0 {
		amplitude = 0.35
	}
	if amplitude < 0 || amplitude > 1 {
		return nil, 0, 0, errors.New("amplitude must be between 0 and 1")
	}
	samples := int(spec.Duration.Seconds() * float64(sampleRate))
	pcm := make([]byte, samples*BenchmarkSampleWidth)
	for i := range samples {
		var value int16
		if spec.Frequency > 0 && amplitude > 0 {
			wave := math.Sin(2 * math.Pi * spec.Frequency * float64(i) / float64(sampleRate))
			value = int16(wave * amplitude * math.MaxInt16)
		}
		binary.LittleEndian.PutUint16(pcm[i*BenchmarkSampleWidth:], uint16(value))
	}
	return pcm, sampleRate, channels, nil
}

func BenchmarkUtteranceFromPCM(
	pcm []byte,
	sampleRate int,
	channels int,
	startedAt time.Time,
	frameDuration time.Duration,
) CapturedUtterance {
	if sampleRate == 0 {
		sampleRate = BenchmarkSampleRate
	}
	if channels == 0 {
		channels = BenchmarkChannels
	}
	if startedAt.IsZero() {
		startedAt = time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	}
	if frameDuration <= 0 {
		frameDuration = 20 * time.Millisecond
	}
	bytesPerFrame := int(float64(sampleRate*channels*BenchmarkSampleWidth) * frameDuration.Seconds())
	if bytesPerFrame <= 0 {
		bytesPerFrame = sampleRate * channels * BenchmarkSampleWidth / 50
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
			SampleWidth: BenchmarkSampleWidth,
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

func EncodeBenchmarkWAV(utterance CapturedUtterance) ([]byte, error) {
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
	if sampleRate <= 0 || channels != BenchmarkChannels {
		return nil, errors.New("benchmark WAV requires positive sample rate and mono audio")
	}
	pcm := flattenUtterancePCM(utterance)
	byteRateInt := sampleRate * channels * BenchmarkSampleWidth
	if len(pcm) > math.MaxUint32 || byteRateInt > math.MaxUint32 {
		return nil, errors.New("benchmark WAV is too large")
	}
	var buf bytes.Buffer
	dataSize := uint32(len(pcm))
	byteRate := uint32(byteRateInt) //nolint:gosec // byteRateInt is checked above against math.MaxUint32.
	blockAlign := uint16(channels * BenchmarkSampleWidth)

	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36)+dataSize)
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&buf, binary.LittleEndian, byteRate)
	_ = binary.Write(&buf, binary.LittleEndian, blockAlign)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, dataSize)
	buf.Write(pcm)
	return buf.Bytes(), nil
}

func DecodeBenchmarkWAV(raw []byte, startedAt time.Time) (CapturedUtterance, error) {
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
			if audioFormat != 1 || channels != BenchmarkChannels || bitsPerSample != 16 {
				return CapturedUtterance{}, errors.New("benchmark WAV must be 16-bit mono PCM")
			}
		case "data":
			pcm = append([]byte(nil), chunk...)
		}
	}
	if len(pcm) == 0 || sampleRate <= 0 {
		return CapturedUtterance{}, errors.New("WAV missing fmt or data chunk")
	}
	return BenchmarkUtteranceFromPCM(pcm, sampleRate, channels, startedAt, 20*time.Millisecond), nil
}

func BenchmarkBytesSHA256(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum[:])
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
	samples := float64(byteCount) / float64(BenchmarkSampleWidth*channels)
	return time.Duration(samples / float64(sampleRate) * float64(time.Second))
}
