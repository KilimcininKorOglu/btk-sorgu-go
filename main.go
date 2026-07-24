package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

// Build değişkenleri (ldflags ile enjekte edilir)
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// RFC 1035 uyumlu domain validasyon regex'i (bir kez derlenir)
var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

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
	_ = godotenv.Overload()

	c.mu.Lock()
	defer c.mu.Unlock()

	// DNS Sunucuları
	if dnsServers := os.Getenv("BTK_DNS_SERVERS"); dnsServers != "" {
		servers := parseCommaSeparated(dnsServers)
		if len(servers) > 0 {
			// Port ekle (yoksa)
			for i, server := range servers {
				if _, _, err := net.SplitHostPort(server); err != nil {
					if strings.Contains(server, ":") {
						// IPv6 adresi, port yok
						servers[i] = "[" + server + "]:53"
					} else {
						// IPv4 adresi, port yok
						servers[i] = server + ":53"
					}
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
	for item := range strings.SplitSeq(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// watchConfigFile .env dosyasını izler ve değişikliklerde yeniden yükler
func watchConfigFile() {
	// Panic recovery - goroutine çökerse logla
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WARN] watchConfigFile panic: %v", r)
			log.Println("[WARN] Hot-reload devre dışı kaldı, uygulama çalışmaya devam ediyor")
		}
	}()

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
			log.Println("[INFO] .env dosyası değişti, konfigürasyon yeniden yükleniyor...")

			if err := config.loadConfig(); err != nil {
				log.Printf("[WARN] Konfigürasyon yükleme hatası: %v", err)
			} else {
				log.Printf("[OK] Konfigürasyon güncellendi:")
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

	// Domain format validasyonu
	if !isValidDomain(domain) {
		response.Success = false
		response.Error = "Geçersiz domain formatı"
		return response
	}

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
		// Ayrıntılı resolver hatasını yalnızca sunucuda logla, istemciye genel mesaj dön
		if lastError != nil {
			log.Printf("[WARN] DNS çözümleme hatası (%s): %v", domain, lastError)
		}
		response.Error = "DNS çözümlemesi başarısız"
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
	lower := strings.ToLower(domain)
	if strings.HasPrefix(lower, "https://") {
		domain = domain[len("https://"):]
	} else if strings.HasPrefix(lower, "http://") {
		domain = domain[len("http://"):]
	}
	// www. önekini büyük/küçük harf duyarsız ayıkla
	if len(domain) >= 4 && strings.EqualFold(domain[:4], "www.") {
		domain = domain[4:]
	}
	domain = strings.TrimSuffix(domain, "/")

	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}

	return domain
}

// isValidDomain domain formatının geçerli olup olmadığını kontrol eder
func isValidDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}
	return domainRegex.MatchString(domain)
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
		// İstek gövdesini 1 KB ile sınırla (bellek tüketimini önle)
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// JSON parse hatası - kullanıcıya bildir
			if domain == "" {
				response := DNSResponse{
					Timestamp: time.Now().Unix(),
					Success:   false,
					Error:     "Geçersiz JSON formatı: " + err.Error(),
				}
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(response)
				return
			}
		} else if domain == "" {
			domain = req.Domain
		}
	}

	response := checkDomain(domain)

	// Hata durumunda uygun HTTP status code
	if !response.Success {
		w.WriteHeader(http.StatusBadRequest)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("JSON encode hatası: %v", err)
	}
}

// handleHealth /health endpoint handler'ı
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   version,
	}); err != nil {
		log.Printf("JSON encode hatası: %v", err)
	}
}

// handleConfig /config endpoint handler'ı - güncel konfigürasyonu gösterir
func handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"dns_servers":     config.GetDNSServers(),
		"blocked_ips":     config.GetBlockedIPs(),
		"server_location": config.GetServerLocation(),
		"hot_reload":      true,
	}); err != nil {
		log.Printf("JSON encode hatası: %v", err)
	}
}

// handleRoot / endpoint handler'ı
func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": "Not found",
		}); err != nil {
			log.Printf("JSON encode hatası: %v", err)
		}
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]any{
		"name":        "BTK Engel Kontrol API",
		"version":     version,
		"description": "Türkiye'de BTK tarafından engellenen domainleri kontrol eden API",
		"endpoints": map[string]string{
			"GET /check?domain={domain}": "Domain engel durumunu kontrol et",
			"GET /health":                "API sağlık durumu",
			"GET /config":                "Güncel konfigürasyonu görüntüle",
		},
		"dns_servers": config.GetDNSServers(),
		"blocked_ips": config.GetBlockedIPs(),
		"features": map[string]any{
			"hot_reload":         true,
			"config_file":        ".env",
			"reload_interval_ms": 2000,
		},
	}); err != nil {
		log.Printf("JSON encode hatası: %v", err)
	}
}

func main() {
	// --version flag kontrolü
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("btk-sorgu %s (commit: %s, built: %s)\n", version, commit, buildDate)
		return
	}

	// Konfigürasyonu yükle
	if err := config.loadConfig(); err != nil {
		log.Printf("[WARN] Konfigürasyon yükleme hatası: %v", err)
	}

	// Hot-reload için file watcher başlat
	go watchConfigFile()

	// Port (sadece başlangıçta okunur, hot-reload desteklemez)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/check", handleCheck)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/config", handleConfig)

	server := &http.Server{
		Addr:        ":" + port,
		Handler:     mux,
		ReadTimeout: 10 * time.Second,
		// En kötü durumda 2 DNS sunucusu x 5s = 10s; encode ve network yazımı için marj bırak
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown için signal handling
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("[INFO] Kapatma sinyali alındı, graceful shutdown başlatılıyor...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("[WARN] Graceful shutdown hatası: %v", err)
		}
	}()

	log.Println("[INFO] BTK Engel Kontrol API başlatıldı")
	log.Printf("[INFO] Dinleniyor: http://localhost:%s", port)
	log.Println("[INFO] Endpoint: GET /check?domain=example.com")
	log.Println("[INFO] Hot-reload: .env dosyası değişikliklerini otomatik algılar")
	log.Printf("[INFO] Konfigürasyon:")
	log.Printf("   DNS Servers: %v", config.GetDNSServers())
	log.Printf("   Blocked IPs: %v", config.GetBlockedIPs())
	log.Printf("   Server Location: %s", config.GetServerLocation())

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Sunucu başlatılamadı: %v", err)
	}

	log.Println("[INFO] Sunucu başarıyla kapatıldı")
}
