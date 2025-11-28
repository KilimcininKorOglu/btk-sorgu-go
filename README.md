# BTK Engel Kontrol API

Türkiye'de BTK (Bilgi Teknolojileri ve İletişim Kurumu) tarafından engellenen web sitelerini tespit eden Go API servisi.

## 🚀 Özellikler

- BTK DNS sunucuları üzerinden domain engel kontrolü
- Hızlı response süreleri (~8ms)
- CORS desteği
- JSON API formatı
- Health check endpoint'i
- **Hot-reload**: `.env` dosyası değişikliklerini otomatik algılar (uygulama yeniden başlatmaya gerek yok)

## 📋 Nasıl Çalışır?

BTK, engellediği sitelerin DNS sorgularını `195.175.254.2` IP adresine yönlendirir. Bu API, belirtilen domain'i BTK DNS sunucuları üzerinden sorgulayarak bu IP'nin döndürülüp döndürülmediğini kontrol eder.

## 🔧 Kurulum

```bash
# Repository'yi klonla
git clone https://github.com/KilimcininKorOglu/btk-sorgu-go.git
cd btk-sorgu-go

# Konfigürasyon dosyasını oluştur
cp .env.example .env

# Çalıştır
go run main.go

# Veya build et
go build -o btk-sorgu-go
./btk-sorgu-go
```

### Cross-Platform Build

```bash
# Windows'ta tüm platformlar için build
build.bat
```

Build çıktıları `build/` klasöründe oluşturulur:

- `btk-sorgu-windows-amd64.exe`
- `btk-sorgu-windows-arm64.exe`
- `btk-sorgu-linux-amd64`
- `btk-sorgu-linux-arm64`

## 🌐 API Endpoint'leri

### GET /

API bilgilerini ve güncel konfigürasyonu döndürür.

### GET /check?domain={domain}

Domain'in engel durumunu kontrol eder.

**Parametreler:**

- `domain` (required): Kontrol edilecek domain (örn: discord.com)

**Örnek İstek:**

```bash
curl "http://localhost:8080/check?domain=discord.com"
```

**Örnek Response (Engelli Site):**

```json
{
  "domain": "discord.com",
  "timestamp": 1764196530,
  "success": true,
  "is_blocked": true,
  "dns_server": "195.175.39.40",
  "resolved_ips": ["195.175.254.2"],
  "blocked_ip": "195.175.254.2",
  "query_time": "01:35:30.077",
  "response_time_ms": 8.09,
  "server_location": "Turkey_VDS"
}
```

**Örnek Response (Engelsiz Site):**

```json
{
  "domain": "google.com",
  "timestamp": 1764196530,
  "success": true,
  "is_blocked": false,
  "dns_server": "195.175.39.39",
  "resolved_ips": ["142.250.185.238"],
  "query_time": "01:35:30.077",
  "response_time_ms": 5.12,
  "server_location": "Turkey_VDS"
}
```

### GET /health

API sağlık durumunu kontrol eder.

```json
{
  "status": "healthy",
  "timestamp": 1764196530,
  "version": "1.0.0"
}
```

### GET /config

Güncel konfigürasyonu görüntüler.

```json
{
  "dns_servers": ["195.175.39.39:53", "195.175.39.40:53"],
  "blocked_ips": ["195.175.254.2", "2a01:358:4014:a00::3"],
  "server_location": "Turkey_VDS",
  "hot_reload": true
}
```

## ⚙️ Konfigürasyon (.env)

Tüm ayarlar `.env` dosyasından okunur. `.env.example` dosyasını `.env` olarak kopyalayın ve düzenleyin.

| Değişken | Varsayılan | Hot-Reload | Açıklama |
|----------|------------|------------|----------|
| `PORT` | `8080` | ❌ | API'nin dinleyeceği port (sadece başlangıçta okunur) |
| `SERVER_LOCATION` | `Unknown` | ✅ | Sunucu lokasyonu (boşluklar otomatik `_` olur) |
| `BTK_DNS_SERVERS` | `195.175.39.39,195.175.39.40` | ✅ | BTK DNS sunucuları (virgülle ayrılmış) |
| `BTK_BLOCKED_IPS` | `195.175.254.2,2a01:358:4014:a00::3` | ✅ | Engel IP adresleri (virgülle ayrılmış) |

**Örnek .env:**

```env
PORT=8080
SERVER_LOCATION=Turkey VDS
BTK_DNS_SERVERS=195.175.39.39,195.175.39.40
BTK_BLOCKED_IPS=195.175.254.2,2a01:358:4014:a00::3
```

> **Not:** `SERVER_LOCATION=Turkey VDS` yazarsanız, sistem otomatik olarak `Turkey_VDS` olarak dönüştürür.

### 🔄 Hot-Reload

`.env` dosyası her 2 saniyede bir kontrol edilir. Değişiklik algılandığında konfigürasyon otomatik olarak güncellenir - uygulamayı yeniden başlatmanıza gerek yoktur.

```text
🔄 .env dosyası değişti, konfigürasyon yeniden yükleniyor...
✅ Konfigürasyon güncellendi:
   DNS Servers: [195.175.39.39:53 195.175.39.40:53]
   Blocked IPs: [195.175.254.2 2a01:358:4014:a00::3]
   Server Location: Turkey_VDS
```

## 🐧 Linux Servis Kurulumu (systemd)

`install/` klasöründe Ubuntu ve CentOS için hazır systemd dosyaları bulunur.

### Ubuntu / Debian

```bash
# Binary ve config'i kopyala
sudo mkdir -p /opt/btk-sorgu-go
sudo cp build/btk-sorgu-linux-amd64 /opt/btk-sorgu-go/
sudo cp .env.example /opt/btk-sorgu-go/.env
sudo chmod +x /opt/btk-sorgu-go/btk-sorgu-linux-amd64

# .env dosyasını düzenle
sudo nano /opt/btk-sorgu-go/.env

# Servisi kur ve başlat
sudo cp install/btk-sorgu.service.ubuntu /etc/systemd/system/btk-sorgu.service
sudo systemctl daemon-reload
sudo systemctl enable btk-sorgu
sudo systemctl start btk-sorgu
```

### CentOS / RHEL / Rocky Linux

```bash
# Binary ve config'i kopyala
sudo mkdir -p /opt/btk-sorgu-go
sudo cp build/btk-sorgu-linux-amd64 /opt/btk-sorgu-go/
sudo cp .env.example /opt/btk-sorgu-go/.env
sudo chmod +x /opt/btk-sorgu-go/btk-sorgu-linux-amd64

# .env dosyasını düzenle
sudo nano /opt/btk-sorgu-go/.env

# Servisi kur ve başlat
sudo cp install/btk-sorgu.service.centos /etc/systemd/system/btk-sorgu.service
sudo systemctl daemon-reload
sudo systemctl enable btk-sorgu
sudo systemctl start btk-sorgu

# SELinux izinleri (gerekirse)
sudo semanage fcontext -a -t bin_t "/opt/btk-sorgu-go/btk-sorgu-linux-amd64"
sudo restorecon -v /opt/btk-sorgu-go/btk-sorgu-linux-amd64
```

### Servis Yönetimi

```bash
sudo systemctl status btk-sorgu    # Durum
sudo systemctl restart btk-sorgu   # Yeniden başlat
sudo journalctl -u btk-sorgu -f    # Logları izle
```

## ⚠️ Önemli Notlar

1. **Sunucu Lokasyonu**: Bu API'nin doğru çalışması için sunucunun Türkiye IP bloklarında olması gerekir.

2. **DNS Yönlendirmesi**: Sunucunun DNS'i BTK DNS sunucularına yönlendirilmelidir:

   ```bash
   sudo resolvectl dns ens32 195.175.39.39 195.175.39.40
   ```

3. **Engel Türleri**: Bu yöntem sadece DNS bazlı engelleri tespit eder. IP/SNI bazlı engeller bu yöntemle tespit edilemez.

4. **Timeout**: BTK DNS sunucularına erişilemezse timeout hatası alınabilir (5 saniye).
