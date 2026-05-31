package network

import (
	"context"
	"errors"
	"testing"
)

// stubSpeedTestPort — фейк SpeedTestPort для изолированного теста use case.
type stubSpeedTestPort struct {
	res SpeedResult
	err error
}

func (s stubSpeedTestPort) Measure(_ context.Context) (SpeedResult, error) {
	return s.res, s.err
}

func TestRunSpeedTest_Execute_OK(t *testing.T) {
	want := SpeedResult{
		DownloadMbps: 95.4,
		UploadMbps:   42.1,
		PingMs:       8.3,
		JitterMs:     1.2,
		Server:       "Moscow, RU",
	}
	uc := NewRunSpeedTest(stubSpeedTestPort{res: want})

	got, err := uc.Execute(context.Background(), RunSpeedTestInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Result != want {
		t.Errorf("got %+v, want %+v", got.Result, want)
	}
}

func TestRunSpeedTest_Execute_PortError(t *testing.T) {
	portErr := errors.New("boom")
	uc := NewRunSpeedTest(stubSpeedTestPort{err: portErr})

	_, err := uc.Execute(context.Background(), RunSpeedTestInput{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, portErr) {
		t.Errorf("error should wrap port error; got %v", err)
	}
}
