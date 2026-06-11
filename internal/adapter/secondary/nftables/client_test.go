package nftables

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// fakeRunner — fake реализации system.Runner. Возвращает запрограммированный stdout/err
// и записывает последнюю команду (для проверки правильной командной строки).
type fakeRunner struct {
	gotName string
	gotArgs []string
	out     []byte
	err     error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.gotName = name
	f.gotArgs = args
	return f.out, f.err
}

const (
	testTable = "inet fw4"
	testSet   = "banned_macs"
)

func TestAdd_OK(t *testing.T) {
	fr := &fakeRunner{}
	c := NewClient(fr, testTable, testSet)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	if err := c.Add(context.Background(), mac); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantArgs := []string{"add", "element", "inet fw4", "banned_macs", "{ aa:bb:cc:11:22:33 }"}
	if fr.gotName != "nft" || !equalArgs(fr.gotArgs, wantArgs) {
		t.Errorf("cmd = %s %v, want nft %v", fr.gotName, fr.gotArgs, wantArgs)
	}
}

func TestAdd_AlreadyExists(t *testing.T) {
	fr := &fakeRunner{err: errors.New("nft: element already exists")}
	c := NewClient(fr, testTable, testSet)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	err := c.Add(context.Background(), mac)
	if err == nil || !errors.Is(err, domain.ErrAlreadyInSet) {
		t.Fatalf("expected ErrAlreadyInSet, got: %v", err)
	}
}

func TestRemove_NotFound(t *testing.T) {
	fr := &fakeRunner{err: errors.New("nft: No such file or directory")}
	c := NewClient(fr, testTable, testSet)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	err := c.Remove(context.Background(), mac)
	if err == nil || !errors.Is(err, domain.ErrNotInSet) {
		t.Fatalf("expected ErrNotInSet, got: %v", err)
	}
}

func TestList_Parse(t *testing.T) {
	// Реальный фрагмент `nft list set inet fw4 banned_macs`.
	stub := []byte(`table inet fw4 {
	set banned_macs {
		type ether_addr
		elements = { aa:bb:cc:11:22:33, 11:22:33:44:55:66 }
	}
}`)
	fr := &fakeRunner{out: stub}
	c := NewClient(fr, testTable, testSet)

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2; %v", len(got), got)
	}
	if got[0].String() != "aa:bb:cc:11:22:33" || got[1].String() != "11:22:33:44:55:66" {
		t.Errorf("parsed wrong: %v", got)
	}
}

func TestList_Empty(t *testing.T) {
	stub := []byte(`table inet fw4 {
	set banned_macs {
		type ether_addr
		elements = {  }
	}
}`)
	fr := &fakeRunner{out: stub}
	c := NewClient(fr, testTable, testSet)

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty list, got %v", got)
	}
}

func equalArgs(a, b []string) bool {
	return strings.Join(a, "|") == strings.Join(b, "|")
}
