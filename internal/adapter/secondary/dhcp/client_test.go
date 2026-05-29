package dhcp

import (
	"context"
	"errors"
	"testing"
)

// fakeFileReader — мок system.FileReader.
type fakeFileReader struct {
	gotPath string
	data    []byte
	err     error
}

func (f *fakeFileReader) ReadFile(_ context.Context, path string) ([]byte, error) {
	f.gotPath = path
	return f.data, f.err
}

const leasesPath = "/tmp/dhcp.leases"

func TestListLeases_ValidLines(t *testing.T) {
	stub := []byte(`1700000000 aa:bb:cc:11:22:33 192.168.88.42 my-laptop 01:aa:bb:cc:11:22:33
1700000100 11:22:33:44:55:66 192.168.88.43 my-phone *
1700000200 ff:ee:dd:cc:bb:aa 192.168.88.44 * *
`)
	fr := &fakeFileReader{data: stub}
	c := NewClient(fr, leasesPath)

	got, err := c.ListLeases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fr.gotPath != leasesPath {
		t.Errorf("read path = %q, want %q", fr.gotPath, leasesPath)
	}
	if len(got) != 3 {
		t.Fatalf("got %d devices, want 3", len(got))
	}
	if got[0].MAC.String() != "aa:bb:cc:11:22:33" || got[0].Hostname != "my-laptop" {
		t.Errorf("first device = %+v", got[0])
	}
	if got[2].Hostname != "" {
		t.Errorf("hostname '*' should become empty, got %q", got[2].Hostname)
	}
}

func TestListLeases_SkipsInvalid(t *testing.T) {
	stub := []byte(`1700000000 aa:bb:cc:11:22:33 192.168.88.42 ok *
short line
1700000200 BAD-MAC 192.168.88.99 broken *
1700000300 ff:ee:dd:cc:bb:aa 192.168.88.44 good *
`)
	c := NewClient(&fakeFileReader{data: stub}, leasesPath)

	got, err := c.ListLeases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 valid devices (skipping junk), got %d", len(got))
	}
}

func TestListLeases_Empty(t *testing.T) {
	c := NewClient(&fakeFileReader{data: []byte("")}, leasesPath)
	got, err := c.ListLeases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
	if got == nil {
		t.Error("should return empty slice, not nil")
	}
}

func TestListLeases_ReaderError(t *testing.T) {
	readErr := errors.New("permission denied")
	c := NewClient(&fakeFileReader{err: readErr}, leasesPath)

	_, err := c.ListLeases(context.Background())
	if err == nil || !errors.Is(err, readErr) {
		t.Errorf("expected wrapped reader error, got: %v", err)
	}
}
