package native

import (
	"encoding/hex"
	"testing"
)

func TestParseUUID(t *testing.T) {
	id, err := parseUUID("12345678-1234-1234-1234-123456789abc")
	if err != nil {
		t.Fatal(err)
	}
	got := hex.EncodeToString(id[:])
	want := "12345678123412341234123456789abc"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestBuildVLESSHeaderDomain(t *testing.T) {
	id, _ := parseUUID("12345678-1234-1234-1234-123456789abc")
	hdr, err := buildVLESSHeader(id, "tcp", "example.com", 443)
	if err != nil {
		t.Fatal(err)
	}
	if hdr[0] != 0 {
		t.Fatalf("version")
	}
	if hdr[17] != 0 { // addon len at offset 1+16
		t.Fatalf("addon")
	}
	if hdr[18] != 1 { // TCP
		t.Fatalf("cmd")
	}
	// port 443 = 0x01bb
	if hdr[19] != 0x01 || hdr[20] != 0xbb {
		t.Fatalf("port %x %x", hdr[19], hdr[20])
	}
	if hdr[21] != 2 { // domain
		t.Fatalf("atype")
	}
	if hdr[22] != byte(len("example.com")) {
		t.Fatalf("dlen")
	}
	if string(hdr[23:]) != "example.com" {
		t.Fatalf("host %q", string(hdr[23:]))
	}
}

func TestBuildVLESSHeaderIPv4(t *testing.T) {
	id, _ := parseUUID("00000000-0000-0000-0000-000000000001")
	hdr, err := buildVLESSHeader(id, "tcp", "1.2.3.4", 80)
	if err != nil {
		t.Fatal(err)
	}
	if hdr[21] != 1 {
		t.Fatalf("atype ipv4 got %d", hdr[21])
	}
	if hdr[22] != 1 || hdr[23] != 2 || hdr[24] != 3 || hdr[25] != 4 {
		t.Fatalf("ip bytes")
	}
}
