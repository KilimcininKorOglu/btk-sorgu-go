# BTK Engel Kontrol API

Türkiye'de BTK (Bilgi Teknolojileri ve İletişim Kurumu) tarafından engellenen web sitelerini tespit eden Go API servisi.

## 🚀 Özellikler

- BTK DNS sunucuları üzerinden domain engel kontrolü
- Hızlı response süreleri (~8ms)
- CORS desteği
- JSON API formatı
- Health check endpoint'i

## 📋 Nasıl Çalışır?

BTK, engellediği sitelerin DNS sorgularını `195.175.254.2` IP adresine yönlendirir. Bu API, belirtilen domain'i BTK DNS sunucuları (`195.175.39.39`, `195.175.39.40`) üzerinden sorgulayarak bu IP'nin döndürülüp döndürülmediğini kontrol eder.

## 🔧 Kurulum

```bash
# Repository'yi klonla
git clone https://github.com/btk-sorgu-go.git
cd btk-sorgu-go

# Çalıştır
go run main.go

# Veya build et
go build -o btk-sorgu-go
./btk-sorgu-go
```

## 🌐 API Endpoint'leri

### GET /
API bilgilerini döndürür.

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
  "method": "dns_turkey",
  "dns_server": "195.175.39.40",
  "resolved_ips": ["195.175.254.2"],
  "blocked_ip": "195.175.254.2",
  "data": {
    "query_time": "01:35:30.077",
    "response_time": "8.09173ms",
    "record_type": "A",
    "all_ips": ["195.175.254.2"],
    "is_blocked_ip": true,
    "source": "my-server"
  },
  "api_info": {
    "processing_time": 0.008820954,
    "method": "dns_turkey",
    "server_location": "Turkey VDS"
  }
}
```

**Örnek Response (Engelsiz Site):**
```json
{
  "domain": "google.com",
  "timestamp": 1764196530,
  "success": true,
  "is_blocked": false,
  "method": "dns_turkey",
  "dns_server": "195.175.39.39",
  "resolved_ips": ["142.250.185.238"],
  "data": {
    "query_time": "01:35:30.077",
    "response_time": "5.123456ms",
    "record_type": "A",
    "all_ips": ["142.250.185.238"],
    "is_blocked_ip": false,
    "source": "my-server"
  },
  "api_info": {
    "processing_time": 0.005123456,
    "method": "dns_turkey",
    "server_location": "Turkey VDS"
  }
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

## ⚙️ Ortam Değişkenleri

| Değişken | Varsayılan | Açıklama |
|----------|------------|----------|
| `PORT` | `8080` | API'nin dinleyeceği port |
| `SERVER_LOCATION` | `Unknown` | Sunucu lokasyonu bilgisi |

## ⚠️ Önemli Notlar

1. **Sunucu Lokasyonu**: Bu API'nin doğru çalışması için sunucunun Türkiye IP bloklarında olması gerekir.

2. **DNS Yönlendirmesi**: Sunucunun DNS'i BTK DNS sunucularına yönlendirilmelidir:
   ```bash
   sudo resolvectl dns ens32 195.175.39.39 195.175.39.40
   ```

3. **Engel Türleri**: Bu yöntem sadece DNS bazlı engelleri tespit eder. IP/SNI bazlı engeller bu yöntemle tespit edilemez.

4. **Timeout**: BTK DNS sunucularına erişilemezse timeout hatası alınabilir.

## 📄 Lisans

MIT License
