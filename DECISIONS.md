# Kararlar

Bu dosya Smart Campaign Composer'daki önemli teknik ve ürün kararlarını, gerekçeleri ve bedelleriyle birlikte kaydeder. Kararlar silinmez; değişen karar yeni bir kayıtla yerini alır.

| No | Karar | Alan | Durum |
|----|-------|------|-------|
| D-001 | Ham stub'ın deterministik üreticiyle segmentasyon verisine zenginleştirilmesi | Mock provider ve seed | Kabul edildi |
| D-002 | Backend dili Go | Takip ve altyapı | Kabul edildi |
| D-003 | Tek Postgres 18 servisi, servis başına veritabanı ve sahip rol | Takip ve altyapı | Kabul edildi |
| D-004 | customer_db filtrelenmiş DWH: yıldız şema, kart/CVV/IBAN hariç | Kitle ve izinler | Kabul edildi |
| D-005 | Şema ve seed Postgres init scriptleriyle yüklenir | Mock provider ve seed | Kabul edildi |

---

## D-001: Ham stub'ın deterministik üreticiyle segmentasyon verisine zenginleştirilmesi

- **Durum:** Kabul edildi · 2026-10-26
- **Alan:** Mock provider ve seed
- **Bağlam:** Brief "birkaç bin kullanıcılık seed" ve örnek olarak "son 30 günde sepete ekleyip almayan İstanbul'daki kullanıcılar" istiyor. Elimizdeki `stub/acw_user_data.csv` 1000 satırlık İngiliz Faker export'u: email, telefon, kanal izni, alışveriş geçmişi yok; `Address City` 980 benzersiz değer (şehir filtresi anlamsız).
- **Karar:** Ham dosya `stub/source/acw_user_data.csv` olarak dondurulur. `stub/generate_stub_data.py` (stdlib, `SEED=20261026`, `AS_OF=2026-10-26`) bundan `stub/acw_user_data.csv` (orijinal 23 kolon + 32 kolon) ve 8 ilişkili CSV üretir: products, orders, order_items, events (son 90 gün), cart_items (30 gün sonra `abandoned`), consents (olay günlüğü; son satır güncel izin), campaigns, campaign_messages. `City` yeniden çekilir: TR %60 / GB %40, nüfus ağırlıklı (İstanbul TR'nin %35'i); `Language` tr/en; tek para birimi GBP; başlıklar orijinalle uyumlu Title Case. Ayrıntı: `stub/README.md`.
- **Alternatifler:**
  - Faker şehirlerini korumak — reddedildi: şehir filtresi en fazla 2 kişi döndürür, örnek brief gösterilemez.
  - Tüm kullanıcıları TR yapmak — reddedildi: isimler/posta kodları İngiliz; TR/GB karışımı "dile göre içerik" bonusunu da test edilebilir kılar.
  - `faker` kütüphanesiyle sıfırdan üretmek — reddedildi: kurulu değil, verilen stub'ı temel almak istendi, stdlib ile determinizm daha basit.
- **Sonuçlar / bedel:** TR kullanıcılarının İngiliz isim/posta kodu taşıması tutarsız ama sahte veri. 1000 kullanıcı "birkaç bin" değil; seed aşamasında çoğaltma gerekebilir. Sepet dönüşümü kalem bazında ~%53 (sektör ~%30). Kart/CVV/IBAN kolonları stub'da duruyor; seed importunda bilinçli olarak dışarıda bırakılmalı (ayrı karar).
- **Doğrulama:** `python3 -I stub/generate_stub_data.py` sonunda örnek brief hunisini basar (`195 -> 90 -> 49 -> 33 sms / 33 email`); iki çalıştırma aynı md5'leri verir. FK/benzersiz ID/izin tutarlılığı/sipariş toplamı kontrolleri bu oturumda elle yapıldı, henüz test değil.
- **AI notu:** Claude'un ilk tasarımı ham kolonlara bakıp bankacılık odaklıydı (hesap/kredi/sigorta tabloları); CLAUDE.md okununca kampanya senaryosuna göre yeniden kuruldu. İlk üretimde sepet dönüşümü %90 ve Platinum oranı %36 çıktı; doğrulama çıktısına göre iki tur ayarlandı.

- **Dipnot:** Data Kaynağı olarak kaggle üzerinden https://www.kaggle.com/datasets/msnbehdani/customer-data kullanılıp genişletilmiştir
## D-002: Backend dili Go

- **Durum:** Kabul edildi · 2026-10-26
- **Alan:** Takip ve altyapı
- **Bağlam:** Case backend için Go veya PHP'ye izin veriyor. Gönderim tarafında asenkron kuyruk, kanal başına 100 msg/s rate limit ve retry'lı worker'lar gerekiyor; canlı durum için SSE/WS akışı sürekli açık bağlantı demek.
- **Karar:** Backend Go. `go.mod`: `module github.com/furknozg/coldbru`, `go 1.27.0`. Kullanıcı tarafından oluşturuldu, bu oturumda onaylandı.
- **Alternatifler:**
  - PHP (Laravel) — reddedildi: kuyruk/migration/seed hazır gelse de eşzamanlı worker'lar, token bucket ve uzun süreli SSE bağlantıları için ek süreç altyapısı gerekiyor.
- **Sonuçlar / bedel:** Admin paneli, ORM ve migration araçları PHP ekosistemine göre daha az "hazır"; migration ve seed komutları elle yazılacak. Tek binary + goroutine ile worker ve rate limiter aynı süreçte kalır, dağıtık limit gerekirse ayrı karar.
- **Doğrulama:** Doğrulanmadı — henüz Go kodu yok; `go build ./...` ilk kod geldiğinde.

## D-003: Tek Postgres 18 servisi, servis başına veritabanı ve sahip rol

- **Durum:** Kabul edildi · 2026-10-26
- **Alan:** Takip ve altyapı
- **Bağlam:** Mikroservis mimarisi gösterilecek ama demo tek `docker compose up` ile kalkmalı. Servisler birbirinin verisine doğrudan dokunmamalı.
- **Karar:** `docker-compose.yml`'de tek `postgres:18-alpine` (18.6) servisi. Her servis ayrı `DATABASE` ve onun sahibi tek bir rol alır; `REVOKE ALL ON DATABASE … FROM PUBLIC`. Bu aşamada yalnızca `customer_db` / `customer_svc` kuruldu. Diğer servisler müşteri verisine customer service üzerinden ulaşacak; ayrı salt-okuma rolü açılmadı (kullanıcı: "customer svc ile etkileşim olacak, oralara daha girme"). Host portu `5433` (makinedeki 5432 başka projede), `POSTGRES_HOST_PORT` ile değişir. Ayrıntı: `docs/database-architecture.md`.
- **Alternatifler:**
  - Servis başına ayrı Postgres konteyneri — reddedildi: demo için kaynak ve kurulum maliyeti, izolasyonu rol/DB düzeyi zaten gösteriyor.
  - Tek DB, servis başına şema — reddedildi: şemalar arası `JOIN` mümkün kalır, servis sınırı gevşer.
  - `postgres:17-alpine` — reddedildi: 18 güncel ana sürüm.
- **Sonuçlar / bedel:** Tüm servisler aynı süreç, bellek ve diski paylaşır. Veritabanları arası `JOIN` yok; çapraz servis verisi API ile gelmeli. Postgres 18 imajında volume yolu `/var/lib/postgresql` (eski örneklerle farklı).
- **Doğrulama:** `docker compose up -d` → `healthy`; `customer_svc` dışındaki bir rol "User does not have CONNECT privilege" alıyor; tüm `dwh` tablolarının sahibi `customer_svc`; host'ta 5433 açık; restart sonrası veri duruyor, init yeniden çalışmıyor (bu oturumda elle).

## D-004: customer_db filtrelenmiş DWH: yıldız şema, kart/CVV/IBAN hariç

- **Durum:** Kabul edildi · 2026-10-26
- **Alan:** Kitle ve izinler
- **Bağlam:** Segment kuralları hem özet alanlarla ("açık sepeti var") hem zaman pencereli davranışla ("son 30 günde sepete ekledi") yazılabilmeli. Ham stub kart, CVV ve IBAN içeriyor.
- **Karar:** `customer_db.dwh` şemasında `dim_customer` (ham+zenginleştirilmiş 55 kolondan kart başlangıç/bitiş, kart no, CVV, IBAN çıkarılmış 50 kolon), `dim_product`, `fact_order`, `fact_order_item` (PK `order_id, line_no`; 100 siparişte aynı ürün iki kez geçiyor), `fact_cart_item`, `fact_event`, `fact_consent`. Kampanya stub'ları (`acw_campaigns`, `acw_campaign_messages`) yüklenmedi. Kategorik kolonlarda `CHECK`, segment filtreleri için indeksler. Boş string → `NULL`, `employer_company = 'N/A'` → `NULL`.
- **Alternatifler:**
  - Tek geniş müşteri tablosu — reddedildi: zaman pencereli kurallar yazılamaz.
  - Sadece kampanya için gereken kolonlar — reddedildi: kullanıcı maaş/araç gibi alanlarla daha zengin segment denemek istedi.
- **Sonuçlar / bedel:** Maaş, emeklilik, işveren, araç ve ham UK adresi gibi gereksiz kişisel veri DB'de; LLM alan beyaz listesi bunları bilinçli olarak dışarıda tutmalı (ayrı karar). `dim_customer`'daki kampanya özeti kolonları, kampanya tabloları olmadan stub'dan gelen anlık görüntüdür.
- **Doğrulama:** Örnek brief sorgusu (İstanbul, son 30 günde sepete ekleyip almayan) DB'de `49 → 33 SMS / 33 email` veriyor, üreticinin hunisiyle aynı. `information_schema`'da `card|cvv|iban` adlı kolon yok. Bu oturumda elle; otomatik test henüz yok.

## D-005: Şema ve seed Postgres init scriptleriyle yüklenir

- **Durum:** Kabul edildi · 2026-10-26
- **Alan:** Mock provider ve seed
- **Bağlam:** `docker compose up` seed dahil tek komutla kalkmalı; henüz Go kodu ve migration aracı yok.
- **Karar:** `infra/postgres/initdb/00-init.sh` sırayla `sql/01_customer_db.sql` (rol+DB), `02_customer_schema.sql` (şema, `customer_svc` olarak), `03_customer_seed.sql` (tek transaction) çalıştırır. `stub/` konteynere `/seed/stub` olarak salt-okunur bağlanır. Her CSV, kolon adları CSV başlığıyla aynı `TEMP … ON COMMIT DROP` tabloya `COPY … HEADER MATCH` ile alınır, tiplenip `dwh`'ye yazılır. Seed sonunda boş tablo, izin bayrağı ↔ son `fact_consent` kaydı ve sipariş ara toplamı ↔ satır toplamı kontrolleri; biri tutmazsa transaction geri alınır. Healthcheck `customer_svc` ile TCP'den `dwh.dim_customer` okur.
- **Alternatifler:**
  - Go migrate + seed servisi — reddedildi: şimdi kütüphane seçimi ve kod gerektirir.
  - Init + ayrı psql seed servisi — reddedildi: ek servis; yeniden seed ihtiyacı henüz yok.
- **Sonuçlar / bedel:** Init yalnızca boş volume'da çalışır; şema/stub değişince `docker compose down -v`. Init yarıda kalırsa volume yarım kalır, yine `down -v` gerekir. Şema evrimi için ileride migration aracına geçilecek.
- **Doğrulama:** İlk `up` 3 sn'de seed'i bitirdi (1000 / 159 / 3923 / 9170 / 3840 / 44114 / 2219 satır, CSV'lerle aynı). İlk kullanıcının IBAN ve kart numarası veri dizininde bulunmuyor; aynı aramayla e-postası bulunuyor (pozitif kontrol). Bu oturumda elle.

---

## Açık konular

- customer service'in diğer servislere veri sunma biçimi (REST / gRPC / olay); campaign_db ve delivery_db
- Migration aracı (init scriptleri yalnızca ilk kurulum için, D-005)
- LLM segment kuralında hangi `dim_customer` kolonlarının beyaz listede olacağı (maaş, araç, adres gibi alanlar DB'de var, D-004)
- Kuyruk teknolojisi ve kanal başına 100 msg/s rate limit yöntemi
- Idempotency anahtarı ve retry stratejisi
- LLM yapılandırılmış çıktı yöntemi ve segment kuralı DSL'i
- Mock provider hata/gecikme/429 oranları
- Seed: 1000 kullanıcının "birkaç bin"e nasıl çıkarılacağı; kampanya stub'larının ne zaman ve hangi DB'ye yükleneceği
- Canlı durum taşıma yöntemi (SSE / WebSocket / polling)
