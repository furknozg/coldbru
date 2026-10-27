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
- **Doğrulama:** Doğrulanmadı — henüz kod yok. Hedef test: örnek brief için DSL → GraphQL → SQL zinciri 33 kişi döndürmeli.

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
- **Doğrulama:** Doğrulanmadı — henüz kod yok. Hedef testler: zamanlanmış kampanyada onaydan sonra izin geri çekilen kişi `skipped_no_consent` olur; Customer kapalıyken hiçbir mesaj gönderilmez.

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
- **Doğrulama:** Doğrulanmadı — henüz uygulanmadı. Hedef: taze init'ten sonra örnek brief sorgusu `now()` ile `49 → 33 SMS / 33 email` vermeli.

## D-018: Dört servis ve mock provider, her biri ayrı konteyner

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** THOUGHTS.md diyagramı Customer, Proposal, Notification ve Analytics servislerini gösteriyor. Araştırma Analytics'i ilk kesilecekler arasına koymuştu (docs/research/README.md).
- **Karar:** Customer, Proposal, Notification ve Analytics ayrı Go süreçleri ve ayrı compose servisleridir. Mock SMS/e-posta provider da ayrı bir konteynerdir. Analytics `notification.completed` ve `token.completed` olaylarını NATS'ten tüketir ve kendi veritabanına yazar.
- **Alternatifler:**
  - Üç servis + mock, Analytics kesilir — reddedildi: diyagrama tam uyum istendi.
  - Tek binary, alt komutlar — reddedildi: mikroservis gösterimi zayıflar.
- **Sonuçlar / bedel:** Bir haftalık yapımda daha fazla iş. Analytics olay sözleşmelerine bağımlıdır ve eventual-consistent'tır. Compose'ta beş uygulama konteyneri, Postgres, NATS ve nginx çalışır. Dashboard'un canlı gönderim durumunu hangi servisten okuyacağı açık.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-019: proposal_db, notification_db, analytics_db; rol ve boş DB initdb'de açılır

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-003 servis başına veritabanı ve sahip rol getirdi. Tasarım belgesi geçici olarak `campaign_db` / `delivery_db` adlarını kullanıyordu; diyagram Proposal / Notification / Analytic DB diyor.
- **Karar:** `proposal_db`/`proposal_svc`, `notification_db`/`notification_svc` ve `analytics_db`/`analytics_svc`. Analytics D-018 ile eklendiği için aynı adlandırma desenini alır. Rol ve boş veritabanı initdb'de `01_customer_db.sql` ile aynı desenle açılır: sahip rol, `REVOKE ALL … FROM PUBLIC`, `CONNECT` ve `TEMPORARY`. Parolalar compose ortam değişkenlerinden gelir, `.env.example`'da yalnız dev varsayılanları bulunur. Tablolar her servisin kendi migration'larıyla açılır (D-020). `customer_db`'nin şeması ve seed'i initdb'de kalır (D-005).
- **Alternatifler:**
  - Tasarım belgesindeki `campaign_db` / `delivery_db` — reddedildi: diyagramla uyum.
  - Servisin açılışta süper kullanıcıyla kendi rolünü ve DB'sini açması — reddedildi: servislere süper kullanıcı parolası verilirdi.
- **Sonuçlar / bedel:** Yeni bir DB eklemek initdb değişikliği ve `down -v` ister, çünkü init yalnız boş volume'da çalışır. İki şema mekanizması yan yana durur: `customer_db` initdb'de, diğerleri migration'da.
- **Doğrulama:** Doğrulanmadı — henüz uygulanmadı.

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
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

## D-021: Tek Go modülü; cmd/ altında servis başına binary, internal/ altında kod

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** D-018 beş binary getiriyor. Kökte `go.mod` var (`github.com/furknozg/coldbru`, D-002).
- **Karar:** Kök modül korunur. `cmd/{customer,proposal,notification,analytics,mockprovider}/main.go`. Servis kodu `internal/<servis>/` altında; servisin migration'ları ve `sqlc.yaml`'ı o klasörde. Ortak altyapı `internal/platform/` altında (db, nats, outbox, http, config). Tek çok aşamalı Dockerfile, `SERVICE` build arg'ıyla hangi binary'nin derleneceğini seçer.
- **Alternatifler:**
  - `go.work` ile servis başına modül — reddedildi: go.mod ve sürüm bakımı artar, kökteki go.mod taşınır.
  - Tek modül, `services/<servis>/` klasörleri — reddedildi: sınır klasörde görünür ama kazanç yok.
- **Sonuçlar / bedel:** Go'nun `internal` kuralı aynı modül içinde servisler arası import'u engellemez; sınırı kod incelemesi korur. Tüm servisler aynı bağımlılık sürümlerini paylaşır.
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

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
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

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
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

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
- **Doğrulama:** Doğrulanmadı — henüz kod yok.

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

---

## Açık konular

Seçenekler, kaynaklar ve öneriler için bkz. [docs/research/](docs/research/README.md) (2026-10-27). Oradaki öneriler karar değildir.

- Retry ayrıntıları: hangi hatalar yeniden denenir, geri çekilme tabanı ve jitter, `sending` lease süresi (D-023, D-024)
- Customer GraphQL filtre girdi tiplerinin kesin şekli: `and`/`or`/`not`, `some`/`none`/`every`, `ago` ve `in` karşılıkları (D-039, D-044, D-045)
- DSL'de null kuralları, sayı biçimi ve hata kodlarının kesin listesi (D-038, D-045)
- `updateConsent` kapsamını hangi servisin taşıyacağı ve arayüzden nasıl çağrılacağı (D-011, D-043)
- Sipariş kalemi ve ürün alanlarının ilişki filtrelerine girip girmeyeceği (D-042)
- `proposal_variant.content` alanının adım türüne göre JSON şeması (D-034)
- Circuit breaker eşikleri ve kütüphanesi, izin kontrolü batch boyutu (D-011)
- Olay listesi ve `data` alanlarının kesinleşmesi (taslak: `docs/domain-model.md`) (D-035)
- Üç onarım denemesi başarısız olunca ve ön eleme brief'i işaretleyince kullanıcıya ne gösterilir (D-026, D-027)
- Gönderim anında bir alıcının SMS metni 160 karakteri aşarsa ne yapılır (D-028)
- HTML email biçimi (araştırma önerisi yuvalı şablon)
- Bütçenin anlamı: kampanya harcama tavanı ve/veya ajan token tavanı
- Mock provider hata/gecikme/429 oranları
- SSE olay tiplerinin sözleşmesi ve belgelenmesi (OpenAPI dışında) (D-013, D-016)
- Kimlik doğrulama (arayüz ve SSE için) (D-013)
- Metrik biçimi ve health check içeriği; Viper ile yapılandırma dosyası kullanılıp kullanılmayacağı (D-031)
- Seed: 1000 kullanıcının "birkaç bin"e nasıl çıkarılacağı; kampanya stub'larının ne zaman ve hangi DB'ye yükleneceği
