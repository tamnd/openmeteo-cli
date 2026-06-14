package openmeteo

import (
	"testing"
)

// These tests are offline: they exercise the domain's pure string functions
// and the Info metadata, which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "openmeteo" {
		t.Errorf("Scheme = %q, want openmeteo", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "openmeteo" {
		t.Errorf("Identity.Binary = %q, want openmeteo", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	typ, id, err := Domain{}.Classify("lat=48.8566&lon=2.3522")
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if typ != "weather" {
		t.Errorf("type = %q, want weather", typ)
	}
	if id == "" {
		t.Error("id is empty")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("weather", "latitude=48.8566&longitude=2.3522")
	if err != nil {
		t.Fatalf("Locate error: %v", err)
	}
	if got == "" {
		t.Error("Locate returned empty URL")
	}
}
