package nftables

import (
	"context"
	"testing"

	"github.com/tumour/openwrt-bot/internal/domain"
)

const testRateTable = "netdev openwrt_bot"

// «Золотой» тест: script-строка транзакции фиксируется целиком — порядок
// destroy element → destroy limit → add limit → add element обязателен
// (нельзя удалить limit-объект, пока на него ссылается элемент map).
func TestRateLimiter_Set_TransactionScript(t *testing.T) {
	fr := &fakeRunner{}
	c := NewRateLimiter(fr, testRateTable)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")
	rate, _ := domain.NewRate(512)

	if err := c.Set(context.Background(), mac, rate); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `destroy element netdev openwrt_bot rate_ul { aa:bb:cc:11:22:33 }; ` +
		`destroy element netdev openwrt_bot rate_dl { aa:bb:cc:11:22:33 }; ` +
		`destroy limit netdev openwrt_bot lim_ul_aabbcc112233; ` +
		`destroy limit netdev openwrt_bot lim_dl_aabbcc112233; ` +
		`add limit netdev openwrt_bot lim_ul_aabbcc112233 { rate over 512 kbytes/second burst 256 kbytes ; }; ` +
		`add limit netdev openwrt_bot lim_dl_aabbcc112233 { rate over 512 kbytes/second burst 256 kbytes ; }; ` +
		`add element netdev openwrt_bot rate_ul { aa:bb:cc:11:22:33 : "lim_ul_aabbcc112233" }; ` +
		`add element netdev openwrt_bot rate_dl { aa:bb:cc:11:22:33 : "lim_dl_aabbcc112233" }`
	if fr.gotName != "nft" || len(fr.gotArgs) != 1 || fr.gotArgs[0] != want {
		t.Errorf("cmd = %s %q\nwant nft [%q]", fr.gotName, fr.gotArgs, want)
	}
}

func TestRateLimiter_Remove_TransactionScript(t *testing.T) {
	fr := &fakeRunner{}
	c := NewRateLimiter(fr, testRateTable)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")

	if err := c.Remove(context.Background(), mac); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `destroy element netdev openwrt_bot rate_ul { aa:bb:cc:11:22:33 }; ` +
		`destroy element netdev openwrt_bot rate_dl { aa:bb:cc:11:22:33 }; ` +
		`destroy limit netdev openwrt_bot lim_ul_aabbcc112233; ` +
		`destroy limit netdev openwrt_bot lim_dl_aabbcc112233`
	if fr.gotName != "nft" || len(fr.gotArgs) != 1 || fr.gotArgs[0] != want {
		t.Errorf("cmd = %s %q\nwant nft [%q]", fr.gotName, fr.gotArgs, want)
	}
}

func TestRateLimiter_Set_ExecError_Propagates(t *testing.T) {
	fr := &fakeRunner{err: execErr("Error: No such file or directory")}
	c := NewRateLimiter(fr, testRateTable)
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")
	rate, _ := domain.NewRate(512)

	if err := c.Set(context.Background(), mac, rate); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Fixture снят с реального вывода `nft list table netdev openwrt_bot`
// (см. ROADMAP итерация 9): limit-объекты многострочные, nft укрупняет единицы
// при печати — лимит 1024 КБ/с печатается как «1 mbytes/second».
func TestRateLimiter_List_Parse(t *testing.T) {
	stub := []byte(`table netdev openwrt_bot {
	limit lim_ul_aabbcc112233 {
		rate over 1 mbytes/second burst 512 kbytes
	}

	limit lim_dl_aabbcc112233 {
		rate over 1 mbytes/second burst 512 kbytes
	}

	limit lim_ul_112233445566 {
		rate over 300 kbytes/second burst 150 kbytes
	}

	limit lim_dl_112233445566 {
		rate over 300 kbytes/second burst 150 kbytes
	}

	map rate_ul {
		type ether_addr : limit
		elements = { 11:22:33:44:55:66 : "lim_ul_112233445566",
			     aa:bb:cc:11:22:33 : "lim_ul_aabbcc112233" }
	}

	map rate_dl {
		type ether_addr : limit
		elements = { 11:22:33:44:55:66 : "lim_dl_112233445566",
			     aa:bb:cc:11:22:33 : "lim_dl_aabbcc112233" }
	}

	chain lan_ingress {
		type filter hook ingress device "br-lan" priority filter; policy accept;
		limit name ether saddr map @rate_ul counter packets 0 bytes 0 drop
	}

	chain lan_egress {
		type filter hook egress device "br-lan" priority filter; policy accept;
		limit name ether daddr map @rate_dl counter packets 0 bytes 0 drop
	}
}`)
	fr := &fakeRunner{out: stub}
	c := NewRateLimiter(fr, testRateTable)

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantArgs := []string{"list", "table", "netdev openwrt_bot"}
	if fr.gotName != "nft" || !equalArgs(fr.gotArgs, wantArgs) {
		t.Errorf("cmd = %s %v, want nft %v", fr.gotName, fr.gotArgs, wantArgs)
	}
	if len(got) != 2 {
		t.Fatalf("got %d limits, want 2; %v", len(got), got)
	}
	mac1, _ := domain.NewMAC("aa:bb:cc:11:22:33")
	mac2, _ := domain.NewMAC("11:22:33:44:55:66")
	if got[mac1].KBps() != 1024 { // «1 mbytes» из вывода → 1024 КБ/с
		t.Errorf("limit[%s] = %s, want 1024", mac1, got[mac1])
	}
	if got[mac2].KBps() != 300 {
		t.Errorf("limit[%s] = %s, want 300", mac2, got[mac2])
	}
}

func TestRateLimiter_List_EmptyTable(t *testing.T) {
	stub := []byte(`table netdev openwrt_bot {
	map rate_ul {
		type ether_addr : limit
	}

	map rate_dl {
		type ether_addr : limit
	}
}`)
	fr := &fakeRunner{out: stub}
	c := NewRateLimiter(fr, testRateTable)

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no limits, got %v", got)
	}
}

func TestRateLimiter_List_ExecError_Propagates(t *testing.T) {
	fr := &fakeRunner{err: execErr("Error: No such file or directory")}
	c := NewRateLimiter(fr, testRateTable)

	if _, err := c.List(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBurstKB(t *testing.T) {
	tests := []struct {
		rate int
		want int
	}{
		{1, 16},      // clamp снизу
		{20, 16},     // 20/2=10 → clamp 16
		{512, 256},   // rate/2
		{2048, 1024}, // rate/2
		{8192, 2048}, // clamp сверху
	}
	for _, tc := range tests {
		rate, _ := domain.NewRate(tc.rate)
		if got := burstKB(rate); got != tc.want {
			t.Errorf("burstKB(%d) = %d, want %d", tc.rate, got, tc.want)
		}
	}
}

func TestMacHexRoundTrip(t *testing.T) {
	mac, _ := domain.NewMAC("aa:bb:cc:11:22:33")
	h := macHex(mac)
	if h != "aabbcc112233" {
		t.Fatalf("macHex = %q, want aabbcc112233", h)
	}
	back, err := hexToMAC(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if back != mac {
		t.Errorf("round trip: %s != %s", back, mac)
	}
}

func TestHexToMAC_Invalid(t *testing.T) {
	for _, bad := range []string{"", "aabbcc", "aabbcc1122334455", "zzbbcc112233"} {
		if _, err := hexToMAC(bad); err == nil {
			t.Errorf("hexToMAC(%q): expected error", bad)
		}
	}
}
