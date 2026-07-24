# Kurulum Rehberi

## Otomatik Kurulum

Tek komutla indirme, kurulum, systemd servisi, nginx reverse proxy ve firewall (80/443) için `install.sh` kullanın. Servis portu (8080) yalnızca localhost'ta kalır.

```bash
curl -fsSLO https://raw.githubusercontent.com/KilimcininKorOglu/btk-sorgu-go/master/install/install.sh
less install.sh
sudo bash install.sh
```

Belirli sürüm: `sudo bash install.sh v1.0.2`. Let's Encrypt TLS için:

```bash
sudo DOMAIN=sorgu.example.com EMAIL=admin@example.com ENABLE_SSL=1 bash install.sh
```

Aşağıdaki manuel adımlar, script kullanmadan kurmak isteyenler için referanstır. systemd unit'i `install.sh` tarafından üretilir; elle kurulumda ana `README.md`'deki "Linux systemd Kurulumu (Manuel)" bölümündeki unit şablonunu kullanın. `<arch>` = `amd64` veya `arm64`.

## Ubuntu / Debian

Binary'yi yerleştirin, `.env`'i hazırlayın, unit'i oluşturun (`User=www-data`, `ProtectSystem=strict`, `ReadWritePaths=/opt/btk-sorgu-go`):

```bash
sudo mkdir -p /opt/btk-sorgu-go
sudo cp btk-sorgu_linux_<arch> /opt/btk-sorgu-go/
sudo cp .env.example /opt/btk-sorgu-go/.env
sudo chmod +x /opt/btk-sorgu-go/btk-sorgu_linux_<arch>
sudo nano /opt/btk-sorgu-go/.env

# Unit şablonu için ana README'ye bakın, sonra:
sudo systemctl daemon-reload
sudo systemctl enable --now btk-sorgu
```

## CentOS / RHEL / Rocky Linux

Ubuntu ile aynı; unit'te `User=nobody` ve `ProtectSystem=full` kullanın. SELinux etkinse:

```bash
sudo semanage fcontext -a -t bin_t "/opt/btk-sorgu-go/btk-sorgu_linux_<arch>"
sudo restorecon -v /opt/btk-sorgu-go/btk-sorgu_linux_<arch>
```

## Servis Yönetimi

```bash
# Başlat / Durdur / Yeniden başlat
sudo systemctl start btk-sorgu
sudo systemctl stop btk-sorgu
sudo systemctl restart btk-sorgu

# Logları görüntüle
sudo journalctl -u btk-sorgu -f

# Servis durumu
sudo systemctl status btk-sorgu
```

## Farklar

| Özellik | Ubuntu | CentOS |
|---------|--------|--------|
| Kullanıcı | `www-data` | `nobody` |
| ProtectSystem | `strict` | `full` |
| SELinux | Yok | Gerekebilir |
