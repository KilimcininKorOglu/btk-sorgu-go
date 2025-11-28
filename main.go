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
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// Config uygulama konfigürasyonu (hot-reload destekli)
type Config struct {
	mu             sync.RWMutex
	DNSServers     []string
	BlockedIPs     []string
	ServerLocation string
}

// Global config instance
var config = &Config{
	DNSServers:     []string{"195.175.39.39:53", "195.175.39.40:53"},
	BlockedIPs:     []string{"195.175.254.2", "2a01:358:4014:a00::3"},
	ServerLocation: "Unknown",
}

// GetDNSServers thread-safe DNS sunucu listesi
func (c *Config) GetDNSServers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, len(c.DNSServers))
	copy(result, c.DNSServers)
	return result
}

// GetBlockedIPs thread-safe engelli IP listesi
func (c *Config) GetBlockedIPs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, len(c.BlockedIPs))
	copy(result, c.BlockedIPs)
	return result
}

// GetServerLocation thread-safe sunucu lokasyonu
func (c *Config) GetServerLocation() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ServerLocation
}

// loadConfig .env dosyasından konfigürasyonu yükler
func (c *Config) loadConfig() error {
	// .env dosyasını yükle (varsa)
	_ = godotenv.Load()

	c.mu.Lock()
	defer c.mu.Unlock()

	// DNS Sunucuları
	if dnsServers := os.Getenv("BTK_DNS_SERVERS"); dnsServers != "" {
		servers := parseCommaSeparated(dnsServers)
		if len(servers) > 0 {
			// Port ekle (yoksa)
			for i, server := range servers {
				if !strings.Contains(server, ":") {
					servers[i] = server + ":53"
				}
			}
			c.DNSServers = servers
		}
	}

	// Engelli IP'ler
	if blockedIPs := os.Getenv("BTK_BLOCKED_IPS"); blockedIPs != "" {
		ips := parseCommaSeparated(blockedIPs)
		if len(ips) > 0 {
			c.BlockedIPs = ips
		}
	}

	// Sunucu Lokasyonu (boşlukları alt çizgiye çevir)
	if location := os.Getenv("SERVER_LOCATION"); location != "" {
		c.ServerLocation = strings.ReplaceAll(location, " ", "_")
	}

	return nil
}

// parseCommaSeparated virgülle ayrılmış string'i slice'a çevirir
func parseCommaSeparated(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// watchConfigFile .env dosyasını izler ve değişikliklerde yeniden yükler
func watchConfigFile() {
	envFile := ".env"
	var lastModTime time.Time

	// İlk mod time'ı al
	if info, err := os.Stat(envFile); err == nil {
		lastModTime = info.ModTime()
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		info, err := os.Stat(envFile)
		if err != nil {
			continue
		}

		if info.ModTime().After(lastModTime) {
			lastModTime = info.ModTime()
			log.Println("🔄 .env dosyası değişti, konfigürasyon yeniden yükleniyor...")

			if err := config.loadConfig(); err != nil {
				log.Printf("⚠️ Konfigürasyon yükleme hatası: %v", err)
			} else {
				log.Printf("✅ Konfigürasyon güncellendi:")
				log.Printf("   DNS Servers: %v", config.GetDNSServers())
				log.Printf("   Blocked IPs: %v", config.GetBlockedIPs())
				log.Printf("   Server Location: %s", config.GetServerLocation())
			}
		}
	}
}

// DNSResponse API response yapısı (sadeleştirilmiş, tekrarsız)
type DNSResponse struct {
	Domain         string   `json:"domain"`
	Timestamp      int64    `json:"timestamp"`
	Success        bool     `json:"success"`
	IsBlocked      bool     `json:"is_blocked"`
	DNSServer      string   `json:"dns_server,omitempty"`
	ResolvedIPs    []string `json:"resolved_ips,omitempty"`
	BlockedIP      string   `json:"blocked_ip,omitempty"`
	Error          string   `json:"error,omitempty"`
	QueryTime      string   `json:"query_time,omitempty"`
	ResponseTimeMs float64  `json:"response_time_ms,omitempty"`
	ServerLocation string   `json:"server_location,omitempty"`
}

// checkDomain belirtilen domain'in BTK tarafından engellenip engellenmediğini kontrol eder
func checkDomain(domain string) DNSResponse {
	startTime := time.Now()
	response := DNSResponse{
		Domain:    domain,
		Timestamp: time.Now().Unix(),
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

	// BTK DNS sunucularını dene (config'den al)
	for _, dnsServer := range config.GetDNSServers() {
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

	// Engel kontrolü (config'den al)
	isBlocked, blockedIP := checkIfBlocked(resolvedIPs, config.GetBlockedIPs())
	response.IsBlocked = isBlocked
	if isBlocked {
		response.BlockedIP = blockedIP
	}

	processingTime := time.Since(startTime)

	// Ek bilgiler
	response.QueryTime = time.Now().Format("15:04:05.000")
	response.ResponseTimeMs = float64(processingTime.Microseconds()) / 1000.0
	response.ServerLocation = config.GetServerLocation()

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

	ips, err := resolver.LookupHost(ctx, domain)
	if err != nil {
		return nil, err
	}

	return ips, nil
}

// checkIfBlocked IP listesinde BTK engel IP'si var mı kontrol eder
func checkIfBlocked(ips []string, blockedIPs []string) (bool, string) {
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

	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}

	return domain
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

	domain := r.URL.Query().Get("domain")

	if r.Method == "POST" {
		var req struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if domain == "" {
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

// handleConfig /config endpoint handler'ı - güncel konfigürasyonu gösterir
func handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"dns_servers":     config.GetDNSServers(),
		"blocked_ips":     config.GetBlockedIPs(),
		"server_location": config.GetServerLocation(),
		"hot_reload":      true,
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
			"GET /config":                "Güncel konfigürasyonu görüntüle",
		},
		"dns_servers": config.GetDNSServers(),
		"blocked_ips": config.GetBlockedIPs(),
		"features": map[string]interface{}{
			"hot_reload":         true,
			"config_file":        ".env",
			"reload_interval_ms": 2000,
		},
	})
}

func main() {
	// Konfigürasyonu yükle
	if err := config.loadConfig(); err != nil {
		log.Printf("⚠️ Konfigürasyon yükleme hatası: %v", err)
	}

	// Hot-reload için file watcher başlat
	go watchConfigFile()

	// Port (sadece başlangıçta okunur, hot-reload desteklemez)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/check", handleCheck)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/config", handleConfig)

	log.Println("🚀 BTK Engel Kontrol API başlatıldı")
	log.Printf("📡 Dinleniyor: http://localhost:%s", port)
	log.Println("📋 Endpoint: GET /check?domain=example.com")
	log.Println("🔄 Hot-reload: .env dosyası değişikliklerini otomatik algılar")
	log.Printf("⚙️  Konfigürasyon:")
	log.Printf("   DNS Servers: %v", config.GetDNSServers())
	log.Printf("   Blocked IPs: %v", config.GetBlockedIPs())
	log.Printf("   Server Location: %s", config.GetServerLocation())

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Sunucu başlatılamadı: %v", err)
	}
}
