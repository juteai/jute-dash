package service

import (
	"context"
	"testing"
	"time"
)

func TestCommandAudioCaptureStreamsPCMFrames(t *testing.T) {
	capture := CommandAudioCapture{
		Command:       "/bin/sh",
		Args:          []string{"-c", "printf '\\040\\003'"},
		SampleRate:    1,
		Channels:      1,
		SampleWidth:   2,
		FrameDuration: time.Second,
	}

	frames, errs := capture.Capture(context.Background())
	frame, ok := <-frames
	if !ok {
		t.Fatal("expected a captured frame")
	}
	if !(EnergyVAD{Threshold: 500}).Speech(frame) {
		t.Fatalf("expected captured frame to be speech: %+v", frame)
	}
	if err := <-errs; err != nil {
		t.Fatalf("capture error = %v", err)
	}
}
