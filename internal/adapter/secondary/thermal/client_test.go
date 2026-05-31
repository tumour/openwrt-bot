package thermal

import (
	"context"
	"errors"
	"testing"
)

// fakeFileReader — мок system.FileReader: отдаёт предзаданный контент/ошибку
// и запоминает запрошенный путь.
type fakeFileReader struct {
	gotPath string
	out     []byte
	err     error
}

func (f *fakeFileReader) ReadFile(_ context.Context, path string) ([]byte, error) {
	f.gotPath = path
	return f.out, f.err
}

func TestParseMilliCelsius(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    float64
		wantErr bool
	}{
		{"typical", "55300\n", 55.3, false},
		{"no newline", "48250", 48.25, false},
		{"zero", "0", 0, false},
		{"high", "100000", 100, false},
		{"surrounding spaces", "  42000 \n", 42, false},
		{"garbage", "n/a", 0, true},
		{"empty", "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMilliCelsius([]byte(tc.in))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %v", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseMilliCelsius(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestClient_Temperature_OK(t *testing.T) {
	fr := &fakeFileReader{out: []byte("48250\n")}
	c := NewClient(fr, "/sys/class/thermal/thermal_zone0/temp")

	got, err := c.Temperature(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 48.25 {
		t.Errorf("Temperature = %v, want 48.25", got)
	}
	if fr.gotPath != "/sys/class/thermal/thermal_zone0/temp" {
		t.Errorf("read path = %q, want thermal_zone0/temp", fr.gotPath)
	}
}

func TestClient_Temperature_ReadError(t *testing.T) {
	readErr := errors.New("no such file")
	c := NewClient(&fakeFileReader{err: readErr}, "/sys/class/thermal/thermal_zone0/temp")

	_, err := c.Temperature(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, readErr) {
		t.Errorf("error should wrap reader error; got %v", err)
	}
}
