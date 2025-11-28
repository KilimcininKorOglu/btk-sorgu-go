# Kurulum Rehberi

## Ubuntu / Debian

```bash
# Binary'yi kopyala
sudo mkdir -p /opt/btk-sorgu-go
sudo cp btk-sorgu-linux-amd64 /opt/btk-sorgu-go/
sudo cp .env.example /opt/btk-sorgu-go/.env
sudo chmod +x /opt/btk-sorgu-go/btk-sorgu-linux-amd64

# .env dosyasını düzenle
sudo nano /opt/btk-sorgu-go/.env

# Systemd servisini kur
sudo cp install/btk-sorgu.service.ubuntu /etc/systemd/system/btk-sorgu.service
sudo systemctl daemon-reload
sudo systemctl enable btk-sorgu
sudo systemctl start btk-sorgu

# Durum kontrolü
sudo systemctl status btk-sorgu
sudo journalctl -u btk-sorgu -f
```

## CentOS / RHEL / Rocky Linux

```bash
# Binary'yi kopyala
sudo mkdir -p /opt/btk-sorgu-go
sudo cp btk-sorgu-linux-amd64 /opt/btk-sorgu-go/
sudo cp .env.example /opt/btk-sorgu-go/.env
sudo chmod +x /opt/btk-sorgu-go/btk-sorgu-linux-amd64

# .env dosyasını düzenle
sudo nano /opt/btk-sorgu-go/.env

# Systemd servisini kur
sudo cp install/btk-sorgu.service.centos /etc/systemd/system/btk-sorgu.service
sudo systemctl daemon-reload
sudo systemctl enable btk-sorgu
sudo systemctl start btk-sorgu

# SELinux izinleri (gerekirse)
sudo semanage fcontext -a -t bin_t "/opt/btk-sorgu-go/btk-sorgu-linux-amd64"
sudo restorecon -v /opt/btk-sorgu-go/btk-sorgu-linux-amd64

# Durum kontrolü
sudo systemctl status btk-sorgu
sudo journalctl -u btk-sorgu -f
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
