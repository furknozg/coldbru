# Kararlar

Bu dosya Smart Campaign Composer'daki önemli teknik ve ürün kararlarını, gerekçeleri ve bedelleriyle birlikte kaydeder. Kararlar silinmez; değişen karar yeni bir kayıtla yerini alır.

| No | Karar | Alan | Durum |
|----|-------|------|-------|
| D-001 | Ham stub'ın deterministik üreticiyle segmentasyon verisine zenginleştirilmesi | Mock provider ve seed | Kabul edildi |
| D-002 | Backend dili Go | Takip ve altyapı | Kabul edildi |
| D-003 | Tek Postgres 18 servisi, servis başına veritabanı ve sahip rol | Takip ve altyapı | Kabul edildi |
| D-004 | customer_db filtrelenmiş DWH: yıldız şema, kart/CVV/IBAN hariç | Kitle ve izinler | Kabul edildi |
| D-005 | Şema ve seed Postgres init scriptleriyle yüklenir | Mock provider ve seed | Kabul edildi |
| D-006 | Tüm servisler arası mesajlaşma, iş kuyruğu ve zamanlama NATS JetStream; yayın transactional outbox ile | Gönderim | Kabul edildi |
| D-007 | Customer Service GraphQL (gqlgen); LLM JS benzeri DSL yazar, Proposal GraphQL'e çevirir; MCP yok | LLM entegrasyonu | Kabul edildi; ayrıştırıcı kısmı yerini aldı → D-038 |
| D-008 | Alıcı verisi zamanlamada NATS snapshot'ıyla taşınır; izin segment sorgusunda baştan uygulanır | Kitle ve izinler | Yerini aldı → D-010, D-011 |
| D-009 | İçerik ve zamanlama alıcının diline göre; tüm diller onaylanmadan teklif onaylanmaz | LLM entegrasyonu | Kabul edildi |
| D-010 | Kanal planı ayrı onay adımı; snapshot kişi başına atanmış kanalı taşır | Kitle ve izinler | Kabul edildi |
| D-011 | Gönderim anında GraphQL ile batch izin kontrolü; circuit breaker ile fail-closed | Kitle ve izinler | Kabul edildi |
| D-012 | Frontend: Vue 3 + TypeScript, pnpm, Tailwind, shadcn-vue, Vue Router + composable'lar; kök dizinde `web/` | Takip ve altyapı | Kabul edildi |
| D-013 | Frontend ↔ backend: istek/yanıt REST, canlı güncellemeler SSE | Takip ve altyapı | Kabul edildi |
| D-014 | Frontend testleri Vitest + Vue Test Utils; uçtan uca tarayıcı testi yok | Test ve kapsam | Kabul edildi |
| D-015 | Tek nginx konteyneri: Vue statik build'ini sunar ve API gateway'dir | Takip ve altyapı | Kabul edildi |
| D-016 | REST sözleşmesi Go kodundan swag v2 ile OpenAPI 3; TS tipleri ve istemci spec'ten üretilir | Takip ve altyapı | Kabul edildi |
| D-017 | Seed zaman damgaları yüklemede bugüne kaydırılır | Mock provider ve seed | Kabul edildi |
| D-018 | Dört servis ve mock provider, her biri ayrı konteyner | Takip ve altyapı | Kabul edildi |
| D-019 | proposal_db, notification_db, analytics_db; rol ve boş DB initdb'de açılır | Takip ve altyapı | Kabul edildi |
| D-020 | Migration ve DB erişimi goose + sqlc + pgx/v5 | Takip ve altyapı | Kabul edildi |
| D-021 | Tek Go modülü; cmd/ altında servis başına binary, internal/ altında kod | Takip ve altyapı | Kabul edildi |
| D-022 | Teklif modeli: campaign → proposal_step → proposal_variant | LLM entegrasyonu | Kabul edildi |
| D-023 | Teslim modeli: UNIQUE(campaign_id, customer_id, channel), durum makinesi, Idempotency-Key | Gönderim | Kabul edildi |
| D-024 | NATS düzeni: dört stream, teslimat başına ileti, kanal başına 100/s limiter, NAK ile retry | Gönderim | Kabul edildi |
| D-025 | Backend testleri: birim sahte, entegrasyon testcontainers-go, mock LLM | Test ve kapsam | Kabul edildi |
| D-026 | Modeller: segment ve metin Sonnet 5.5, enjeksiyon ön elemesi Haiku 5.5 | LLM entegrasyonu | Kabul edildi |
| D-027 | Yapılandırılmış çıktı: segmentte strict tool döngüsü, metinde native structured outputs | LLM entegrasyonu | Kabul edildi |
| D-028 | SMS en fazla 160 karakter; kodlama ve segment sayısı gösterilir | LLM entegrasyonu | Kabul edildi |
| D-029 | Proje iskeleti bu adımda yalnız klasörler ve domain belgesi; Go kodu yok | Test ve kapsam | Kabul edildi |
| D-030 | Yeni servis tablolarında birincil anahtar ve olay kimliği UUIDv7 | Takip ve altyapı | Kabul edildi |
| D-031 | Go servis altyapısı: chi v5 router, log/slog, Viper ile yapılandırma | Takip ve altyapı | Kabul edildi |
| D-032 | Vue projesi create-vue ile kurulur; kod özellik klasörlerinde | Takip ve altyapı | Kabul edildi |
| D-033 | Kampanya durumu kaba yaşam döngüsü; adım durumu ayrı | LLM entegrasyonu | Kabul edildi |
| D-034 | Tüm adımlar tek varyant tablosunda; adım türüne göre CHECK | LLM entegrasyonu | Kabul edildi |
| D-035 | NATS olayları kendi JSON zarfıyla; konu evt.<servis>.<olay>.v1 | Gönderim | Kabul edildi |
| D-036 | Canlı dashboard Notification'dan; kampanya özetleri Analytics'te | Takip ve altyapı | Kabul edildi |
| D-037 | Vue: oxlint ESLint'in yanında kalır; Geist fontu pakete gömülür | Takip ve altyapı | Kabul edildi |
| D-038 | DSL kendi Pratt ayrıştırıcımızla ayrıştırılır; D-007'deki expr-lang seçiminin yerini alır | LLM entegrasyonu | Kabul edildi |
| D-039 | Göreli zaman ago("30d") ile yazılır, Customer sorgu anında now() ile çözer | LLM entegrasyonu | Kabul edildi |
| D-040 | Şehir kapalı listeden ve kanonik anahtarla eşleşir | Kitle ve izinler | Kabul edildi |
| D-041 | LLM yalnız filtreyi yazar; varlık ve dönen alanlar Proposal kodunda sabittir | LLM entegrasyonu | Kabul edildi |
| D-042 | Filtre kataloğu kişisel olmayan tüm alanları içerir; izin ve hesap alanları dışarıda | Kitle ve izinler | Kabul edildi |
| D-043 | Erişim kontrolü Customer GraphQL uç noktasında; servis token'ı ve kapsam | Kitle ve izinler | Kabul edildi |
| D-044 | Customer GraphQL şeması tek kaynak; Proposal kataloğu build'de üretilir; alan adları camelCase | LLM entegrasyonu | Kabul edildi |
| D-045 | DSL 0.1 kapsamı: çekirdek işleçler, ilişki niceleyicileri, ago/date, in, boolean | LLM entegrasyonu | Kabul edildi |
| D-046 | DSL 0.1: yalın boolean alan yazımı yok; boş in listesi hata | LLM entegrasyonu | Kabul edildi |
| D-047 | Claude API istemcisi resmi anthropic-sdk-go v2 | LLM entegrasyonu | Kabul edildi |
| D-048 | Servis DB'lerinde tablolar servis adlı şemada; healthcheck her servis rolünü kontrol eder | Takip ve altyapı | Kabul edildi |
| D-049 | NATS: nats:2.15-alpine, dosya depolama, stream'leri nats-box init konteyneri kurar | Gönderim | Kabul edildi |
| D-050 | Platform: /healthz + /readyz, ortam + isteğe bağlı .env, slog JSON istek logu, Prometheus /metrics | Takip ve altyapı | Kabul edildi |
| D-051 | sqlc go tool ile, üretilen kod commit edilir; entegrasyon testleri integration tag'iyle make test içinde; migration'lar sıralı numaralı | Test ve kapsam | Kabul edildi |
| D-052 | Unit test coverage yerel araçlarla raporlanır (go cover, vitest v8); eşik yok | Test ve kapsam | Kabul edildi |
| D-053 | NATS Go istemcisi nats.go v1.54.0, jetstream paketi | Gönderim | Kabul edildi |
| D-054 | Outbox ve processed_event platform migration setinde; relay LISTEN/NOTIFY + 1 sn poll; tüketicide NakWithDelay | Gönderim | Kabul edildi; tüketici backoff parametreleri yerini aldı → D-055 |
| D-055 | Olay tüketicisi backoff'u: 1 sn'den üstel, 15 dk tavan, 20 deneme | Gönderim | Kabul edildi |
| D-056 | Servis konteynerleri: platforma bağlı iskelet main'ler, distroless nonroot imaj, customer dışında host portları, postgres ve nats-init beklenir | Takip ve altyapı | Kabul edildi |
| D-057 | Geliştirmede API belgeleri: API_DOCS_ENABLED ile servis başına Scalar (/docs) ve /openapi.json; swag v2 go tool, spec commit edilir | Takip ve altyapı | Kabul edildi |
| D-058 | Customer GraphQL sözleşmesi: alan başına tipli filtre işleçleri, TimeValue @oneOf, tam işlem seti, Decimal string, null-güvenli eq/ne; gqlgen v0.17.95 | LLM entegrasyonu | Kabul edildi |
| D-059 | Prompt'lar repoda dosya, go:embed ile gömülü; sürüm kimliği ad@semver + istek öneki sha256'sı, kilit dosyası testi | LLM entegrasyonu | Kabul edildi |
| D-060 | Prompt'lar düz metin; llm_call sürüm, önbellek ve sonuç sütunları alır; sürüm token olayına ve llm_usage'a taşınır; prompt_snapshot yok | LLM entegrasyonu | Kabul edildi |
| D-061 | Prompt değerlendirmesi 3 katman (make test, make eval, make eval-live) + promptfoo; LLM hakem yok; arayüzde sürüm yok; live testte önbellek tanılama | Test ve kapsam | Kabul edildi |
| D-062 | Filtre derleyicisi: ulaşılabilirlik koşulu, bellekte şehir sözlüğü, CHECK'li kategoriler doğrulanır, elle SQL, sınırlar 8/100/100/5 sn | Kitle ve izinler | Kabul edildi |
| D-063 | Customer kimliği: çağıran başına env token, kapsamlar kodda ve şemada @hasScope; 401 ve alan başına FORBIDDEN; consent:write yalnız admin token | Kitle ve izinler | Kabul edildi |
| D-064 | İzin işlemleri: id başına consentStatus satırı, kilit + max+1 kimlik, yalnız değişiklikte kayıt, kapalı source listesi, make consent | Kitle ve izinler | Kabul edildi |
| D-065 | DSL ayrıştırıcısı: dar sözcük kuralları, ilk hata + satır/sütun, istemci sınırları, kanonik yazıcı, kesin hata kodları, nitelikli lambda alanları | LLM entegrasyonu | Kabul edildi |

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

## D-006: Tüm servisler arası mesajlaşma, iş kuyruğu ve zamanlama NATS JetStream; yayın transactional outbox ile

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Gönderim
- **Bağlam:** Brief asenkron kuyruk, kanal başına 100 msg/s ve "aynı mesaj iki kez gitmez" istiyor. THOUGHTS.md olay güdümlü eventual consistency ve zamanlanmış gönderim hedefliyor. Araştırma (docs/research/01) olaylar için NATS, Notification içi işler için River önerdi.
- **Karar:** Tek mesajlaşma altyapısı NATS JetStream (tek konteyner). Servisler arası olaylar, Notification'ın mesaj başına iş kuyruğu (work-queue stream) ve zamanlanmış gönderimler NATS'te; River kullanılmaz. Her servis olayı iş satırıyla aynı transaction'da kendi DB'sindeki outbox tablosuna yazar, relay NATS'e yayınlar (`Nats-Msg-Id` = outbox id). İptal: Proposal iptal olayı yayınlar, Notification'ın olay işleyici worker'ı kampanyayı/varyantı iptal olarak işaretler; zamanlanmış mesaj tetiklendiğinde worker durumu kontrol eder ve iptal edilmişse düşürür. Kullanıcıya göre ayrı bir servisler arası iptal kanalı da açılabilir; ikisi de bu kararla uyumlu.
- **Alternatifler:**
  - Olaylar NATS, işler River — reddedildi: iki kuyruk teknolojisinin bakımı; NATS hem altyapı olarak daha kolay hem servisler arası mesajlaşmayı sağlıyor.
  - RabbitMQ — reddedildi: daha ağır konteyner, delayed-message eklentisi 2026-09'da arşivlendi.
  - Commit sonrası doğrudan yayın — reddedildi: commit ile publish arasında çökmede olay kaybolur.
- **Sonuçlar / bedel:** NATS zamanlanmış mesajları (2.12+) için stream'de geri alınamayan `AllowMsgSchedules` bayrağı gerekir, konu başına tek zamanlama vardır ve iptal API'si yoktur; bu yüzden her dil varyantı (D-009) kendi zamanlama konusunu kullanır ve iptal tetiklenme anındaki durum kontrolüne dayanır. Retry ve rate limit (NAK gecikmesi, `MaxDeliver`, kanal başına limiter) uygulamada kurulacak. JetStream teslimi en az bir kezdir; "aynı mesaj iki kez gitmez" kuralının nasıl sağlanacağı (idempotency anahtarı, tüketici tarafı dedup) açık konu. Senkron Proposal → Customer çağrısı NATS değil, GraphQL (D-007).
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-007: Customer Service GraphQL (gqlgen); LLM JS benzeri DSL yazar, Proposal GraphQL'e çevirir; MCP yok

- **Durum:** Kabul edildi · 2026-10-27; `expr-lang/expr` ile ayrıştırma kısmı yerini aldı → D-038 · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** LLM'in ürettiği segment kuralı backend'de doğrulanmalı ve SQL olarak çalıştırılmamalı. THOUGHTS.md GraphQL, MCP ve JS benzeri filtre dilini değerlendiriyor; hedef çıktı token verimliliği.
- **Karar:**
  - Customer Service müşteri ambarını **gqlgen** ile GraphQL olarak sunar; GraphQL filtre girdisi beyaz listeli bir derleyiciyle parametreli SQL'e çevrilir; girdi tiplerinin şekli açık konu.
  - **Erişilebilirlik ve izin koşulu Customer'ın GraphQL resolver'ında** her sorguya eklenir (aktif hesap ve izinli e-posta veya telefon); hangi istemci çağırırsa çağırsın atlanamaz.
  - Claude filtreyi **JS benzeri DSL string** olarak yazar. **Proposal Service** GraphQL istemcisidir: DSL'i `expr-lang/expr` ile ayrıştırır, katalogla tip denetler (hiç çalıştırmaz) ve GraphQL `where` girdisine çevirir.
  - **MCP kullanılmaz.** Claude'a sunulan araç, Proposal'ın tanımlayıp kendisinin çalıştırdığı istemci tarafı tool'dur.
  - **DSL ile gql etkileşimi opt in olur** herhangi bir durumda dsl'in kapatılıp raw GraphQL validasyonu yapılması ve gönderimi mümkün kılınmalı
- **Alternatifler:**
  - MCP (Claude API connector veya iç MCP sunucusu) — reddedildi: token tüketimini artırır; connector herkese açık `https://` sunucu ister.
  - DSL'i doğrudan Customer'da SQL'e derlemek (araştırma önerisi) — reddedildi: GraphQL çağrısı istemci tarafında; çeviri GraphQL istemcisi olan Proposal'da daha mantıklı.
  - Sabit derinlikli JSON AST — reddedildi: ~3 kat çıktı token'ı, structured output özyinelemeli şemayı desteklemiyor.
  - REST + JSON AST — reddedildi: GraphQL tercih edildi.
  - graph-gophers/graphql-go — reddedildi: codegen yok, daha az tip güvenliği.
- **Sonuçlar / bedel:** İki çeviri katmanı (DSL → GraphQL Proposal'da, GraphQL → SQL Customer'da) ve iki doğrulama noktası. Customer'ın GraphQL girdisi her istemciye açık olduğu için alan beyaz listesi, düğüm/derinlik ve karmaşıklık sınırları Customer'da da zorunlu. gqlgen codegen adımı ekler ve hâlâ 0.x. Hassas hedefleme alanlarının kataloğa girip girmeyeceği ayrı bir karar. Hipotez olarak: DSL sayesinde output tokenleri filtre oluşturmada azaltılabilir, bunun yerine input token masraf bağlamında daha tasarrufludur ve context cache'e dsl spesifikasyonunun girmesinin ihtimali ölçekle artar
- **Doğrulama:** Kısmen, 2026-10-27'de (T-015): GraphQL → SQL zinciri örnek brief için 33 SMS / 33 e-posta uygun kişi döndürüyor; ulaşılabilirlik koşulu resolver'da her sorguya ekleniyor. DSL → GraphQL kısmı Proposal'da (T-018–T-020) doğrulanacak.

## D-008: Alıcı verisi zamanlamada NATS snapshot'ıyla taşınır; izin segment sorgusunda baştan uygulanır

- **Durum:** Yerini aldı → D-010, D-011 · 2026-10-27 (ilk kabul 2026-10-27)
- **Alan:** Kitle ve izinler
- **Bağlam:** Notification gönderim için alıcıların iletişim bilgisine ihtiyaç duyar; brief izinsiz kanaldan gönderimi yasaklar. Araştırma gönderim anında izin kontrolü veya `consent.changed` olayları önerdi.
- **Karar:** "İzin" alıcının kanal iznidir (`email_opt_in` / `sms_opt_in`). Bu izin, segment sorgusunda Customer resolver'ı tarafından baştan uygulanır (D-007); dönen veri zaten izne göre süzülmüştür. Zamanlama onaylandığında alıcıların gerekli bilgileri (id, ad, dil, kanal için izinli e-posta veya telefon) batch'ler hâlinde NATS üzerinden Notification'a gönderilir. Gönderim anında ayrıca izin kontrolü veya izin olayı geliştirilmez. Alıcı batch'leri work-queue stream'de tutulur ve Notification kendi DB'sine yazıp ack verdiğinde silinir.
- **Alternatifler:**
  - Snapshot + `consent.changed` olaylarıyla engelleme listesi — reddedildi: izin kavramı için ek geliştirme istenmiyor.
  - Snapshot + gönderimde GraphQL ile izin kontrolü — reddedildi: aynı neden; gönderimde Customer'a senkron bağımlılık.
  - Süreli saklama (ör. 7 gün) — reddedildi: PII mesaj altyapısında daha uzun kalır.
- **Sonuçlar / bedel:** Zamanlanmış bir kampanyada onay ile gönderim arasında geri çekilen izinler dikkate alınmaz. `customer_db` seed'li statik bir ambar olduğu için demoda bu pencerede izin değişmez. NATS varsayılan mesaj sınırı 1 MB; alıcılar batch'lere bölünür. Yeniden oynatma kaynağı Notification DB'sidir.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-009: İçerik ve zamanlama alıcının diline göre; tüm diller onaylanmadan teklif onaylanmaz

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** Brief bonusu "kullanıcının diline göre içerik". `dim_customer.language` şu an `tr` ve `en` içeriyor. Kullanıcı ara adımlarda bağlam ekleyebilmeli.
- **Karar:**
  - Onaylanan kitlede bulunan her benzersiz dil için, seçilen her kanalda ayrı bir metin teklifi üretilir.
  - Her dil varyantı ayrı zamanlanır; dil başına ayrı gönderim zamanı.
  - Bir adım (metin veya zamanlama), kitledeki tüm dillerin varyantları onaylanmadan onaylanmış sayılmaz.
  - Kullanıcı tek bir dili gerekçesiyle reddedebilir; yalnızca o varyant yeniden üretilir.
  - Kullanıcı herhangi bir adımda ek bağlam verebilir. Bağlam kampanyanın bağlamına eklenir, mevcut adımın onaylanmamış teklifleri yeniden üretilir, sonraki adımlar da onu görür. Onaylanmış teklifler korunur.
- **Alternatifler:**
  - Tek dil — reddedildi: bonus ve kullanıcı isteği dil bazlı içerik.
  - Tek gönderim zamanı, tüm diller — reddedildi: dil başına zamanlama istendi.
  - Alıcının yerel saati — reddedildi: dil başına ayrı zaman seçildi.
  - Ek bağlam yalnızca sonraki adımlara — reddedildi: kullanıcı değişiklik için teklifi reddetmek zorunda kalırdı.
  - Kullanıcının bağlamın hedefini seçmesi — reddedildi: daha karmaşık arayüz ve durum makinesi.
- **Sonuçlar / bedel:** LLM çağrısı sayısı dil × kanal × onarım denemesi kadar çoğalır; token ve maliyet artar. Teklif onay durumu dil varyantı düzeyinde tutulmalı; durum makinesinin şeması açık konu. Ek bağlam yeniden üretimi ekstra çağrı ister. Her dil varyantı ayrı bir NATS zamanlama konusu kullanır (D-006).
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-010: Kanal planı ayrı onay adımı; snapshot kişi başına atanmış kanalı taşır

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Kitle ve izinler
- **Bağlam:** D-008 izni segment sorgusunda tek seferlik bir süzgeç olarak uyguluyordu. Örnek brief kitlesinde (49 kişi) 20 kişi iki kanala, 13 yalnızca e-postaya, 13 yalnızca SMS'e açık, 3 kişiye ulaşılamıyor; e-posta ve SMS 33'er kişiyle berabere. THOUGHTS.md §5 kanal kapsamasının kullanıcıya sorulmasını istiyor.
- **Karar:**
  - Kitle sorgusu en az bir kanaldan ulaşılabilen aktif kişileri döndürür (koşul Customer resolver'ında, D-007) ve kişileri kanala göre elemez; her kişi için e-posta ve SMS uygunluğunu ayrı taşır.
  - Kitle onayından sonra ayrı bir **kanal planı** adımı gelir. Kapsama tablosunu (iki kanal / yalnızca e-posta / yalnızca SMS / ulaşılamaz, dil kırılımıyla) LLM değil SQL hesaplar. Ajan bir plan önerir; kullanıcı onaylar veya reddeder.
  - Plan seçenekleri: **tek kanal** (yalnızca o kanala açık olanlar), **birincil + yedek** (kişi başına tek mesaj; birincil kanala kapalı olanlar diğerinden), **iki kanal birden**. Ajanın varsayılan önerisi birincil + yedek; beraberlikte birincil kanal e-posta.
  - Onaylanan planla her kişinin kanalı atanır. Zamanlama onaylandığında alıcılar (id, ad, dil, atanmış kanal ve o kanalın iletişim bilgisi) batch'ler hâlinde NATS work-queue stream'iyle Notification'a gider ve Notification kendi DB'sine yazıp ack verdiğinde stream'den silinir (D-008'den korunur). Notification kanal seçmez.
  - Metin teklifleri (D-009) yalnızca alıcısı olan dil-kanal çiftleri için üretilir.
  - Kanal planı reddedilirse aynı adımda yeni plan önerilir. Kitle yeniden onaylanırsa kanal planı ve metinler geçersiz olur; kanal planı değişirse yalnızca metin teklifleri geçersiz olur.
- **Alternatifler:**
  - D-008'deki gibi sorguda kanal bazında süzme — reddedildi: kanal kapsaması kullanıcıya gösterilemez, beraberlik ve yedek kanal karar dışı kalır.
  - Kanalı gönderim anında Notification'ın seçmesi — reddedildi: onaylanan plandan sapma riski; seçim insan onayında kalmalı.
- **Sonuçlar / bedel:** Akışa bir onay adımı daha eklenir. "İki kanal birden" seçilirse aynı kişi iki kanaldan mesaj alır; bu seçenek kullanıcının açık onayına bağlıdır. Kişi başına kanal ataması snapshot'ı biraz büyütür.
- **Doğrulama:** Doğrulanmadı — henüz kod yok. Kapsama sayıları 2026-10-27'de `customer_db` üzerinde sorguyla ölçüldü (20 / 13 / 13 / 3).

## D-011: Gönderim anında GraphQL ile batch izin kontrolü; circuit breaker ile fail-closed

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Kitle ve izinler
- **Bağlam:** D-008 gönderim anında izin kontrolü yapmıyordu. Zamanlanmış bir kampanyada onay ile gönderim arasında geri çekilen izin, brief'in "izni olmayan kullanıcıya o kanaldan mesaj gönderilmez" kuralını çiğner. Kullanıcı ek kontrol istedi.
- **Karar:**
  - Notification her gönderim batch'inden önce Customer Service'e **GraphQL** ile tek sorgu atar ve batch'teki kişilerin atanmış kanaldaki güncel iznini alır. İzin tek doğruluk kaynağı Customer'dır.
  - Atanmış kanalda izni geri çekilmiş kişiye mesaj gönderilmez; teslim kaydı `skipped_no_consent` olur ve dashboard'da raporlanır. Diğer kanala düşülmez.
  - Customer çağrısı **circuit breaker** arkasındadır. Customer hata verirse veya ulaşılamazsa breaker açılır, batch'ler gönderilmeden ertelenir ve yeniden denenir; izni doğrulanmamış kimseye mesaj gitmez (fail-closed). Breaker yarı açıkken deneme çağrılarıyla toparlanmayı yoklar.
  - Customer Service'e `updateConsent` GraphQL mutation'ı eklenir: yeni bir `fact_consent` satırı yazar ve `dim_customer` üzerindeki ilgili izin bayrağını günceller. Demoda izin geri çekme arayüzden canlı gösterilebilir.
- **Alternatifler:**
  - NATS `consent.changed` olayları + engelleme listesi — reddedildi: eventual-consistency penceresi; Customer'da outbox gerekir.
  - NATS request-reply — reddedildi: Customer GraphQL'in yanında ikinci bir sözleşme taşır.
  - İzin yoksa diğer kanala düşmek — reddedildi: onaylanan plandan sapma, daha karmaşık worker mantığı.
  - Customer'a ulaşılamazsa snapshot ile göndermek (fail-open) — reddedildi: Notification görev kritik; kesinti sırasında izin çiğnenir.
  - İzin değişikliğini yalnızca testlerde doğrudan DB'ye yazmak — reddedildi: demoda gösterilemez.
- **Sonuçlar / bedel:** D-006'daki NATS kapsamı asenkron mesajlaşmadır; bu izin sorgusu, D-007'deki Proposal → Customer çağrısı gibi senkron bir GraphQL çağrısıdır. Gönderim Customer'ın erişilebilirliğine senkron bağımlıdır; Customer kesintisinde gönderim gecikir ama izin çiğnenmez. Kontrol ile gönderim arasında milisaniyelik bir pencere kalır. `customer_db` artık yalnızca seed'le dolan bir ambar değildir; izin için bir yazma yolu vardır ve `dim_customer` bayrağı ile son `fact_consent` kaydı aynı transaction'da güncellenmelidir (seed'deki tutarlılık kontrolü bu kuralı varsayar). Breaker eşikleri, batch boyutu ve kütüphane seçimi açık.
- **Doğrulama:** Kısmen, 2026-10-27'de (T-017): Customer tarafı hazır; `consentStatus` güncel izni ve uygunluğu döndürüyor, `updateConsent` `fact_consent` ve `dim_customer`'ı aynı transaction'da güncelliyor (D-064). Notification tarafı (batch kontrolü, `skipped_no_consent`, circuit breaker) T-029'da. Hedef testler: zamanlanmış kampanyada onaydan sonra izin geri çekilen kişi `skipped_no_consent` olur; Customer kapalıyken hiçbir mesaj gönderilmez.

## D-012: Frontend: Vue 3 + TypeScript, pnpm, Tailwind, shadcn-vue, Vue Router + composable'lar; kök dizinde `web/`

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** Case frontend için Vue.js şart koşuyor. Arayüzde sohbet terminali, teklif onay/red adımları, kanal planı, SMS/HTML önizleme, tarih-saat seçicili zamanlama ve canlı gönderim panosu olacak (THOUGHTS.md, D-009, D-010).
- **Karar:**
  - Vue'nun güncel sürümü (2026-10-27 itibarıyla `vue` 3.5.44), paket yöneticisi ve çalışma ortamı **pnpm** (güncel 12.15.0; makinede Node 24 ve pnpm 12.8.1).
  - **TypeScript** ile Composition API ve `<script setup lang="ts">`; tip kontrolü `vue-tsc` ile.
  - Stil **Tailwind CSS** (güncel 4.3.3); bileşenler **shadcn-vue** (Reka UI tabanlı, projeye kopyalanan bileşenler).
  - Yönlendirme **Vue Router**; paylaşılan durum için global store yok, sayfalar veriyi composable'larla alır (teklif durumu sunucuda tutulduğu için).
  - Konum: depo kökünde **`web/`** dizini; Go modülü (`go.mod`) kökte, yanında.
- **Alternatifler:**
  - JavaScript — reddedildi: API sözleşmesi hataları çalışma zamanında yakalanırdı.
  - Pinia — reddedildi: sunucudaki teklif durumunun istemcide ikinci kopyası olurdu. Paylaşılan durum büyürse sonradan eklenebilir.
  - Reka UI (headless) veya bileşen kütüphanesi olmadan — reddedildi: tarih seçici, diyalog ve tabloları elle stillemek bir haftalık süreye ağır.
- **Sonuçlar / bedel:** shadcn-vue bileşenleri kopyalandığı için bakımı bizde; sürümü iskelet kurulurken doğrulanacak. create-vue iskeleti Vite ile gelir. Sürümler iskelet kurulurken `package.json` ve `pnpm-lock.yaml` ile sabitlenir. Tek depoda Go ve pnpm araç zincirleri yan yana; `make test` ikisini de çalıştırmalı.
- **Doğrulama:** Doğrulanmadı — henüz `web/` iskeleti yok. Sürümler 2026-10-27'de npm registry'den okundu.

## D-013: Frontend ↔ backend: istek/yanıt REST, canlı güncellemeler SSE

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** Arayüz teklif oluşturma, onay/red ve zamanlama için backend'i çağırır; teklif ilerlemesi, metin akışı ve gönderim durumu canlı görünmeli. THOUGHTS.md canlı durum için WebSocket veya event stream öneriyor.
- **Karar:** Tarayıcı ile backend arasındaki istek/yanıt etkileşimleri **REST**. Canlı güncellemeler (teklif adımları, kitle sayıları, metin akışı, gönderim durumu) **Server-Sent Events** ile tarayıcıya gelir. GraphQL (D-007) tarayıcıya açılmaz; yalnızca servisler arası iç arayüzdür.
- **Alternatifler:**
  - WebSocket — reddedildi: sürdürme ve ping'ler elle kurulur, nginx'te Upgrade ayarı gerekir; istemciden sunucuya yön zaten REST ile karşılanıyor.
  - Polling — reddedildi: 1–2 sn gecikme, metin akışı canlı görünmez.
- **Sonuçlar / bedel:** `EventSource` yalnızca GET destekler ve özel header gönderemez; POST ile başlayan metin akışı için `fetch` + `ReadableStream` gerekir, kimlik doğrulama eklenirse cookie veya kısa ömürlü token gerekir (kimlik doğrulama henüz kararlaştırılmadı). SSE'nin `Last-Event-ID` ile sürdürülmesi ve nginx'te tamponlamanın kapatılması uygulamada kurulacak.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-014: Frontend testleri Vitest + Vue Test Utils; uçtan uca tarayıcı testi yok

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Test ve kapsam
- **Bağlam:** Brief `make test` ile çalışan testler istiyor; değerlendirmenin %15'i test ve kod kalitesi. Frontend D-012 ile Vue 3 + TypeScript.
- **Karar:** Bileşen ve composable testleri **Vitest** + **Vue Test Utils** ile, tarayıcısız Node ortamında yazılır ve `make test` içinde çalışır. Uçtan uca tarayıcı testi (Playwright) şimdilik yok.
- **Alternatifler:**
  - Ek olarak Playwright ile tek kritik akışın uçtan uca testi — reddedildi: tarayıcı indirmesi, daha yavaş ve kırılgan testler, compose'da tam yığın gerekir.
- **Sonuçlar / bedel:** Brief → teklif → onay → gönderim akışının tamamı tarayıcıda otomatik sınanmaz; servis sınırları backend testleri ve mock LLM ile, arayüz bileşen düzeyinde test edilir.
- **Doğrulama:** Doğrulanmadı — henüz `web/` yok.

## D-015: Tek nginx konteyneri: Vue statik build'ini sunar ve API gateway'dir

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** `docker compose up` arayüz dahil her şeyi tek komutla kaldırmalı. THOUGHTS.md diyagramında tarayıcının önünde bir nginx API gateway var. SSE (D-013) proxy'de tamponlamanın kapatılmasını gerektirir.
- **Karar:** Çok aşamalı Docker build: `web/` pnpm ile build edilir, çıktı nginx imajına kopyalanır. Aynı nginx konteyneri statik arayüzü sunar, `/api/*` isteklerini ilgili servislere yönlendirir ve SSE konumlarında tamponlamayı kapatıp okuma zaman aşımını uzatır. Geliştirmede sıcak yeniden yükleme için `pnpm dev` compose dışında çalışır.
- **Alternatifler:**
  - Ayrı web konteyneri + önünde nginx gateway — reddedildi: fazladan konteyner, kazanç az.
  - Compose içinde Vite dev sunucusu — reddedildi: üretime benzemez, ayrıca yine bir gateway gerekir.
- **Sonuçlar / bedel:** Arayüz değişikliği compose'da görünmek için imajın yeniden build edilmesini ister. Servisler arası iç trafik (GraphQL, NATS) nginx'ten geçmez. Yönlendirme tablosu ve nginx sürümü uygulamada belirlenecek.
- **Doğrulama:** Doğrulanmadı — henüz nginx yapılandırması yok.

## D-016: REST sözleşmesi Go kodundan swag v2 ile OpenAPI 3; TS tipleri ve istemci spec'ten üretilir

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-013 ile tarayıcı backend'e REST ile bağlanıyor. Go ve TypeScript tipleri aynı sözleşmeden gelmeli ve API belgelenmeli.
- **Karar:**
  - REST sözleşmesi Go handler'larındaki annotation'lardan **swag** ile üretilir; sözleşme ve API belgesi bu çıktıdır.
  - Sürüm **swag v2** (2026-10-27 itibarıyla `v2.0.0-rc6`), çünkü doğrudan OpenAPI 3 hedefliyor.
  - Vue tarafında tipler **openapi-typescript** (7.13.0), tipli istemci **openapi-fetch** (0.17.0) ile üretilen spec'ten oluşturulur; Go değişince tipler yeniden üretilir.
- **Alternatifler:**
  - OpenAPI önce, oapi-codegen ile Go'ya üretim — reddedildi: kullanıcı sözleşmenin Go kodundan üretilmesini istedi.
  - swag v1 (kararlı v1.16.6) + Swagger 2.0 → OpenAPI 3 dönüştürme — reddedildi: fazladan dönüştürme adımı ve iki dosya.
  - swag v1, yalnızca Swagger 2.0 — reddedildi: openapi-typescript Swagger 2.0 okumaz, TS tipleri elle yazılırdı.
  - Elle yazılmış TS tipleri — reddedildi: Go ve TS tipleri zamanla ayrışır.
  - huma v2 — reddedildi: swag tercih edildi.
- **Sonuçlar / bedel:** swag v2 hâlâ sürüm adayı; annotation sözdizimi ve çıktısı değişebilir, sürüm `go.mod`'da sabitlenmeli. v2'nin ürettiği OpenAPI 3 dosyasının openapi-typescript ile uyumu iskelette doğrulanacak; sorun çıkarsa D-016 yeniden açılır. Annotation'lar handler'la elle senkron tutulur; yanlış annotation yanlış tip üretir. SSE akışları (D-013) OpenAPI'de tam ifade edilemeyebilir; olay tipleri ayrıca belgelenmeli.
- **Doğrulama:** Doğrulanmadı — henüz kod yok. Sürümler 2026-10-27'de Go modül proxy'si ve npm registry'den okundu.

## D-017: Seed zaman damgaları yüklemede bugüne kaydırılır

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Mock provider ve seed
- **Bağlam:** Üretici veriyi `AS_OF = 2026-10-26 12:00 UTC` anına göre üretir (D-001); segment pencereleri gerçek `now()` kullanacak. Kaydırma olmazsa örnek brief kitlesi ("son 30 günde") 2026-11-25 civarında 0'a düşer.
- **Karar:** `03_customer_seed.sql`, staging'den `dwh`'ye yazarken tüm zaman kolonlarını ileri kaydırır. `timestamptz` kolonları `now() - TIMESTAMPTZ '2026-10-26 12:00Z'` aralığı kadar, `date` kolonları aynı farkın tam gün sayısı kadar kayar. `date_of_birth` da kayar, böylece `age_years` tutarlı kalır. CSV'ler ve üreticinin md5 determinizmi değişmez; uygulama ve testler gerçek saati kullanır.
- **Alternatifler:**
  - Yapılandırılabilir saat (`CLOCK_NOW`) — reddedildi: zamanlama ve gönderim gerçek saatle çalışmalı, iki saat kavramı oluşur.
  - Build'de `AS_OF=bugün` ile yeniden üretmek — reddedildi: CSV'ler her gün değişir, commit edilen veri ve md5 determinizmi bozulur.
- **Sonuçlar / bedel:** Volume eskidikçe veri yine kayar; demodan önce `docker compose down -v && docker compose up -d` gerekir. DB'deki mutlak tarihler artık CSV'dekilerle aynı değildir; CSV ile DB'yi karşılaştıran kontroller farkı hesaba katmalı. Tasarım belgesindeki örnek sorgu `now()` kullanacak şekilde güncellenmeli (uygulandığında).
- **Doğrulama:** 2026-10-27'de `docker compose down -v && up` sonrası (T-007): seed tüm kontrollerden geçti, satır sayıları ve NULL sayıları CSV ile aynı, en yeni olay `now()`'dan birkaç dakika önce. Örnek brief sorgusu `now()` ile `90 → 49 → 33 SMS / 33 email` verdi. Seed'e zaman damgalarının `now()`'ı aşmadığı kontrolü eklendi; tutmazsa yükleme geri alınır (kullanıcı onayladı). Elle doğrulandı, otomatik test henüz yok.

## D-018: Dört servis ve mock provider, her biri ayrı konteyner

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** THOUGHTS.md diyagramı Customer, Proposal, Notification ve Analytics servislerini gösteriyor. Araştırma Analytics'i ilk kesilecekler arasına koymuştu (docs/research/README.md).
- **Karar:** Customer, Proposal, Notification ve Analytics ayrı Go süreçleri ve ayrı compose servisleridir. Mock SMS/e-posta provider da ayrı bir konteynerdir. Analytics `notification.completed` ve `token.completed` olaylarını NATS'ten tüketir ve kendi veritabanına yazar.
- **Alternatifler:**
  - Üç servis + mock, Analytics kesilir — reddedildi: diyagrama tam uyum istendi.
  - Tek binary, alt komutlar — reddedildi: mikroservis gösterimi zayıflar.
- **Sonuçlar / bedel:** Bir haftalık yapımda daha fazla iş. Analytics olay sözleşmelerine bağımlıdır ve eventual-consistent'tır. Compose'ta beş uygulama konteyneri, Postgres, NATS ve nginx çalışır. Dashboard'un canlı gönderim durumunu hangi servisten okuyacağı açık.
- **Doğrulama:** 2026-10-27'de (T-013): dört servis ve mock provider ayrı compose servisleri olarak iskelet hâlinde çalışıyor; iş mantığı yok.

## D-019: proposal_db, notification_db, analytics_db; rol ve boş DB initdb'de açılır

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-003 servis başına veritabanı ve sahip rol getirdi. Tasarım belgesi geçici olarak `campaign_db` / `delivery_db` adlarını kullanıyordu; diyagram Proposal / Notification / Analytic DB diyor.
- **Karar:** `proposal_db`/`proposal_svc`, `notification_db`/`notification_svc` ve `analytics_db`/`analytics_svc`. Analytics D-018 ile eklendiği için aynı adlandırma desenini alır. Rol ve boş veritabanı initdb'de `01_customer_db.sql` ile aynı desenle açılır: sahip rol, `REVOKE ALL … FROM PUBLIC`, `CONNECT` ve `TEMPORARY`. Parolalar compose ortam değişkenlerinden gelir, `.env.example`'da yalnız dev varsayılanları bulunur. Tablolar her servisin kendi migration'larıyla açılır (D-020). `customer_db`'nin şeması ve seed'i initdb'de kalır (D-005).
- **Alternatifler:**
  - Tasarım belgesindeki `campaign_db` / `delivery_db` — reddedildi: diyagramla uyum.
  - Servisin açılışta süper kullanıcıyla kendi rolünü ve DB'sini açması — reddedildi: servislere süper kullanıcı parolası verilirdi.
- **Sonuçlar / bedel:** Yeni bir DB eklemek initdb değişikliği ve `down -v` ister, çünkü init yalnız boş volume'da çalışır. İki şema mekanizması yan yana durur: `customer_db` initdb'de, diğerleri migration'da.
- **Doğrulama:** 2026-10-27'de `docker compose down -v && up` sonrası (T-008): üç servis rolü kendi DB'sine bağlanıp tablo yazabiliyor; beş çapraz bağlantı denemesi (ör. `proposal_svc → customer_db`, `customer_svc → analytics_db`) "User does not have CONNECT privilege" ile reddedildi. Elle doğrulandı.

## D-020: Migration ve DB erişimi goose + sqlc + pgx/v5

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-005'teki init scriptleri yalnız ilk kurulum içindir. Yeni servislerin şemaları evrilecek.
- **Karar:** Her servisin SQL migration'ları `goose` ile yazılır, servis binary'sine `embed` ile gömülür ve servis açılışta kendi DB'sine uygular. Sorgular SQL olarak yazılır, `sqlc` tipli Go kodu üretir; sürücü `pgx/v5`. `customer_db` şimdilik initdb'de kalır.
- **Alternatifler:**
  - golang-migrate + pgx — reddedildi: kod üretimi yok, sorgular elle yazılır.
  - Atlas + pgx — reddedildi: ek araç ve öğrenme maliyeti.
  - Gömülü düz SQL ve kendi koşucumuz — reddedildi: her şey elle.
- **Sonuçlar / bedel:** `sqlc` bir kod üretim adımı ekler (D-007'deki gqlgen gibi); sürümü sabitlenmeli. Customer'daki dinamik segment SQL'i (D-007) `sqlc`'ye uymaz; derleyici parametreli sorguyu doğrudan `pgx` ile kurar.
- **Doğrulama:** 2026-10-27'de (T-011): `internal/platform/db` (pgx/v5 v5.11.0) ve `internal/platform/migrate` (goose v3.29.0, gömülü `fs.FS`, Postgres session locker) entegrasyon testleriyle doğrulandı. sqlc henüz bir servis sorgusunda kullanılmadı.

## D-021: Tek Go modülü; cmd/ altında servis başına binary, internal/ altında kod

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-018 beş binary getiriyor. Kökte `go.mod` var (`github.com/furknozg/coldbru`, D-002).
- **Karar:** Kök modül korunur. `cmd/{customer,proposal,notification,analytics,mockprovider}/main.go`. Servis kodu `internal/<servis>/` altında; servisin migration'ları ve `sqlc.yaml`'ı o klasörde. Ortak altyapı `internal/platform/` altında (db, nats, outbox, http, config). Tek çok aşamalı Dockerfile, `SERVICE` build arg'ıyla hangi binary'nin derleneceğini seçer.
- **Alternatifler:**
  - `go.work` ile servis başına modül — reddedildi: go.mod ve sürüm bakımı artar, kökteki go.mod taşınır.
  - Tek modül, `services/<servis>/` klasörleri — reddedildi: sınır klasörde görünür ama kazanç yok.
- **Sonuçlar / bedel:** Go'nun `internal` kuralı aynı modül içinde servisler arası import'u engellemez; sınırı kod incelemesi korur. Tüm servisler aynı bağımlılık sürümlerini paylaşır.
- **Doğrulama:** 2026-10-27'de (T-013): tek modül, `cmd/<servis>/main.go` ve tek Dockerfile ile beş imaj derlendi ve compose'da çalıştı.

## D-022: Teklif modeli: campaign → proposal_step → proposal_variant

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-009 ve D-010 adım başına onay, dil-kanal varyantı başına onay ve red, ek bağlam ve geçersiz kılma kuralları getirdi. Durum makinesinin şeması açıktı.
- **Karar:** `proposal_db`'de:
  - `campaign`: brief ve kampanyanın genel durumu.
  - `proposal_step`: adım türü (kitle, kanal planı, metin, zamanlama), sürüm ve durum.
  - `proposal_variant`: dil, kanal, içerik `jsonb`, durum `draft | approved | rejected | superseded`, red gerekçesi.
  - Bir adım, tüm varyantları onaylanınca onaylanır (D-009).
  - `context_note` ve `llm_call` ekleme-yalnız tablolardır.
  - Geçersiz kılmada (D-010) eski satır `superseded` olur ve yeni sürüm açılır; satır silinmez.
- **Alternatifler:**
  - Adım başına JSONB belge — reddedildi: durum kuralları uygulama kodunda kalır, sorgulaması zor.
  - Olay kaynağı (event sourcing) — reddedildi: kod ve test yükü en fazla.
- **Sonuçlar / bedel:** Daha fazla tablo ve join; sürümler birikir. `campaign` ve `proposal_step` durum değerleri ve geçişleri, dil ve kanalı olmayan adımların (kitle, kanal planı) varyant biçimi açık.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-023: Teslim modeli: UNIQUE(campaign_id, customer_id, channel), durum makinesi, Idempotency-Key

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Gönderim
- **Bağlam:** Brief "aynı mesaj iki kez gitmez" diyor. JetStream en az bir kez teslim eder (D-006); retry ve worker çökmesi aynı işi iki kez işletebilir.
- **Karar:**
  - `notification_db.delivery` tablosunda `UNIQUE(campaign_id, customer_id, channel)`; alıcı batch'leri `ON CONFLICT DO NOTHING` ile yazılır.
  - Durum: `pending → sending (lease) → sent | failed | skipped_no_consent | cancelled`.
  - Provider'a `Idempotency-Key` = delivery uuid gönderilir.
  - Tüketici dedup'u: olayı işleyen transaction'da `processed_event` tablosuna ekleme yapılır, aynı olay ikinci kez işlenmez.
- **Alternatifler:**
  - Yalnız `Nats-Msg-Id` ve JetStream dedup penceresi — reddedildi: pencere dışı tekrarları ve yeniden başlatmayı korumaz.
  - Unique kısıt, idempotency anahtarı yok — reddedildi: timeout sonrası yeniden denemede provider tarafında çift gönderim olabilir.
- **Sonuçlar / bedel:** Her gönderim bir DB yazımı ekler. Garanti "en az bir kez teslim + DB'de tekil satır"dır. Provider isteği kabul edip yanıt kaybolursa yeniden deneme provider'ın idempotency'sine güvenir; mock provider anahtarı saklamalıdır. Lease süresi açık.
- **Doğrulama:** Doğrulanmadı — henüz kod yok. Hedef testler: aynı alıcı batch'i iki kez gelince tek satır; eşzamanlı iki worker tek gönderim yapar.

## D-024: NATS düzeni: dört stream, teslimat başına ileti, kanal başına 100/s limiter, NAK ile retry

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Gönderim
- **Bağlam:** D-006 tüm asenkron işi NATS JetStream'e verdi; rate limit ve retry parametreleri açıktı. Brief kanal başına 100 msg/s ve retry istiyor.
- **Karar:**
  - `EVENTS`: servisler arası olaylar, ör. `evt.proposal.confirmed.v1`; servis başına durable tüketici.
  - `RECIPIENTS`: work-queue; alıcı batch'leri (D-010).
  - `SCHEDULES`: `AllowMsgSchedules` açık; dil varyantı başına bir zamanlama konusu (D-006, D-009).
  - `SEND`: work-queue; `send.sms` ve `send.email`, teslimat başına bir ileti.
  - Rate limit: Notification içinde kanal başına `golang.org/x/time/rate` limiter, 100 msg/s. 429 gelirse o kanal `Retry-After` kadar duraklar.
  - Retry: `NakWithDelay` ile üstel geri çekilme, `MaxDeliver` 5; son denemeden sonra teslimat `failed` olur.
- **Alternatifler:**
  - Tek stream, konu hiyerarşisi — reddedildi: work-queue ve olay saklama politikaları karışır, `AllowMsgSchedules` hepsine açılır.
  - Teslimatları DB'den `SKIP LOCKED` ile claim etmek — reddedildi: D-006'daki "ileti başına iş kuyruğu NATS'te" kararından sapar.
- **Sonuçlar / bedel:** Limiter süreç içidir; Notification tek replika varsayılır, yatay ölçek dağıtık limit ister. Kampanya boyu kadar ileti stream'e girer. `AllowMsgSchedules` geri alınamadığı için zamanlamalar ayrı stream'dedir. Hangi hataların yeniden deneneceği, geri çekilme tabanı ve jitter açık.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-025: Backend testleri: birim sahte, entegrasyon testcontainers-go, mock LLM

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Test ve kapsam
- **Bağlam:** "İki kez gitmez", unique kısıt ve JetStream davranışı ancak gerçek Postgres ve NATS'e karşı kanıtlanır. Testler gerçek API anahtarı olmadan koşmalı (CLAUDE.md).
- **Karar:** Birim testleri sahte arayüzlerle. Entegrasyon testleri `go test` içinde `testcontainers-go` ile geçici Postgres 18 ve NATS açar; compose'un ayakta olması gerekmez. LLM için senaryolu sahte ve `httptest` Messages SSE sunucusu kullanılır; gerçek adaptör `option.WithBaseURL` ile bu sunucuya bağlanır. Gerçek API çağrısı yalnız `live` build tag'iyle yapılır; testler `ANTHROPIC_API_KEY` okumaz.
- **Alternatifler:**
  - Çalışan compose yığınına karşı test — reddedildi: testler paylaşılan duruma bağlı, paralel çalışması zor.
  - Yalnız birim testi — reddedildi: gerçek SQL, unique kısıt ve JetStream davranışı test edilmez.
- **Sonuçlar / bedel:** Testler Docker ister ve ilk çalıştırmada imaj çekimi yavaştır. Bu makinede Docker CLI PATH'te değil; testcontainers'ın Docker soketini bulup bulmadığı doğrulanmadı.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-026: Modeller: segment ve metin Sonnet 5.5, enjeksiyon ön elemesi Haiku 5.5

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** Yanlış segment kuralı gerçek müşterilere dokunur; Türkçe ton kalitesi önemli; brief içinde prompt enjeksiyonu olabilir. Fiyatlar 2026-10-27 araştırmasından (docs/research/03 §1).
- **Karar:** Segment çağrısı `claude-sonnet-5-5`, `effort: "medium"`. Metin çağrısı `claude-sonnet-5-5`, `effort` low veya medium. Brief'in enjeksiyon ön elemesi `claude-haiku-5-5`. Uzun sistem prompt'u ve alan kataloğu prompt caching ile önbelleğe alınır.
- **Alternatifler:**
  - Yalnız Sonnet 5.5 — reddedildi: enjeksiyon savunması yalnız doğrulama hattında kalır.
  - Segmentte Opus 5.5 — reddedildi: maliyet ve gecikme yükselir.
  - Yalnız Haiku 5.5 — reddedildi: Türkçe ton ve karmaşık kural kalitesi düşebilir.
- **Sonuçlar / bedel:** Her brief'e bir ön eleme çağrısı ve gecikmesi eklenir. 5.x modellerinde `temperature` ve prefill 400 döner; istemci bunları göndermemeli. Ön elemenin işaretlediği brief'te ne olacağı açık.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-027: Yapılandırılmış çıktı: segmentte strict tool döngüsü, metinde native structured outputs

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-007 LLM'in segment kuralını DSL string olarak yazmasını istiyor. Strict şemalar özyinelemeli şemayı ve `maxLength`'i desteklemez; Sonnet 5.5 zorlanmış `tool_choice`'u desteklemez (docs/research/03 §2).
- **Karar:**
  - Segment: `tool_choice: auto` ile strict tool döngüsü. `propose_segment` tool'unun tek alanı DSL string; ajan `count_audience` ile kitle sayısını yoklayabilir.
  - Metin: native structured outputs (JSON şeması) ve streaming.
  - Claude'a sade bir "tel" şeması gider; Go'da `maxLength` ve aralıkları içeren tam doğrulama şeması çalışır.
  - Doğrulama başarısızsa hata mesajıyla onarım istenir, en fazla 3 deneme.
- **Alternatifler:**
  - Her şey strict tool — reddedildi: metin akışı tool girdisi streaming'iyle gelir, arayüzde parçalı dolum zorlaşır.
  - Her şey native çıktı, tool yok — reddedildi: ajan kitle sayısını kendisi yoklayamaz.
- **Sonuçlar / bedel:** İki mekanizma ve iki şema bakımı. `tool_choice: auto` ile model tool çağırmayabilir; bu durum onarım denemesi sayılmalı. `refusal` ve `max_tokens` durumlarında JSON ayrıştırmadan önce `stop_reason` kontrol edilmeli. Üç onarım başarısız olursa kullanıcıya ne gösterileceği açık.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-028: SMS en fazla 160 karakter; kodlama ve segment sayısı gösterilir

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** Brief SMS'i 160 karakterle sınırlıyor. Türkçe ç, ş, ğ, ı, İ mesajı UCS-2'ye çevirir: tek SMS 70, birleştirilmiş mesajda segment başına 67 karakter (docs/research/03 §5).
- **Karar:** SMS metni en fazla 160 karakterdir. Ölçüm `{{first_name}}` kitledeki en uzun adla genişletilerek yapılır ve gönderimden hemen önce alıcı başına tekrar edilir. Teklifte kodlama (GSM-7 / UCS-2), segment sayısı ve maliyet gösterilir.
- **Alternatifler:**
  - Tek segment sınırı (GSM-7'de 160, UCS-2'de 70) — reddedildi: Türkçe metin çok kısa kalır.
  - ASCII katlama ile GSM-7 — reddedildi: metin bozulur.
- **Sonuçlar / bedel:** Türkçe SMS çoğunlukla 3 UCS-2 segmentidir, maliyet buna göre artar. Segment sayısı UCS-2'de UTF-16 kod birimiyle hesaplanmalıdır. Gönderim anında bir alıcının metni 160'ı aşarsa ne yapılacağı açık.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-029: Proje iskeleti bu adımda yalnız klasörler ve domain belgesi; Go kodu yok

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Test ve kapsam
- **Bağlam:** Kullanıcı Go ve Vue projelerinin başlatılmasını ve data/domain tasarımını istedi; servis geliştirmesi bu adımın dışında.
- **Karar:** Go tarafında D-021'deki klasörler boş olarak açılır (`.gitkeep`); Go kaynak dosyası, migration, Dockerfile ve servis kodu yazılmaz. Domain tasarımı `docs/domain-model.md` belgesindedir: ER diyagramları, durum makineleri ve NATS olay sözleşmeleri. Vue projesi `web/` altında kurulur (D-032).
- **Alternatifler:**
  - Go domain paketleri (tipler, geçiş kuralları, birim testleri) + belge — reddedildi: kullanıcı bu adımda yalnız belge istedi.
  - Belgeye ek olarak ilk goose migration'ları — reddedildi: şema indeksler ve sorgular yazılmadan sabitlenirdi.
- **Sonuçlar / bedel:** Domain tasarımı testle doğrulanmaz; belge ile ileride yazılacak kod arasında sapma riski vardır. chi ve Viper (D-031) kod gelene kadar `go.mod`'a girmez.
- **Doğrulama:** `find cmd internal` D-021'deki klasörleri gösteriyor. Go paketi olmadığı için `make test` Go adımını atlıyor.

## D-030: Yeni servis tablolarında birincil anahtar ve olay kimliği UUIDv7

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** Kimlikler servisler arasında olaylarla taşınacak; olay id'si (D-006) ve provider `Idempotency-Key`'i (D-023) de bir uuid istiyor.
- **Karar:** `proposal_db`, `notification_db` ve `analytics_db` tablolarının birincil anahtarları `uuid`, varsayılan değer Postgres 18'in yerleşik `uuidv7()` fonksiyonu. NATS olay id'si ve `Idempotency-Key` de UUIDv7'dir. `customer_db`'deki mevcut müşteri ve ürün kimlikleri değişmez.
- **Alternatifler:**
  - `bigint identity` — reddedildi: servisler arası benzersiz değil, olay ve idempotency için ayrı uuid gerekirdi.
  - UUIDv4 — reddedildi: sırasız, indeks yerleşimi kötüleşir.
- **Sonuçlar / bedel:** Anahtar 16 bayt, bigint'in iki katı. UUIDv7 oluşturma zamanını açığa vurur; bu demoda sorun değil.
- **Doğrulama:** Doğrulanmadı — henüz tablo yok.

## D-031: Go servis altyapısı: chi v5 router, log/slog, Viper ile yapılandırma

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** Beş Go binary'si (D-018, D-021) HTTP sunucusu, yapılandırılmış log ve ortamdan yapılandırma okuma ihtiyacını paylaşıyor. Go HTTP router açık konudaydı.
- **Karar:** HTTP router `github.com/go-chi/chi/v5`; loglar `log/slog` ile. Yapılandırma `github.com/spf13/viper` ile ortam değişkenlerinden ve varsayılanlardan okunur (kullanıcı ek olarak istedi). Ortak kurulum `internal/platform/` altındadır.
- **Alternatifler:**
  - Yalnız stdlib (`net/http` ServeMux) — reddedildi: kullanıcı chi'nin hazır middleware'lerini seçti.
  - Echo veya Gin — reddedildi: handler imzaları stdlib'den sapar, gqlgen ve SSE için ek adaptör gerekir.
- **Sonuçlar / bedel:** İki dış bağımlılık. Viper büyük bir bağımlılık ağacı getirir ve anahtarları küçük harfe çevirir; ortam değişkeni eşlemesi açıkça kurulmalı. Yapılandırma dosyası kullanılıp kullanılmayacağı açık.
- **Doğrulama:** Doğrulanmadı — henüz Go kodu yok (D-029).

## D-032: Vue projesi create-vue ile kurulur; kod özellik klasörlerinde

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-012 Vue 3 + TypeScript, pnpm, Tailwind, shadcn-vue ve Vue Router'ı seçti; D-014 Vitest'i. Proje kurulum yöntemi ve klasör düzeni açıktı.
- **Karar:** `web/`, `pnpm create vue` ile TypeScript, Vue Router, Vitest, ESLint ve Prettier seçenekleriyle kurulur. Uygulama kodu `src/features/<alan>/` altında (campaigns, proposals, deliveries): sayfa, bileşen ve composable bir arada. shadcn-vue bileşenleri `src/components/ui/`, üretilen OpenAPI istemcisi `src/api/` altında (D-016).
- **Alternatifler:**
  - Tür klasörleri (`src/views`, `src/components`, `src/composables`) — reddedildi: bir özellik birçok klasöre dağılır.
  - Elle Vite kurulumu — reddedildi: lint ve test ayarları elle yapılır.
- **Sonuçlar / bedel:** create-vue'nun ürettiği örnek dosyalar temizlenmeli. Özellikler arası paylaşılan kodun yeri (`src/shared/` gibi) gerektiğinde belirlenir.
- **Doğrulama:** 2026-10-27'de create-vue 3.24.0 ile kuruldu. `pnpm -C web lint`, `pnpm -C web build` (vue-tsc dahil) ve `make test` (tek uygulama kabuğu testi) geçti.

## D-033: Kampanya durumu kaba yaşam döngüsü; adım durumu ayrı

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-022 `campaign` ve `proposal_step` durum değerlerini açık bırakmıştı.
- **Karar:**
  - `campaign.status`: `drafting → scheduled → sending → completed`; gönderim bitmeden her durumdan `cancelled`.
  - `proposal_step.status`: `open → approved`, ya da `superseded`.
  - Adım sırası kitle → kanal planı → metin → zamanlama; önceki adım onaylanmadan sonraki açılmaz.
  - Güncel adım `proposal_step` tablosundan türetilir, kampanyada saklanmaz.
- **Alternatifler:**
  - Kampanya durumu adımı yansıtır (`awaiting_audience` …) — reddedildi: aynı bilgi iki yerde tutulur.
  - Kampanya durumu saklanmaz, sorguyla türetilir — reddedildi: teslimat durumu başka serviste, servisler arası sorgu gerekirdi.
- **Sonuçlar / bedel:** `scheduled → sending → completed` geçişleri Notification'dan gelen olaylarla Proposal'da güncellenir; arada eventual-consistency penceresi vardır. Bu olayların adları ve içerikleri `docs/domain-model.md`'de taslaktır.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-034: Tüm adımlar tek varyant tablosunda; adım türüne göre CHECK

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-022 dil ve kanalı olmayan adımların (kitle, kanal planı) varyant biçimini açık bırakmıştı.
- **Karar:** `proposal_variant` tüm adımlar için tek tablodur. Kitle ve kanal planı adımları tek varyantlıdır (`language` ve `channel` NULL). Zamanlama adımı dil başına bir varyanttır (`channel` NULL, D-009). Metin adımı dil × kanal varyantıdır. `CHECK` kısıtları adım türüne göre hangi alanların dolu olacağını zorlar.
- **Alternatifler:**
  - Adım türü başına ayrı tablo — reddedildi: onay ve red mantığı dört kez yazılır.
  - Yalnız metin adımında varyant, diğerleri `proposal_step.payload` — reddedildi: onay ve red mantığı iki yerde olur.
- **Sonuçlar / bedel:** `content jsonb` alanının şeması adım türüne göre değişir ve uygulamada doğrulanır. Adım türü `proposal_variant`'ta da tutulmalı ki `CHECK` onu görebilsin.
- **Doğrulama:** Doğrulanmadı — henüz tablo yok.

## D-035: NATS olayları kendi JSON zarfıyla; konu evt.<servis>.<olay>.v1

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Gönderim
- **Bağlam:** D-006 ve D-024 olay stream'ini kurdu; zarf biçimi ve sürümleme açıktı.
- **Karar:** Her olay `{id, type, version, occurred_at, producer, correlation_id, data}` JSON zarfıdır. `id` UUIDv7'dir (D-030) ve `Nats-Msg-Id` olarak da gönderilir; `correlation_id` kampanya id'sidir. Konu `evt.<servis>.<olay>.v1`. Sürüm içinde yalnız alan eklenir; kırıcı değişiklik `.v2` konusuna gider.
- **Alternatifler:**
  - CloudEvents 1.0 JSON — reddedildi: ek bağımlılık ve NATS bağlaması için ek ayar.
  - Protobuf — reddedildi: kod üretimi ekler, mesajlar elle okunamaz.
- **Sonuçlar / bedel:** Şema doğrulaması ve sürüm uyumu elle korunur; ortak bir şema kaydı yoktur. Olay listesi ve `data` alanları `docs/domain-model.md`'de taslaktır.
- **Doğrulama:** 2026-10-27'de (T-012): `internal/platform/events` zarfı ve konu adı birim testleriyle, relay üzerinden yayınlanan zarfın `id`'sinin `Nats-Msg-Id` ile aynı olması entegrasyon testiyle doğrulandı. Olay kataloğu hâlâ taslak.

## D-036: Canlı dashboard Notification'dan; kampanya özetleri Analytics'te

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-018 Analytics'i ekledi ama dashboard'un canlı durumu hangi servisten okuyacağını açık bıraktı.
- **Karar:** Canlı gönderim durumu Notification'dan SSE ile gelir; kaynak `delivery` tablosunun anlık sayımlarıdır. Analytics olaylardan kampanya özetlerini tutar: kanal ve durum sayıları, token kullanımı ve maliyet. Geçmiş rapor ekranı Analytics'ten beslenir.
- **Alternatifler:**
  - Dashboard tamamen Analytics'ten, teslimat başına durum olayı — reddedildi: olay hacmi teslimat sayısı kadar.
  - Analytics yalnız token ve maliyet — reddedildi: Analytics'in rolü daralır.
- **Sonuçlar / bedel:** Aynı sayılar iki serviste farklı gecikmeyle görünebilir: Notification'da anlık, Analytics'te olay geldikten sonra. nginx'in SSE yolunu Notification'a yönlendirmesi gerekir.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-037: Vue: oxlint ESLint'in yanında kalır; Geist fontu pakete gömülür

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-032 kurulumunda create-vue 3.24.0, ESLint'in yanına oxlint'i de ekledi. shadcn-vue 2.8.2 kurulumu ise Geist fontunu çalışma anında Google Fonts'tan yüklüyordu. İkisi de kullanıcıya sorulmadan gelmişti.
- **Karar:** `pnpm lint` önce `oxlint`, sonra `eslint` çalıştırır; `eslint-plugin-oxlint` çakışan ESLint kurallarını kapatır. Geist fontu `@fontsource-variable/geist` paketiyle build'e girer; `src/assets/main.css`'te Google Fonts isteği yoktur.
- **Alternatifler:**
  - Yalnız ESLint — reddedildi: create-vue'nun güncel varsayılanı korundu.
  - Google Fonts'tan yükleme — reddedildi: her açılışta dış istek, internetsiz ortamda yedek fonta düşer.
  - Sistem fontu — reddedildi: görünüm işletim sistemine göre değişir.
- **Sonuçlar / bedel:** İki lint aracı ve iki yapılandırma dosyası (`.oxlintrc.json`, `eslint.config.ts`). Font dosyaları build'i yaklaşık 46 kB büyütür.
- **Doğrulama:** 2026-10-27'de `pnpm -C web lint` ve `pnpm -C web build` geçti; `src/` ve `dist/` içinde `googleapis` geçmiyor.

## D-038: DSL kendi Pratt ayrıştırıcımızla ayrıştırılır; D-007'deki expr-lang seçiminin yerini alır

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-007, Proposal'ın DSL'i `expr-lang/expr` ile ayrıştırmasını öngörüyordu. Kullanıcının DSL belgesi (`docs/coldbru-gqdsl.md`) `===`, `~=` ve `orders.some(o => …)` sözdizimini tanımlıyor; bunlar expr'in sözdizimi değil.
- **Karar:** Proposal'da DSL için kendi lexer'ımız ve öncelik farkındalıklı (Pratt) ayrıştırıcımız yazılır. Çıktı tipli bir AST'dir; ad çözümleme ve tip denetimi ayrı bir geçiştir. Hata mesajları kaynak konumu taşır, böylece LLM onarım denemesinde (D-027) hatayı görür. D-007'nin geri kalanı geçerlidir.
- **Alternatifler:**
  - Dil bilgisini expr sözdizimine uydurmak — reddedildi: `~=` ve ok fonksiyonları gider, sözdizimi LLM'e daha az tanıdık.
  - goja + ön işleme — reddedildi: token farkındalıklı ön işleme ve konum eşlemesi gerekir.
- **Sonuçlar / bedel:** Lexer, ayrıştırıcı ve testleri bizim bakımımızda; fuzz testi gerekir. Dil bilgisindeki muğlaklıklar (alan ile ilişki referansının aynı başlaması, `some`/`none`/`every`'nin ayrılmış kelime olması, sayı biçimi) ayrıştırıcı yazılırken kesinleşmeli.
- **Doğrulama:** 2026-10-28'de (T-018): Pratt ayrıştırıcısı, tipli AST, konumlu hatalar ve kanonik yazıcı `internal/proposal/dsl`'de; ayrıntı D-065.

## D-039: Göreli zaman ago("30d") ile yazılır, Customer sorgu anında now() ile çözer

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** Örnek brief "son 30 günde" diyor. DSL belgesinin 0.1 kapsamında tarih değeri yoktu, bu yüzden örnek brief ifade edilemiyordu.
- **Karar:** DSL'de göreli zaman `ago("30d")`, mutlak tarih `date("2026-01-01")` ile yazılır. Derleyici göreli değeri GraphQL'e göreli biçimde taşır (ör. `{gte: {ago: "30d"}}`); Customer bunu sorgu anında `now()` ile mutlak zamana çevirir. LLM tarih aritmetiği yapmaz.
- **Alternatifler:**
  - Yalnız mutlak tarih — reddedildi: LLM tarih hesabında hata yapabilir, sorgu sonra çalışınca bayatlar.
  - Yalnız `dim_customer` özet alanları — reddedildi: ilişki pencereli kurallar kaba kalır.
- **Sonuçlar / bedel:** Aynı kural farklı anda farklı kitle döndürür; önizleme ile alıcı listesi arasındaki fark bu yüzden beklenir. D-017'deki seed kaydırmasıyla birlikte örnek brief tazeyken çalışır. Süre birimleri (`d`, `h` …) ve GraphQL girdi biçimi şema yazılırken kesinleşir.
- **Doğrulama:** Doğrulanmadı — henüz kod yok. Hedef test: örnek brief'in DSL'i `ago("30d")` ile 33 kişi döndürmeli.

## D-040: Şehir kapalı listeden ve kanonik anahtarla eşleşir

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Kitle ve izinler
- **Bağlam:** "İstanbul", "istanbul" ve "ISTANBUL" aynı şehir olmalı. Alpine/musl'de `ILIKE` noktalı İ için eşleşiyor, noktasız ı için eşleşmiyor (docs/research/04 §4).
- **Karar:** `city === "…"` iki tarafı kanonik anahtara çevirerek karşılaştırır: ı, İ ve I önce i olur, sonra NFD ve küçük harf. Değer bilinen şehir listesinde yoksa `UNKNOWN_VALUE` hatası öneri listesiyle döner. Birden çok şehir `city in ["İstanbul", "Ankara"]` ile yazılır. Kural Customer'da uygulanır, collation'a dayanmaz.
- **Alternatifler:**
  - Büyük/küçük harfe duyarlı tam eşleşme — reddedildi: "istanbul" 0 kişi döndürür.
  - `~=` ile trigram benzerliği — reddedildi: `pg_trgm` gerekir, yanlış şehri eşleyebilir.
- **Sonuçlar / bedel:** Şehir listesi ve kanonik anahtar Customer'da tutulmalı (ör. kanonik anahtar kolonu veya bellekte sözlük; seçim migration/uygulama sırasında). Yeni şehir eklenince liste güncellenir.
- **Doğrulama:** 2026-10-27'de (T-015): `İstanbul`, `istanbul`, `ISTANBUL`, `Istanbul`, `ıstanbul` aynı sayımı verdi; `Istnbul` `UNKNOWN_VALUE` ve `İstanbul` önerisi döndü (entegrasyon testi, seed'li customer_db).

## D-041: LLM yalnız filtreyi yazar; varlık ve dönen alanlar Proposal kodunda sabittir

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** DSL belgesinin sorgu zarfı entity, filter ve selection içeriyor. D-027'de `propose_segment` tool'unun tek alanı DSL metni.
- **Karar:** LLM yalnız filtre ifadesini yazar. Kök varlık her zaman müşteridir. Dönen alanlar Proposal kodunda amaç başına sabittir: önizlemede sayım ve kanal kapsaması, alıcı listesinde kimlik, ad, dil ve iletişim bilgisi. D-027'deki tek alanlı tool korunur.
- **Alternatifler:**
  - LLM varlık ve filtre yazar — reddedildi: kampanyada kök hep müşteri, kazanç yok.
  - LLM tüm zarfı yazar — reddedildi: tool şeması büyür, kişisel veri istenebilir.
- **Sonuçlar / bedel:** DSL belgesindeki seçim (§7) bu projede LLM'e açılmaz; yalnız Proposal kodu kullanır. Yeni bir görünüm gerekirse kod değişikliği ister.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-042: Filtre kataloğu kişisel olmayan tüm alanları içerir; izin ve hesap alanları dışarıda

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Kitle ve izinler
- **Bağlam:** D-004 ve D-007 hassas hedefleme alanlarının kataloğa girip girmeyeceğini ayrı karara bırakmıştı. Kullanıcı erişim yönetiminin her durumda GraphQL uç noktasında olacağını belirtti.
- **Karar:**
  - Customer GraphQL filtre girdisi `dim_customer`'ın kişisel olmayan tüm alanlarını içerir: şehir, ülke, dil, saat dilimi, platform, kayıt ve aktivite tarihleri, sipariş/harcama/sepet özetleri, favori kategori, sadakat alanları, kampanya sayıları; yaş, cinsiyet, medeni durum, bakmakla yükümlü olunan kişi sayısı, maaş, emeklilik, emekli olma, işe uzaklık ve araç alanları dahil.
  - Dışarıda kalanlar: ad, soyad, e-posta, telefon, ham adres alanları, doğum tarihi ve işveren adı (kişisel tanımlayıcılar); `emailOptIn`, `smsOptIn`, `emailBounced`, `accountStatus`, `deletedAt` ve `consentUpdatedAt` (ulaşılabilirlik sunucu kuralı ve kanal planı adımında kalır, D-007, D-010).
  - İlişkiler: siparişler, sepet kalemleri ve olaylar, kendi kişisel olmayan alanlarıyla.
  - Katalogda olmayan alan sunucuda reddedilir; Proposal'daki denetim yalnız kolaylıktır.
- **Alternatifler:**
  - Yalnız davranış ve temel profil — reddedildi: kullanıcı daha geniş hedefleme istedi.
  - Bunlara ek yalnız yaş ve cinsiyet — reddedildi: aynı nedenle.
- **Sonuçlar / bedel:** Yaş, cinsiyet, maaş ve araç gibi alanlarla ayrımcı veya hassas hedefleme mümkün olur; bu bilinçli olarak kabul edildi. Ad, soyad, doğum tarihi ve işveren adının "kişisel" sayılmasını kullanıcı ayrıca onayladı. Sipariş kalemi ve ürün alanlarının ilişki filtresine girip girmeyeceği açık.
- **Doğrulama:** Doğrulanmadı — henüz şema yok.

## D-043: Erişim kontrolü Customer GraphQL uç noktasında; servis token'ı ve kapsam

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Kitle ve izinler
- **Bağlam:** Kullanıcı erişim yönetiminin her durumda GraphQL uç noktası üzerinden olacağını belirtti. Proposal ve Notification aynı uç noktayı farklı amaçlarla çağırıyor (D-007, D-011).
- **Karar:** Her servis Customer'ı kendi token'ıyla çağırır (`Authorization` header; değer compose ortam değişkeninden, `.env.example`'da yalnız dev varsayılanı). Resolver çağıranın kapsamını kontrol eder. Proposal: sayım, önizleme ve alıcı listesi. Notification: izin kontrolü. `updateConsent` ayrı bir kapsamdır. nginx Customer'ı dışarı açmaz. Alan beyaz listesi, derinlik ve karmaşıklık sınırları da aynı uç noktada zorlanır.
- **Alternatifler:**
  - Çağıran kimliği olmadan şema şekli + ağ yalıtımı — reddedildi: iç ağdaki her istemci her şeyi çağırabilir.
  - JWT + rol — reddedildi: anahtar yönetimi ve süre yenileme demo için ağır.
- **Sonuçlar / bedel:** Statik token'lar süresiz; sızarsa elle değiştirilir. Arayüzden izin geri çekme gösterimi (D-011) için `updateConsent` kapsamını hangi servisin taşıyacağı açık.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-044: Customer GraphQL şeması tek kaynak; Proposal kataloğu build'de üretilir; alan adları camelCase

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** Proposal'daki DSL derleyicisi alan ve tip bilgisine ihtiyaç duyar; erişimi Customer zorlar (D-043).
- **Karar:** Customer'ın gqlgen şeması (`.graphqls`) tek kaynaktır. Proposal'ın kataloğu build sırasında bu şemadan ve prompt açıklamalarını içeren küçük bir metadata dosyasından üretilir; çalışma anında introspection yapılmaz. GraphQL ve DSL alan adları camelCase'dir (`totalSpendGbp`); snake_case kolonlara eşleme Customer'dadır.
- **Alternatifler:**
  - Çalışma anında introspection — reddedildi: Customer ayakta olmadan Proposal başlayamaz.
  - Elle yazılmış katalog — reddedildi: şema değişince iki yer elle güncellenir.
  - snake_case alan adları — reddedildi: GraphQL ve JS benzeri DSL için alışılmadık.
- **Sonuçlar / bedel:** Bir kod üretim adımı daha (gqlgen ve sqlc'nin yanında). Metadata dosyası şemayla elle senkron tutulur; sapma build'de yakalanmalı.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-045: DSL 0.1 kapsamı: çekirdek işleçler, ilişki niceleyicileri, ago/date, in, boolean

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** DSL belgesinin §15 kapsamı örnek brief'i ifade edemiyordu ve tanımsız eşleşme modlu `~=` ile alanlar arası karşılaştırmayı içeriyordu.
- **Karar:** 0.1'de `===`, `!==`, `>`, `>=`, `<`, `<=`, `&&`, `||`, `!`, parantez, `some` / `none` / `every`, `ago()` ve `date()` (D-039), `in [...]` ve `true` / `false` desteklenir. `~=` ve alanlar arası karşılaştırma ayrılmıştır; kullanılırsa `UNSUPPORTED_OPERATOR` döner. `==` ve `!=` tanımsız kalır ve reddedilir.
- **Alternatifler:**
  - Belgedeki §15'in tamamı ve eklemeler — reddedildi: `~=` için eşleşme modu ve `pg_trgm` kararı gerekir, `compare` girdi tipi şemayı büyütür; katalogda serbest metin alanı yok.
  - Yalnız §15 — reddedildi: örnek brief ifade edilemez.
- **Sonuçlar / bedel:** Metin araması ve alan-alan karşılaştırması gerekirse yeni bir karar ve sürüm ister. Null kuralları ve sayı biçimi ayrıştırıcı yazılırken kesinleşmeli.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-046: DSL 0.1: yalın boolean alan yazımı yok; boş in listesi hata

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-045 boolean değerleri ve `in`'i 0.1'e aldı. DSL belgesi güncellenirken iki dil bilgisi ayrıntısı ortaya çıktı.
- **Karar:** Boolean alanlar yalnız açık karşılaştırmayla yazılır (`hasOpenCart === true`); `hasOpenCart` veya `!retired` gibi yalın yazım 0.1 dil bilgisinde yoktur. `in` listesi en az bir değer içerir; `city in []` `SYNTAX_ERROR` döner.
- **Alternatifler:**
  - Yalın boolean yazımı da desteklemek — reddedildi: dil bilgisi karmaşıklaşır, her yüklem bir karşılaştırma olmaz.
  - Boş listeyi geçerli ve her zaman yanlış saymak — reddedildi: 0 kişilik kitle sessizce üretilir.
- **Sonuçlar / bedel:** LLM boolean koşulları biraz daha uzun yazar ve yalın yazımı denerse onarım turu harcar; sistem prompt'u açık karşılaştırmayı örneklemeli.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-047: Claude API istemcisi resmi anthropic-sdk-go v2

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-026 ve D-027 modelleri ve çıktı yöntemini seçti; Proposal'ın Claude API'ye hangi istemciyle bağlanacağı kayıtlı değildi. `tasks.csv` görev listesi hazırlanırken fark edildi.
- **Karar:** Proposal, resmi Go SDK'sı `github.com/anthropics/anthropic-sdk-go/v2` ile çağırır (2026-10-27 araştırmasına göre son sürüm v2.3.0). Mock LLM testleri gerçek adaptörü `option.WithBaseURL` ile `httptest` sunucusuna bağlar (D-025). API anahtarı ortamdaki `ANTHROPIC_API_KEY`'den okunur; testler okumaz.
- **Alternatifler:**
  - `net/http` ile kendi istemcimiz — reddedildi: retry, streaming ve tipler elle yazılırdı.
- **Sonuçlar / bedel:** Bir dış bağımlılık. v2 kırıcı bir sürümdü; eski örnekler v1 modül yolunu gösterir. SDK varsayılan olarak 2 kez yeniden dener; onarım döngüsüyle (D-027) birlikte toplam deneme sayısı hesaba katılmalı.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-048: Servis DB'lerinde tablolar servis adlı şemada; healthcheck her servis rolünü kontrol eder

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-019 servis veritabanlarını ve rollerini initdb'ye verdi; tabloların hangi şemada duracağı ve compose sağlık kontrolünün yeni DB'leri kapsayıp kapsamayacağı açıktı (T-008).
- **Karar:** initdb her servis DB'sinde servis adıyla bir şema açar (`proposal`, `notification`, `analytics`), sahibi servis rolüdür; rolün o DB'deki `search_path`'i `<şema>, public` olur (customer_db'deki `dwh, public` deseni). goose sürüm tablosu da bu şemaya düşer. Postgres healthcheck'i mevcut customer kontrolüne ek olarak her servis rolünün kendi DB'sine TCP'den bağlanıp kendi şemasını gördüğünü kontrol eder.
- **Alternatifler:**
  - `public` şeması — reddedildi: customer_db'deki desenle tutarsız.
  - Şemayı ilk migration'ın açması — reddedildi: ilk migration'a kadar `search_path` olmayan bir şemayı gösterir.
  - Healthcheck yalnız customer_db — reddedildi: yeni DB'lerin açılışı sağlık durumuna yansımaz.
- **Sonuçlar / bedel:** Şema adı initdb ile migration'lar arasında sabit bir sözleşmedir; değişirse ikisi birlikte değişir. Healthcheck komutu uzar.
- **Doğrulama:** 2026-10-27'de (T-008): her DB'de servis adlı şema servis rolünün sahipliğinde, `current_schema()` servis adını döndürüyor, `uuidv7()` varsayılanlı bir deneme tablosu o şemaya yazıldı ve silindi; compose healthcheck `healthy`. Elle doğrulandı.

## D-049: NATS: nats:2.15-alpine, dosya depolama, stream'leri nats-box init konteyneri kurar

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Gönderim
- **Bağlam:** D-006 ve D-024 NATS JetStream'i ve dört stream'i seçti; imaj, depolama, stream'leri kimin kuracağı, stream ayarları ve erişim açıktı (T-009). `RECIPIENTS` ve `SCHEDULES` konuları domain belgesinde taslaktı.
- **Karar:**
  - İmaj `nats:2.15-alpine` (2.15.0: 2026-09-17; kurulum anında 2.15.1). JetStream dosya depolaması ayrı bir compose volume'unda.
  - Stream'leri compose'daki tek seferlik bir nats-box konteyneri kurar: `infra/nats/` altındaki JSON tanımlarını `nats stream add` / `edit` ile uygular. Servisler stream oluşturmaz, yalnız kullanır.
  - Stream'ler, hepsi dosya depolamalı, tek replika, 2 dakika dedup penceresi, 1 MB ileti sınırı:
    - `EVENTS`: `evt.>`, limits, 7 gün.
    - `RECIPIENTS`: `recipients.>`, work-queue.
    - `SCHEDULES`: `sched.>` ve `fire.>`, limits, `AllowMsgSchedules`, süre sınırı yok.
    - `SEND`: `send.>`, work-queue.
  - Kimlik doğrulama yok (compose iç ağı). 4222 (istemci) ve 8222 (izleme) host'a açılır, `NATS_HOST_PORT` / `NATS_MONITOR_HOST_PORT` ile değişir. Healthcheck `8222/healthz?js-enabled-only=true`.
- **Alternatifler:**
  - `nats:2.14-alpine` — reddedildi: daha yeni seri tercih edildi.
  - Bellek depolama — reddedildi: restart'ta zamanlamalar ve kuyruktaki teslimatlar kaybolur.
  - Stream'leri servislerin açılışta oluşturması veya ikisi birden — reddedildi: tanımlar tek yerde ve servis kodu olmadan doğrulanabilsin.
  - `EVENTS` 24 saat veya süresiz — reddedildi.
  - Servis başına NATS kullanıcısı veya kapalı port — reddedildi: demo ve yerel testler için gereksiz sıkılık.
- **Sonuçlar / bedel:** İç ağdaki her istemci her konuya yayın yapabilir; erişim kontrolü yalnız Customer GraphQL'de (D-043). `SCHEDULES` süresiz olduğu için tetiklenen `fire.*` iletileri birikir; temizlik veya `fire.*` tüketim politikası Notification yazılırken ele alınmalı. Stream tanımı değişince init konteyneri yeniden çalıştırılmalı.
- **Doğrulama:** 2026-10-27'de (T-009): `docker compose up` ile NATS `healthy`, `nats-init` dört stream'i oluşturdu ve çıkış kodu 0; ikinci çalıştırma stream'leri güncelledi. Her `docker compose up` init'i yeniden çalıştırır. nats-box ile denendi: `@at` zamanlaması 6 saniye sonra `fire.*` konusuna düştü; aynı `Nats-Msg-Id` ile iki yayın tek ileti oldu; `SEND` iletisi ack sonrası silindi; NATS restart'ından sonra iletiler kaldı. Deneme iletileri temizlendi. Elle doğrulandı.

## D-050: Platform: /healthz + /readyz, ortam + isteğe bağlı .env, slog JSON istek logu, Prometheus /metrics

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-031 chi, slog ve Viper'ı seçti; health içeriği, metrik biçimi ve yapılandırma dosyası açıktı (T-010). Brief canlı durum, metrics ve health check uç noktaları istiyor; API anahtarı `.env`'den okunmalı.
- **Karar:**
  - Health: `/healthz` süreç ayaktaysa 200 döner, bağımlılığa bakmaz. `/readyz` servisin kaydettiği kontrolleri (DB ping, NATS bağlantısı vb.) kontrol başına 2 saniye sınırla çalıştırır; yanıt JSON `{status, checks}`, biri başarısızsa 503. Compose ve nginx `/readyz`'e bakar.
  - Yapılandırma: önek olmadan ortam değişkenleri (`HTTP_ADDR`, `DATABASE_URL`, `NATS_URL`, `ANTHROPIC_API_KEY` …), kodda varsayılanlar; çalışma dizininde `.env` varsa Viper onu da okur, ortam önceliklidir. Zorunlu anahtarlar açılışta doğrulanır. YAML dosyası yok.
  - Log: `log/slog` JSON stdout'a, seviye `LOG_LEVEL` (varsayılan `info`). chi `RequestID` ve `Recoverer`; kendi slog istek logu (method, path, status, süre, request_id).
  - Metrik: `github.com/prometheus/client_golang` (2026-10-27 itibarıyla v1.25.0); platformda `/metrics` ve HTTP istek metrikleri (sayı ve süre histogramı, route şablonu etiketiyle). Servise özel metrikler servis görevlerinde eklenir.
- **Alternatifler:**
  - Tek `/health` veya yalnız `/healthz` — reddedildi: bağımlılık kesintisi süreç sağlığıyla karışır ya da hiç görünmez.
  - Yalnız ortam veya YAML + ortam — reddedildi: brief yerelde `.env` okumayı istiyor; YAML gereksiz dosya ekler.
  - Ortama göre log formatı veya istek logu olmadan — reddedildi.
  - Prometheus'u servis görevlerine bırakmak veya OpenTelemetry — reddedildi: metrik uç noktası erkenden ve az bağımlılıkla gelsin.
- **Sonuçlar / bedel:** JSON loglar terminalde okunması zor. `/readyz`'in 503'ü yük dengeleyiciyi o örnekten uzaklaştırır; DB kısa süre kesilse süreç ayakta kalır. Önek olmadan ortam değişkenleri servisler arasında aynı adları kullanır; her servis kendi değerini compose'da alır. Route şablonu etiketi kardinaliteyi düşük tutar.
- **Doğrulama:** 2026-10-27'de (T-010): `internal/platform/{config,logging,health,metrics,httpserver}` birim testleri `go test -race` ile geçti; `go vet` temiz. Testler: ortamın `.env`'i ve varsayılanı ezmesi, eksik `.env`'in hata olmaması, `Require`'ın tüm eksikleri raporlaması, `/readyz`'in başarısız ve zaman aşımına uğrayan kontrolde 503 dönmesi, metriklerde `route` etiketinin chi şablonu olması, panik yapan isteğin 500 ile loglanıp sayılması, bağlam iptalinde sunucunun temiz kapanması. Henüz bir servis bu paketleri kullanmıyor.

## D-051: sqlc go tool ile, üretilen kod commit edilir; entegrasyon testleri integration tag'iyle make test içinde; migration'lar sıralı numaralı

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Test ve kapsam
- **Bağlam:** D-020 goose, sqlc ve pgx/v5'i, D-025 testcontainers-go ile entegrasyon testlerini seçti. sqlc'nin nasıl çalıştırılacağı, üretilen kodun durumu, entegrasyon testlerinin `make test`'teki yeri ve migration numaralandırması açıktı (T-011).
- **Karar:**
  - sqlc v1.31.1 `go.mod`'da `tool` olarak sabitlenir ve `go tool sqlc generate` ile çalışır. `make generate` hedefi eklenir. Üretilen kod commit edilir.
  - Entegrasyon testleri `//go:build integration` ile ayrılır. `make test` birim ve entegrasyon testlerini birlikte çalıştırır (Docker gerekir); `make test-unit` yalnız birim testleri çalıştırır.
  - goose migration dosyaları sıralı numaralıdır: `00001_<ad>.sql`.
- **Alternatifler:**
  - sqlc Docker imajıyla — reddedildi: Docker'a bağımlı kod üretimi.
  - Üretilen kodu commit etmeyip build'de üretmek — reddedildi: derleme ve Dockerfile sqlc'ye bağımlı olur.
  - Entegrasyonu ayrı `make test-integration` hedefine almak veya tag olmadan Docker yoksa atlamak — reddedildi: brief'in `make test` beklentisi tek komutta karşılansın, atlanan testler gözden kaçmasın.
  - Zaman damgalı migration adları — reddedildi.
- **Sonuçlar / bedel:** sqlc'nin Postgres ayrıştırıcısı cgo ister; ilk `go tool sqlc` derlemesi yavaştır ve bir C derleyicisi gerekir. `make test` Docker olmadan başarısız olur. Paralel dallarda aynı migration numarası çakışabilir; merge'de düzeltilir.
- **Doğrulama:** 2026-10-27'de (T-011): `make test` birim ve entegrasyon testlerini birlikte çalıştırdı; testcontainers ile `postgres:18-alpine` üzerinde 7 test geçti (`TestUp_AppliesMigrations`, `TestUp_Idempotent`, `TestUp_UsesSearchPathSchema`, `TestUp_ConcurrentCallers`, `TestOpenAndCheck` …). `go tool sqlc version` v1.31.1 basıyor; `make generate` henüz `sqlc.yaml` olmadığı için hiçbir şey yapmıyor, döngüsü sınanmadı.

## D-052: Unit test coverage yerel araçlarla raporlanır (go cover, vitest v8); eşik yok

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Test ve kapsam
- **Bağlam:** Kullanıcı unit test coverage'ının bir araçla takip edilmesini istedi (T-011 sırasında).
- **Karar:** Go için `go test -coverprofile` ve `go tool cover` (fonksiyon özeti + HTML), web için `@vitest/coverage-v8`. `make coverage` raporları kökteki `coverage/` dizinine yazar (gitignore) ve toplamları basar. Coverage yalnız raporlanır, alt sınır zorlanmaz. Kurulum T-011 içinde yapılır.
- **Alternatifler:**
  - Codecov / Coveralls — reddedildi: uzak repo ve CI gerekir, ikisi de henüz yok.
  - Yerel + SonarQube — reddedildi: ağır konteyner ve kurulum.
  - Go için %70 veya paket başına %80 alt sınır — reddedildi: kod tabanı yeni başlıyor; eşik servisler yazıldıkça ayrıca kararlaştırılabilir.
  - Ayrı bir görev — reddedildi: Makefile aynı görevde bir kez düzenlensin.
- **Sonuçlar / bedel:** Geçmiş ve trend yok; düşüş yalnız raporda görülür. Üretilen kod (sqlc, gqlgen) geldiğinde rapordan dışlanması gerekebilir.
- **Doğrulama:** 2026-10-27'de (T-011): `make coverage` Go unit toplamını %80,2 olarak bastı, `coverage/go.out` ve `coverage/go.html` yazıldı; web özeti satırlarda %33,3 (tek kabuk testi). `migrate` paketi yalnız entegrasyon testine sahip olduğu için unit coverage'da %0 görünür.

## D-053: NATS Go istemcisi nats.go v1.54.0, jetstream paketi

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Gönderim
- **Bağlam:** D-006, D-024 ve D-049 NATS JetStream'i kurdu; Go tarafındaki istemci seçilmemişti (T-012).
- **Karar:** `github.com/nats-io/nats.go` v1.54.0 (2026-09-18) ve yeni `jetstream` paketi: pull consumer, PubAck beklemeli yayın, `Nats-Msg-Id` ile dedup.
- **Alternatifler:**
  - v1.55.0 — reddedildi: 2026-10-23'te çıktı, henüz az sınanmış.
- **Sonuçlar / bedel:** Zamanlanmış mesaj header'ları (`Nats-Schedule`) istemci desteği gerektirmez, düz header olarak gönderilir. Sürüm yükseltmesi ayrıca yapılır.
- **Doğrulama:** 2026-10-27'de (T-012): `jetstream` paketiyle PubAck'li yayın, `Nats-Msg-Id` dedup'u, pull consumer, `NakWithDelay` ve `Term` testcontainers `nats:2.15-alpine` üzerinde entegrasyon testleriyle sınandı.

## D-054: Outbox ve processed_event platform migration setinde; relay LISTEN/NOTIFY + 1 sn poll; tüketicide NakWithDelay

- **Durum:** Kabul edildi · 2026-10-27; tüketici backoff parametreleri (2 dk tavan, `MaxDeliver` 5) yerini aldı → D-055 · 2026-10-27
- **Alan:** Gönderim
- **Bağlam:** D-006 transactional outbox'ı, D-023 tüketici dedup'unu, D-035 olay zarfını seçti. Tabloları kimin açacağı, relay yöntemi, tüketici hata politikası, saklama ve indeksler açıktı (T-012).
- **Karar:**
  - `internal/platform` kendi gömülü migration setiyle `outbox` ve `processed_event` tablolarını, ayrı bir goose sürüm tablosuyla (`goose_platform_version`) servis migration'larından önce uygular. Tablolar servis şemasına düşer (D-048).
  - Relay: `outbox` üzerinde `AFTER INSERT FOR EACH STATEMENT` trigger'ı `pg_notify('outbox', '')` ile relay'i uyandırır (yük yok); relay ayrı bir bağlantıda `LISTEN` eder, 1 saniyelik poll yedektir. `FOR UPDATE SKIP LOCKED` ile okur, `Nats-Msg-Id` = olay id'si ile yayınlar, PubAck'i bekler, `published_at`'i işaretler.
  - İndeks: `outbox (created_at) WHERE published_at IS NULL` kısmi indeksi; `processed_event`'te yalnız birincil anahtar.
  - Saklama: yayınlanmış `outbox` satırları ve `processed_event` satırları silinmez.
  - Tüketici: servis başına durable pull consumer, açık ack. Handler hatasında `NakWithDelay`: 1 saniyeden başlayıp ×2 artan, %20 jitter'lı, en fazla 2 dakika; `MaxDeliver` 5, son denemede log ve `Term`.
- **Alternatifler:**
  - DDL'i her servisin kendi migration'ına kopyalamak veya Go migration sabiti — reddedildi: DDL tek yerde olsun, SQL dosyası olarak okunsun.
  - 200 ms poll — reddedildi: kullanıcı NOTIFY ile daha düşük gecikmeyi seçti.
  - Sunucu tarafı BackOff veya yeniden denemesiz — reddedildi: gecikme mantığı kodda, sunucudan bağımsız test edilsin.
  - 7 gün saklama veya outbox'ı hemen silmek — reddedildi: tam olay geçmişi tutulsun.
  - Satır başına trigger veya tam indeks — reddedildi.
- **Sonuçlar / bedel:** Tablolar sınırsız büyür; demo için sorun değil, uzun çalışan ortamda temizlik gerekir. `LISTEN` için havuz dışında ayrı bir bağlantı tutulur. Relay iki kez yayınlayabilir; JetStream dedup penceresi (2 dk) ve `processed_event` tekrarı engeller. İki servisin migration seti aynı şemada iki goose sürüm tablosu demektir.
- **Ek (kullanıcı onayı, 2026-10-27):** Tüketicinin `AckWait`'i 30 saniyedir. Geçersiz JSON, boş veya UUID olmayan id taşıyan iletiler yeniden denenmeden loglanır ve `Term` edilir.
- **Doğrulama:** 2026-10-27'de (T-012), testcontainers Postgres 18 + NATS 2.15 üzerinde: poll aralığı 1 saatken trigger relay'i uyandırdı ve olay yayınlandı (`TestRelay_NotifyWakesRelay`); relay başlamadan yazılan olaylar oluşturulma sırasıyla yayınlandı; geri alınan transaction hiçbir şey yayınlamadı; aynı outbox satırı yeniden yayınlanınca stream'de tek ileti kaldı; aynı olay iki farklı Msg-Id ile gelince handler bir kez çalıştı; sürekli hata veren handler tam `MaxDeliver` kez çağrılıp sonlandırıldı; bozuk ileti handler'a ulaşmadan sonlandırıldı. `UpPlatform` tabloları servis şemasına açtı. Kopan LISTEN bağlantısından toparlanma yolu kodda var, testi yok.

## D-055: Olay tüketicisi backoff'u: 1 sn'den üstel, 15 dk tavan, 20 deneme

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Gönderim
- **Bağlam:** D-054 olay tüketicisi için 1 sn ×2, %20 jitter, 2 dk tavan ve 5 deneme seçmişti. Kullanıcı T-012 sırasında varsayılanın üstel kalmasını, en uzun beklemenin 15 dakika ve deneme sayısının 20 olmasını istedi. D-054'ün yalnız bu parametreleri değişir.
- **Karar:** `consumer.DefaultBackoff` 1 saniyeden başlayıp ×2 artan, %20 jitter'lı, en fazla 15 dakikalık üstel geri çekilmedir; `consumer.MaxDeliver` 20'dir, son denemede log ve `Term`. Kapsam olay tüketicileridir; D-024'teki gönderim (`SEND`) retry'ı gönderim worker'ı yazılırken (T-029) ayrıca kararlaştırılır.
- **Alternatifler:**
  - D-054'teki 2 dk tavan ve 5 deneme — reddedildi: kullanıcı daha uzun toparlanma penceresi istedi.
  - Aynı varsayılanı `SEND`'e de uygulamak — reddedildi: başarısız bir mesaj ~2,8 saat sonra `failed` olurdu; gönderim için ayrı karar.
- **Sonuçlar / bedel:** Toplam yeniden deneme penceresi yaklaşık 2,8 saattir (1+2+…+512 sn, sonra 11 kez ~15 dk). Kalıcı bir hata, olayın terminate edilmesini saatlerce geciktirir ve aynı olay bu süre boyunca 20 kez işlenmeye çalışılır. `AckWait` 30 saniye kalır.
- **Doğrulama:** 2026-10-27'de (T-012): `TestBackoff_Delay` 1, 2, 4 … 512 sn ve 11. denemeden itibaren 15 dk (±%20) değerlerini doğruluyor; `TestProcess_AlwaysFailingIsTerminatedAtMaxDeliver` testcontainers NATS üzerinde handler'ın tam 20 kez çağrılıp sonlandırıldığını gösterdi (testte küçük bir backoff ile).

## D-056: Servis konteynerleri: platforma bağlı iskelet main'ler, distroless nonroot imaj, customer dışında host portları, postgres ve nats-init beklenir

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-018 beş servisi ayrı konteyner yaptı, D-021 tek çok aşamalı Dockerfile ve `SERVICE` build arg'ını seçti. `cmd/` altında henüz `main` yoktu; binary'lerin ne yapacağı, çalışma imajı, portlar ve açılış sırası açıktı (T-013).
- **Karar:**
  - Her servisin `main`'i platforma bağlı bir iskelettir: yapılandırma, log, `/healthz` `/readyz` `/metrics`, sinyalle kapanış. DB'li servislerde pgx havuzu ve readiness kontrolü; Proposal, Notification ve Analytics'te ayrıca `UpPlatform` ve NATS bağlantısı ile readiness kontrolü. İş mantığı yoktur.
  - İmaj: builder `golang:1.27-alpine`, `CGO_ENABLED=0` statik binary; çalışma imajı `gcr.io/distroless/static-debian12:nonroot`. Kabuk olmadığı için compose healthcheck binary'nin kendi `-healthcheck` bayrağıyla `/readyz`'i yoklar.
  - Portlar: konteyner içinde hepsi `:8080`. Host'ta proposal 8081, notification 8082, analytics 8083, mockprovider 8084 (ortam değişkeniyle değişir); customer yalnız compose ağında (D-043).
  - Açılış: servisler `postgres` sağlıklı olana ve `nats-init` başarıyla bitene kadar bekler (`depends_on`); mockprovider hiçbir şeyi beklemez.
- **Alternatifler:**
  - Yalnız HTTP iskeleti veya compose profile'ında bekletmek — reddedildi: compose'da servisler gerçek bağımlılıklarıyla sağlıklı görünsün.
  - alpine veya scratch çalışma imajı — reddedildi: daha büyük saldırı yüzeyi ya da elle eklenen sertifika/kullanıcı.
  - Hiç port açmamak veya customer'ı da açmak — reddedildi.
  - `depends_on` olmadan servisin kendisinin yeniden denemesi — reddedildi.
- **Sonuçlar / bedel:** İmajda kabuk yok; hata ayıklama için `docker compose exec` ile kabuk açılamaz. Statik binary cgo gerektiren kütüphane kullanamaz (pgx ve nats.go saf Go). `depends_on` yalnız ilk açılışı sıralar; bağımlılık sonradan düşerse `/readyz` 503 döner.
- **Doğrulama:** 2026-10-27'de (T-013): `docker compose up` ile beş servis, postgres ve nats `healthy`, `nats-init` 0 ile çıktı. 8081–8083 `/readyz` postgres ve nats için `ok`, 8084 `ok`; customer host'tan erişilemiyor, ağ içinde sağlıklı. NATS durdurulunca proposal `/readyz` 503 döndü, `/healthz` 200 kaldı; NATS dönünce 2 saniyede 200'e döndü. `proposal_db`'de platform tabloları `proposal` şemasında. Boş `SERVICE` ile build açık hata mesajıyla duruyor. `coldbru-proposal:dev` 27,4 MB. `internal/platform/{app,natsconn}` testleri `make test`'te geçti.

## D-057: Geliştirmede API belgeleri: API_DOCS_ENABLED ile servis başına Scalar (/docs) ve /openapi.json; swag v2 go tool, spec commit edilir

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** Kullanıcı geliştirme ortamında Scalar API referans arayüzünün, ayarlanabilir bir ortam değişkeniyle açılmasını istedi (T-013'ten sonra). D-016 REST sözleşmesinin Go annotation'larından swag v2 ile OpenAPI 3 olarak üretilmesini seçmişti; çalıştırma yöntemi açıktı ve henüz spec yoktu. İş yeni görev T-042 olarak eklendi.
- **Karar:**
  - Scalar arayüzü `github.com/bdpiprava/scalar-go` (v0.13.0, MIT) ile üretilir.
  - `API_DOCS_ENABLED` kodda varsayılan `false`; compose geliştirme yığını `${API_DOCS_ENABLED:-true}` verir. Kapalıyken yollar hiç kaydedilmez.
  - Açıkken spec'i olan her HTTP servisi `GET /docs` (Scalar) ve `GET /openapi.json` (binary'ye gömülü spec) sunar. Spec'i olmayan servis bu yolları açmaz; customer GraphQL olduğu için kapsam dışıdır. nginx gelince (T-034) belgeler onun üzerinden de erişilir.
  - swag `v2.0.0-rc6` `go.mod`'da `tool` olarak sabitlenir; `make generate` servis başına `go tool swag init` çalıştırır; üretilen spec commit edilir ve `go:embed` ile binary'ye girer (D-051 deseni). İlk spec platform uç noktalarını (`/healthz`, `/readyz`) belgeler.
- **Alternatifler:**
  - Scalar'ı sabit sürümle CDN'den yükleyen kendi HTML sayfamız veya standalone bundle'ı gömmek — reddedildi: kullanıcı Go paketini seçti.
  - `APP_ENV=development` veya her yerde kapalı varsayılan — reddedildi.
  - nginx'te tek birleşik belge sayfası — reddedildi: servis başına belge.
  - Yalnız mekanizma veya T-025'e bırakmak — reddedildi: gerçek bir spec'le şimdi doğrulansın.
  - swag'ı Docker build'de çalıştırıp spec'i commit etmemek — reddedildi.
- **Sonuçlar / bedel:** scalar-go az bilinen bir bağımlılıktır ve Scalar'ın JS'ini tarayıcıda CDN'den yükler; internet yoksa arayüz açılmaz, `/openapi.json` yine çalışır. swag v2 hâlâ sürüm adayı (D-016). Annotation'lar değişince `make generate` çalıştırılmalı; unutulursa spec eski kalır.
- **Uygulama notu:** swag v2 yalnız OpenAPI 3.1 çıktısı verir (`--v3.1`) ve dosya adı `swagger.json`'dur; servis `/openapi.json` yolunda sunar. scalar-go yalnız mutlak spec adresi kabul ettiği için sayfa bir yer tutucu adresle üretilip göreli `openapi.json`'a çevrilir; yer tutucu bulunamazsa servis açılmaz. Scalar JS'i `scalargo.WithCDN` ile `@scalar/api-reference@1.74.0`'a sabitlendi.
- **Doğrulama:** 2026-10-27'de (T-042): `make generate` iki kez çalıştırıldığında spec'ler ve ağaç değişmedi; dört serviste `/docs` ve `/openapi.json` 200, spec `openapi: 3.1.0` ve yalnız `/healthz`, `/readyz` yollarını içeriyor; `API_DOCS_ENABLED=false` ile proposal'da ikisi 404, `/readyz` 200; yeniden açınca 200. `internal/platform/apidocs` ve spec testleri `make test`'te geçti.

## D-058: Customer GraphQL sözleşmesi: alan başına tipli filtre işleçleri, TimeValue @oneOf, tam işlem seti, Decimal string, null-güvenli eq/ne; gqlgen v0.17.95

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-007 Customer'ın gqlgen ile GraphQL sunmasını, D-044 şemanın tek kaynak ve camelCase olmasını seçti; filtre girdi tiplerinin kesin şekli, null kuralları, sayı biçimi ve `ago` birimleri açıktı (T-014).
- **Karar:**
  - Filtre: `CustomerFilter { and, or, not, <alan>: <Tip>Filter …, orders/cartItems/events: { some, none, every } }`. `StringFilter { eq, ne, in }`; sayı, Decimal, Date ve DateTime filtreleri `{ eq, ne, gt, gte, lt, lte, in }`; `BooleanFilter { eq, ne }`. Kategorik alanlar (`loyaltyTier`, `sex` …) String'dir, geçerli değerleri sunucu doğrular. İlişki filtreleri ilgili tablonun katalog alanlarını alır (D-042).
  - Zaman değeri: `input TimeValue @oneOf { ago: String, date: Date, at: DateTime }`; tarih ve zaman işleçleri bunu alır (`{ gte: { ago: "30d" } }`). `ago` birimi `d` veya `h`, pozitif tam sayı (`30d`, `12h`).
  - Skalerler: `Decimal` JSON'da string (`"1234.50"`), `Date` `YYYY-MM-DD`, `DateTime` RFC 3339 UTC.
  - Null: `eq`/`ne` null-güvenli (`IS NOT DISTINCT FROM` / `IS DISTINCT FROM`); sıralama işleçleri null'da eşleşmez; 0.1'de `isNull` yok.
  - İşlemler (sözleşme bir kerede): `audiencePreview(filter)` → sayım ve kanal kapsaması (dil kırılımlı); `audienceRecipients(filter, first, after)` → `customerId` üzerinde keyset sayfalama, `first` en fazla 1000, opak cursor; `consentStatus(customerIds, channel)` → `[{customerId, channel, optedIn, eligible}]`; mutation `updateConsent(input {customerId, channel, optedIn, source})` → `ConsentStatus`. Kanal `"email"`/`"sms"` String. `Recipient { customerId, firstName, language, emailEligible, smsEligible, email, phone }`; iletişim bilgisi yalnız ilgili kanal uygunsa döner. Resolver'lar T-015 ve T-017'ye kadar "not implemented" döner.
  - Araç: gqlgen v0.17.95 (2026-09-01) `go.mod`'da `tool`; şema `internal/customer/graph/schema.graphqls`; `make generate` gqlgen'i çalıştırır, üretilen kod commit edilir.
- **Alternatifler:**
  - Kategorik alanlar için GraphQL enum'ları veya genel koşul listesi — reddedildi.
  - Ayrı `…Ago` işleçleri veya düz string zaman — reddedildi.
  - Yalnız kitle işlemleri veya sayfalamasız alıcı listesi — reddedildi.
  - gqlgen v0.17.98 — reddedildi: 5 günlük.
  - Decimal'i JSON sayısı olarak taşımak — reddedildi: float yuvarlama.
  - Düz SQL null anlamı veya `isNull` işleci — reddedildi.
  - `ago`'da yalnız gün veya hafta birimi — reddedildi.
  - Kanal enum'u veya alıcıda iletişim bilgisi olmaması — reddedildi.
- **Sonuçlar / bedel:** Kategorik değer hataları GraphQL doğrulamasında değil sunucu kodunda yakalanır. `@oneOf` görece yeni bir GraphQL özelliğidir; istemci araçlarının desteği değişebilir. Alıcı listesi kişisel veri taşır; erişimi kapsam kontrolü korur (D-043). Kampanya kitlesi büyüdükçe alıcılar birden çok sayfada alınır.
- **Ek (kullanıcı onayı, 2026-10-27):** Introspection kapalıdır. Alıcı sayfası `RecipientConnection { nodes, pageInfo { hasNextPage, endCursor } }` biçimindedir. Bir ilişki filtresindeki birden çok niceleyici (`some`/`none`/`every`) VE ile birleşir. `audiencePreview.count` yalnız en az bir kanaldan ulaşılabilenleri sayar; `coverage.unreachable` filtreye uyup hiçbir kanaldan ulaşılamayanlardır ve `count`'a dahil değildir.
- **Uygulama notu:** gqlgen v0.17.95, `@oneOf`'u yalnız sorgu içine yazılmış değerlerde doğruluyor, değişkenle gelen değerlerde doğrulamıyor; Proposal filtreyi değişkenle gönderdiği için kural şemada açık bir `@oneOf` directive'iyle çalışma anında da zorlanır (`BAD_USER_INPUT`). Introspection eklenmedi (D-044 build-time katalog); yalnız `POST /graphql` sunulur. `ago` biçimi T-015'ten beri sunucuda doğrulanıyor (D-062).
- **Doğrulama:** 2026-10-27'de (T-014): `make generate` iki kez çalıştırıldığında değişiklik yok; handler testleri DSL belgesinin örnek filtresinin doğrulamadan geçtiğini, değişkenle gelen çift alanlı `TimeValue`'nun reddedildiğini, 13 kişisel/izin alanının şemada olmadığını, yer tutucu resolver'ların `NOT_IMPLEMENTED` döndüğünü ve introspection'ın kapalı olduğunu gösteriyor. Çalışan customer konteynerinde aynı davranış compose ağı içinden doğrulandı.

## D-059: Prompt'lar repoda dosya, go:embed ile gömülü; sürüm kimliği ad@semver + istek öneki sha256'sı, kilit dosyası testi

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** Kullanıcı prompt'ların bundan sonra nasıl sürümleneceğinin araştırılmasını istedi; bulgular `docs/research/05-prompt-versioning.md`'de. Kullanıcı `go:embed` ile dosya yaklaşımını ve araştırmanın sürüm kimliği önerisini kabul etti. Araştırmadaki diğer maddeler açık kaldı.
- **Karar:**
  - Prompt'ların kaynağı repodaki dosyalardır; Proposal binary'sine `go:embed` ile gömülür. Değişiklikler PR'da incelenir, git geçmişi sürüm geçmişidir. Prompt değişikliği yeniden derleme ve dağıtım demektir.
  - Her prompt paketinin sürüm kimliği iki parçalıdır: elle verilen `ad@semver` (ör. `segment@1.2.0`) ve otomatik `sha256`. Hash, gerçekten gönderilen önek üzerinden kanonik biçimde hesaplanır: `system` + `tools` + `output_config.format` şeması (anahtar sıralı JSON) + `model` + `effort`. Kullanıcı mesajı hash'e girmez.
  - Commit edilen `prompts.lock.json` her paket için `{name, version, sha256}` tutar; `make generate` yeniden üretir. Bir test hash'i yeniden hesaplar: kilitle uyuşmazsa `make generate` ister; hash değişip sürüm aynı kaldıysa sürüm artırılmasını ister.
  - Semver anlamı: MAJOR = çıktı sözleşmesi değişti (tool/JSON şeması, DSL sürümü); MINOR = davranış değişikliği (talimat, örnek); PATCH = davranışı etkilemeyen düzeltme. Hash her değişiklikte değiştiği için PATCH'te de sürüm artırımı zorunludur. Paketin gerektirdiği DSL sürümü (`requires_dsl`, D-045) ayrıştırıcının sürümüyle testte karşılaştırılır.
- **Alternatifler:**
  - `proposal_db` içinde prompt kayıt defteri — reddedildi: PR incelemesi ve DSL/kod ile birlikte sürümlenme kaybolur, yeni tablo gerekir.
  - Harici araçlar (Langfuse, PromptLayer, Braintrust, Humanloop, Helicone, Anthropic Console) — reddedildi: `docker compose up` ve anahtarsız test koşullarını bozar, Go desteği zayıf veya ücretli.
  - Yalnız içerik hash'i, yalnız elle semver veya tam sayı revizyon — reddedildi: hash insan tarafından okunmaz; elle sürüm unutulur ve kayıt yanlış olur.
- **Sonuçlar / bedel:** Prompt canlıda kod değiştirmeden düzenlenemez. Her sürüm artışı Claude prompt önbelleğini bir kez soğutur. Sistem metnine istemeden dinamik veri girmesi önbelleği ve hash'i bozar; dinamik veri kullanıcı mesajında kalmalıdır. Kilit dosyası ve test küçük bir bakım yüküdür. Biçim (düz metin veya şablon), `llm_call` sütunları, olay alanları, değerlendirme katmanları ve arayüz gösterimi açık konudur.
- **Doğrulama:** Doğrulanmadı — henüz kod yok (Proposal LLM görevlerinde, T-022).

## D-060: Prompt'lar düz metin; llm_call sürüm, önbellek ve sonuç sütunları alır; sürüm token olayına ve llm_usage'a taşınır; prompt_snapshot yok

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-059 prompt'ların `go:embed` ile dosya olarak tutulmasını ve sürüm kimliğini belirledi; biçim ve sürümün sistemde nasıl izleneceği açıktı (`docs/research/05-prompt-versioning.md` §1.1, §5).
- **Karar:**
  - Prompt dosyaları düz metindir; şablon motoru kullanılmaz. Katalog ve DSL dilbilgisi parçaları açılışta sabit sırayla birleştirilir. Brief, bağlam notları, dil gibi çağrıya özgü veri kullanıcı mesajında JSON blok olarak gider; sistem metni bayt bayt sabittir.
  - `llm_call` (proposal_db, D-022) şu sütunları alır: `prompt_name`, `prompt_version`, `prompt_sha256` (üçü NOT NULL), `cache_creation_tokens` ve `outcome` (`accepted` | `validation_failed` | `refusal`). Bir varyantı hangi denemenin ürettiği `outcome`'dan okunur; `proposal_variant`'a sütun eklenmez. Kullanıcı bu şema değişikliğini onayladı (kodlama kuralı 7); kesin tipler ve kısıtlar migration yazılırken.
  - `evt.proposal.token_completed.v1`'in `data`'sına `prompt_name` ve `prompt_version` eklenir (sürüm içinde alan ekleme, D-035); Analytics `llm_usage`'a `prompt_version` sütunu eklenir. Loglarda `prompt_version` alanı, Prometheus etiketlerinde yalnız `prompt_name` (kardinalite).
  - `prompt_snapshot` tablosu şimdilik yoktur; eski bir çağrının tam metni git commit'i ve hash ile bulunur.
- **Alternatifler:**
  - `text/template` — reddedildi: modele öğretilen `{{first_name}}` gibi yer tutucular her yerde kaçırılmalı, dinamik veri önbelleği bozabilir.
  - `outcome` olmadan veya yalnız `prompt_version` — reddedildi: kabul edilen deneme ve önbellek maliyeti görünmez olur.
  - Sürümü yalnız `llm_call`'da tutmak — reddedildi: sürüm başına maliyet ve onarım oranı Analytics'te görülsün.
  - `prompt_snapshot`'ı baştan eklemek — reddedildi: tablo ve migration gerekir, git yeterli.
- **Sonuçlar / bedel:** `llm_call` satırları biraz büyür. Analytics bilinmeyen alanları yok saymalıdır. Prompt metni git dışında saklanmadığı için eski bir sürümü görmek repo erişimi ister.
- **Doğrulama:** Doğrulanmadı — henüz kod yok (T-021, T-022, T-032).

## D-061: Prompt değerlendirmesi 3 katman (make test, make eval, make eval-live) + promptfoo; LLM hakem yok; arayüzde sürüm yok; live testte önbellek tanılama

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Test ve kapsam
- **Bağlam:** Prompt sürümlemesi ancak değişikliğin etkisi ölçülebilirse anlamlıdır (`docs/research/05-prompt-versioning.md` §4). Brief testlerin gerçek API anahtarı olmadan çalışmasını istiyor; D-025 `live` build tag'ini ayırdı.
- **Karar:**
  - `make test` (anahtarsız): kilit dosyası ve `requires_dsl` uyumu (D-059), araç/çıktı şemalarının geçerliliği, mock sunucuya giden isteklerde `system`/`tools` baytlarının gömülü paketle aynı olması, altın dosyaların kaydedildiği `prompt_sha256`'nın güncel olması.
  - `make eval` (anahtarsız): `eval/cases/` altındaki vakalar kayıtlı yanıtlarla Go içindeki tüm doğrulama hattından geçer.
  - `make eval-live` (`live` tag, `ANTHROPIC_API_KEY`, elle): aynı vakalar gerçek modele birden çok kez gider; kontroller kodla yapılır (DSL ayrıştırma, katalog, SQL sayımı, SMS ≤ 160, konu ≤ 80, yer tutucular, enjeksiyon reddi). Sonuç prompt sürümü artırılan PR'a eklenir.
  - promptfoo ek olarak ton ve çok-model karşılaştırması için kullanılır; Go doğrulayıcılarının yerine geçmez.
  - LLM hakem ilk sürümde yoktur; öznel ton insan onayında kalır.
  - Arayüzde prompt sürümü gösterilmez; sürüm yalnız DB, olaylar ve loglardadır.
  - Önbellek tanılama (cache diagnostics) `live` etiketli bir testte, aynı sürümün iki ardışık çağrısında önek ayrışması olmadığını doğrulamak için kullanılır; Go SDK v2.3.0'da düz `Messages` üzerinde desteklenmiyorsa kontrol `usage` token'larıyla yapılır.
- **Alternatifler:**
  - Yalnız Go içi 3 katman (promptfoo'suz) — reddedildi: kullanıcı ton ve model karşılaştırması için promptfoo'yu da istedi.
  - Yalnız `make test` — reddedildi: model davranışı ölçülmez.
  - Haiku 5.5 hakem — reddedildi: ilk sürümde gerek yok.
  - Salt okunur "AI notları" rozeti veya yönetici sayfası — reddedildi: arayüz sade kalsın.
  - Önbellek tanılamayı hiç kullanmamak — reddedildi.
- **Sonuçlar / bedel:** promptfoo Node bağımlılığı ekler ve kurallar kısmen YAML'da ikinci kez yazılır; model değerlendirmeli kontrolleri sağlayıcı (API anahtarı) ister. `make eval-live` her çalıştırmada küçük bir API maliyeti doğurur (araştırmadaki tahmin tur başına yaklaşık 0,3 $, ölçülmedi). promptfoo sürümü ve vaka biçimi açıktır. İnceleme görüşmesinde sürüm izi arayüzden değil DB ve git'ten anlatılır.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-062: Filtre derleyicisi: ulaşılabilirlik koşulu, bellekte şehir sözlüğü, CHECK'li kategoriler doğrulanır, elle SQL, sınırlar 8/100/100/5 sn

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Kitle ve izinler
- **Bağlam:** D-007 GraphQL filtresinin beyaz listeli derleyiciyle parametreli SQL'e çevrilmesini ve her kitle sorgusuna ulaşılabilirlik koşulu eklenmesini, D-040 kapalı şehir listesini, D-058 filtre sözleşmesini belirledi. Koşulun tam tanımı, şehir anahtarının nerede tutulacağı, hangi değerlerin doğrulanacağı, SQL kurma yöntemi ve sınırlar açıktı (T-015).
- **Karar:**
  - E-posta uygun: `email` dolu, `email_opt_in` ve `email_bounced` değil. SMS uygun: `phone_e164` dolu ve `sms_opt_in`. Kitlenin tabanı filtre + `account_status = 'active'`; ulaşılabilir = tabandakiler içinde en az bir kanala uygun olanlar. `audiencePreview.count` ulaşılabilirleri sayar; `coverage.unreachable` tabanda olup hiçbir kanala uygun olmayanlardır (D-058); filtreye uyan inactive ve deleted hesaplar hiçbir sayıma girmez (kullanıcı ayrıca onayladı).
  - Şehir: Customer açılışta `SELECT DISTINCT city`'yi yükler ve `key(city) → kanonik ad` sözlüğü kurar (`ı`, `İ`, `I` → `i`, sonra NFD, birleşen işaretler atılır, küçültülür; `golang.org/x/text`). Filtredeki değer kanonik ada çevrilir, SQL `city = $n` olarak kalır. customer_db şeması değişmez.
  - Kapalı listeyle doğrulanan alanlar: veritabanında CHECK kısıtı olanlar (`sex`, `language`, `countryCode`, `preferredPlatform`, `loyaltyTier`, `orderStatus`, `cartStatus`, `eventType`; değerler kodda sabit) ve `city` (sözlükten). Bilinmeyen değer `UNKNOWN_VALUE` döner; `extensions.suggestions` kanonik anahtarlar üzerinde Levenshtein uzaklığı ≤ 2 veya önek eşleşmesiyle en yakın en fazla 3 değeri taşır (`github.com/agnivade/levenshtein`, gqlgen ile zaten gelen). Diğer String alanlar tam eşleşir, doğrulanmaz.
  - SQL elle, küçük bir derleyiciyle kurulur: kolon adları yalnız katalog beyaz listesinden, değerler yalnız `$n` argümanı; ilişkiler `EXISTS` / `NOT EXISTS` alt sorgularıdır.
  - Sınırlar: `and`/`or`/`not` ve ilişki iç içeliği en fazla 8 seviye, en fazla 100 koşul düğümü, bir `in` listesi en fazla 100 değer; aşılırsa `COMPLEXITY_LIMIT`. Kitle sorguları `statement_timeout = 5s` olan salt okunur bir transaction'da çalışır.
  - Kapsam: derleyiciyle birlikte `audiencePreview` ve `audienceRecipients` resolver'ları gerçek veriyle çalışır; `consentStatus` ve `updateConsent` T-017'de.
- **Alternatifler:**
  - Bounce'u saymamak veya inactive hesapları dahil etmek — reddedildi.
  - Üretilmiş `city_key` kolonu veya sorguda SQL fonksiyonu — reddedildi: şema değişikliği ya da indekssiz sorgu, öneri üretilemez.
  - Tüm kategorikleri DB'den doğrulamak veya yalnız şehri — reddedildi.
  - squirrel — reddedildi: bir bağımlılık, EXISTS ve null-güvenli işleçler için yine özel kod.
  - Daha sıkı (5/50/50/2 sn) veya daha gevşek (12/300/1000/15 sn) sınırlar — reddedildi.
  - Tüm geçerli değerleri listelemek veya öneri vermemek — reddedildi.
  - Yalnız derleyici — reddedildi: gerçek veriyle doğrulama bu görevde yapılsın.
- **Sonuçlar / bedel:** Yeni bir şehir eklenirse sözlük Customer yeniden başlatılınca güncellenir. Kodda sabit kategori listeleri CHECK kısıtlarıyla elle senkron tutulur. Örnek brief'te `count` 46 olur (20 iki kanal + 13 yalnız e-posta + 13 yalnız SMS), `unreachable` 3; SMS'e 33, e-postaya 33 kişi uygundur (D-010).
- **Ek (kullanıcı onayı, 2026-10-27):** Boş `and`/`or` listesi, işleçsiz alan (`city: {}`) ve niceleyicisiz ilişki (`orders: {}`) `BAD_USER_INPUT` döner; tamamen boş kök filtre `{}` tüm müşteriler demektir. `ago` en fazla 36500 gün / 876000 saattir. `byLanguage` kitle tabanındaki her dili listeler, yalnız ulaşılamayanları olan diller dahil (`count` 0).
- **Uygulama notu:** Sınır sayımı: kök filtre derinlik 1'dir; `and`/`or` elemanı, `not` işleneni veya `some`/`none`/`every` nesnesi bir seviye derindir. Her karşılaştırma işleci (`in` listesi uzunluğundan bağımsız) ve her `and`/`or`/`not` ile niceleyici bir düğümdür. Hatalar `extensions.filterPath` ile filtredeki yeri gösterir; zaman aşımı `QUERY_TIMEOUT`, iç hata `INTERNAL` döner (iç ayrıntı yalnız loglanır); bu iki kod ve `filterPath` kullanıcı tarafından ayrıca onaylandı. CHECK'i olmayan `orders.countryCode`, `shippingCity` ve `platform` alanları serbest metindir.
- **Doğrulama:** 2026-10-27'de (T-015), testcontainers ile seed'lenen taze bir customer_db'de örnek brief: `count` 46, kapsama 20 / 13 / 13 / 3, yani 33 SMS ve 33 e-posta uygun; alıcılar 10'luk sayfalarla 46 kişiyi tekrarsız ve sıralı döndürdü; İstanbul'un beş yazımı aynı sayımı verdi; `Istnbul` → `İstanbul`, `gold` → `Gold` önerisi; `ne` null'ları kapsıyor; sınır ve biçim hataları beklenen kodları döndürdü; 27 altın derleme vakası Postgres'te çalıştı ve 14 filtre elle yazılmış SQL ile aynı sonucu verdi. Compose'daki 7 saatlik veritabanı aynı brief için 47 döndürdü: bir müşterinin son siparişi 30 günlük pencereden çıktı (D-017'deki bilinen kayma; `down -v` ile tazelenince 46).

## D-063: Customer kimliği: çağıran başına env token, kapsamlar kodda ve şemada @hasScope; 401 ve alan başına FORBIDDEN; consent:write yalnız admin token

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Kitle ve izinler
- **Bağlam:** D-043 Customer GraphQL'e her servisin kendi token'ı ve kapsamıyla erişmesini seçti; token'ların nasıl tanımlanacağı, `updateConsent` kapsamını kimin taşıyacağı, hata davranışı ve kapsamların nerede belirtileceği açıktı (T-016). D-011 izin geri çekmenin demoda arayüzden gösterilebileceğini öngörüyordu.
- **Karar:**
  - Token'lar çağıran başına ortam değişkenidir: `CUSTOMER_TOKEN_PROPOSAL`, `CUSTOMER_TOKEN_NOTIFICATION`, `CUSTOMER_TOKEN_ADMIN`. Hangi çağıranın hangi kapsamı aldığı kodda sabittir: Proposal `audience:read` (`audiencePreview`, `audienceRecipients`), Notification `consent:read` (`consentStatus`), admin `consent:write` (`updateConsent`) ve `consent:read` (sonucu doğrulamak için; kullanıcı ayrıca onayladı). İstek `Authorization: Bearer <token>` taşır; karşılaştırma sabit zamanlıdır. Token'lardan biri tanımlı değilse Customer açılmaz. Dev değerleri compose ve `.env.example`'dadır.
  - `consent:write`'ı hiçbir servis taşımaz; `updateConsent` yalnız admin token'ıyla, geliştirici komutu veya compose ağı içinden çağrılır.
  - Token yok veya geçersizse `/graphql` HTTP 401 döner, GraphQL'e ulaşılmaz. Token geçerli ama kapsam yoksa yalnız o alan GraphQL hatası `FORBIDDEN` ile döner. `/healthz`, `/readyz`, `/metrics` token istemez (yalnız iç ağ, D-056).
  - Kapsamlar şemada `@hasScope(scope: "...")` directive'iyle belirtilir ve gqlgen tarafından çalışma anında zorlanır; sözleşmede görünür (D-044).
- **Alternatifler:**
  - Tek ortam değişkeninde JSON token → kapsam eşlemesi — reddedildi: yanlış yapılandırma kapsamı sessizce genişletebilir.
  - HMAC imzalı token — reddedildi: D-043'te reddedilen JWT ağırlığına yaklaşır.
  - `consent:write`'ı Notification'a veya Proposal'a vermek — reddedildi: kullanıcı izin değişikliğini servislerden ayırmayı seçti.
  - Her durumda GraphQL hatası (HTTP 200) — reddedildi: HTTP seviyesinde izlenebilirlik.
  - Kapsam kontrolünü yalnız resolver içinde yapmak — reddedildi: kapsamlar sözleşmede görünmez, unutulan kontrol risktir.
- **Sonuçlar / bedel:** D-011'deki "izin geri çekme arayüzden canlı gösterilebilir" fikri arayüz üzerinden olmaz; demoda izin bir geliştirici komutuyla geri çekilir ve etkisi gönderim panosunda görülür. Statik token'lar süresizdir (D-043). Yeni bir çağıran eklemek kod değişikliği ister.
- **Ek (kullanıcı onayı, 2026-10-27):** Kök alanlar non-null olduğu için (D-058) kapsamı olmayan bir alan, yanıtta tek bir `FORBIDDEN` hatası ve `data: null` doğurur; kısmi veri dönmez. Şema değişmez; her çağıran tek işlem istediğinden bu yeterlidir.
- **Uygulama notu:** Customer token'ların yalnız SHA-256 özetini tutar ve gelen token'ı her kayıtla sabit zamanlı karşılaştırır. Ara katmanı olmadan çağrılan handler korunan her alanı reddeder. Token'lar loglanmaz.
- **Doğrulama:** 2026-10-27'de (T-016): ara katman ve token seti birim testleri, gerçek GraphQL handler'ı üzerinden 9 kapsam vakası, yasak alanın resolver'ı çalıştırmadığı ve token'sız isteğin GraphQL'e ulaşmadığı testleri geçti. Çalışan customer'da compose ağı içinden: token yok ve geçersiz token 401; Proposal token'ı `audiencePreview` sayımını aldı; Notification token'ı aynı sorguda `FORBIDDEN`; admin token'ı `consentStatus` ve `updateConsent`'e ulaştı (henüz `NOT_IMPLEMENTED`, T-017); `/healthz` token'sız 200; loglarda token yok.

## D-064: İzin işlemleri: id başına consentStatus satırı, kilit + max+1 kimlik, yalnız değişiklikte kayıt, kapalı source listesi, make consent

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Kitle ve izinler
- **Bağlam:** D-011 Notification'ın gönderimden önce Customer'a toplu izin sorgusu atmasını ve `updateConsent` ile izin değişikliğini; D-063 `updateConsent`'in yalnız admin token'ıyla, bir geliştirici komutuyla çağrılmasını belirledi. Yanıt biçimi, kimlik üretimi, değişiklik kuralları ve komut açıktı (T-017). Seed'deki kimlikler `CNS-000001` … `CNS-002219`.
- **Karar:**
  - `consentStatus(customerIds, channel)`: en fazla 1000 id, fazlası `BAD_USER_INPUT`. İstenen her id için bir satır: `optedIn` `dim_customer`'daki kanal bayrağı, `eligible` D-062'deki kanal koşulu (aktif hesap + izin + iletişim, e-postada bounce değil). Bilinmeyen id `optedIn: false`, `eligible: false` döner; Notification onu atlar (fail-closed).
  - Yeni `fact_consent` kimliği: transaction içinde `pg_advisory_xact_lock` alınır, en büyük `CNS-` numarası +1 kullanılır (`CNS-002220` …). customer_db şeması değişmez.
  - `updateConsent`: müşteri satırı `FOR UPDATE` ile kilitlenir. Durum zaten istenen gibiyse satır yazılmaz, mevcut durum döner. Aksi hâlde aynı transaction'da yeni `fact_consent` satırı yazılır ve `dim_customer`'daki bayrak ile `consent_updated_at` güncellenir (seed'deki tutarlılık kuralı korunur, D-005). Bilinmeyen müşteri `NOT_FOUND`, silinmiş hesap `BAD_USER_INPUT`. `source` kapalı listedir: `preference_centre`, `support_request`, `unsubscribe_link`, `admin_tool`.
  - Geliştirici komutu: `make consent CUSTOMER=<id> CHANNEL=<email|sms> OPTED_IN=<true|false>`; compose ağında admin token'ıyla `updateConsent`'i çağırır ve yeni durumu basar.
- **Alternatifler:**
  - Bilinmeyen id'de tüm sorguyu reddetmek veya bilinmeyenleri yanıttan çıkarmak — reddedildi.
  - Sequence veya `CNS-<uuidv7>` — reddedildi: şema değişikliği ya da karışık kimlik biçimi.
  - Her çağrıda satır yazmak veya serbest metin `source` — reddedildi.
  - Ayrı Go CLI veya yalnız README'de curl — reddedildi.
- **Sonuçlar / bedel:** Eşzamanlı izin güncellemeleri advisory lock'ta sıraya girer; yalnız admin aracı yazdığı için sorun değildir. `signup_form` ve `account_deletion` yalnız seed'den gelir, bu araçla yazılamaz. Yeni bir kaynak değeri kod değişikliği ister.
- **Doğrulama:** 2026-10-27'de (T-017), seed'li customer_db üzerinde entegrasyon testleri: bilinmeyen id `false/false`, istek sırası korunuyor; 1000 müşteride `consentStatus` uygunluğu `audienceRecipients` ile birebir (e-posta 703, SMS 586); geri çekme `CNS-002220`, geri açma `CNS-002221` yazıyor, bayrak ile son kayıt uyumlu, kitle kapsaması değişiyor ve geri dönüyor; aynı istek tekrarlanınca satır yazılmıyor; 8 paralel güncelleme ardışık ve tekil kimlik alıyor; hata kodları beklendiği gibi. Compose'da `make consent` ile `CUS-000180` için SMS izni geri çekildi (aynı istek tekrarlanınca yeni satır yok) ve geri açıldı; kötü niyetli müşteri değeri `make` aşamasında reddedildi.

## D-065: DSL ayrıştırıcısı: dar sözcük kuralları, ilk hata + satır/sütun, istemci sınırları, kanonik yazıcı, kesin hata kodları, nitelikli lambda alanları

- **Durum:** Kabul edildi · 2026-10-28
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-038 kendi Pratt ayrıştırıcımızı, D-045 ve D-046 0.1 kapsamını belirledi. Sayı ve metin sözdizimi, hata raporlama, istemci tarafı sınırlar, kanonik yazıcı, kesin hata kodu listesi, lambda kuralları ve ters yönlü karşılaştırmalar açıktı (T-018 analizi).
- **Karar:**
  - Sözcük kuralları: sayı `-?[0-9]+(\.[0-9]+)?` (üs, baştaki `+`, alt çizgi yok); AST'de metin olarak saklanır, float'a çevrilmez. Metin `'` veya `"` ile; kaçışlar yalnız `\\`, `\"`, `\'`; metin içinde satır sonu yok; UTF-8 serbest. Boşluk ve satır sonu serbest; yorum yok.
  - Hata raporlama: ilk sözdizimi hatasında durulur. Hata kod, mesaj, 1 tabanlı satır ve rune sütunu, başlangıç-bitiş ve mesajda `near "…"` parçası taşır (D-027 onarımı için).
  - İstemci sınırları: Customer'ınkilerle aynı (derinlik 8, 100 düğüm, `in` ≤ 100, D-062) ve kaynak en fazla 4096 bayt; aşılırsa `COMPLEXITY_LIMIT`. Customer yine son sözü söyler.
  - Kanonik yazıcı: `Print(ast)` tek biçimli DSL üretir (çift tırnak, tek boşluk, gereksiz parantez yok). Fuzz testi `Parse(Print(Parse(x))) == Parse(x)` ve panik olmamasını doğrular. Normalize DSL teklif kaydında saklanabilir (D-022).
  - Hata kodları kesindir (`docs/coldbru-gqdsl.md` §12.1): ayrıştırıcı `SYNTAX_ERROR`, `UNSUPPORTED_OPERATOR` (`~=`, `==`, `!=`, alan-alan), `UNSUPPORTED_EXPRESSION` (`ago`/`date`/`some`/`none`/`every` dışı çağrı, atama vb.), `INVALID_RELATION_SCOPE`, `COMPLEXITY_LIMIT` üretir; anlamsal geçiş (T-019) `UNKNOWN_FIELD`, `UNKNOWN_RELATION`, `UNKNOWN_VALUE`, `TYPE_MISMATCH` üretir. Yeni kod karar ister.
  - Lambda: parametre tek bir tanımlayıcıdır, ayrılmış kelime olamaz, iç içe lambdalarda tekrar kullanılamaz. Gövdede her alan bu değişkenle nitelenir (`o.totalGbp`); gövdede yalın kök alan `INVALID_RELATION_SCOPE` döner.
  - Solda değer olan karşılaştırma (`1000 < totalSpendGbp`) `SYNTAX_ERROR` döner, mesaj alanın sola yazılmasını söyler.
- **Alternatifler:**
  - JS'e yakın sözcük kuralları (üs, JSON kaçışları, `//` yorumları) veya dar kurallar + yorum — reddedildi: dil yüzeyi büyür.
  - Birden çok hata toplamak veya yalnız bayt ofseti — reddedildi.
  - Yalnız uzunluk sınırı veya hiç sınır — reddedildi: LLM aşırı kuralı ağ çağrısından önce düzeltsin.
  - Yazıcısız, yalnız panik fuzz — reddedildi.
  - İki kaba hata kodu — reddedildi: onarım ve testler hata türünü ayırt etsin.
  - Lambda gövdesinde yalın adın ilişkiye ait sayılması — reddedildi: aynı adlı müşteri ve sipariş alanlarında belirsizlik.
  - Ters karşılaştırmayı çevirip kabul etmek — reddedildi: tek biçim.
- **Sonuçlar / bedel:** LLM'in JS alışkanlıkları (yorum, `1e3`, ters karşılaştırma) onarım turu harcatabilir; sistem prompt'u bunları örneklemeli. Sınırlar iki yerde (Proposal ve Customer) tutulur ve senkron kalmalıdır.
- **Uygulama notu:** `!` karşılaştırmadan sıkı bağlar (§3.9); `!a === 1` `SYNTAX_ERROR` döner ve `!(a === 1)` yazılmasını söyler. Aynı işleç zincirleri parantez içinden de düzleştirilir (`(a && b) && c` tek `And`), böylece yazıcının parantez atması round-trip'i bozmaz. `null`/`undefined`/`NaN` `SYNTAX_ERROR` ('null değeri yok') döner. Derinlik ve düğüm sınırları ayrıştırma bittikten sonra, `in` sınırı ayrıştırırken denetlenir. Kod sabitleri Go adlarıyla (`CodeSyntaxError` …), değerleri §12.1'deki kodlardır. `!` bağlanması ve bu yorumlar kullanıcı tarafından ayrıca onaylandı.
- **Doğrulama:** 2026-10-28'de (T-018): `internal/proposal/dsl` birim testleri `-race` ile geçti, kapsama %98,1; her hata kodu kesin satır:sütun ile, çok satırlı kaynak ve Türkçe metinle sınandı; §8.2 ve §13 örnekleri ayrıştırılıp round-trip yapıyor; sınır testleri 4096/4097 bayt, derinlik 8/9, 100/101 düğüm, `in` 100/101. `FuzzParse` 60 saniyede 3.066.670 çalıştırma, sonra 20 saniyede 1.311.644 çalıştırma yaptı; çökme yok.

---

## Açık konular

Seçenekler, kaynaklar ve öneriler için bkz. [docs/research/](docs/research/README.md) (2026-10-27). Oradaki öneriler karar değildir.

- Retry ayrıntıları: gönderimde (`SEND`) hangi hatalar yeniden denenir, geri çekilme ve deneme sayısı, `sending` lease süresi; olay tüketicisi için D-055 (D-023, D-024)
- Sipariş kalemi ve ürün alanlarının ilişki filtrelerine girip girmeyeceği (D-042)
- `proposal_variant.content` alanının adım türüne göre JSON şeması (D-034)
- Circuit breaker eşikleri ve kütüphanesi, izin kontrolü batch boyutu (D-011)
- Olay listesi ve `data` alanlarının kesinleşmesi (taslak: `docs/domain-model.md`) (D-035)
- Üç onarım denemesi başarısız olunca ve ön eleme brief'i işaretleyince kullanıcıya ne gösterilir (D-026, D-027)
- Gönderim anında bir alıcının SMS metni 160 karakteri aşarsa ne yapılır (D-028)
- HTML email biçimi (araştırma önerisi yuvalı şablon)
- Bütçenin anlamı: kampanya harcama tavanı ve/veya ajan token tavanı
- promptfoo sürümü, `eval/cases/` vaka biçimi ve `make eval-record` akışı (D-061)
- Mock provider hata/gecikme/429 oranları
- SSE olay tiplerinin sözleşmesi ve belgelenmesi (OpenAPI dışında) (D-013, D-016)
- Kimlik doğrulama (arayüz ve SSE için) (D-013)
- Seed: 1000 kullanıcının "birkaç bin"e nasıl çıkarılacağı; kampanya stub'larının ne zaman ve hangi DB'ye yükleneceği
