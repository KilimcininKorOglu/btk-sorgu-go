package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCleanDomain verifies protocol and path stripping while preserving www
func TestCleanDomain(t *testing.T) {
	cases := map[string]string{
		"example.com":                  "example.com",
		"https://example.com":          "example.com",
		"http://example.com":           "example.com",
		"www.example.com":              "www.example.com",
		"https://www.example.com/":     "www.example.com",
		"HTTPS://WWW.Example.com":      "WWW.Example.com",
		"WwW.test.org":                 "WwW.test.org",
		"http://discord.com/path":      "discord.com",
		"  spaced.com  ":               "spaced.com",
		"blog.example.com":             "blog.example.com",
		"https://api.v2.example.com/x": "api.v2.example.com",
		"www.sub.example.com":          "www.sub.example.com",
	}
	for in, want := range cases {
		if got := cleanDomain(in); got != want {
			t.Errorf("cleanDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestIsValidDomain verifies RFC 1035 format and length limit
func TestIsValidDomain(t *testing.T) {
	valid := []string{"example.com", "a.io", "sub.example.co.uk", "xn--nxasmq6b.com", "blog.example.com", "api.v2.example.com", "a.b.c.d.example.com", "cdn-1.assets.example.io"}
	for _, d := range valid {
		if !isValidDomain(d) {
			t.Errorf("isValidDomain(%q) = false, want true", d)
		}
	}
	invalid := []string{"", "no-tld", "-bad.com", "bad-.com", "exa mple.com", "example", strings.Repeat("a", 254) + ".com"}
	for _, d := range invalid {
		if isValidDomain(d) {
			t.Errorf("isValidDomain(%q) = true, want false", d)
		}
	}
}

// TestCheckIfBlocked verifies blocked IP matching
func TestCheckIfBlocked(t *testing.T) {
	blocked := []string{"195.175.254.2", "2a01:358:4014:a00::3"}

	if ok, ip := checkIfBlocked([]string{"1.2.3.4", "195.175.254.2"}, blocked); !ok || ip != "195.175.254.2" {
		t.Errorf("beklenen (true, 195.175.254.2), gelen (%v, %q)", ok, ip)
	}
	if ok, ip := checkIfBlocked([]string{"2a01:358:4014:a00::3"}, blocked); !ok || ip != "2a01:358:4014:a00::3" {
		t.Errorf("IPv6 engel eşleşmedi: (%v, %q)", ok, ip)
	}
	if ok, _ := checkIfBlocked([]string{"8.8.8.8", "1.1.1.1"}, blocked); ok {
		t.Error("engelsiz IP listesi engelli döndü")
	}
	if ok, _ := checkIfBlocked(nil, blocked); ok {
		t.Error("boş IP listesi engelli döndü")
	}
}

// TestParseCommaSeparated verifies whitespace trimming and empty-item skipping
func TestParseCommaSeparated(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ,c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
		{"single", []string{"single"}},
		{"", nil},
		{"  ", nil},
		{"x, ,y,", []string{"x", "y"}},
	}
	for _, c := range cases {
		got := parseCommaSeparated(c.in)
		if len(got) != len(c.want) {
			t.Errorf("parseCommaSeparated(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("parseCommaSeparated(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// TestConfigGettersReturnCopy verifies getters return a defensive copy
func TestConfigGettersReturnCopy(t *testing.T) {
	c := &Config{
		DNSServers:     []string{"1.1.1.1:53"},
		BlockedIPs:     []string{"9.9.9.9"},
		ServerLocation: "TestLoc",
	}
	got := c.GetDNSServers()
	got[0] = "değiştirildi"
	if c.GetDNSServers()[0] != "1.1.1.1:53" {
		t.Error("GetDNSServers iç slice'ı sızdırdı, savunmacı kopya değil")
	}
	blocked := c.GetBlockedIPs()
	blocked[0] = "değiştirildi"
	if c.GetBlockedIPs()[0] != "9.9.9.9" {
		t.Error("GetBlockedIPs iç slice'ı sızdırdı, savunmacı kopya değil")
	}
	if c.GetServerLocation() != "TestLoc" {
		t.Error("GetServerLocation yanlış değer döndürdü")
	}
}

// TestLoadConfigPortNormalization verifies DNS port appending and IPv6 bracketing
func TestLoadConfigPortNormalization(t *testing.T) {
	t.Setenv("BTK_DNS_SERVERS", "8.8.8.8, 9.9.9.9:5353, 2001:4860:4860::8888")
	t.Setenv("BTK_BLOCKED_IPS", "9.9.9.9")
	t.Setenv("SERVER_LOCATION", "Test Region")

	c := &Config{}
	if err := c.loadConfig(); err != nil {
		t.Fatalf("loadConfig hata döndürdü: %v", err)
	}

	servers := c.GetDNSServers()
	want := []string{"8.8.8.8:53", "9.9.9.9:5353", "[2001:4860:4860::8888]:53"}
	if len(servers) != len(want) {
		t.Fatalf("DNS sunucu sayısı = %d, want %d (%v)", len(servers), len(want), servers)
	}
	for i := range want {
		if servers[i] != want[i] {
			t.Errorf("DNS[%d] = %q, want %q", i, servers[i], want[i])
		}
	}
	if loc := c.GetServerLocation(); loc != "Test_Region" {
		t.Errorf("SERVER_LOCATION boşluk normalize edilmedi: %q, want Test_Region", loc)
	}
}

// TestHandleCheckEmptyDomain verifies an empty domain returns HTTP 400
func TestHandleCheckEmptyDomain(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	rec := httptest.NewRecorder()
	handleCheck(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("boş domain kodu = %d, want 400", rec.Code)
	}
	var resp DNSResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("response decode hatası: %v", err)
	}
	if resp.Success {
		t.Error("boş domain success:true döndü")
	}
}

// TestHandleCheckInvalidDomain verifies an invalid format returns HTTP 400 (no DNS call)
func TestHandleCheckInvalidDomain(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/check?domain=not_a_valid_domain", nil)
	rec := httptest.NewRecorder()
	handleCheck(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("geçersiz domain kodu = %d, want 400", rec.Code)
	}
}

// TestHandleCheckOptionsCORS verifies OPTIONS preflight returns 200 with CORS headers
func TestHandleCheckOptionsCORS(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/check", nil)
	rec := httptest.NewRecorder()
	handleCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("OPTIONS kodu = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS origin = %q, want *", got)
	}
}

// TestHandleCheckOversizedBody verifies MaxBytesReader rejects a large POST body
func TestHandleCheckOversizedBody(t *testing.T) {
	big := `{"domain":"` + strings.Repeat("a", 2048) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/check", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleCheck(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("büyük gövde kodu = %d, want 400", rec.Code)
	}
}

// TestHandleHealth verifies /health returns a healthy status
func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health kodu = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("health decode hatası: %v", err)
	}
	if body["status"] != "healthy" {
		t.Errorf("health status = %v, want healthy", body["status"])
	}
}

// TestHandleConfig verifies /config returns the active configuration
func TestHandleConfig(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rec := httptest.NewRecorder()
	handleConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("config kodu = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("config decode hatası: %v", err)
	}
	if _, ok := body["dns_servers"]; !ok {
		t.Error("config yanıtında dns_servers alanı yok")
	}
}

// TestHandleRoot verifies the root path returns info and an unknown path returns 404
func TestHandleRoot(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handleRoot(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("kök yol kodu = %d, want 200", rec.Code)
	}

	reqNF := httptest.NewRequest(http.MethodGet, "/bilinmeyen", nil)
	recNF := httptest.NewRecorder()
	handleRoot(recNF, reqNF)
	if recNF.Code != http.StatusNotFound {
		t.Errorf("bilinmeyen yol kodu = %d, want 404", recNF.Code)
	}
}
