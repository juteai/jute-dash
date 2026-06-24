package service

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os/exec"
	"path/filepath"
	"time"
)

type CommandAudioCapture struct {
	Command       string
	Args          []string
	SampleRate    int
	Channels      int
	SampleWidth   int
	FrameDuration time.Duration
}

func (c CommandAudioCapture) Capture(ctx context.Context) (<-chan AudioFrame, <-chan error) {
	frames := make(chan AudioFrame)
	errs := make(chan error, 1)
	go func() {
		defer close(frames)
		defer close(errs)
		if err := c.capture(ctx, frames); err != nil && ctx.Err() == nil {
			errs <- err
		}
	}()
	return frames, errs
}

func (c CommandAudioCapture) capture(ctx context.Context, frames chan<- AudioFrame) error {
	if !filepath.IsAbs(c.Command) {
		return errors.New("audio capture command must be absolute")
	}
	c = normalizeCommandAudioCapture(c)
	//nolint:gosec // command capture is an explicit local hub setting.
	cmd := exec.CommandContext(ctx, c.Command, c.Args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	frameBytes := int(float64(c.SampleRate)*c.FrameDuration.Seconds()) * c.Channels * c.SampleWidth
	if frameBytes <= 0 {
		frameBytes = c.SampleRate * c.Channels * c.SampleWidth / 10
	}
	buf := make([]byte, frameBytes)
	timestamp := time.Now().UTC()
	for {
		n, err := io.ReadFull(stdout, buf)
		if n > 0 {
			pcm := append([]byte(nil), buf[:n]...)
			select {
			case frames <- AudioFrame{
				PCM:         pcm,
				SampleRate:  c.SampleRate,
				SampleWidth: c.SampleWidth,
				Channels:    c.Channels,
				Timestamp:   timestamp,
				Duration:    c.FrameDuration,
			}:
				timestamp = timestamp.Add(c.FrameDuration)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		return err
	}
	if err := <-waitErr; err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func normalizeCommandAudioCapture(c CommandAudioCapture) CommandAudioCapture {
	if c.SampleRate == 0 {
		c.SampleRate = 16000
	}
	if c.Channels == 0 {
		c.Channels = 1
	}
	if c.SampleWidth == 0 {
		c.SampleWidth = 2
	}
	if c.FrameDuration == 0 {
		c.FrameDuration = 100 * time.Millisecond
	}
	return c
}

type EnergyVAD struct {
	Threshold int
}

func (v EnergyVAD) Speech(frame AudioFrame) bool {
	threshold := v.Threshold
	if threshold <= 0 {
		threshold = 500
	}
	for i := 0; i+1 < len(frame.PCM); i += 2 {
		sample := int(binary.LittleEndian.Uint16(frame.PCM[i : i+2]))
		if sample >= 32768 {
			sample -= 65536
		}
		if math.Abs(float64(sample)) >= float64(threshold) {
			return true
		}
	}
	return false
}
