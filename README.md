# BTK Engel Kontrol API

Türkiye'de BTK (Bilgi Teknolojileri ve İletişim Kurumu) DNS yanıtlarına göre domain engel durumunu kontrol eden Go HTTP API servisi.

## Özellikler

- Yapılandırılmış BTK DNS sunucuları üzerinden domain kontrolü
- IPv4 ve IPv6 engel IP adreslerini destekleme
- GET ve JSON gövdeli POST istekleri
- JSON response formatı
- `/health` sağlık kontrolü
- `/config` ile aktif runtime konfigürasyonunu görüntüleme
- `/check` endpoint'inde CORS desteği
- `.env` değişikliklerini uygulamayı yeniden başlatmadan algılayan hot-reload
- `SIGINT` ve `SIGTERM` için graceful shutdown
- Linux, Windows ve macOS için amd64 ve arm64 build desteği

## Nasıl Çalışır?

BTK tarafından engellenen domainler, BTK DNS sunucuları üzerinden çözümlendiğinde yapılandırılmış engel IP adreslerine yönlendirilebilir. API şu akışı izler:

1. Domain girdisinden `http://`, `https://` ve path bölümünü temizler. `www.` öneki korunur; `www.example.com` ile `example.com` ayrı DNS kayıtları olarak ayrı sorgulanabilir.
2. Domain formatını kontrol eder.
3. Yapılandırılmış DNS sunucularını sırayla dener.
4. Dönen IPv4 ve IPv6 adreslerini `BTK_BLOCKED_IPS` listesiyle karşılaştırır.

Bu yöntem yalnızca DNS bazlı engelleri tespit eder. IP veya SNI bazlı engelleri tespit etmez.

## Gereksinimler

- Go 1.24.4 veya uyumlu bir Go sürümü
- BTK DNS sunucularına ağ erişimi
- Doğru sonuçlar için Türkiye IP bloklarında çalışan bir sunucu

## Hızlı Başlangıç

```bash
git clone https://github.com/KilimcininKorOglu/btk-sorgu-go.git
cd btk-sorgu-go
cp .env.example .env
make run
```

Sunucu varsayılan olarak `http://localhost:8080` adresinde dinler.

Sadece build almak için:

```bash
make build
./bin/btk-sorgu_darwin_arm64 --version
```

`bin/` altındaki dosya adı işletim sistemi ve mimariye göre değişir. Tüm Make hedeflerini görmek için `make help` komutunu kullanın.

Windows'ta eşdeğer komutlar `build.bat build`, `build.bat run` ve `build.bat help` biçimindedir.

## Geliştirme Komutları

| Komut | Açıklama |
|---|---|
| `make build` | `bin/btk-sorgu_<GOOS>_<GOARCH>` binary'sini oluşturur. |
| `make run` | Build alır ve API sunucusunu başlatır. |
| `make test` | Tüm Go testlerini çalıştırır. |
| `make test-race` | Race detector ile test çalıştırır. |
| `make test-cover` | Coverage ile test çalıştırır. |
| `make test-verbose` | Testleri ayrıntılı çıktıyla çalıştırır. |
| `make bench` | Benchmark testlerini çalıştırır. |
| `make fmt` | Go dosyalarını formatlar. |
| `make vet` | `go vet ./...` çalıştırır. |
| `make lint` | Önce `go fmt ./...`, sonra `go vet ./...` çalıştırır. Kaynak dosyalarını değiştirebilir. |
| `make clean` | Build çıktısını kaldırır ve `go clean` çalıştırır. |

Şu anda repository'de `*_test.go` dosyası bulunmamaktadır. Test eklendiğinde tek bir testi çalıştırmak için `go test -run '^TestName$' ./...` kullanılabilir.

Build sırasında version bilgisi linker flags ile binary'ye eklenir. `--version` çıktısı version, commit ve build tarihini gösterir.

## API Endpoint'leri

### `GET /`

API adı, version bilgisi, endpoint listesi ve güncel konfigürasyonu döndürür.

### `GET /check?domain={domain}`

Query string içindeki domain'i kontrol eder.

```bash
curl "http://localhost:8080/check?domain=discord.com"
```

### `POST /check`

Domain'i JSON gövdesiyle kontrol eder:

```bash
curl -X POST "http://localhost:8080/check" \
  -H "Content-Type: application/json" \
  -d '{"domain":"discord.com"}'
```

Başarılı response örneği:

```json
{
  "domain": "discord.com",
  "timestamp": 1764196530,
  "success": true,
  "is_blocked": true,
  "dns_server": "195.175.39.40",
  "resolved_ips": ["195.175.254.2"],
  "blocked_ip": "195.175.254.2",
  "query_time": "12:34:56.789",
  "response_time_ms": 8.09,
  "server_location": "Turkey_VDS"
}
```

`success` değeri `false` olduğunda `error` alanı açıklama içerir. Geçersiz domain, bozuk JSON veya DNS çözümleme hataları HTTP 400 döndürür.

### `GET /health`

API'nin çalıştığını ve build version bilgisini döndürür:

```json
{
  "status": "healthy",
  "timestamp": 1764196530,
  "version": "1.0.1"
}
```

`version` değeri build sırasında linker flags ile enjekte edilir.

### `GET /config`

Aktif runtime konfigürasyonunu döndürür:

```json
{
  "dns_servers": ["195.175.39.39:53", "195.175.39.40:53"],
  "blocked_ips": ["195.175.254.2", "2a01:358:4014:a00::3"],
  "server_location": "Turkey_VDS",
  "hot_reload": true
}
```

### `OPTIONS /check`

CORS preflight isteklerini karşılar. `/check` endpoint'i `Access-Control-Allow-Origin: *` header'ını döndürür.

## Konfigürasyon

`.env.example` dosyasını `.env` olarak kopyalayın. Ayarlar aşağıdaki environment variable'lar üzerinden yapılır:

| Değişken | Varsayılan | Hot-reload | Açıklama |
|---|---|---|---|
| `PORT` | `8080` | Hayır | API'nin dinleyeceği port. Yalnızca başlangıçta okunur. |
| `SERVER_LOCATION` | `Unknown` | Evet | Response içindeki sunucu lokasyonu. Boşluklar `_` karakterine çevrilir. |
| `BTK_DNS_SERVERS` | `195.175.39.39,195.175.39.40` | Evet | Virgülle ayrılmış DNS sunucuları. Port belirtilmezse `:53` eklenir. |
| `BTK_BLOCKED_IPS` | `195.175.254.2,2a01:358:4014:a00::3` | Evet | Virgülle ayrılmış engel IPv4 ve IPv6 adresleri. |

Örnek `.env`:

```env
PORT=8080
SERVER_LOCATION=Turkey VDS
BTK_DNS_SERVERS=195.175.39.39,195.175.39.40
BTK_BLOCKED_IPS=195.175.254.2,2a01:358:4014:a00::3
```

`.env` dosyası iki saniyede bir kontrol edilir. `BTK_DNS_SERVERS`, `BTK_BLOCKED_IPS` veya `SERVER_LOCATION` değiştiğinde yeni değerler otomatik olarak yüklenir. `PORT` değişikliği için uygulamayı yeniden başlatmak gerekir.

## Build ve Release

GoReleaser yapılandırması `.goreleaser.yml` dosyasındadır. `v*` formatındaki bir tag push edildiğinde GitHub Actions release workflow'u çalışır ve şu platformlar için binary üretir:

- Linux, Windows ve macOS
- amd64 ve arm64
- `CGO_ENABLED=0`

Local binary doğrulaması:

```bash
make build
./bin/btk-sorgu_<os>_<arch> --version
```

## Otomatik Kurulum (Linux)

`install/install.sh`, en son release'i indirir, `/opt/btk-sorgu-go` altına kurar, systemd servisini başlatır, nginx reverse proxy'yi (80 -> 127.0.0.1:8080) yapılandırır ve firewall'da 80/443 portlarını açar. Servis portu (8080) yalnızca localhost'ta dinler. Ubuntu/Debian (ufw) ve RHEL/CentOS/Rocky (firewalld) desteklenir.

Güvenlik için script'i indirip inceledikten sonra çalıştırın:

```bash
curl -fsSLO https://raw.githubusercontent.com/KilimcininKorOglu/btk-sorgu-go/master/install/install.sh
less install.sh
sudo bash install.sh
```

Belirli bir sürüm için argüman verin: `sudo bash install.sh v1.0.2`

Non-interaktif kurulum ve Let's Encrypt TLS için environment değişkenleri:

```bash
sudo DOMAIN=sorgu.example.com EMAIL=admin@example.com ENABLE_SSL=1 bash install.sh
```

`ENABLE_SSL=1` ve `DOMAIN` verildiğinde certbot ile 443 ve otomatik yenileme etkinleştirilir. Interaktif çalıştırmada script HTTPS kurulumunu sorar.

## Linux systemd Kurulumu (Manuel)

`install/install.sh` systemd unit'ini otomatik üretir ve tek doğruluk kaynağıdır. Script kullanmadan elle kurmak isterseniz binary'yi yerleştirip aşağıdaki unit şablonunu kullanın. `<arch>` değerini mimarinize göre `amd64` veya `arm64` yapın.

```bash
sudo mkdir -p /opt/btk-sorgu-go
sudo cp bin/btk-sorgu_linux_<arch> /opt/btk-sorgu-go/
sudo cp .env.example /opt/btk-sorgu-go/.env
sudo chmod +x /opt/btk-sorgu-go/btk-sorgu_linux_<arch>
sudo nano /opt/btk-sorgu-go/.env
```

Unit dosyasını oluşturun (`/etc/systemd/system/btk-sorgu.service`). Ubuntu/Debian için `User=www-data` ve `ProtectSystem=strict` (+ `ReadWritePaths=/opt/btk-sorgu-go`); CentOS/RHEL/Rocky için `User=nobody` ve `ProtectSystem=full` kullanın:

```ini
[Unit]
Description=BTK Engel Kontrol API
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/opt/btk-sorgu-go
ExecStart=/opt/btk-sorgu-go/btk-sorgu_linux_<arch>
Restart=always
RestartSec=5
EnvironmentFile=/opt/btk-sorgu-go/.env
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/opt/btk-sorgu-go
StandardOutput=journal
StandardError=journal
SyslogIdentifier=btk-sorgu

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now btk-sorgu
```

SELinux etkinse (CentOS/RHEL/Rocky) gerekebilecek izinler:

```bash
sudo semanage fcontext -a -t bin_t "/opt/btk-sorgu-go/btk-sorgu_linux_<arch>"
sudo restorecon -v /opt/btk-sorgu-go/btk-sorgu_linux_<arch>
```

Servis yönetimi:

```bash
sudo systemctl status btk-sorgu
sudo systemctl restart btk-sorgu
sudo journalctl -u btk-sorgu -f
```

## Operasyonel Notlar

- BTK DNS sunucularına erişim yoksa DNS çözümleme timeout ile sonuçlanabilir. Resolver ve context timeout değerleri 5 saniyedir.
- `SERVER_LOCATION` yalnızca response bilgisi olarak kullanılır, sunucunun gerçek lokasyonunu doğrulamaz.
- `/config` endpoint'i aktif DNS sunucularını, engel IP'lerini ve server location bilgisini döndürür. Bu endpoint'i public erişime açmadan önce deployment gereksinimlerini değerlendirin.
- `/check` endpoint'inde CORS tüm origin'lere açıktır. Production ortamında bu davranışı ihtiyaçlarınıza göre sınırlandırın.
- API'de authentication veya authorization bulunmaz. Public erişim kararı reverse proxy, firewall veya uygulama katmanında ayrıca verilmelidir.
- `/check` her istekte dışa DNS sorgusu tetikler ve uygulama içi rate limiting yoktur. İstek başına DNS yükünü ve kötüye kullanımı sınırlamak için rate limiting'i reverse proxy (örneğin nginx `limit_req`) katmanında yapılandırın.
