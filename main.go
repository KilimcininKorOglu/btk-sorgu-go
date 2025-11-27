package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// BTK DNS sunucuları
var btkDNSServers = []string{
	"195.175.39.39:53",
	"195.175.39.40:53",
}

// BTK engel sayfası IP adresleri
var blockedIPs = []string{
	"195.175.254.2",
	"2a01:358:4014:a00::3",
}

// DNSResponse API response yapısı
type DNSResponse struct {
	Domain      string            `json:"domain"`
	Timestamp   int64             `json:"timestamp"`
	Success     bool              `json:"success"`
	IsBlocked   bool              `json:"is_blocked"`
	Method      string            `json:"method"`
	DNSServer   string            `json:"dns_server"`
	ResolvedIPs []string          `json:"resolved_ips"`
	BlockedIP   string            `json:"blocked_ip,omitempty"`
	Error       string            `json:"error,omitempty"`
	Data        *DNSData          `json:"data,omitempty"`
	APIInfo     *APIInfo          `json:"api_info,omitempty"`
}

// DNSData detaylı DNS bilgileri
type DNSData struct {
	QueryTime    string   `json:"query_time"`
	ResponseTime string   `json:"response_time"`
	RecordType   string   `json:"record_type"`
	AllIPs       []string `json:"all_ips"`
	IsBlockedIP  bool     `json:"is_blocked_ip"`
	Source       string   `json:"source"`
}

// APIInfo API meta bilgileri
type APIInfo struct {
	ProcessingTime float64 `json:"processing_time"`
	Method         string  `json:"method"`
	ServerLocation string  `json:"server_location"`
}

// checkDomain belirtilen domain'in BTK tarafından engellenip engellenmediğini kontrol eder
func checkDomain(domain string) DNSResponse {
	startTime := time.Now()
	response := DNSResponse{
		Domain:    domain,
		Timestamp: time.Now().Unix(),
		Method:    "dns_turkey",
	}

	// Domain validasyonu
	if domain == "" {
		response.Success = false
		response.Error = "Domain parametresi boş olamaz"
		return response
	}

	// Domain temizleme (http://, https://, www. vs. kaldır)
	domain = cleanDomain(domain)
	response.Domain = domain

	var lastError error
	var resolvedIPs []string
	var usedServer string

	// BTK DNS sunucularını dene
	for _, dnsServer := range btkDNSServers {
		ips, err := resolveDNS(domain, dnsServer)
		if err != nil {
			lastError = err
			continue
		}
		resolvedIPs = ips
		usedServer = dnsServer
		break
	}

	if len(resolvedIPs) == 0 {
		response.Success = false
		if lastError != nil {
			response.Error = fmt.Sprintf("DNS çözümlemesi başarısız: %v", lastError)
		} else {
			response.Error = "DNS çözümlemesi başarısız: IP adresi bulunamadı"
		}
		return response
	}

	response.Success = true
	response.DNSServer = strings.TrimSuffix(usedServer, ":53")
	response.ResolvedIPs = resolvedIPs

	// Engel kontrolü
	isBlocked, blockedIP := checkIfBlocked(resolvedIPs)
	response.IsBlocked = isBlocked
	if isBlocked {
		response.BlockedIP = blockedIP
	}

	processingTime := time.Since(startTime)

	// Detaylı veri
	response.Data = &DNSData{
		QueryTime:    time.Now().Format("15:04:05.000"),
		ResponseTime: processingTime.String(),
		RecordType:   "A",
		AllIPs:       resolvedIPs,
		IsBlockedIP:  isBlocked,
		Source:       getSource(),
	}

	response.APIInfo = &APIInfo{
		ProcessingTime: processingTime.Seconds(),
		Method:         "dns_turkey",
		ServerLocation: getServerLocation(),
	}

	return response
}

// resolveDNS belirtilen DNS sunucusu üzerinden domain'i çözümler
func resolveDNS(domain, dnsServer string) ([]string, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 5 * time.Second,
			}
			return d.DialContext(ctx, "udp", dnsServer)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// A kayıtlarını çözümle
	ips, err := resolver.LookupHost(ctx, domain)
	if err != nil {
		return nil, err
	}

	return ips, nil
}

// checkIfBlocked IP listesinde BTK engel IP'si var mı kontrol eder
func checkIfBlocked(ips []string) (bool, string) {
	for _, ip := range ips {
		for _, blockedIP := range blockedIPs {
			if ip == blockedIP {
				return true, blockedIP
			}
		}
	}
	return false, ""
}

// cleanDomain domain'den protokol ve www önekini temizler
func cleanDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "www.")
	domain = strings.TrimSuffix(domain, "/")
	
	// Path varsa kaldır
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	
	return domain
}

// getSource sunucu kaynağını döndürür
func getSource() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// getServerLocation sunucu lokasyonunu döndürür
func getServerLocation() string {
	// Gerçek uygulamada bu değer config'den alınabilir
	location := os.Getenv("SERVER_LOCATION")
	if location == "" {
		return "Unknown"
	}
	return location
}

// handleCheck /check endpoint handler'ı
func handleCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Domain parametresini al
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		// POST body'den de dene
		if r.Method == "POST" {
			var req struct {
				Domain string `json:"domain"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				domain = req.Domain
			}
		}
	}

	response := checkDomain(domain)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("JSON encode hatası: %v", err)
	}
}

// handleHealth /health endpoint handler'ı
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

// handleRoot / endpoint handler'ı
func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":        "BTK Engel Kontrol API",
		"version":     "1.0.0",
		"description": "Türkiye'de BTK tarafından engellenen domainleri kontrol eden API",
		"endpoints": map[string]string{
			"GET /check?domain={domain}": "Domain engel durumunu kontrol et",
			"GET /health":                "API sağlık durumu",
		},
		"dns_servers": btkDNSServers,
		"blocked_ips": blockedIPs,
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/check", handleCheck)
	http.HandleFunc("/health", handleHealth)

	log.Printf("🚀 BTK Engel Kontrol API başlatıldı")
	log.Printf("📡 Dinleniyor: http://localhost:%s", port)
	log.Printf("📋 Endpoint: GET /check?domain=example.com")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Sunucu başlatılamadı: %v", err)
	}
}
