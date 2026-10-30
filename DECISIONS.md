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
| D-009 | İçerik ve zamanlama alıcının diline göre; tüm diller onaylanmadan teklif onaylanmaz | LLM entegrasyonu | Kabul edildi; dil başına zamanlama kısmı yerini aldı → D-076 |
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
| D-034 | Tüm adımlar tek varyant tablosunda; adım türüne göre CHECK | LLM entegrasyonu | Kabul edildi; zamanlama yuvası yerini aldı → D-076 |
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
| D-060 | Prompt'lar düz metin; llm_call sürüm, önbellek ve sonuç sütunları alır; sürüm token olayına ve llm_usage'a taşınır; prompt_snapshot yok | LLM entegrasyonu | Kabul edildi; outcome listesi yerini aldı → D-068 |
| D-061 | Prompt değerlendirmesi 3 katman (make test, make eval, make eval-live) + promptfoo; LLM hakem yok; arayüzde sürüm yok; live testte önbellek tanılama | Test ve kapsam | Kabul edildi |
| D-062 | Filtre derleyicisi: ulaşılabilirlik koşulu, bellekte şehir sözlüğü, CHECK'li kategoriler doğrulanır, elle SQL, sınırlar 8/100/100/5 sn | Kitle ve izinler | Kabul edildi |
| D-063 | Customer kimliği: çağıran başına env token, kapsamlar kodda ve şemada @hasScope; 401 ve alan başına FORBIDDEN; consent:write yalnız admin token | Kitle ve izinler | Kabul edildi |
| D-064 | İzin işlemleri: id başına consentStatus satırı, kilit + max+1 kimlik, yalnız değişiklikte kayıt, kapalı source listesi, make consent | Kitle ve izinler | Kabul edildi |
| D-065 | DSL ayrıştırıcısı: dar sözcük kuralları, ilk hata + satır/sütun, istemci sınırları, kanonik yazıcı, kesin hata kodları, nitelikli lambda alanları | LLM entegrasyonu | Kabul edildi |
| D-066 | Proposal kataloğu: make generate ile şemadan + metadata'dan üretilir, commit edilir, tazelik testi; sabit değer listeleri Proposal'da, şehir Customer'da; açıklamalar İngilizce | LLM entegrasyonu | Kabul edildi |
| D-067 | DSL → GraphQL derleyicisi AST'ye bire bir; elle yazılmış Customer istemcisi; 10 sn, 3 deneme; filterPath DSL konumuna çevrilir | LLM entegrasyonu | Kabul edildi |
| D-068 | proposal_db şeması: taslak + denetim kolonları, kısmi unique ve FK indeksleri, CHECK'ler, ekleme-yalnız trigger; sqlc ve açılışta migration | LLM entegrasyonu | Kabul edildi; llm_call.purpose listesi yerini aldı → D-070; canlı varyant yuvası unique'i yerini aldı → D-076 |
| D-069 | LLM katmanı: aynı konuşmada tool_result onarımı (1 + 2), 8 tur; ön eleme fail-closed; thinking/effort/max_tokens; refusal'da fallback yok; prompt paketleri T-022'de | LLM entegrasyonu | Kabul edildi; ajanın soru ve özet araçları eklendi → D-088 |
| D-070 | Teklif akışı: plan ve zamanlamayı Haiku önerir, kod doğrular (1 + 2 onarım); elle düzenleme yeni varyant; süreç içi asenkron + olay dağıtıcısı; metin üreticisi arayüzle | LLM entegrasyonu | Kabul edildi; dil → saat dilimi eşlemesi yerini aldı → D-076; plan ve zamanlama çıktısına soru ve özet alanları eklendi → D-088 |
| D-071 | Metin: yuvalı HTML e-posta şablonu, yalnız {{first_name}}, en uzun ad Customer'dan, 1 + 2 onarım, yapılandırılabilir fiyat ve CTA | LLM entegrasyonu | Kabul edildi; SMS'te link yasağı ve 160 bütçesi yerini aldı → D-082; maliyet para birimi yerini aldı → D-083 |
| D-072 | Proposal REST kaynak + eylem uçları, problem+json; SSE snapshot + canlı olaylar; demo'da kimlik doğrulama yok; LLM_MODE=auto ile anahtarsız mock | Takip ve altyapı | Kabul edildi |
| D-073 | DSL vs GraphQL JSON ölçümü: token sayımı canlı deneyde, make bench + bench-smoke, string içinde JSON kolu, pilot sonra tam; compose bench profili, llm_call maliyet raporu, LLM/DSL/SSE metrikleri, bayraklı pprof | Test ve kapsam | Kabul edildi |
| D-074 | Onay: alıcılar onay anında Customer'dan, saklanmaz; olay ve 500'lük RECIPIENTS batch'leri tek outbox transaction'ında; cancel ve confirm uçları | Gönderim | Kabul edildi |
| D-075 | Zamanlama prompt'u schedule@1.1.0: brief'teki zaman tüm dillere; konum bazlı zamanlama T-043'te, T-027'den önce | LLM entegrasyonu | Kabul edildi |
| D-076 | Konum bazlı zamanlama: (dil, saat dilimi) yuvaları, dim_customer.timezone, GraphQL byLocale + Recipient.timezone, migration 00004, schedule@2.0.0, olayda timezone | LLM entegrasyonu | Kabul edildi |
| D-077 | notification_db şeması: locale_schedule, message, delivery'de schedule_id/message_id FK, recipient_batch + batch_count, CHECK'ler ve iki indeks, kampanya bitince PII temizliği | Gönderim | Kabul edildi |
| D-078 | Notification tüketicileri: outbox'a headers, sched.<id> → fire.<id>, iş anahtarıyla idempotent ham tüketici, tetiklenmede SEND kuyruğu, iptalde durum + PII temizliği | Gönderim | Kabul edildi |
| D-079 | Gönderim worker'ları: provider sözleşmesi, 5 deneme 2 sn ×2 1 dk tavan, 30 sn lease, x/time/rate 100/s burst 10, 100'lük izin kontrolü, gobreaker 5/30 sn/1, SMS'te ad kısaltma | Gönderim | Kabul edildi |
| D-080 | Canlı durum ve olaylar: 2 sn tamamlanma denetleyicisi, started zamanlama başına / completed kampanya başına, Proposal tüketir, Notification'da 1 sn yoklamalı SSE | Takip ve altyapı | Kabul edildi; completed olayının maliyet alanı yerini aldı → D-083 |
| D-081 | Mock provider: Go cephe D-079 sözleşmesini uygular, e-postayı Mailpit'e, SMS'i MockServer'a iletir; %5 5xx, %2 429, %1 zaman aşımı, 50–300 ms; kanal başına 110/s; bellekte 24 sa idempotency; receipt yok | Gönderim | Kabul edildi |
| D-082 | Tıklama takibi (T-044): imzalı token ve Analytics yönlendirme ucu + nginx yedeği, tekil CTR, SMS'e kodla kısa link | Takip ve altyapı | Kabul edildi |
| D-083 | Analytics: normalize analytics_db, llm_call başına token_completed, model başına yapılandırılabilir fiyat, özet/liste/prompt sürümü raporları | Takip ve altyapı | Kabul edildi |
| D-084 | Web API istemcisi: servis başına /api/<servis> öneki, commit edilen spec'lerden üretilen ve commit edilen tipler, EventSource + tipli olaylar, problem+json ApiError | Takip ve altyapı | Kabul edildi |
| D-085 | nginx gateway: web/Dockerfile (node 24 build → nginx stable-alpine), host 8080, /api/<servis> öneki, proxy_redirect, /api altında tamponsuz ve 1 sa okuma; /r/ T-044'te | Takip ve altyapı | Kabul edildi |
| D-086 | Web renk paleti logo.png'den: beyaz zemin, karamel primary, krem yalnız hover/aktif, adaçayı ve espresso destek; shadcn token'ları + Tailwind marka renkleri | Takip ve altyapı | Kabul edildi |
| D-087 | Arayüz: Türkçe + vue-i18n, üç kolon (kampanyalar / olay zaman çizelgesi / adım paneli), düz metin, eylemler panelde, istemci araması, profil yok, e-posta önizleme ucu ve önizlemede düzenleme | Takip ve altyapı | Kabul edildi; istemci araması, markdown yokluğu ve yan panel yerleşimi yerini aldı → D-088, D-089; LLM rozeti → D-094 |
| D-088 | Ajan araçları: soru sorma (cevapla yeniden üretim, adım başına 2), adım özetleri, ön elemede başlık; chat_message tablosu, campaign.title; ajan metni markdown | LLM entegrasyonu | Kabul edildi |
| D-089 | Sohbet ağırlıklı arayüz: sade sol menü (+ ikonu, arama/filtre yok), adımlar sohbette akordiyon, e-posta önizleme ucu ve önizlemede düzenleme | Takip ve altyapı | Kabul edildi; liste görünümü ve sohbet kart/akordiyon yerleşimi yerini aldı → D-091 |
| D-090 | E-posta önizleme ucu: `POST /variants/{id}/preview` kaydedilmemiş içerikle, dile göre sabit örnek ad, sandbox iframe; önizlemede alan formu, soru kartında seçenek düğmeleri | Takip ve altyapı | Kabul edildi |
| D-091 | Sade sohbet: kenar çubuğunda durum gruplu akordiyon, satırda başlık ve tarih; sohbette yalnız varyantlar kart, adım olayları ve eylemleri adım başına akordiyon; iptal sohbet çubuğunda | Takip ve altyapı | Kabul edildi; LLM rozeti yerini aldı → D-094 |
| D-092 | Kitle ve plan dağılımları shadcn-vue chart pasta grafikleriyle; sayılar lejantta, tablolar kalkar | Takip ve altyapı | Kabul edildi |
| D-093 | Metin varyantları tek yatay kaydırıcıda (shadcn-vue carousel); onaydan sonra sıradaki bekleyen slayt, köşede kalan yuva sayısı | Takip ve altyapı | Kabul edildi; tek kaydırıcı yerini aldı → D-095 |
| D-094 | Masaüstünde üst başlık yok, LLM modu rozeti arayüzden kalktı; mobil üst çubukta menü ve logo | Takip ve altyapı | Kabul edildi |
| D-095 | Metin adımında kanal başına ayrı kaydırıcı (önce e-posta, sonra SMS); zamanlama adımı da kaydırıcıda | Takip ve altyapı | Kabul edildi |
| D-096 | Kitle adımında kural (DSL) ve kitle düzenleme arayüzden kalkar; API değişmez | Takip ve altyapı | Kabul edildi |
| D-097 | `make test-go` paket paralelliğini `-p 2` ile sınırlar (`GO_TEST_P` ile değişir); `TestNATSScheduling` zamanı sunucu saatiyle ölçer | Test ve kapsam | Kabul edildi |

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

- **Durum:** Kabul edildi · 2026-10-27; dil başına zamanlama kısmı yerini aldı → D-076 · 2026-10-29
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
- **Doğrulama:** 2026-10-28'de (T-023, D-070): `TestSampleBriefEndToEnd` metin varyantlarını yalnız alıcısı olan dil-kanal çiftleri için (en/email, tr/email, tr/sms) ve zamanlamayı dil başına üretir; `TestRejectRegeneratesOnlyThatSlot` yalnız reddedilen dili yeniden üretir; `TestAddContextRegeneratesUnapprovedAndKeepsApproved` onaylıları korur. Gerçek metin üreticisi T-024'te.

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
- **Doğrulama:** 2026-10-28'de (T-023, D-070): `TestSampleBriefEndToEnd` örnek brief'te planı birincil e-posta + yedek SMS olarak e-posta 33 / SMS 13 alıcıyla kurar; `TestInvalidPlanIsRepairedAgainstTheCoverage`, `TestReopenSupersedesTheStepAndEveryLaterOne` (kitle yeniden açılınca plan, metin ve zamanlama `superseded`). Alıcı batch'leri T-026'da.

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
- **Doğrulama:** Kısmen, 2026-10-27'de (T-017): Customer tarafı hazır; `consentStatus` güncel izni ve uygunluğu döndürüyor, `updateConsent` `fact_consent` ve `dim_customer`'ı aynı transaction'da güncelliyor (D-064). Notification tarafı 2026-10-29'da (T-029, D-079): 100'lük batch başına tek `consentStatus`, `skipped_no_consent`, gobreaker ile fail-closed ve Customer dönene kadar yeniden kuyruğa alma `internal/notification/sender` integration testleriyle doğrulandı. Hedef testler: zamanlanmış kampanyada onaydan sonra izin geri çekilen kişi `skipped_no_consent` olur; Customer kapalıyken hiçbir mesaj gönderilmez.

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
- **Doğrulama:** 2026-10-30'da (T-035, D-087): Vue 3 + TS, Tailwind ve shadcn-vue bileşenleri, Vue Router ve feature composable'larıyla ilk ekranlar; global store yok. Sürümler 2026-10-27'de npm registry'den okundu.

## D-013: Frontend ↔ backend: istek/yanıt REST, canlı güncellemeler SSE

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Takip ve altyapı
- **Bağlam:** Arayüz teklif oluşturma, onay/red ve zamanlama için backend'i çağırır; teklif ilerlemesi, metin akışı ve gönderim durumu canlı görünmeli. THOUGHTS.md canlı durum için WebSocket veya event stream öneriyor.
- **Karar:** Tarayıcı ile backend arasındaki istek/yanıt etkileşimleri **REST**. Canlı güncellemeler (teklif adımları, kitle sayıları, metin akışı, gönderim durumu) **Server-Sent Events** ile tarayıcıya gelir. GraphQL (D-007) tarayıcıya açılmaz; yalnızca servisler arası iç arayüzdür.
- **Alternatifler:**
  - WebSocket — reddedildi: sürdürme ve ping'ler elle kurulur, nginx'te Upgrade ayarı gerekir; istemciden sunucuya yön zaten REST ile karşılanıyor.
  - Polling — reddedildi: 1–2 sn gecikme, metin akışı canlı görünmez.
- **Sonuçlar / bedel:** `EventSource` yalnızca GET destekler ve özel header gönderemez; POST ile başlayan metin akışı için `fetch` + `ReadableStream` gerekir, kimlik doğrulama eklenirse cookie veya kısa ömürlü token gerekir (kimlik doğrulama henüz kararlaştırılmadı). SSE'nin `Last-Event-ID` ile sürdürülmesi ve nginx'te tamponlamanın kapatılması uygulamada kurulacak.
- **Doğrulama:** 2026-10-28'de (T-025, D-072): Proposal REST uçları ve `GET /campaigns/{id}/events` SSE; `TestSampleBriefOverRESTAndSSE` (integration, gerçek HTTP) örnek brief'i SSE'den izleyip REST ile zamanlamaya kadar onaylar; `TestEventsStartWithASnapshotAndForwardEvents`, `TestEventsPing`, `TestEventsResnapshotAfterDroppedEvents`. Compose'da `X-Accel-Buffering: no` başlığı gönderiliyor; nginx tamponlaması T-034'te.

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
- **Doğrulama:** 2026-10-30'da (T-034, D-085): `web/Dockerfile` ve `web/nginx.conf`; canlı yığında arayüz, `/api/*` yönlendirmesi ve tamponsuz SSE doğrulandı.

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
- **Doğrulama:** Doğrulanmadı — henüz kod yok. Sürümler 2026-10-27'de Go modül proxy'si ve npm registry'den okundu. 2026-10-30'da (T-033, D-084): TS tipleri üç servisin spec'inden `web/src/api/<servis>/schema.d.ts` olarak üretilir ve `schema-freshness.spec.ts` ile güncel tutulur.

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
- **Doğrulama:** 2026-10-28'de (T-021, D-068): `internal/proposal/store/migrations` tabloları açar; `TestConstraints_ProposalStep`, `TestConstraints_ProposalVariantSlots` ve `TestAppendOnly` (integration) durum, sürüm ve ekleme-yalnız kurallarını sınar.

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
- **Doğrulama:** Kısmen, 2026-10-29'da (T-027, D-077): `UNIQUE(campaign_id, customer_id, channel)` ve `ON CONFLICT DO NOTHING` ile tekrar gelen teslimatın yok sayılması `TestQueries` (notification store) ile doğrulandı. Worker tarafı T-029'da (aynı teslimat için tekrar gelen mesaj tek provider çağrısı) ve T-031'de (mock aynı `Idempotency-Key`'e saklı 202'yi döner, zaman aşımından sonra yeniden deneme ikinci bir e-posta üretmez; canlı gönderimde 1 zaman aşımı tek e-postayla sonuçlandı) doğrulandı. Hedef testler: aynı alıcı batch'i iki kez gelince tek satır; eşzamanlı iki worker tek gönderim yapar.

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
- **Doğrulama:** 2026-10-28'de (T-022, D-069): `internal/proposal/llm/llmtest` senaryolu sahteyi (`Scripted`) ve `httptest` Messages sunucusunu (JSON ve SSE, 429/529, refusal, max_tokens, kesik JSON, akış ortası hata) sağlar; gerçek adaptör `option.WithBaseURL` ile ona bağlanır. `live` etiketli `TestLiveScreenAndSegment` yazıldı, çalıştırılmadı; testler `ANTHROPIC_API_KEY` okumaz. Docker, testcontainers tarafından `~/.docker/run/docker.sock` üzerinden bulunuyor.

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
- **Doğrulama:** 2026-10-28'de (T-022, D-069): `TestRequestBodyScreen`, `TestRequestBodySegment`, `TestRequestBodyStream` her çağrının modelini, thinking türünü, `effort` ve `max_tokens` değerini ve `temperature`/prefill gönderilmediğini; `TestRepeatedCallsSendIdenticalPrefixBytes` önbellek önekinin brief'ten bağımsız olduğunu doğrular.

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
- **Doğrulama:** 2026-10-28'de (T-022, D-069): `TestSegmentValidFirstTry`, `TestSegmentRepairsInTheSameConversation`, `TestSegmentRepairExhaustedAfterThreeProposes`, `TestSegmentEndTurnWithoutToolCountsAsAnAttempt`, `TestSegmentRefusalStopsAtOnce`, `TestSegmentToolsAreStrictAndCanonical`, `TestStreamStructuredDeltasInOrderThenFinalJSON`, `TestStreamStructuredFailures`. Integration: `TestSampleBriefRepairedAgainstTheRealCustomerService` (ilk öneri `Istnbul` → satır 1 sütun 10'da `UNKNOWN_VALUE` ve `İstanbul` önerisi → düzeltilmiş öneri count 46, kapsama 20/13/13/3).

## D-028: SMS en fazla 160 karakter; kodlama ve segment sayısı gösterilir

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** LLM entegrasyonu
- **Bağlam:** Brief SMS'i 160 karakterle sınırlıyor. Türkçe ç, ş, ğ, ı, İ mesajı UCS-2'ye çevirir: tek SMS 70, birleştirilmiş mesajda segment başına 67 karakter (docs/research/03 §5).
- **Karar:** SMS metni en fazla 160 karakterdir. Ölçüm `{{first_name}}` kitledeki en uzun adla genişletilerek yapılır ve gönderimden hemen önce alıcı başına tekrar edilir. Teklifte kodlama (GSM-7 / UCS-2), segment sayısı ve maliyet gösterilir.
- **Alternatifler:**
  - Tek segment sınırı (GSM-7'de 160, UCS-2'de 70) — reddedildi: Türkçe metin çok kısa kalır.
  - ASCII katlama ile GSM-7 — reddedildi: metin bozulur.
- **Sonuçlar / bedel:** Türkçe SMS çoğunlukla 3 UCS-2 segmentidir, maliyet buna göre artar. Segment sayısı UCS-2'de UTF-16 kod birimiyle hesaplanmalıdır. Gönderim anında bir alıcının metni 160'ı aşarsa ne yapılacağı açık.
- **Doğrulama:** 2026-10-28'de (T-024, D-071): `internal/content`: `TestMeasureSMSEncodings` (ö ü Ö Ü Ç GSM-7; ç ş Ş ğ Ğ ı İ UCS-2; uzantı karakterleri 2 septet; emoji 2 UTF-16 birimi), `TestMeasureSMSSegmentBoundaries` (160/153, 70/67), `TestCheckSMSExpandedLengthDecidesNotTemplateLength`; maliyet `TestEstimatesAreExactWhereFloatsDrift`. Gönderim anında yeniden ölçüm Notification'da (T-029).

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
- **Doğrulama:** 2026-10-28'de (T-021, D-068): proposal_db tablolarının anahtarları `uuid DEFAULT uuidv7()`; `TestQueries` (integration) ekleme sorgularının UUIDv7 döndürdüğünü gösterir.

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
- **Doğrulama:** 2026-10-28'de (T-023, D-070): adım sırası ve `open → approved → superseded` geçişleri `internal/proposal/flow`'da; `TestSampleBriefEndToEnd`, `TestConcurrentApprovalsOpenTheNextStepOnce`, `TestReopenSupersedesTheStepAndEveryLaterOne`. Kampanya `drafting → cancelled` (ön eleme) burada. T-026'da (D-074) `drafting → scheduled` onayda ve `drafting | scheduled | sending → cancelled` iptalde, T-030'da (D-080) `scheduled → sending → completed` Notification olaylarıyla doğrulandı.

## D-034: Tüm adımlar tek varyant tablosunda; adım türüne göre CHECK

- **Durum:** Kabul edildi · 2026-10-27; zamanlama yuvası yerini aldı → D-076 · 2026-10-29
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-022 dil ve kanalı olmayan adımların (kitle, kanal planı) varyant biçimini açık bırakmıştı.
- **Karar:** `proposal_variant` tüm adımlar için tek tablodur. Kitle ve kanal planı adımları tek varyantlıdır (`language` ve `channel` NULL). Zamanlama adımı dil başına bir varyanttır (`channel` NULL, D-009). Metin adımı dil × kanal varyantıdır. `CHECK` kısıtları adım türüne göre hangi alanların dolu olacağını zorlar.
- **Alternatifler:**
  - Adım türü başına ayrı tablo — reddedildi: onay ve red mantığı dört kez yazılır.
  - Yalnız metin adımında varyant, diğerleri `proposal_step.payload` — reddedildi: onay ve red mantığı iki yerde olur.
- **Sonuçlar / bedel:** `content jsonb` alanının şeması adım türüne göre değişir ve uygulamada doğrulanır. Adım türü `proposal_variant`'ta da tutulmalı ki `CHECK` onu görebilsin.
- **Doğrulama:** 2026-10-28'de (T-021, D-068): `proposal_variant_shape_check` ve `proposal_variant_step_kind_fkey`; `TestConstraints_ProposalVariant` (integration) her tür için geçerli ve geçersiz biçimi sınar.

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
- **Doğrulama:** Notification tarafı 2026-10-29'da (T-030, D-080): canlı sayımlar `delivery` tablosundan REST ve SSE ile; Analytics özetleri 2026-10-29'da (T-032, D-083): `notification.completed` ve `proposal.token_completed` olaylarından kampanya özeti ve LLM kullanımı.

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
- **Doğrulama:** `internal/proposal/customer` (T-020, D-067): seçimler `operations.go`'da sabit iki sorgudur; `TestQueriesMatchSchema` onları Customer şemasına karşı doğrular, `TestSampleBriefPreview` (integration) sorgunun gerçek Customer'da çalıştığını gösterir.

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
- **Doğrulama:** `internal/proposal/catalog` (T-019, D-066): `TestGeneratedCatalogIsFresh` üretilen kataloğun şemayla güncel olduğunu, `TestGenerateDrift` şema ile metadata sapmasının üretimi durdurduğunu doğrular.

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
- **Doğrulama:** 2026-10-28'de (T-022, D-069): `github.com/anthropics/anthropic-sdk-go/v2` v2.4.0 (araştırma tarihindeki son sürüm v2.3.0'dı); SDK yeniden denemesi 2 (`TestScreenTransportRetriesThenSucceeds`, `TestScreenRetriesGiveUpAfterTwo`). Anahtar yalnız platform config'ten okunur, SDK ortamı kendisi okumaz (`option.WithoutEnvironmentDefaults`); `TestConfigNeverPrintsTheKey`.

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
- **Doğrulama:** 2026-10-28'de (T-022, D-069): `internal/proposal/prompts`: `screen@1.0.0`, `segment@1.0.0`, `prompts.lock.json`, `make generate-prompts`; `TestLockFileIsFresh`, `TestLockCheck` (sürüm artırılmadan değişen hash `ErrVersionNotBumped`), `TestRequiresDSLMatchesParser`, `TestBuildHashCoversThePrefix`, `TestSegmentSystemIsByteStable`.

## D-060: Prompt'lar düz metin; llm_call sürüm, önbellek ve sonuç sütunları alır; sürüm token olayına ve llm_usage'a taşınır; prompt_snapshot yok

- **Durum:** Kabul edildi · 2026-10-27; `outcome` listesi (`accepted` | `validation_failed` | `refusal`) yerini aldı → D-068 · 2026-10-28
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
- **Doğrulama:** Doğrulanmadı — henüz kod yok (T-021, T-022, T-032). 2026-10-29'da (T-032, D-083): `evt.proposal.token_completed.v1` `prompt_name` ve `prompt_version` taşır, Analytics `llm_usage`'a sürümle yazar ve `/llm-usage?group=prompt_version` sürüm başına maliyet ve onarım oranı verir.

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

## D-066: Proposal kataloğu: make generate ile şemadan + metadata'dan üretilir, commit edilir, tazelik testi; sabit değer listeleri Proposal'da, şehir Customer'da; açıklamalar İngilizce

- **Durum:** Kabul edildi · 2026-10-28
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-044 Proposal'ın kataloğunu build sırasında Customer şemasından ve bir metadata dosyasından üretmeyi seçti; üretim yöntemi, değer denetiminin kapsamı ve prompt'a girecek açıklamaların dili açıktı (T-019). `go:embed` paket dışındaki dosyayı gömemediği için şema doğrudan gömülemez. D-065 anlamsal geçişin `UNKNOWN_FIELD`, `UNKNOWN_RELATION`, `UNKNOWN_VALUE`, `TYPE_MISMATCH` üretmesini belirledi.
- **Karar:**
  - Küçük bir Go üretici (`github.com/vektah/gqlparser/v2`, zaten bağımlılık) Customer'ın `schema.graphqls`'ini ve Proposal'daki metadata dosyasını okuyup `internal/proposal/catalog` altında bir Go dosyası üretir. `make generate`'e eklenir, üretilen kod commit edilir (D-051 deseni). Bir test üreticiyi yeniden çalıştırır ve fark varsa başarısız olur.
  - Alanlar, tipler, işleçler ve ilişkiler şemadan türetilir. Metadata dosyası prompt açıklamalarını ve CHECK kısıtlı alanların sabit değer listelerini (`sex`, `language`, `countryCode`, `preferredPlatform`, `loyaltyTier`, `orderStatus`, `cartStatus`, `eventType`) tutar. Proposal bu listelerde `UNKNOWN_VALUE` ve öneri döner; şehir listesi veritabanında olduğu için yalnız Customer denetler (D-062). Bir test metadata listelerini Customer'ın kodundaki listelerle karşılaştırır.
  - Katalog açıklamaları İngilizcedir; segment prompt'u da İngilizce olur, brief Türkçe gelir.
  - Tip kuralları: String alanlarda `===`, `!==`, `in` ve string literal; Boolean alanlarda `===`, `!==` ve `true`/`false`; Int alanlarda kesirsiz ve int32 aralığında sayı, Decimal alanlarda en fazla 2 ondalık (Customer'ın `Decimal` deseni); Date/DateTime alanlarda `date()` ile tüm işleçler, `ago()` ile yalnız `>`, `>=`, `<`, `<=`. Diğer her eşleşme `TYPE_MISMATCH` olur.
  - Anlamsal geçiş, ayrıştırıcıdan farklı olarak ilk hatada durmaz: tüm hataları kaynak sırasıyla, en fazla 10 tane döner.
  - Metadata dosyası YAML'dır ve `go.yaml.in/yaml/v3` ile okunur (viper üzerinden zaten dolaylı bağımlılıktı, doğrudana döner).
- **Alternatifler:**
  - Şemayı Proposal paketine kopyalayıp gömmek ve açılışta ayrıştırmak — reddedildi: kopyanın tazeliği yine test ister, açılışta iş artar.
  - Hiç değer denetimi yapmamak — reddedildi: LLM değer hatalarını bir ağ çağrısı sonra görürdü.
  - Şehir listesini Proposal'a çekmek için yeni bir GraphQL sorgusu — reddedildi: sözleşme (D-058) ve kapsamlar (D-063) değişirdi.
  - Türkçe açıklamalar — reddedildi: alan adları İngilizce kaldığı için karışık metin.
  - Tarih alanlarında şemadaki her işleç — reddedildi: `eventTime === ago("12h")` gibi hiç eşleşmeyecek ifadeler geçerdi. Yalnız sıralama — reddedildi: belirli bir günü seçmek iki sınır gerektirirdi.
  - Yalnız ilk anlamsal hata — reddedildi: birden çok hatalı ifade 3 onarım hakkından birkaçını harcardı.
  - JSON metadata — reddedildi: uzun açıklamalar okunaksız olurdu.
- **Sonuçlar / bedel:** Customer şeması değişince `make generate` çalıştırılmalı; tazelik testi bunu yakalar. Sabit değer listeleri metadata'da ve Customer kodunda iki yerdedir, bir test eşitliklerini korur. Yanlış yazılmış şehir yalnız Customer çağrısından sonra fark edilir.
- **Doğrulama:** `go run ./internal/proposal/catalog/gen` (`make generate-catalog`); `TestGeneratedCatalogIsFresh`, `TestGenerateDrift`, `TestGenerateReportsAllProblems` (üretici); `TestClosedListsMatchCustomer` ve Customer tarafında `TestClosedLists` (sabit listeler); `TestCheckValid` (örnek brief dahil), `TestCheckErrors`, `TestCheckMultipleErrorsInSourceOrder`, `TestCheckCapsErrors`, `TestCheckPositions` (tip kuralları ve hata sınırı); `FuzzCheck` 30 sn, 2,28M çalıştırma, panik yok. `make test` geçti.

## D-067: DSL → GraphQL derleyicisi AST'ye bire bir; elle yazılmış Customer istemcisi; 10 sn, 3 deneme; filterPath DSL konumuna çevrilir

- **Durum:** Kabul edildi · 2026-10-28
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-041 seçimi Proposal kodunda sabitledi, D-058 Customer'ın filtre sözleşmesini, D-062 hata kodlarını ve `extensions.filterPath`'i, D-066 anlamsal geçişi belirledi. Denetlenmiş AST'nin GraphQL değişkenine hangi biçimde çevrileceği, Customer istemcisinin nasıl yazılacağı, zaman aşımı ve yeniden deneme davranışı ve Customer hatalarının ajana nasıl döneceği açıktı (T-020).
- **Karar:**
  - Derleme AST'ye bire birdir: `&&` → `and: [...]`, `||` → `or: [...]`, `!` → `not`; her karşılaştırma ve `in` tek alanlı bir nesnedir (`{ "city": { "eq": "İstanbul" } }`); ilişki `{ "orders": { "none": {...} } }` olur; kök tek bir koşulsa `and` sarılmaz. Aynı alanlar birleştirilmez, mantıksal sadeleştirme yapılmaz. Değerler D-058'e göre: Int JSON sayısı, Decimal JSON string'i, Boolean `true`/`false`, `date()` → `{ "date": "…" }`, `ago()` → `{ "ago": "…" }`. Derleme önce `catalog.Check`'i çalıştırır.
  - Customer istemcisi `net/http` ile elle yazılır; seçimler amaç başına sabit iki sorgudur (`audiencePreview` ve `audienceRecipients`, D-041). Bir test sorgu metinlerini `gqlparser` ile Customer şemasına karşı doğrular. Yeni bağımlılık yoktur.
  - İstek başına 10 sn zaman aşımı. Yalnız ağ hatası, 502, 503 ve 504'te yeniden denenir: toplam en fazla 3 deneme, üstel backoff (200 ms taban, jitter). GraphQL hataları, 401 ve diğer 4xx tekrarlanmaz.
  - Derleyici her GraphQL filtre yolu için DSL kaynak aralığını tutar. Customer'ın `extensions.filterPath`'li hatası (ör. bilinmeyen şehir) aynı kod, satır, sütun, `near` ve önerilerle bir `dsl.Error`'a çevrilir; ajan onu parser hatası gibi onarır.
- **Alternatifler:**
  - Aynı alanları birleştirmek — reddedildi: birleştirme kuralları ve `filterPath` eşlemesi karmaşıklaşırdı. Mantıksal sadeleştirme — reddedildi: önizlemedeki filtre LLM'in yazdığından farklı görünürdü.
  - genqlient — reddedildi: iki sorgu için yeni bir codegen aracı. Genel GraphQL kütüphanesi — reddedildi: şemaya karşı doğrulama yok.
  - Yeniden deneme yok — reddedildi: anlık bir aksaklık bir onarım turu harcardı. Platform outbox politikası (D-054, D-055) — reddedildi: 15 dk ve 20 deneme senkron istek için fazla uzun.
  - Customer mesajını olduğu gibi iletmek — reddedildi: ajan hatanın yerini yalnız mesajdan çıkarırdı.
- **Sonuçlar / bedel:** Derlenmiş JSON birleştirilmiş biçimden uzundur. Yanıt tipleri elle yazılır; şema değişirse sorgu doğrulama testi kırılır. En kötü durumda bir çağrı yaklaşık 31 sn sürer. Yol-aralık eşlemesi derleyiciye kod ekler.
- **Doğrulama:** `TestCompileSampleBriefGolden` örnek brief'in §8.2'deki JSON'a birebir derlendiğini, `TestCompileValueEncodings` ve `TestCompileDecimalIsNeverAJSONNumber` değer biçimlerini doğrular. `TestQueriesMatchSchema` sorguları şemaya karşı denetler. `TestRetryStatusesAndAttemptCap`, `TestRetryOnTransportError`, `TestNoRetry`, `TestPerAttemptTimeout`, `TestContextCancelStopsRetrying` yeniden deneme ve zaman aşımı kurallarını; `TestUnknownValueBecomesDSLError`, `TestUnknownValueInListPointsAtTheElement` ve `TestSpan*` hata eşlemesini doğrular. Integration (gerçek Customer handler'ı + seed'li Postgres): `TestSampleBriefPreview` count 46, kapsama 20 / 13 / 13 / 3; `TestSampleBriefRecipientsOverAllPages` 46 tekil ve sıralı alıcı; `TestUnknownCityComesBackAtTheLiteral` `Istnbul` için literal konumunda `UNKNOWN_VALUE` ve `İstanbul` önerisi; `TestWrongTokenIsUnauthorized`. `make test` geçti.
- **Uygulama notu:** Int, literal metninden `int64` olarak kodlanır (`007` → `7`); Decimal yazıldığı gibi string kalır. gqlgen bazı GraphQL hatalarını 4xx ile döndüğü için `errors` taşıyan bir gövde durum kodundan önce gelir ve tekrarlanmaz. Bir denemenin kendi zaman aşımı ağ hatası sayılır ve tekrarlanır; üst bağlam iptal olursa hemen döner. Yanıt en fazla 8 MiB'tır. Ortam değişkenleri: `CUSTOMER_GRAPHQL_URL`, `CUSTOMER_TOKEN` (compose'da `CUSTOMER_TOKEN_PROPOSAL`'dan); compose başlangıç sırası (D-056) değişmedi.

## D-068: proposal_db şeması: taslak + denetim kolonları, kısmi unique ve FK indeksleri, CHECK'ler, ekleme-yalnız trigger; sqlc ve açılışta migration

- **Durum:** Kabul edildi · 2026-10-28; `llm_call.purpose` listesi yerini aldı → D-070 · 2026-10-28; canlı varyant yuvası unique'i yerini aldı → D-076 · 2026-10-29
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-022, D-033 ve D-034 tablo sınırlarını, adım türlerini ve durum değerlerini; D-030 UUIDv7 anahtarları; D-060 `llm_call`'ın prompt sürüm sütunlarını belirledi. Kolon listeleri, indeksler, kısıtların DB'de ne kadar zorlanacağı ve görevin kapsamı açıktı (T-021; kodlama kuralı 7). `outbox` ve `processed_event` zaten platform setindedir (D-054).
- **Karar:**
  - Kolonlar `docs/domain-model.md` §3 taslağıdır, şu eklemelerle: `proposal_variant.approved_at` ve `rejected_at`; `llm_call`'a D-060 sütunları (`prompt_name`, `prompt_version`, `prompt_sha256` NOT NULL, `cache_creation_tokens`, `outcome`) ile `step_id`, modelin yapılandırılmış çıktısı `response jsonb` ve `error text`. `outcome` listesi `accepted` | `validation_failed` | `refusal` | `api_error` olur (D-060'tan sapma, kullanıcı onayladı).
  - Kısmi unique: bir kampanyada tür başına en fazla bir `superseded` olmayan adım; bir adımda (`language`, `channel`) başına en fazla bir `draft` veya `approved` varyant (`NULLS NOT DISTINCT`); `(campaign_id, kind, version)` unique. Tüm yabancı anahtarlarda indeks.
  - Durum ve tür sütunları `text` + `CHECK`; D-034 varyant `CHECK`'i; `approved` satırda `approved_at` dolu, `open` adımda ve `draft` varyantta boş; `rejected` varyantta `rejected_at` ve `rejection_reason` dolu; `superseded` satır önceki damgalarını tarih için korur. Varyantın `kind`'ı `(step_id, kind)` bileşik yabancı anahtarıyla adımınkine bağlanır. Bunun için `proposal_step`'te `UNIQUE (id, kind)` açılır (kullanıcı ayrıca onayladı). `context_note` ve `llm_call`'da `UPDATE` ve `DELETE`'i reddeden bir trigger.
  - Kapsam: Proposal'ın migration seti, `app.Options`'a servis migration'ları ve açılışta uygulanması, `sqlc.yaml` ile temel ekleme ve okuma sorguları, kısıtları sınayan entegrasyon testleri.
- **Alternatifler:**
  - Yalnız D-060 sütunları — reddedildi: başarısız bir denemenin neden reddedildiği yalnız loglarda kalırdı, API hataları `llm_call`'a yazılamazdı.
  - Yalnız FK indeksleri — reddedildi: çift açık adım veya çift varyant yalnız uygulama koduyla engellenirdi.
  - Trigger'sız yalnız `CHECK` — reddedildi: elle bir `UPDATE` engellenmezdi. Postgres `ENUM` tipleri — reddedildi: değer eklemek `ALTER TYPE` ister.
  - Yalnız migration — reddedildi: sqlc ve açılış T-023'ü büyütürdü.
- **Sonuçlar / bedel:** `llm_call` satırları model çıktısını taşıdığı için büyür. Birkaç indeks yazım maliyeti ekler. Bir PL/pgSQL fonksiyonu ve trigger bakım ister. Görev T-023'ten bir parça iş alır.
- **Doğrulama:** 2026-10-28'de (T-021, D-068): `internal/proposal/store` integration testleri (115 alt test): `TestMigrations_Applied`, `TestMigrations_DownThenUp`, `TestConstraints_Campaign`, `_ProposalStep`, `_ProposalVariant`, `_ProposalVariantSlots`, `_ContextNote`, `_LLMCall` (her CHECK için red ve kabul, kısmi unique'ler iki yönde, bileşik FK), `TestAppendOnly`, `TestQueries` (10 sqlc sorgusu). `TestRun_ServiceMigrationsRequireDatabase` (app). Gerçek yığında `docker compose up -d --build proposal` sonrası proposal `healthy`; `proposal_db`'de beş servis tablosu, `outbox`, `processed_event`, `goose_db_version` ve `goose_platform_version` `proposal` şemasında. `make generate` ikinci çalıştırmada fark bırakmadı; `make test` geçti.
- **Uygulama notu:** Migration'lar `internal/proposal/store/migrations` altında iki dosyadır (çekirdek tablolar; denetim tabloları ve trigger). "Boş olmayan" metin `~ '\S'` ile denetlenir. Trigger fonksiyonu `reject_row_change()` SQLSTATE P0001 ile hata verir. `TRUNCATE` engellenmez. `content` için JSON şeması uygulamada doğrulanır (açık konu).

## D-069: LLM katmanı: aynı konuşmada tool_result onarımı (1 + 2), 8 tur; ön eleme fail-closed; thinking/effort/max_tokens; refusal'da fallback yok; prompt paketleri T-022'de

- **Durum:** Kabul edildi · 2026-10-28; ajanın soru ve özet araçları eklendi → D-088 · 2026-10-30
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-026 modelleri, D-027 strict tool döngüsünü, structured outputs'u ve "en fazla 3 deneme"yi, D-047 SDK'yı, D-059 ve D-060 prompt paketlerini belirledi. Onarımın nasıl yapılacağı, ön elemenin sonucu, çağrı parametreleri, refusal davranışı, tur sınırı ve görevin kapsamı açıktı (T-022).
- **Karar:**
  - Onarım aynı konuşmadadır: geçersiz bir `propose_segment` (ayrıştırma, katalog veya Customer hatası, ya da kitle 0) `is_error` tool_result olarak satır, sütun, `near` ve önerilerle döner. Toplam en fazla 3 `propose_segment` denemesi (1 + 2 onarım); modelin tool çağırmadan bitirmesi de bir deneme sayılır. `count_audience` dahil toplam en fazla 8 model turu; aşılırsa döngü başarısız biter. SDK'nın taşıma yeniden denemeleri ayrıdır (varsayılan 2). Thinking blokları değiştirilmeden geri gönderilir.
  - Ön eleme (Haiku 5.5) structured outputs ile `{verdict: allow | block, reason}` döner. `block`'ta segment çağrısı yapılmaz, kullanıcı gerekçeyi görüp brief'i düzenler. Ön eleme çağrısı hata verirse de akış durur (fail-closed).
  - Parametreler: segment Sonnet 5.5, adaptive thinking, `effort: medium`, `max_tokens` 4096. Metin Sonnet 5.5, thinking `between_tools` (Sonnet 5.5'te `disabled` 400 döner), `effort: low`, `max_tokens` 2048, stream. Ön eleme Haiku 5.5, thinking `disabled`, `effort: low`, `max_tokens` 256. `temperature` ve prefill gönderilmez (D-026).
  - Ön eleme ölçütü (kullanıcı ayrıca onayladı): talimatları geçersiz kılma, izinden bağımsız herkese gönderme, veri sızdırma veya sistem kurallarını değiştirme girişimi `block`; agresif ama meşru pazarlama metni `allow`. Segment prompt'unun sorumlulukları: brief JSON blokta güvenilmeyen veridir; ajan yalnız katalog alanlarıyla DSL filtresi yazar (D-041), göreli zaman için `ago()` kullanır, tarih hesabı yapmaz, boyutu `count_audience` ile yoklar, son DSL'i bir kez `propose_segment` ile verir ve dönen hatalarla onarır.
  - Segment çağrısı `max_tokens` ile kesilirse başarısız bir deneme sayılır (kullanıcı ayrıca onayladı); daha yüksek limitle tekrar yoktur.
  - Kayıtlar (kullanıcı ayrıca onayladı): `StreamStructured` çağıranın verdiği bir doğrulama fonksiyonunu akış bittikten sonra çalıştırır; `llm_call` kaydı sonuç belli olunca bir kez yazılır (`accepted` veya `validation_failed`), çünkü tablo ekleme-yalnızdır. Kitle 0 hatasının kodu `EMPTY_AUDIENCE` yalnız `llm` paketindedir; D-065'in DSL hata listesi değişmez. `SegmentTools` (Customer) altyapı hatası `outcome = api_error` olarak, ayrıntısı `error` metninde kaydedilir.
  - `stop_reason: refusal`: server-side fallback yok. Onarım denenmez; `llm_call`'a `outcome = refusal` ve `stop_details` kategorisi yazılır, kullanıcıya brief'i düzenlemesi söylenir.
  - Kapsam: LLM portu, Anthropic adaptörü, onarım döngüsü, senaryolu sahte ve `httptest` SSE sunucusu, `screen@1.0.0` ve `segment@1.0.0` düz metin prompt'ları, `prompts.lock.json` ve kilit testi (D-059). Metin çağrısı için jenerik, stream'li structured output yöntemi yazılır; somut SMS ve e-posta şeması ile `content` prompt'u T-024'tedir.
- **Alternatifler:**
  - Her onarım yeni durumsuz bir çağrı — reddedildi: `count_audience` geçmişi kaybolurdu ve her onarım daha pahalı olurdu. 1 + 3 onarım — reddedildi: D-027'deki "en fazla 3 deneme"den sapma.
  - Ön elemede uyarıp onayla devam — reddedildi: savunma insan dikkatine kalırdı. Yalnız kaydetmek — reddedildi: ön eleme savunma olmaktan çıkardı.
  - Thinking'siz ve her yerde `low` — reddedildi: karmaşık brief'te kural kalitesi düşebilirdi. Metinde `medium` — reddedildi: akış başlangıcı gecikirdi.
  - `fallbacks: "default"` — reddedildi: beta bağımlılığı, yalnız cyber ve frontier_llm reddini kapsar, farklı model çıktısı verir.
  - Tur sınırı yok — reddedildi: sınırsız yoklama. 15 tur — reddedildi: en kötü durumda maliyet iki katına yaklaşır.
  - Metin şemasını da T-022'de yapmak — reddedildi: HTML email ve SMS kararları (T-024) önceden verilmek zorunda kalırdı. Yalnız mekanizma — reddedildi: kilit ve istek bayt testleri yer tutucu prompt'larla kalırdı.
- **Sonuçlar / bedel:** Aynı konuşmada onarım konuşmayı uzatır. Ön elemenin yanlış pozitifleri kullanıcıyı durdurur. Segment çağrısı thinking yüzünden daha yavaş ve pahalıdır. Nadir bir yanlış red akışı durdurur. Çok keşifçi bir ajan 8 turda kesilebilir. T-022 metni uçtan uca göstermez.
- **Doğrulama:** 2026-10-28'de (T-022, D-069): yukarıdaki D-026, D-027 ve D-059 testlerine ek olarak `TestSegmentTurnLimit`, `TestSegmentMaxTokensIsAFailedAttempt`, `TestSegmentEmptyAudienceIsAValidationError`, `TestSegmentBackendFailureStopsTheLoop`, `TestScreenFailsClosed`, `TestScreenBlockIsAnErrorToo`, `TestScreenRefusalRecordsCategory`, `TestStreamStructuredValidate`, `TestStreamStructuredDoesNotValidateAFailedStream`; integration `TestEmptyAudienceAgainstTheRealCustomerService`. `make generate` ikinci çalıştırmada fark bırakmadı; `make test` geçti.
- **Uygulama notu:** Kayıt `llm.WithRecorder(ctx, r)` ile toplanır; `CallRecord.Attempt` segmentte model turu numarasıdır (1–8) ve kabul edilen öneriyi taşıyan tur `SegmentResult.Turn`'dür. Refusal kategorisi `error` metnine yazılır. Onarım uyarı metinleri adaptörde sabittir, prompt hash'ine girmez. Segment prompt'u "hafta = 7d, ay = 30d" kuralı ve derlenen 3 örnek içerir. `ANTHROPIC_BASE_URL` isteğe bağlıdır.

## D-070: Teklif akışı: plan ve zamanlamayı Haiku önerir, kod doğrular (1 + 2 onarım); elle düzenleme yeni varyant; süreç içi asenkron + olay dağıtıcısı; metin üreticisi arayüzle

- **Durum:** Kabul edildi · 2026-10-28; dil → saat dilimi eşlemesi yerini aldı → D-076 · 2026-10-29; plan ve zamanlama çıktısına soru ve özet alanları eklendi → D-088 · 2026-10-30
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-009, D-010, D-022, D-033 ve D-034 adımları, varyantları ve durum makinelerini; D-069 segment ve ön eleme çağrılarını belirledi. Kanal planını ve zamanlamayı kimin önereceği, elle düzenleme, uzun LLM adımlarının nasıl çalışacağı ve metin adımının T-024 ile sınırı açıktı (T-023). Brief "pazarlamacı önizler, gerekirse düzenler ve onaylar" diyor.
- **Karar:**
  - Kanal planını ve dil başına gönderim zamanını LLM önerir: Haiku 5.5, thinking `disabled`, `effort: low`, structured outputs, `max_tokens` 512; yeni prompt paketleri `channel_plan@1.0.0` ve `schedule@1.0.0` (D-059).
    - Plan prompt'u SQL'in hesapladığı kapsama tablosuna, brief'e ve bağlam notlarına bakar. Varsayılan birincil + yedek, berabere e-posta (D-010); "iki kanal birden" yalnız brief açıkça isterse.
    - Zamanlama prompt'u brief ve notlardaki açık zamanı dil başına yorumlar (tr → Europe/Istanbul, en → Europe/London; şu anki zaman kullanıcı mesajındadır). Zaman yoksa `null` (hemen). Brief burada da güvenilmeyen veridir.
  - Kod doğrular: plan `{mode, primary, rationale}`, seçilen kanal(lar)da alıcı > 0; zamanlama dil başına `{send_at RFC 3339 | null, rationale}`, `null` ya da şimdi + 5 dk ile şimdi + 30 gün arası. Hata aynı konuşmada geri verilir, toplam 3 deneme (1 + 2 onarım).
  - `llm_call.purpose` CHECK'i yeni bir goose migration'ı (`00003`) ile `screen`, `segment`, `content`, `channel_plan`, `schedule` olur; Down eski listeye döner (D-068'den sapma, kodlama kuralı 7, kullanıcı onayladı).
  - Elle düzenleme: düzenlenen içerik üretilenle aynı doğrulayıcılardan geçer, yeni `draft` varyant açılır, eskisi `superseded` olur. Kitle adımında düzenlenen DSL yeniden derlenir ve sayılır.
  - Çalışma: istekler hemen döner. Üretim kampanya başına Postgres advisory lock ile sıralanan bir goroutine'de çalışır ve ilerlemeyi bellekteki bir olay dağıtıcısına yazar (T-025'in SSE'si dinler). Durum DB'dedir; servis yeniden başlarsa yarım kalan adım yeniden üretilebilir.
  - Kullanıcı ayrıca onayladı: ön eleme brief'i engellerse kampanya `cancelled` olur, düzenlenen brief yeni bir kampanya açar. Onaylanmış bir önceki adım `Reopen` ile revize edilir: o adım ve sonraki tüm adımlar hemen `superseded` olur, yeni sürüm (`version + 1`) açılıp yeniden üretilir. Başarısız veya yarım kalan üretim adımın son `llm_call` satırlarından türetilir ve kullanıcı `Regenerate` ile sürdürür; açılışta otomatik devam yoktur. Kanal planının alıcı sayıları kodda kapsamadan hesaplanır: tek kanal = o kanala uygun olanlar; birincil + yedek = birincil `both + birincil_only`, diğeri `diğer_only`; iki kanal = her kanal `both + o_only`.
  - Sınırlar (kullanıcı ayrıca onayladı): zamanlama adımını kapatan `Approve`, commit'ten sonra `Confirmer.Confirm`'i senkron ve bir kez çağırır; hata kullanıcıya döner, onay kalır, T-026 kendi outbox transaction'ını açar ve yeniden deneme yolu sunar. `ContentGenerator` `Generate` ile birlikte `Validate(language, channel, content)` sağlar; elle düzenleme ve üretim aynı doğrulayıcıdan geçer. Red gerekçesi modele kullanıcı mesajındaki bağlam notlarına eklenen bir metin olarak gider, `context_note`'a yazılmaz; prompt'lar değişmez. Süreç ön eleme sırasında düşerse adımsız `drafting` kampanya `RegenerateCampaign` ile yeniden başlatılır.
  - Metin adımı `ContentGenerator` arayüzüyle dil × kanal varyantlarını yönetir; T-023'te testler için sahte üretici vardır, gerçek üretici T-024'te takılır.
- **Alternatifler:**
  - Plan ve zamanlama kod kuralıyla, LLM'siz — reddedildi: kullanıcı brief'teki zaman ifadelerinin ve kanal isteğinin ajan tarafından okunmasını istedi. Yalnız zamanlamada LLM — reddedildi.
  - Sonnet 5.5 — reddedildi: girdiler küçük, seçim dar; yaklaşık 20 kat pahalı.
  - Geçersiz çıktıda kod kuralına sessizce düşmek — reddedildi: LLM hatası görünmez olurdu.
  - Uygulanmış `00002`'yi düzenlemek — reddedildi: compose'daki DB'de `down -v` gerekirdi.
  - "İki kanal"ı ajana serbest bırakmak — reddedildi: kişiye istemeden iki mesaj önerilebilirdi.
  - Düzenleme yalnız metin ve zamanlamada veya hiç yok — reddedildi: kullanıcı DSL'i elle düzeltemezdi / brief'in "düzenler" adımı karşılanmazdı.
  - Senkron istek — reddedildi: 10–30 sn süren istekler, ilerleme gösterilemez. NATS iş kuyruğu — reddedildi: yeni stream ve tüketici, daha fazla kod.
  - T-024'ü öne almak — reddedildi: görev sırası değişirdi.
- **Sonuçlar / bedel:** İki prompt paketi, iki doğrulayıcı ve onarım yolu eklenir; her kampanyaya iki Haiku çağrısı. Bir migration daha. Olay dağıtıcısı süreç içidir, çoklu replikada paylaşılmaz. T-024'e kadar canlı akışta metin adımı çalışmaz.
- **Ek (kullanıcı onayı, 2026-10-28, T-024):** `ContentGenerator.Validate` imzası `Validate(req ContentRequest, content) (json.RawMessage, error)` olur; düzenlenen içeriğin ölçümleri (en uzun ad, alıcı, maliyet) yuvanın bağlamıyla yeniden hesaplanır ve saklanacak içerik döner.
- **Doğrulama:** 2026-10-28'de (T-023, D-070): `internal/proposal/flow` integration testleri (24 test, gerçek Customer handler'ı + seed'li Postgres, senaryolu LLM, sahte metin üreticisi): `TestSampleBriefEndToEnd` (46, 20/13/13/3; plan e-posta 33 / SMS 13; 3 metin varyantı; Confirmer bir kez; `llm_call` satırları amaçlarıyla), `TestScreenBlockCancelsTheCampaign`, `TestEditAudienceRecompilesAndRecounts`, `TestEditScheduleValidatesTheWindow`, `TestReopenSupersedesTheStepAndEveryLaterOne`, `TestSegmentRepairExhaustedIsReportedAndRegenerateRecovers`, `TestRefusalFailsThePlanStepAndRegenerateRecovers`, `TestGenerationWaitsForTheAdvisoryLock`, `TestConfirmerFailureIsReportedAndTheApprovalStands`, `TestRegenerateCampaign*`; store `TestMigration00003_PurposeList`, `_DownRestoresTheOldList`; adaptörde plan ve zamanlama istek gövdesi ve onarım testleri. `make generate` ikinci çalıştırmada fark bırakmadı; `make test` geçti (ilk çalıştırmada Customer `audience` paketi bir kez düştü, sonraki üç çalıştırmada geçti; neden kaydedilmedi).
- **Uygulama notu:** Üretim sonucu, beklerken kampanya `drafting` dışına çıktıysa, adım kapandıysa, yeni bir not eklendiyse veya yuva dolduysa atılır (çağrılar kaydedilir, varyant açılmaz). `llm_call.variant_id` yalnız tek varyant üreten kabul edilmiş son kayda yazılır; kayıtlar varyantla aynı transaction'da yazılır. Worker sayısı havuzun yarısıyla sınırlıdır (kilit için bir, transaction için bir bağlantı). Bir metin yuvası düşerse diğerleri üretilmeye devam eder; `Regenerate` yalnız eksik yuvaları doldurur. Plan doğrulayıcısı yalnız mod, kanal ve alıcı > 0'ı denetler; beraberlik ve "iki kanal yalnız açık istenirse" kuralları prompt'tadır. Çalışan bir başlangıç varken `RegenerateCampaign` hiçbir şey yapmadan döner; adımlar açıldıktan sonra `ErrHasSteps` döner.

## D-071: Metin: yuvalı HTML e-posta şablonu, yalnız {{first_name}}, en uzun ad Customer'dan, 1 + 2 onarım, yapılandırılabilir fiyat ve CTA

- **Durum:** Kabul edildi · 2026-10-28; SMS'te link yasağı ve 160 karakter bütçesi yerini aldı → D-082 · 2026-10-29; maliyet para birimi yerini aldı → D-083 · 2026-10-29
- **Alan:** LLM entegrasyonu
- **Bağlam:** Brief SMS'i 160, e-posta konusunu 80 karakterle sınırlıyor ve `{{first_name}}` gibi değişkenler istiyor. D-028 ölçümün en uzun adla yapılmasını, kodlama, segment ve maliyetin gösterilmesini; D-069 metin çağrısının parametrelerini ve doğrulama fonksiyonunu; D-070 `ContentGenerator` arayüzünü belirledi. HTML e-posta biçimi, değişken listesi, en uzun adın kaynağı, onarım, fiyat, e-posta blok sınırları ve `content` prompt'unun sorumlulukları açıktı (T-024).
- **Karar:**
  - E-posta yuvalı şablondur: Claude `subject`, `preheader`, `headline`, `paragraphs[]`, `cta_label` döner; Go bunları sabit, tablo tabanlı, inline-CSS bir şablona `html/template` kaçışıyla yerleştirir, düz metin alternatifi aynı bloklardan üretilir. SMS çıktısı `{text}`'tir. Şema kaynağında `sms` ve `subject` önce gelir; API'ye giden kanonik JSON (D-059) anahtarları alfabetik sıralar, bu sıra korunmaz (kullanıcı onayladı: her çağrıda tek kanal dolu, e-postada konunun sonda akması yalnız görünüş).
  - Sınırlar (karakter = Unicode karakter, `{{first_name}}` en uzun adla genişletilerek): SMS ≤ 160; `subject` ≤ 80, `preheader` ≤ 100, `headline` ≤ 80, 1–4 paragraf her biri ≤ 600, `cta_label` ≤ 30. SMS için GSM-7 / UCS-2 kodlaması ve segment sayısı (UCS-2'de UTF-16 kod birimi) hesaplanır ve varyantta saklanır.
  - CTA URL'si LLM'den değil `CAMPAIGN_CTA_URL` yapılandırmasından gelir (demo varsayılanı `https://shop.example.com`) ve `utm_campaign=<kampanya id>` alır. Abonelikten çıkma alt bilgisi kodda eklenir.
  - Değişken beyaz listesi yalnız `{{first_name}}`'dir; başka `{{…}}`, URL veya HTML içeren metin reddedilir. Değişken yerleştirmesi kendi kodumuzla yapılır, LLM metni şablon motorundan geçmez.
  - En uzun ad Customer'dan gelir: `AudiencePreview.maxFirstNameLength` ve `LanguageCoverage.maxFirstNameLength` eklenir (D-058 sözleşmesine ekleme). Her dil varyantı kendi kitlesinin en uzun adıyla ölçülür.
  - Doğrulamayı geçmeyen metin için 1 + 2 onarım: hata listesi ve önceki çıktı yeni, stream'li bir çağrıya verilir; toplam 3 deneme, her deneme `llm_call`'a yazılır.
  - Kullanıcı ayrıca onayladı: `content@1.0.0` tek şemadır; `sms` ve `email` ikisi de nullable, kullanıcı mesajı hangisinin doldurulacağını söyler, doğrulayıcı yalnız istenenin dolu olmasını ister. Varyant e-posta bloklarını ve ölçümlerini saklar, HTML saklanmaz; önizleme ve gönderim ortak `internal/content` paketiyle alıcı adıyla çizilir.
  - Kullanıcı ayrıca onayladı: segment ve maliyet tahmini `{{first_name}}` yerine ASCII dolgu (`x`) ile ölçülür; kodlama yalnız şablon metnine bağlıdır, Türkçe karakterli bir ad gönderimde mesajı UCS-2'ye çevirebilir ve Notification gerçek adla yeniden ölçmelidir. Abonelikten çıkma alt bilgisi yalnız metindir; link ve `List-Unsubscribe` header'ı yoktur (mock sağlayıcı, README'de belirtilecek).
  - Maliyet: `SMS_SEGMENT_PRICE_GBP` (demo varsayılanı 0.04) ve `EMAIL_PRICE_GBP` (0.001); varyant tahmini segment × alıcı × fiyat, kanal planı toplamı gösterilir.
  - `content@1.0.0` prompt'u: yuvanın dilinde (tr/en) ve kanalında yazar, brief'teki tonu izler; indirim ve teklif yalnız brief'te yazıldığı kadardır, yeni vaat yoktur; yalnız `{{first_name}}` kullanılabilir, URL ve başka değişken yoktur; SMS kısa ve tek mesajdır; sınırlar ve en uzun ad uzunluğu kullanıcı mesajındadır; brief ve notlar güvenilmeyen veridir.
- **Alternatifler:**
  - Serbest HTML + bluemonday — reddedildi: CSS temizlenmez, stil silmek düzeni bozar. Yalnız düz metin — reddedildi: HTML önizleme olmazdı.
  - `{{city}}` de — reddedildi: Customer'ın alıcı sorgusu ve uzunluk ölçümü genişlerdi.
  - Sabit 20 karakter varsayımı — reddedildi: gerçek kitlede daha uzun ad limiti aşabilirdi. Yalnız genel en uzun ad — reddedildi: bir dildeki uzun ad diğerini kısıtlardı.
  - Onarımsız — reddedildi: Türkçe SMS'te sık limit aşımı kullanıcıyı yorardı.
  - Yalnız segment sayısı — reddedildi: D-028 maliyetin gösterilmesini istiyor.
  - Geniş sınırlar ve CTA'sız e-posta — reddedildi: düzen garantisi zayıf.
  - Serbest yaratıcılık — reddedildi: brief'te olmayan indirim veya vaat üretilebilirdi.
- **Sonuçlar / bedel:** Tasarım esnekliği şablonla sınırlıdır. Customer şeması ve sorgusu değişir. Başarısızlıkta 3 kat maliyet. Fiyatlar gerçek sağlayıcı fiyatı değil demo değerleridir. Uzun e-posta yazılamaz. Gönderim anında bir alıcının metni sınırı aşarsa ne yapılacağı açık kalır (D-028, Notification).
- **Doğrulama:** 2026-10-28'de (T-024, D-071): `internal/content` (%96,9): `TestPlaceholderWhitelist`, `TestURLAndHTMLRejection`, `TestCheckEmailLimits`, `TestRenderEmailEscapesEverything`, `TestCTAURL`, `TestFooterPerLanguage`; `internal/proposal/contentgen` (%95,7): `TestGenerateRepairsAnOverLongAnswer`, `TestGenerateRepairExhausted`, `TestGenerateWrongChannelIsRepaired`, `TestGenerateRepairsACutOffAnswer`, `TestGenerateThreeCutOffsExhaustTheRepairs`, `TestGenerateCutOffAndNonJSONRecordsPerAttempt`, `TestGenerateRecordsOneCallPerAttempt`, `TestGenerateRequestUsesTheContentPackage`, `TestValidateRecomputesMeasurements`; Customer `TestSampleBriefMaxFirstNameLength` (doğrudan SQL `max` ile aynı, genel ve dil başına); flow `TestSampleBriefContentWithTheRealGenerator`. `make generate` ikinci çalıştırmada fark bırakmadı; `make test` geçti.
- **Uygulama notu:** `ContentGenerator.Validate` imzası `Validate(req ContentRequest, content) (json.RawMessage, error)` oldu (D-070 eki). Konu, önbaşlık, başlık, paragraf ve CTA tek satırdır; SMS satır sonu içerebilir. URL reddi şema, `www.`, bilinen şemalar ve sabit bir TLD listesiyle çıplak alan adlarını kapsar. Maliyet tam sayı mikro-GBP ile hesaplanır, 4 ondalıkla gösterilir. Kesik (`max_tokens`) veya JSON olmayan çıktı segmentteki gibi başarısız bir deneme sayılır ve onarılır (kullanıcı ayrıca onayladı); refusal ve API hatası onarılmaz. Segment modeli `count_audience` sonucunda en uzun ad alanlarını görmez (D-069 davranışı korunur). Ayrıca `outputConfig` şemadaki sayıları string olarak gönderiyordu; düzeltildi (`TestOutputSchemaNumbersStayNumbers`).

## D-072: Proposal REST kaynak + eylem uçları, problem+json; SSE snapshot + canlı olaylar; demo'da kimlik doğrulama yok; LLM_MODE=auto ile anahtarsız mock

- **Durum:** Kabul edildi · 2026-10-28
- **Alan:** Takip ve altyapı
- **Bağlam:** D-013 tarayıcı ile backend arasında REST ve SSE'yi, D-016 sözleşmenin swag v2 ile üretilmesini, D-070 akış servisini ve süreç içi olay dağıtıcısını belirledi. Uç noktaların biçimi, SSE olay sözleşmesi (açık konu), arayüz için kimlik doğrulama (açık konu) ve anahtar olmadan servisin nasıl çalışacağı açıktı (T-025). D-071 metin onarımında akışın baştan başladığını arayüze bildiren bir olay gerektiğini not etti.
- **Karar:**
  - REST kaynak + eylem uçlarıdır: `POST /campaigns`, `GET /campaigns`, `GET /campaigns/{id}`; `POST /variants/{id}/approve | reject | edit`; `POST /campaigns/{id}/context | reopen | regenerate`; `POST /steps/{id}/regenerate`. Üretim başlatan istekler 202 döner. Hatalar RFC 9457 `application/problem+json`; flow hataları 404, 409 ve 422'ye eşlenir. Uçlar swag v2 annotation'larıyla belgelenir (D-016).
  - SSE: `GET /campaigns/{id}/events`. İlk olay tam kampanya görünümüdür (`snapshot`), ardından flow olayları aynı adlarla gelir; 15 sn'de bir `ping`. Tekrar oynatma tamponu yoktur; yeniden bağlanan istemci yeni bir snapshot alır. Metin onarımında `content_delta` olayları `attempt` taşır ve yeni denemenin başında `content_reset` gönderilir. Olay şemaları OpenAPI'de components olarak belgelenir.
  - Arayüz ve SSE için kimlik doğrulama yoktur (demo); Proposal yalnız nginx arkasındadır (T-034). README'de kapsam dışı olarak belirtilir.
  - `LLM_MODE` = `auto` | `anthropic` | `mock`, varsayılan `auto`: `ANTHROPIC_API_KEY` doluysa gerçek model, boşsa senaryolu mock kullanılır; seçilen mod log'da ve `/readyz` kontrolünde görünür. Mock, örnek brief ve stub'daki geçmiş kampanya brief'leri için elle yazılmış yanıtlar verir; eşleşmeyen brief'te basit bir varsayılan filtre (`totalOrders >= 1`) kullanır, metne `[mock]` işareti ve gerekçeye "mock yanıt" notu koyar.
  - Kullanıcı ayrıca onayladı: platform `health.CheckResult` isteğe bağlı bir `detail` alanı alır (`{"llm":{"status":"ok","detail":"mode=mock"}}`); dört servisin spec'i bu alanla genişler. Girdi sınırları: gövde 64 KiB (aşımda 413), brief 2000, not 2000, red gerekçesi 1000 karakter (aşımda 422). Mock deterministiktir; bir çağrı yeniden üretimse (red gerekçesi veya bağlam notu varsa) plan ve zamanlama gerekçesine "[mock] yeniden üretim" notu, metne `[mock]` işareti eklenir. `Location` başlığı mutlak `/campaigns/{id}` yoludur; nginx önekiyle uyumu T-034'te çözülür, arayüz gövdedeki id'yi kullanır.
- **Alternatifler:**
  - Saf kaynak + `PATCH` — reddedildi: durum makinesi kuralları PATCH gövdesine gizlenir, hata kodları belirsizleşir.
  - `Last-Event-ID` ile tekrar oynatma — reddedildi: bellekte tampon ve sıra numarası yönetimi; snapshot durumu zaten düzeltir.
  - Tek paylaşılan token — reddedildi: EventSource header gönderemediği için ek iş, demo için gereksiz.
  - Anahtar zorunlu veya anahtar yokken 503 — reddedildi: anahtarsız `docker compose up` demosu olmazdı.
  - Mock'ta eşleşmeyen brief'e hata — reddedildi: anahtarsız demo yalnız birkaç brief'le çalışırdı.
- **Sonuçlar / bedel:** Eylem uçları REST purizmine tam uymaz. Kopmada aradaki olaylar kaybolur, snapshot durumu düzeltir. Kimlik doğrulama olmadığı için üretimde kullanılamaz. Mock'un varsayılan yanıtı brief'le ilgisiz olabilir; bu arayüzde işaretlidir.
- **Doğrulama:** 2026-10-28'de (T-025, D-072): `internal/proposal/api` (%90,9): `TestErrorMapping`, `TestRequestValidation`, `TestInvalidPathIDs`, `TestEvents*`; `internal/proposal/boot`: `TestSelectLLM`, `TestSetupInMockMode`; `internal/proposal/llm/mock`: `TestEveryCannedFilterIsValid`, `TestEveryCannedTextPassesTheContentValidators`, `TestRegeneratedContentCarriesTheMarkerAndStaysValid`; integration `TestSampleBriefOverRESTAndSSE`, `TestRejectAndEditOverREST`, `TestConflictsArriveAsProblemJSON`, `TestAnUnknownBriefGetsTheMarkedMockAnswers`, `TestEveryMockFilterHasAnAudienceInTheSeed`, `TestRegenerationAfterARejectionIsMarkedByTheMock`. Canlı yığında (`LLM_MODE=mock`): `/readyz` `llm: mode=mock`, `POST /campaigns` 202 + `Location`, SSE önce `snapshot`, kitle 46 ve kapsama 20/13/13/3; subagent'ın çalıştırmasında REST ile tüm adımlar onaylandı (plan e-posta 33 / SMS 13, 3 metin yuvası, toplam maliyet 0.5530), `/docs` ve `/openapi.json` 11 ucu listeledi. Son mock değişikliğinden sonra imaj yeniden derlenemedi (Docker'dan `gcr.io`'ya DNS erişimi yoktu); o değişiklik yalnız testlerle doğrulandı. `make test` geçti.
- **Uygulama notu:** Bağlama `internal/proposal/boot`'tadır; `app.Deps.OnShutdown` flow'u HTTP sunucusundan sonra, havuzdan önce kapatır. Onay ve düzenleme 200, red, bağlam, yeniden açma ve yeniden üretim 202 döner. Problem `type` alanı `urn:coldbru:proposal:problem:<code>` URN'üdür. Liste yanıtı `{items, limit, offset}`'tir (toplam yok, varsayılan 20, en fazla 100). `content_reset` her deneme başında boş bir delta ile tetiklenir. Mock stub'daki 12 geçmiş kampanya brief'ini ve örnek brief'i normalize metinle eşler.

## D-073: DSL vs GraphQL JSON ölçümü: token sayımı canlı deneyde, make bench + bench-smoke, string içinde JSON kolu, pilot sonra tam; compose bench profili, llm_call maliyet raporu, LLM/DSL/SSE metrikleri, bayraklı pprof

- **Durum:** Kabul edildi · 2026-10-28
- **Alan:** Test ve kapsam
- **Bağlam:** Kullanıcı DSL yaklaşımının, modelin GraphQL filtresini doğrudan yazmasına karşı token maliyeti, gecikme/CPU (flamegraph), yük ve kalite açısından nasıl ölçüleceğini sordu. Araştırma `docs/research/06-dsl-benchmarking.md`'de. Örnek brief'te DSL 115 bayt, derlendiği GraphQL JSON'u 150 bayttır (yaklaşık 1,3 kat); D-007 ve araştırma 02'deki "yaklaşık 3 kat çıktı token'ı" bir tahmindi, ölçülmemişti. Claude için resmi çevrimdışı tokenizer yoktur; `count_tokens` ücretsizdir ama API anahtarı ister.
- **Karar:**
  - Token sayımı canlı deneylere bırakılır; şimdilik çevrimdışı token karşılaştırması yapılmaz. Canlı deneyde karşılaştırılan biçimler yalnız tool içindeki DSL ile sıkıştırılmış GraphQL JSON'udur; fiyat Sonnet 5.5'e göre hesaplanır.
  - Korpus `bench/corpus/*.dsl` dosyalarına taşınır (prompt ve doküman örnekleri, altın vakalar); testler ve benchmark'lar aynı kaynağı okur; dilbilgisine dayalı bir üretici ek çeşitlilik verir.
  - Go benchmark'ları (`dsl.Parse`, `catalog.Check`, `compile.Compile`, JSON doğrulama alternatifi, Customer SQL derleyicisi) ayrı `make bench` hedefindedir (`-count 10`, benchstat, `-cpuprofile`/`-memprofile`, `go tool pprof -http` flamegraph); `make bench-smoke` (`-benchtime 1x`) benchmark kodunun derlendiğini sınar. `make test`'e girmez.
  - Canlı karşılaştırmanın GraphQL kolunda model filtreyi strict bir tool'un string alanı içinde JSON olarak yazar (DSL koluyla simetrik). Bu kolun prompt'u `eval/prompts/` altındadır, üretim kilidine (D-059) girmez. Önce yaklaşık 2–3 $'lık pilot, sonra tam koşu (yaklaşık 40 brief × 5 tekrar × 2 kol, tahmini 10–40 $); her koşuyu kullanıcı elle başlatır (D-061 `live`).
  - Altyapı (kodlama kuralı 7, kullanıcı onayladı): compose'a yalnız `--profile bench` ile açılan bir profil: vegeta konteyneri, Customer için port override ve isteğe bağlı `pg_stat_statements`. Prometheus/Grafana yoktur; ölçüm `/metrics` farklarıyla yapılır. Varsayılan yığın değişmez.
  - Kampanya ve adım başına LLM maliyeti `llm_call`'dan hesaplanır (yeni sütun yok), fiyatlar yapılandırmadadır. Önce Sonnet 5.5 cache-read fiyatı çelişkisi (fiyat sayfası 0,10 $, yerel model belgesi 0,20 $) doğrulanır.
  - Yeni Prometheus metrikleri: `llm_call_duration_seconds` (0,5–60 sn kovalar), `llm_tokens_total`, `dsl_errors_total{code}`, `sse_clients`.
  - Profil: benchmark profilleri ve flamegraph; ayrıca `PPROF_ADDR` verilirse yalnız 127.0.0.1'de dinleyen `net/http/pprof` (chi router'a bağlanmaz, varsayılan kapalı). Sürekli profilleme yoktur.
- **Alternatifler:**
  - Çevrimdışı vekil + tek seferlik `count_tokens` önbelleği veya yalnız çevrimdışı vekil — reddedildi: kullanıcı token sayımını canlı deneylere bıraktı.
  - Tüm DSL ve JSON biçimleri — reddedildi: rapor uzar. Haiku veya Opus fiyatları — reddedildi.
  - Korpusu kopyalamak veya yalnız üretici — reddedildi: kopyalar kayar / gerçekçi olmayan ifadeler ağırlıkta.
  - Yalnız `make bench` veya smoke'u `make test`'e koymak — reddedildi.
  - Serbest `object` alanı veya iki kol — reddedildi: kollar eşit koşulda olmaz / maliyet yaklaşık %50 artar.
  - Prompt'u `internal/proposal/prompts/` altına koymak veya geçici dal — reddedildi: üretim kilidi kirlenir / deney tekrar üretilemez.
  - Doğrudan tam koşu veya canlı deney yok — reddedildi.
  - Compose'u değiştirmemek veya Prometheus + Grafana — reddedildi. Yalnız rapor script'i veya Analytics fiyat sütunu — reddedildi. Yalnız `llm_*` veya hiç metrik — reddedildi. Pyroscope/Parca — reddedildi.
- **Sonuçlar / bedel:** DSL ile JSON'un token farkı canlı deneyden önce yalnız bayt düzeyinde bilinir. Canlı deney para harcar ve elle başlatılır. Compose dosyası bir profil kadar büyür. Fiyat değişince yapılandırma güncellenir. Metriklerde etiket kardinalitesine dikkat edilmelidir. Seed'in 1000 kullanıcıyla SQL zamanlaması için anlamsız olduğu, seed ölçeği açık konusuna bağlıdır.
- **Doğrulama:** Doğrulanmadı — henüz uygulanmadı (araştırma: `docs/research/06-dsl-benchmarking.md`).

## D-074: Onay: alıcılar onay anında Customer'dan, saklanmaz; olay ve 500'lük RECIPIENTS batch'leri tek outbox transaction'ında; cancel ve confirm uçları

- **Durum:** Kabul edildi · 2026-10-28
- **Alan:** Gönderim
- **Bağlam:** D-010 alıcıların Proposal'dan kişi başına atanmış kanalla RECIPIENTS work-queue'suna gitmesini, D-006 ve D-054 yayının outbox ile yapılmasını, D-033 `drafting → scheduled` geçişini ve iptali, D-070 onayın zamanlama adımını kapatan `Approve` sonrasında `Confirmer` ile yapılmasını belirledi. Platform outbox'ı (`outbox.Write`) yalnız `evt.<servis>.<olay>.v<n>` konulu olay zarfı yazabiliyordu. Alıcı listesinin ne zaman alınacağı ve saklanıp saklanmayacağı, batch ve olay biçimi, iptal ve başarısız onayın yolu açıktı (T-026).
- **Karar:**
  - Platform outbox'ına konu verilebilen bir yazım eklenir (`WriteMessage(tx, subject, payload)`; tablo değişmez). Kampanyanın `scheduled` olması, `evt.proposal.confirmed.v1` ve tüm `recipients.<campaign_id>` batch'leri tek transaction'da yazılır; relay hepsini taşır.
  - Alıcı listesi zamanlama onaylanınca Customer'dan sayfa sayfa (1000) alınır ve onaylı plana göre her kişiye kanal atanır (D-010). Liste `proposal_db`'de ayrı bir tabloda saklanmaz; yalnız outbox mesajlarının payload'ında bulunur.
  - Batch: `{campaign_id, batch_no, batch_count, recipients[{customer_id, first_name, language, channel, contact}]}`, en fazla 500 alıcı; "iki kanal" modunda kişi kanal başına bir kez yer alır. Olay `data`'sı: `{campaign_id, schedules[{language, send_at}], messages[{language, channel, content}], cta_url, recipient_batches, recipient_counts}`; Notification metni olaydan çizer (D-071 `internal/content`).
  - `POST /campaigns/{id}/cancel`: `drafting`'te yalnız durum `cancelled` olur; `scheduled` veya `sending`'de ayrıca `evt.proposal.cancelled.v1` outbox ile yayınlanır (D-033). `POST /campaigns/{id}/confirm`: zamanlama onaylı ama kampanya hâlâ `drafting` ise onayı yeniden dener (D-070'teki yeniden deneme yolu). Onay anında alıcı sayısı 0 ise 409 döner.
  - Kullanıcı ayrıca onayladı: onay anında kitle değişmişse (`ago`, D-039) onaylı metni veya zamanlaması olmayan dil-kanal çiftine düşen alıcılar düşürülür, loglanır ve sayılara girmez. Olayda `send_at: null` Notification için "olayı işlediği anda hemen gönder" demektir. Proposal'ın havuzu compose'da `pool_max_conns=16`'dır (D-020: havuz ayarları URL'den).
  - Kullanıcı ayrıca onayladı: başarılı onayda SSE'ye `campaign_confirmed {recipient_counts, recipient_batches}` olayı gider (D-072 olay listesine ekleme). Taslak kampanya iptal edilince süren üretimin sonucu atılır ve yeni üretim başlamaz.
- **Alternatifler:**
  - Batch'leri commit'ten sonra doğrudan yayınlamak — reddedildi: çökmede olay gitmiş ama batch'ler eksik kalabilirdi.
  - `campaign_recipient` tablosu — reddedildi: `proposal_db`'de kişisel veri ve yeni şema.
  - 1000'lik batch ve yalnız id taşıyan olay — reddedildi: Notification metin için Proposal'a senkron bağımlı olurdu (D-006).
  - Yalnız cancel — reddedildi: başarısız onay zamanlamanın yeniden onaylanmasını gerektirirdi.
- **Sonuçlar / bedel:** Platform outbox API'si genişler. Outbox satırları silinmediği için (D-054) alıcıların ad ve iletişim bilgisi yayınlanmış payload'larda kalır. Önizleme ile onay arasında kitle değişebilir (`ago`, D-039). Olay içeriği büyüktür. İki uç daha eklenir.
- **Doğrulama:** 2026-10-28'de (T-026): `internal/proposal/confirm` integration testleri (gerçek Postgres, Customer handler'ı, NATS ve relay): örnek brief REST ile onaylanınca kampanya `scheduled`; EVENTS'te tek `evt.proposal.confirmed.v1` (33 e-posta / 13 SMS / 46, 3 metin, `utm_campaign`'li `cta_url`); RECIPIENTS'te 46 alıcılı tek batch, kanallar Customer'ın gerçek uygunluğuyla aynı; ikinci onay yazmaz; 5 eşzamanlı onay tek kez yazar; Customer 500 → 502, boş kitle → 409 `no_recipients`, ardından `POST /confirm` onaylar; zamanlanmış kampanyanın iptali `evt.proposal.cancelled.v1` yayınlar, taslak iptali hiçbir şey yayınlamaz. Outbox: ham batch satırının id'si `Nats-Msg-Id` ile aynıdır ve payload'a id eklenmez. Birim: `Assign` tüm modlar, `Batches` 0/1/499/500/501/1000/1001. Canlı yığında (`LLM_MODE=mock`) aynı akış: `scheduled`, EVENTS'te onay olayı, RECIPIENTS'te 46 alıcılı batch, `POST /cancel` sonrası `evt.proposal.cancelled.v1`. `make test` geçti.
- **Uygulama notu:** Ham mesaj ile olay zarfı konu ad alanından ayrılır: `WriteMessage` `evt.` ile başlayan konuyu reddeder, relay yalnız `evt.` satırlarının payload'ına id ekler; payload `jsonb` olduğu için normalize edilmiş JSON yayınlanır. Alıcılar transaction dışında çekilir; transaction içinde kampanya satırı kilitlenir ve onaylı adımlar yeniden okunur, araya bir yeniden açma girdiyse hiçbir şey yazılmadan 502 döner. Boş iptal gerekçesi "cancelled by the marketer" olur. Tamamlanmış veya iptal edilmiş kampanyanın iptali 409 `campaign_not_cancellable` döner. `messages[].content` saklı varyant JSON'unun kendisidir.

## D-075: Zamanlama prompt'u schedule@1.1.0: brief'teki zaman tüm dillere; konum bazlı zamanlama T-043'te, T-027'den önce

- **Durum:** Kabul edildi · 2026-10-28
- **Alan:** LLM entegrasyonu
- **Bağlam:** Gerçek modelle (Haiku 5.5) "… Yarın Sabah'a schedule la" brief'i `tr` için yarın 09:00 (İstanbul), `en` için `null` (hemen) üretti. Sebep `schedule@1.0.0`'ın 3. kuralıydı ("brief bir zamanı yalnız bir dil için belirtirse…"): model brief'in yazıldığı dili kitle dili saydı. Bu kural D-070'te onaylanan sorumluluklarda yoktu, prompt yazılırken eklenmişti. Ayrıca D-070'in dil → saat dilimi eşlemesi (tr → Europe/Istanbul, en → Europe/London) örnek brief'te yanlıştır: kitle tamamen İstanbul'dadır, `en` konuşanlar da orada yaşar. Kullanıcı zamanlamanın dil yerine konuma (bölge, saat dilimi) dayanmasını istedi: brief bölgeleri belirtiyorsa (ör. İstanbul'da x, İtalya'da y saatinde) ona göre, belirtmiyorsa konum bazlı gerekçeyle.
- **Karar:**
  - Ara çözüm `schedule@1.1.0`: brief'te veya notlarda geçen zaman kitlenin tüm dillerine uygulanır; brief'in yazıldığı dil bir kitle dili sayılmaz; yalnız brief açıkça bir kitle grubunu sınırlarsa (ör. "İngilizce konuşanlara akşam") o gruba uygulanır. Gerekçe her girdide brief'in dilindedir.
  - Konum bazlı zamanlama yeni görev T-043'tür ve T-027'den (notification_db) önce yapılır. Etki alanı: Customer GraphQL'e saat dilimi kırılımı ve `Recipient.timezone` (D-058 sözleşmesine ekleme), zamanlama yuvaları (D-034 CHECK ve olası migration, kodlama kuralı 7), schedule prompt'unun saat dilimlerini kitlenin konumundan alması, `evt.proposal.confirmed.v1` `schedules` ve batch alanları, Notification zamanlama konuları. D-009'un zamanlama kısmı ("alıcının yerel saati reddedildi") ve D-070'in dil → saat dilimi eşlemesi o görevde yeniden karara bağlanır; ayrıntılar orada sorulur.
- **Alternatifler:**
  - 3. kuralı kaldırmak — reddedildi: dil bazlı farklı saat isteyen brief'ler tek saate inerdi.
  - Prompt'u T-043'e kadar olduğu gibi bırakmak — reddedildi: aynı çelişki tekrarlanırdı.
  - Konum bazlı zamanlamayı Notification'dan sonra veya T-026 içinde yapmak — reddedildi: Notification'ın şeması sonradan değişirdi / T-026 büyürdü.
- **Sonuçlar / bedel:** Kısa ömürlü bir prompt sürümü ve önbelleğin bir kez soğuması. T-043'e kadar saat dilimi hâlâ dilden türetilir; İstanbul'daki `en` kitle Londra saatiyle zamanlanır. T-027 bir görev gecikir.
- **Doğrulama:** 2026-10-28'de: `schedule@1.1.0` (`internal/proposal/prompts/schedule/system.txt`), kilit yeniden üretildi; `TestPlanAndSchedulePackages` yeni 3. kuralı ve gerekçe kuralını arar, eski "only for one language" ifadesinin kalmadığını doğrular; `make test` geçti. Gerçek modelle aynı brief yeniden denenmedi (ücretli çağrı).

## D-076: Konum bazlı zamanlama: (dil, saat dilimi) yuvaları, dim_customer.timezone, GraphQL byLocale + Recipient.timezone, migration 00004, schedule@2.0.0, olayda timezone

- **Durum:** Kabul edildi · 2026-10-29
- **Alan:** LLM entegrasyonu
- **Bağlam:** D-075 zamanlamanın dil yerine konuma dayanmasını T-043'e bıraktı. D-009 dil başına ayrı gönderim zamanını seçmiş ve alıcının yerel saatini reddetmişti; D-070 saat dilimini dilden türetiyordu (tr → Europe/Istanbul, en → Europe/London); D-034 zamanlama varyantını dil başına tanımlıyordu. Seed'de her müşterinin `timezone` alanı doludur ve dil ile konum örtüşmez: Europe/Istanbul'da 557 `tr` + 53 `en`, Europe/London'da 377 `en` + 13 `tr` müşteri vardır. Kullanıcı brief bölge belirtiyorsa ona göre, belirtmiyorsa konum bazlı gerekçeyle zamanlama istedi.
- **Karar:**
  - Zamanlama varyantı kitlede alıcısı olan her (dil, saat dilimi) çifti için açılır. Metin yine dil × kanal başınadır.
  - Konum `dim_customer.timezone` (IANA) alanından gelir.
  - Customer GraphQL'e (D-058'e ekleme): `AudiencePreview.byLocale [{language, timezone, count, coverage, maxFirstNameLength}]` ve `Recipient.timezone`. `byLanguage` korunur.
  - Migration `00004` (kodlama kuralı 7, kullanıcı onayladı): `proposal_variant.timezone text NULL`; CHECK: `schedule` türünde dolu, diğer türlerde NULL; canlı yuva unique'i `(step_id, language, channel, timezone) NULLS NOT DISTINCT` olur.
  - `schedule@2.0.0` (çıktı şeması değiştiği için MAJOR, D-059): girdi kitledeki her (dil, saat dilimi) çifti, alıcı sayısı ve yerel "now"dur. Brief bir bölge, şehir veya ülke için zaman verirse yalnız o saat dilimindeki çiftlere uygulanır; genel bir zaman ("yarın sabah") her çiftte kendi yerel saatiyle aynı duvar saatidir; brief bir dil grubunu sınırlarsa yalnız o dilin çiftlerine uygulanır; zaman yoksa `null`. Kitlede olmayan bölgeler yok sayılır ve gerekçede belirtilir. Gerekçe brief'in dilindedir.
  - `evt.proposal.confirmed.v1` `schedules[{language, timezone, send_at}]` olur ve batch alıcısı `timezone` alır (v1 içinde alan eklemesi, D-035). Zamanlaması olmayan (dil, saat dilimi) çiftine düşen alıcı D-074'teki gibi düşürülür ve loglanır. Notification'ın zamanlama konuları kampanya × dil × saat dilimi olur (T-028).
  - Kullanıcı ayrıca onayladı: `00004` mevcut zamanlama satırlarını CHECK'ten önce eski D-070 eşlemesiyle doldurur (tr → Europe/Istanbul, en → Europe/London). Doğrulayıcı `send_at` ofsetinin o anda yuvanın saat dilimine uymasını ister (yaz saati dahil); yanlış ofset onarılacak bir sorun olarak modele geri verilir.
  - Kullanıcı ayrıca onayladı: `00004` ek olarak `proposal_variant_timezone_check (timezone ~ '\S')` kısıtını açar (D-068'deki boş olmayan metin kuralıyla tutarlı). D-076'dan önce kaydedilmiş, `by_locale` taşımayan bir kitle varyantıyla zamanlama adımına gelinirse üretim görünür bir hatayla ("kitle adımını yeniden aç") başarısız olur.
  - D-009'un dil başına zamanlama kısmı, D-070'in dil → saat dilimi eşlemesi ve D-034'ün zamanlama yuvası bu kararla yerini alır.
- **Alternatifler:**
  - Yalnız saat dilimi yuvası — reddedildi: dil grubuna özel zaman ifade edilemezdi. Ülke — reddedildi: çok saat dilimli ülkelerde yanlış.
  - `country_code` → saat dilimi tablosu — reddedildi: müşterinin kayıtlı saat dilimini yok sayardı.
  - Ayrı `byTimezone` — reddedildi: çift yuvalarının sayıları birleştirilemezdi.
  - Saat dilimini yalnız `content`'te tutmak — reddedildi: aynı dilde iki saat dilimi mevcut unique index'e takılırdı.
  - Genel zamanı brief dilinin saat diliminde tek an saymak — reddedildi: Londra'daki alıcıya sabah 7'de giderdi.
  - `evt.proposal.confirmed.v2` — reddedildi: henüz tüketici yok.
- **Sonuçlar / bedel:** Daha çok onay yuvası (seed'de en fazla 4). Bir migration ve index değişimi. Prompt'un MAJOR sürümü önbelleği bir kez soğutur. GraphQL'de iki alan ve SQL'de bir gruplama daha.
- **Doğrulama:** 2026-10-29'da (T-043): Customer `locale_integration_test.go` örnek brief'te `byLocale` toplamının 46 olduğunu ve doğrudan SQL `group by` ile aynı olduğunu, `totalOrders >= 1` için dört çifti, `Recipient.timezone`'un saklı sütunla aynı olduğunu doğrular. Store `TestMigration00004_BackfillAndRoundTrip` ve timezone kısıt vakaları. Flow `locale_integration_test.go`: örnek brief'te yalnız (tr, Europe/Istanbul) ve (en, Europe/Istanbul) yuvaları; "yarın sabah" ikisinde 09:00+03:00; `totalOrders >= 1` dört yuva; yanlış Londra ofseti aynı konuşmada onarılır; yaz saati geçişi birim testinde. Confirm `locale_integration_test.go`: olay `schedules` ve batch alıcıları `timezone` taşır, çift başına sayılar planla aynıdır. `schedule@2.0.0` kilidi yeniden üretildi. Canlı yığında `00004` uygulandı (`goose_db_version` 4, sütun, index ve iki CHECK var); servis gerçek model modunda olduğu için kampanya açılmadı. `make test` geçti.
- **Uygulama notu:** Customer tek SQL sorgusuyla (dil, saat dilimi) gruplar, `byLanguage` bu satırlardan toplanır. Yuva adı `tr@Europe/Istanbul` biçimindedir. Doğrulayıcı yalnız sayısal ofseti karşılaştırır (`Z`, +00:00 olan bölgede kabul). Saat dilimi verisi için `time/tzdata` gömülüdür. Prompt'a çift başına alıcı sayısı ve İstanbul/Londra ofset ipucu eklendi. Kanal planı içeriği `by_locale` almaz; çift başına alıcılar kitle ve plandan hesaplanır. Onayda düşürülen alıcılar (dil/saat dilimi/kanal) başına sayıyla loglanır.

## D-077: notification_db şeması: locale_schedule, message, delivery'de schedule_id/message_id FK, recipient_batch + batch_count, CHECK'ler ve iki indeks, kampanya bitince PII temizliği

- **Durum:** Kabul edildi · 2026-10-29
- **Alan:** Gönderim
- **Bağlam:** D-023 `delivery` için `UNIQUE(campaign_id, customer_id, channel)`, durum makinesini ve `Idempotency-Key`'i; D-074 onay olayını ve 500'lük batch'leri; D-076 zamanlamayı (dil, saat dilimi) çiftine bağladı. `docs/domain-model.md` §4 taslağı dil başına zamanlama içeriyordu ve kolonları kesin değildi. Olay EVENTS'ten, batch'ler RECIPIENTS'ten sırasız ve en az bir kez gelir. `outbox` ve `processed_event` platform setindedir (D-054). Kodlama kuralı 7 gereği şema kullanıcıyla kesinleşti (T-027).
- **Karar:**
  - `send_campaign(id = Proposal campaign.id, status scheduled | sending | completed | cancelled, cta_url, batch_count ≥ 1, created_at, updated_at, completed_at)`.
  - `locale_schedule(id, campaign_id FK, language, timezone, send_at NULL = hemen, status scheduled | fired | cancelled, fired_at)`, `UNIQUE(campaign_id, language, timezone)` (D-076).
  - `message(id, campaign_id FK, language, channel, content jsonb)`, `UNIQUE(campaign_id, language, channel)`.
  - `delivery`: `id` (UUIDv7, `Idempotency-Key`), `campaign_id`, `schedule_id` ve `message_id` FK'leri, `customer_id text`, `channel`, `first_name`, `contact`, `status`, `attempts`, `lease_until`, `provider_message_id`, `last_error`, `sent_at`, `updated_at`; `UNIQUE(campaign_id, customer_id, channel)` (D-023). Eşleşmeyen alıcı FK ile reddedilir.
  - `recipient_batch(campaign_id FK, batch_no ≥ 1, recipients, received_at)`, `PRIMARY KEY(campaign_id, batch_no)`. Batch tüketicisi batch satırını ve teslimatları tek transaction'da `ON CONFLICT DO NOTHING` ile yazar. Bir zamanlama ancak `recipient_batch` sayısı `batch_count`'a eşitse gönderime başlar; değilse tetiklenme iletisi, kampanya henüz yoksa batch NAK ile ertelenir.
  - Bütünlük: durumlar `text` + `CHECK`; `sent` ⇒ `sent_at`, `failed` ⇒ `last_error`, `sending` ⇒ `lease_until` dolu; `attempts ≥ 0`. İndeksler: `delivery (campaign_id, channel, status)` (dashboard sayımları, D-036) ve kısmi `delivery (schedule_id) WHERE status = 'pending'`. Ekleme-yalnız trigger yoktur. Lease süresi T-029'dadır.
  - Kapsam (kullanıcı onayladı): T-021 ile aynı; Notification'ın migration seti, açılışta `app.Options.Migrations` ile uygulanması, `sqlc.yaml` ile temel ekleme ve okuma sorguları, kısıtları sınayan entegrasyon testleri.
  - Kullanıcı ayrıca onayladı: D-068 desenindeki ek CHECK'ler (`completed` ⇒ `completed_at`, `fired` ⇒ `fired_at`, `cta_url` ve `timezone` boş olamaz); bileşik FK'ler: `locale_schedule UNIQUE (id, campaign_id)`, `message UNIQUE (id, campaign_id, channel)`, `delivery (schedule_id, campaign_id)` ve `(message_id, campaign_id, channel)` bunlara bağlanır, böylece teslimat başka kampanyanın zamanlaması veya metniyle ya da başka kanalın metniyle eşleşemez; kapsanmayan her FK sütununa indeks.
  - Kişisel veri: bekleyen teslimatta `first_name` ve `contact` zorunludur (CHECK); kampanya `completed` veya `cancelled` olduğu transaction'da bu iki alan `NULL` yapılır. Sayılar, durumlar, `customer_id` ve `provider_message_id` kalır.
- **Alternatifler:**
  - Teslimatın yalnız değer taşıması — reddedildi: metni veya zamanlaması olmayan alıcı sessizce hiç gönderilmezdi.
  - Batch geldikçe göndermek — reddedildi: "hemen" zamanlamasında yarım kitleye gönderim ve belirsiz tamamlanma.
  - Yalnız unique ve FK — reddedildi: durum kuralları yalnız kodda kalır, dashboard tabloyu tarar.
  - Kişisel veriyi süresiz tutmak — reddedildi: `notification_db` kalıcı bir kişisel veri deposu olurdu.
- **Sonuçlar / bedel:** Batch tüketicisi (dil, saat dilimi) → zamanlama ve (dil, kanal) → metin eşlemesini yapar. Bir tablo ve birkaç kısıt ile indeks eklenir. Tamamlanmış kampanyada alıcı yalnız `customer_id` ile görülür. Proposal'ın outbox payload'larındaki kişisel veri (D-074) bu kararla temizlenmez.
- **Doğrulama:** 2026-10-29'da (T-027): `internal/notification/store` integration testleri: `TestMigrations_Applied`, `TestMigrations_DownThenUp`, `TestConstraints_SendCampaign`, `_LocaleSchedule`, `_Message`, `_RecipientBatch`, `_Delivery` (her CHECK için red ve kabul; başka kampanyanın zamanlaması veya metni ve farklı kanallı metin bileşik FK'lerle reddedilir), `_NotNull`, `TestQueries` (12 sqlc sorgusu; tekrar gelen batch ve teslimat yok sayılır, ilk yazım kalır; teslimat beklerken kişisel veri temizliği reddedilir, bitince temizlenir). Canlı yığında notification `healthy`; `notification_db`'de beş servis tablosu, `outbox`, `processed_event` ve iki goose sürüm tablosu `notification` şemasında; `delivery` kısıt adları migration'la aynı. `make test` geçti.
- **Uygulama notu:** Migration'lar iki dosyadır (tablolar; `delivery`). `delivery.campaign_id` ayrıca tek sütunlu FK'dir. `delivery_schedule_id_idx` ve `delivery_message_id_idx` FK indeksleridir; `campaign_id` FK'leri unique/PK'nin öndeki sütunuyla kapsanır. `InsertRecipientBatch` ve `InsertDelivery` eklenen satır sayısını döner (0 = tekrar). `ListPendingDeliveries` `(schedule_id, limit)` alır ve `id`'ye göre sıralar. `send_campaign.updated_at` uygulamada güncellenir.

## D-078: Notification tüketicileri: outbox'a headers, sched.<id> → fire.<id>, iş anahtarıyla idempotent ham tüketici, tetiklenmede SEND kuyruğu, iptalde durum + PII temizliği

- **Durum:** Kabul edildi · 2026-10-29
- **Alan:** Gönderim
- **Bağlam:** D-006 ve D-024 zamanlamayı NATS SCHEDULES stream'ine (`AllowMsgSchedules`), gönderimi SEND work-queue'suna verdi; D-049 `sched.>` ve `fire.>` konularını açtı; D-074 onay olayını ve RECIPIENTS batch'lerini, D-076 (dil, saat dilimi) zamanlamasını, D-077 `notification_db`'yi belirledi. NATS zamanlanmış mesajları `Nats-Schedule` ve `Nats-Schedule-Target` header'larıyla yayınlanır; platform outbox'ı header taşımıyordu. Platform tüketicisi (`consumer.Handler`) yalnız `evt.*` zarflarını işliyordu; batch'ler ve tetiklenmeler ham mesajdır (T-028).
- **Karar:**
  - Platform migration `00003` (kodlama kuralı 7, kullanıcı onayladı): `outbox.headers jsonb NULL`. `outbox.WriteMessage` isteğe bağlı header alır, relay onları yayınlar. Onay olayının işlendiği transaction'da `send_campaign`, `locale_schedule`, `message` satırları ve zamanlama mesajları birlikte yazılır.
  - Zamanlama konusu `sched.<schedule_id>`, hedefi `fire.<schedule_id>`'dir (`locale_schedule.id`, UUIDv7). `send_at = NULL` ise zamanlama header'ı olmadan doğrudan `fire.<schedule_id>` yayınlanır.
  - Platform tüketicisine zarfsız mesajlar için bir işleyici eklenir: aynı transaction modeli, D-055 NAK geri çekilmesi, bozuk mesajda `Term`. Tekrar koruması iş anahtarından gelir: batch için `recipient_batch` PK (D-077), tetiklenme için zamanlama durumu `scheduled → fired`. Ham mesajlar `processed_event` kullanmaz; olay zarfları kullanmaya devam eder.
  - Kullanıcı ayrıca onayladı: bir batch'te zamanlaması veya metni olmayan alıcı atlanır, batch'in geri kalanı yazılır ve batch alınmış sayılır; atlanan sayı kampanya, batch ve (dil, saat dilimi, kanal) ile ERROR log'a yazılır.
  - Tetiklenme: kampanya veya zamanlama iptal edilmişse ileti düşürülür; batch'ler eksikse NAK ile ertelenir (D-077); değilse tek transaction'da zamanlama `fired`, kampanya `sending` olur ve her bekleyen teslimat için `send.<kanal> {delivery_id}` outbox ile yazılır; SEND'i T-029 tüketir.
  - `evt.proposal.cancelled.v1`: kampanya, zamanlamaları ve bekleyen teslimatları `cancelled` olur, kişisel veri temizlenir (D-077). Kuyruktaki `send.*` mesajları worker'da durum kontrolüyle düşer (T-029).
  - Kullanıcı ayrıca onayladı: bilinmeyen bir kampanyanın iptal olayı yok sayılmaz, `RetryLater` ile ertelenir; Proposal iptali yalnız onaydan sonra yayınladığı için (D-074) onay olayı stream'de öndedir, `MaxDeliver` sonunda ERROR ve `Term`. Zamanlama tüketicisi işlediği `fire.<id>` mesajını commit ve ack'ten sonra SCHEDULES'tan siler (`RetryLater`'da silmez); silme hatası WARN'dır. D-049'un SCHEDULES ayarı değişmez.
  - Kullanıcı ayrıca onayladı (bozuk girdi politikası): geçersiz onay olayı (boş `cta_url`, eksik zamanlama veya metin, desteklenmeyen dil veya kanal, tekrarlı yuva) ve `batch_no > batch_count` olan batch ERROR ile `Term` edilir, satır yazılmaz; iptal edilmiş veya tamamlanmış kampanyanın batch'i ack ile düşürülür; boş `customer_id` veya `contact` ya da bilinmeyen kanallı alıcı "invalid recipient" olarak atlanır; `send_at` bir sonraki tam saniyeye yukarı yuvarlanır.
- **Alternatifler:**
  - Commit sonrası doğrudan yayın ve uzlaştırıcı — reddedildi: `locale_schedule`'a `published_at` ve ayrı bir uzlaştırma işi gerekirdi.
  - DB yoklayıcı — reddedildi: D-006 ve D-024'teki SCHEDULES kararından sapardı.
  - `sched.<kampanya>.<dil>.<saat dilimi>` — reddedildi: saat dilimindeki `/` ve `+` kaçış ister.
  - `Nats-Msg-Id` ile `processed_event` — reddedildi: iş anahtarı zaten koruyorken ikinci kayıt.
  - Tetiklenmede doğrudan göndermek — reddedildi: D-024'teki SEND kuyruğu ve kanal başına limiter tasarımından sapardı.
- **Sonuçlar / bedel:** Platform tablosu ve API'si genişler. Konudan kampanya veya dil okunmaz, payload'dadır. Büyük kampanyada tetiklenme transaction'ı teslimat başına bir outbox satırı yazar.
- **Doğrulama:** 2026-10-29'da (T-028): `TestNATSScheduling` gerçek `nats:2.15-alpine` ve repodaki SCHEDULES tanımıyla `Nats-Schedule: @at …` + `Nats-Schedule-Target` header'lı mesajın zamanında `fire.<id>`'ye düştüğünü, hedefin aynı stream'de ve farklı bir konu olması gerektiğini doğrular. `internal/notification/intake` integration testleri: onay olayı satırları ve zamanlamaları yazar; `send_at = NULL` hemen tetiklenir ve teslimat başına bir `send.<kanal>` SEND'e düşer; zamanlanmış mesaj NATS ile tetiklenir; olaydan önce gelen batch ertelenip kabul edilir; tekrar gelen olay, batch ve tetiklenme etkisizdir; eksik batch tetiklenmeyi erteler; tetiklenmeden önce iptal SEND'e hiçbir şey yazmaz ve kişisel veriyi temizler; onaydan önce gelen iptal onay işlenince etki eder; işlenen `fire.<id>` mesajı stream'den silinir. Platform: outbox header testleri, `RawProcessor` ve `RetryLater` testleri. Canlı yığında platform migration `00003` iki veritabanında uygulandı, üç durable tüketici (`notification-events`, `notification-recipients`, `notification-schedules`) çalışıyor. `make test` geçti.
- **Uygulama notu:** Notification, Proposal'ın tel tiplerini import etmez, aynasını tutar; `TestWireMirrorsProposal` JSON etiketlerini karşılaştırır. Tüketiciler: `notification-events` (EVENTS, `evt.proposal.>`, ack wait 30 sn), `notification-recipients` (RECIPIENTS, 30 sn, en fazla 500 bekleyen ack), `notification-schedules` (SCHEDULES `fire.>`, 60 sn); hepsi `MaxDeliver` 20. SEND satırları tek `INSERT … SELECT` ile yazılır. Olay işlemcisi de `RetryLater` ve `Malformed` destekler. Kalan yarış: hemen tetiklenen bir zamanlama, erken gelen iptalin yeniden denemesinden önce çalışırsa gönderim başlar; iptal yalnız bekleyenleri iptal eder. T-029'da (D-079) çözüldü: iptalde kişisel veri temizliği `pending` ve `sending` satırları atlar, iptal hemen işlenir; worker iptal edilmiş kampanyanın teslimatını bitirirken kişisel veriyi temizler.

## D-079: Gönderim worker'ları: provider sözleşmesi, 5 deneme 2 sn ×2 1 dk tavan, 30 sn lease, x/time/rate 100/s burst 10, 100'lük izin kontrolü, gobreaker 5/30 sn/1, SMS'te ad kısaltma

- **Durum:** Kabul edildi · 2026-10-29
- **Alan:** Gönderim
- **Bağlam:** D-011 gönderim öncesi batch izin kontrolünü ve circuit breaker'ı, D-023 teslim durumlarını ve `Idempotency-Key`'i, D-024 kanal başına 100 msg/s limiter'ı, 429'da `Retry-After`'ı ve `NakWithDelay` + `MaxDeliver` 5'i, D-028 gönderimde yeniden ölçümü belirledi. D-078 tetiklenmede her bekleyen teslimat için `send.<kanal> {delivery_id}` yazar. Provider sözleşmesi, geri çekilme parametreleri, lease süresi, belirsiz sonuç, limiter'ın kapsamı, izin kontrolü batch'i, breaker kütüphanesi ve eşikleri, gönderimde 160'ı aşan SMS ve tamamlanmanın hangi görevde olduğu açıktı (T-029; açık konular: retry ayrıntıları, breaker, SMS aşımı).
- **Karar:**
  - Provider sözleşmesi: `POST /v1/sms {to, text}`, `POST /v1/email {to, subject, html, text}`, header `Idempotency-Key` = delivery id; başarı 202 `{message_id}`; 429 + `Retry-After`; 5xx geçici; 4xx (408 ve 429 hariç) kalıcı. T-029 bunu `httptest` sahtesiyle sınar, T-031 aynı sözleşmeyle mock'u yazar.
  - Yeniden deneme: ağ hatası, zaman aşımı, 5xx ve 429 yeniden denenir; kalıcı 4xx hemen `failed`. `NakWithDelay` 2 sn'den başlayıp ×2, %20 jitter, 1 dk tavan; `MaxDeliver` 5, sonra `failed` (`last_error` dolu). 429'da kanal `Retry-After` kadar tüm worker'lar için durur.
  - Lease: worker teslimatı `pending → sending` yapar (`lease_until` = şimdi + 30 sn); çöken worker'ın satırı lease dolunca yeniden alınır; yanıt kaybolursa aynı `Idempotency-Key` ile tekrar edilir, provider aynı yanıtı döner (mock anahtarı saklar, T-031).
  - Limiter: süreç içi `golang.org/x/time/rate`, kanal başına `rate.NewLimiter(100, 10)`; tek Notification replikası varsayılır (README).
  - İzin (D-011): worker SEND'den kanal başına en fazla 100 mesaj çeker ve bu teslimatların müşterileri için tek `consentStatus(ids, channel)` sorgusu atar (`CUSTOMER_TOKEN_NOTIFICATION`, `consent:read`, D-063); uygun olmayan `skipped_no_consent` olur, diğer kanala düşülmez.
  - Circuit breaker: `github.com/sony/gobreaker/v2`; ardışık 5 hatada açılır, 30 sn açık kalır, yarı açıkta 1 deneme çağrısı. Açıkken mesajlar gönderilmeden NAK ile ertelenir (fail-closed).
  - Gönderimde gerçek adla SMS 160 karakteri aşarsa ad metin sığacak kadar kısaltılır, sığmazsa ad kaldırılır; onaylanan metin korunur, kodlama ve segment gerçek metinle yeniden ölçülür.
  - Kullanıcı ayrıca onayladı: izin kontrolü yapılamazken (Customer hatası, breaker açık) mesaj son teslim hakkına ulaşırsa satır `failed` olmaz; aynı transaction'da teslimat için outbox'a yeni bir `send.<kanal>` yazılır ve eski mesaj ack'lenir, satır `pending` kalır ve Customer dönünce gönderilir (D-011). İptalde kişisel veri temizliği yalnız `pending` veya `sending` olmayan satırları temizler; bekleyenler aynı transaction'da önce `cancelled` olur, `sending` satırları worker bitirirken temizlenir.
  - Kullanıcı ayrıca onayladı (provider istemcisi): `Retry-After` en fazla 30 sn'ye kırpılır; 429'da `Retry-After` yoksa, bozuksa veya geçmişteyse 1 sn durulur; `message_id` taşımayan 2xx geçici sayılır; 3xx kalıcı hatadır ve yönlendirme izlenmez; istemci kendi içinde yeniden denemez, yeniden deneme NAK'tır.
  - Kapsam: T-029 teslimatları son duruma getirir, iptal edilmiş kampanyanın teslimatını bitirirken kişisel veriyi temizler (D-078 notu) ve kuyruktaki mesajı düşürür. Kampanyanın `completed` olması, kampanya düzeyinde kişisel veri temizliği ve `started`/`completed` olayları T-030'dadır.
- **Alternatifler:**
  - Önce T-031 — reddedildi: görev sırası değişirdi.
  - D-055 olay tüketicisi politikası (15 dk tavan, 20 deneme) — reddedildi: D-024'teki `MaxDeliver` 5'ten sapma, başarısız mesaj saatlerce beklerdi.
  - Belirsiz sonuçta `unknown` durumu — reddedildi: yeni durum ve mesaj kaybı.
  - Dağıtık limiter — reddedildi: demo için ağır.
  - Teslimat başına izin kontrolü — reddedildi: Customer'a mesaj başına çağrı. Elle yazılmış breaker — reddedildi: test edilecek ek kod.
  - SMS'i `failed` yapmak veya aşarak göndermek — reddedildi: alıcı mesaj almazdı / brief'teki 160 kuralı çiğnenirdi.
  - Tamamlanmayı T-029'a almak — reddedildi: T-029 büyürdü.
- **Sonuçlar / bedel:** Uzun bir provider kesintisinde mesajlar yaklaşık 30 sn içinde `failed` olur. Provider'ın idempotency'sine güvenilir. Çoklu replikada limit replika sayısıyla çarpılır. İzin kontrolü ile gönderim arasında en fazla bir batch süresi pencere kalır. Yeni bağımlılıklar: `golang.org/x/time`, `github.com/sony/gobreaker/v2`. Bazı alıcılarda ad kısaltılır. T-030'a kadar kampanya `sending`'de kalır.
- **Doğrulama:** 2026-10-29'da (T-029): `internal/notification/sender` integration testleri (gerçek Customer handler'ı ve seed'li Postgres, NATS, `httptest` provider): her teslimat bir kez ve teslimat id'si `Idempotency-Key` olarak gönderilir; 503, 503, 202 → `sent`, 3 deneme; 400 → hemen `failed`; sürekli 503 → 5 teslimden sonra `failed`; 429 + `Retry-After` kanalı durdurur; başlatmadan sonra izni geri çekilen alıcı `skipped_no_consent`, diğerleri gönderilir; Customer 5 teslimden uzun kapalıyken hiçbir şey gönderilmez, mesajlar yeniden kuyruğa alınır ve Customer dönünce her teslimat bir kez gönderilir; aynı teslimat için tekrar gelen mesaj tek provider çağrısı yapar; süresi dolan lease yeniden alınır; gönderim sürerken gelen iptal hemen işler ve `sending` satırının kişisel verisi worker bitirince temizlenir. Hız: 300 SMS ilk istekten sona 2,9 sn (~104/s), hiçbir 1 sn'lik pencerede 115'ten fazla istek yok. Birim: provider yanıt sınıfları ve `Retry-After`, breaker döngüsü, SMS'te ad kısaltma (160 sınırı, Türkçe karakter, emoji). `make test` geçti (41 Go paketi).
- **Uygulama notu:** Tüketiciler `notification-send-sms` ve `notification-send-email` (SEND, `MaxDeliver` 5, AckWait 45 sn, en fazla 300 bekleyen ack); kanal başına 1 çekici ve 16 gönderici; limiter beklemesi lease'ten önce yapılır. Sonlandırma sorguları `attempts` ile çitlenir, devralınmış bir lease hiçbir şeyi değiştirmez; kampanya satırı `FOR SHARE` ile okunur, böylece eşzamanlı bir iptal ile sonlandırma kişisel veriyi kaçırmaz. Ad sığmazsa SMS metni toplanır ("Merhaba , x" → "Merhaba, x"). Notification'ın havuzu compose'da `pool_max_conns=16`'dır. Yeni bağımlılık sürümleri: `github.com/sony/gobreaker/v2` v2.4.0, `golang.org/x/time` v0.16.0. Canlı yığında SEND tüketicileri çalışıyor; mock provider sözleşmeyi henüz uygulamadığı için (T-031) canlı gönderim denenmedi.

## D-080: Canlı durum ve olaylar: 2 sn tamamlanma denetleyicisi, started zamanlama başına / completed kampanya başına, Proposal tüketir, Notification'da 1 sn yoklamalı SSE

- **Durum:** Kabul edildi · 2026-10-29; completed olayının maliyet alanı yerini aldı → D-083 · 2026-10-29
- **Alan:** Takip ve altyapı
- **Bağlam:** D-033 kampanya durumunu `scheduled → sending → completed` olarak, Notification olaylarıyla güncellenecek biçimde; D-036 canlı gönderim durumunu Notification'dan SSE ile, kaynağı `delivery` sayımları olarak belirledi. D-077 kampanya bitince kişisel verinin temizlenmesini, D-079 tamamlanmanın T-030'da olmasını kararlaştırdı. Tamamlanmanın nasıl algılanacağı, `evt.notification.started/completed` içerikleri, Proposal'ın bunları tüketip tüketmeyeceği ve canlı SSE'nin mekanizması açıktı (T-030).
- **Karar:**
  - Tamamlanma: Notification'da her 2 sn'de bir (ve bir teslimat sonlandırıldıktan sonra) çalışan bir denetleyici, `sending` bir kampanyada tüm zamanlamalar `fired` ve hiç `pending`/`sending` teslimat yoksa tek transaction'da kampanyayı `completed` yapar (`completed_at`), kişisel veriyi temizler (D-077) ve `evt.notification.completed.v1`'i outbox ile yazar.
  - `evt.notification.started.v1 {campaign_id, schedule_id, language, timezone, started_at}` her tetiklenmede (D-076). `evt.notification.completed.v1 {campaign_id, completed_at, counts[{channel, status, count}], sms_segments, estimated_cost_gbp}`; segment ve maliyet onaylı metnin ölçümlerinden (D-071) hesaplanır, şema değişmez.
  - Proposal `evt.notification.>` için bir EVENTS tüketicisi açar (`processed_event`): ilk `started` kampanyayı `sending`, `completed` kampanyayı `completed` yapar; iptal edilmiş kampanyada olay yok sayılır. Flow hub'ına `campaign_status` olayı eklenir (SSE, D-072'ye ekleme).
  - Canlı durum: Notification'da `GET /campaigns/{id}/deliveries` (anlık sayımlar) ve `GET /campaigns/{id}/deliveries/events` (SSE: bağlanınca snapshot, sayılar değiştikçe yeni snapshot, 15 sn ping, kampanya bitince `completed` olayı ve kapanış). Kaynak her istemci için 1 sn'lik DB yoklamasıdır ve dashboard indeksini (D-077) kullanır. Kimlik doğrulama yoktur (D-072). Kullanıcı bu soruda tercih belirtmedi; önerilen seçenek uygulandı.
- **Alternatifler:**
  - Her teslimat sonlandırmasında tamamlanma — reddedildi: eşzamanlı son teslimatlarda kimse tamamlamayabilirdi.
  - Teslimat başına gerçek segment saklamak — reddedildi: `delivery.segments` için migration gerekirdi.
  - Proposal'ın tüketmemesi — reddedildi: D-033'teki `sending` ve `completed` geçişleri yapılmazdı.
  - LISTEN/NOTIFY trigger'ı — reddedildi: trigger ve 100/s'de bildirim fırtınası. Süreç içi hub — reddedildi: tek replika varsayımı.
- **Sonuçlar / bedel:** Tamamlanma en fazla yaklaşık 2 sn gecikir. Gerçek adla değişen segment sayısı olayda tahmindir. Proposal'a bir tüketici eklenir. İzleyen istemci başına saniyede bir sorgu.
- **Doğrulama:** 2026-10-29'da (T-030): `internal/notification/sender/completion_integration_test.go` (intake + sender + httptest provider): tetiklenmede EVENTS'e `evt.notification.started.v1`; tüm teslimatlar gönderilince kampanya son gönderimden yaklaşık 240 ms sonra `completed`, kişisel veri temizlenir, sayımlar, `sms_segments` ve maliyet doğru tek bir `completed` olayı yazılır; iptal edilmiş kampanya tamamlanmaz. `internal/notification/api` testleri: SSE önce snapshot, değişiklikte yeni snapshot, bitince `done` ve kapanış, bilinmeyen kampanya 404. `internal/proposal/progress` integration testleri: `started` sonra `completed` kampanyayı `sending` ve `completed` yapar, hub `campaign_status` olayları çıkar, tekrarlar etkisiz, iptal edilmiş kampanyada yok sayılır. Canlı yığında notification ve proposal `healthy`, `/openapi.json` iki yeni ucu listeler, rastgele id 404 problem+json, `proposal-notification-events` tüketicisi çalışıyor; kampanya açılmadı. `make test` geçti.
- **Uygulama notu:** Denetleyici 2 sn'de bir ve her sonlandırmadan sonra (en az 250 ms arayla) çalışır. REST/SSE gövdesindeki `updated_at` sayımların okunduğu andır; `status` Notification'ın `send_campaign.status`'üdür. Zaten bitmiş kampanyaya bağlanan istemci hemen `snapshot` + `done` alır. Proposal'da hub olayları commit'ten sonra yayınlanır. `started` her tetiklenmede gelir, Proposal ikinci ve sonrakileri yok sayar. nginx'te `/campaigns/{id}/deliveries*` yolunun Notification'a yönlendirilmesi T-034'tedir.

## D-081: Mock provider: Go cephe D-079 sözleşmesini uygular, e-postayı Mailpit'e, SMS'i MockServer'a iletir; %5 5xx, %2 429, %1 zaman aşımı, 50–300 ms; kanal başına 110/s; bellekte 24 sa idempotency; receipt yok

- **Durum:** Kabul edildi · 2026-10-29
- **Alan:** Gönderim
- **Bağlam:** Brief gerçek bir servisi taklit eden, zaman zaman hata, gecikme ve 429 döndüren bir mock provider istiyor. D-018 mock'u ayrı bir servis olarak, D-079 provider sözleşmesini (`POST /v1/sms`, `POST /v1/email`, `Idempotency-Key`, 202 `{message_id}`, 429 + `Retry-After`) belirledi; D-079 yeniden denemede provider'ın aynı anahtara aynı yanıtı vermesine güvenir. Kullanıcı SMS için MockServer'ı, e-posta için Mailpit'i önerdi. Mailpit bir SMTP sunucusudur, idempotency ve 429 kavramı yoktur; MockServer durum tutmaz, 429'u yalnız senaryoyla verebilir. Hata oranları açık konuydu (T-031).
- **Karar:**
  - D-079 sözleşmesini mevcut Go `mockprovider` servisi uygular. Kabul ettiği e-postayı SMTP ile Mailpit'e, SMS'i HTTP ile MockServer'a iletir; gönderilen mesajlar bu iki aracın arayüzlerinden görülür. Notification ve T-029 değişmez.
  - Hata ve gecikme: istek başına %5 5xx, %2 kendiliğinden 429 (`Retry-After`), %1 zaman aşımı (mesaj kabul edilip iletilir, yanıt 12 sn geciktirilir; "kabul edildi ama yanıt kayboldu" durumu), 50–300 ms gecikme. Oranlar ortam değişkenleriyle ayarlanır; `MOCK_SEED` verilirse rastgelelik deterministiktir.
  - Mock'un kendi hız sınırı kanal başına 110/s'dir; aşımda 429 + `Retry-After: 1`. Böylece Notification'ın 100/s limiter'ının altında kalındığı kanıtlanır (araştırma 01 §7).
  - Idempotency deposu bellekte, 24 sa TTL'lidir: aynı anahtar ilk yanıtı ve aynı `message_id`'yi döner, Mailpit veya MockServer'a ikinci kez iletmez; aynı anahtar farklı gövdeyle gelirse 422.
  - Teslim raporu (receipt, webhook) yoktur; 202 `sent` demektir. README'de kapsam dışı olarak belirtilir.
- **Alternatifler:**
  - Doğrudan MockServer + Mailpit — reddedildi: idempotency kalkardı (D-023, D-079; zaman aşımında çift gönderim), 429 gerçek hız sınırını sınamazdı, e-posta istemcisi SMTP'ye dönerdi.
  - Yalnız Go mock — reddedildi: gönderilenleri görecek arayüz olmazdı.
  - Daha sert oranlar (%15 5xx, %5 429) — reddedildi: başarısız teslimatlar belirgin artardı. Varsayılanda hatasız — reddedildi: brief'teki hata, gecikme ve 429 varsayılanda görünmezdi.
  - Mock'ta hız sınırı olmaması — reddedildi: limiter'ın doğruluğu sınanmazdı.
  - Diskte idempotency deposu — reddedildi: yeni bağımlılık veya volume.
  - Gecikmeli webhook — reddedildi: yeni uç, durumlar ve Notification şeması değişirdi.
- **Sonuçlar / bedel:** Compose'a iki konteyner (Mailpit, MockServer) eklenir. Oranlar ölçülmüş değil, seçilmiş değerlerdir. Mock yeniden başlarsa idempotency deposu boşalır. Gerçek teslim durumu yoktur.
- **Doğrulama:** 2026-10-29'da (T-031): `internal/mockprovider` birim testleri (aynı anahtar ve gövdeyle tekrar aynı `message_id` ve tek iletim; farklı gövdeyle 422; sahte saatle TTL; 110/s'de 429 + `Retry-After: 1`; sabit tohumla hata sayıları; zaman aşımında yanıt beklerken tekrar saklanmış 202'yi alır; 25 eşzamanlı istek tek iletim; Türkçe konulu MIME); sözleşme testi: Notification'ın gerçek `provider.Client`'ı her yanıt sınıfını alır; integration `TestInboxesReceiveWhatTheMockAccepts` gerçek Mailpit ve MockServer'la. İlk canlı uçtan uca gönderim (Proposal geçici olarak mock LLM modunda, sonra eski moduna döndürüldü): örnek brief kampanyası `completed`, 48 teslimat (e-posta 35, SMS 13 `sent`, 0 `failed`; compose veritabanı uzun süredir açık olduğu için kitle 48'di, D-017 kayması); mock metrikleri e-postada 1 enjekte 5xx, 3 hız sınırı 429'u, 1 zaman aşımı ve onun 1 tekrarı, SMS'te 1 enjekte 5xx — hepsi yeniden denenip gönderildi; Mailpit'te 35 e-posta, MockServer'da 13 SMS. `make test` geçti.
- **Uygulama notu:** İmajlar `axllent/mailpit:v1.31.5` (GHSA-FPXJ-M5Q8-FPHW 1.30.0'da kapandı) ve `mockserver/mockserver:8.0.0`; ikisi de yayından en az iki hafta sonra seçildi. Mailpit arayüzü `${MAILPIT_UI_HOST_PORT:-8025}`, MockServer `${MOCKSERVER_HOST_PORT:-1080}`. Idempotency anahtarları kanal başınadır; gövde karşılaştırması kanonik JSON'un sha256'sıyla yapılır; kayıtlı anahtar doğrulama, hız sınırı, hata ve gecikmeden önce tekrar oynatılır. Anahtar başına bir kilit eşzamanlı tekrarları bekletir. 400 geçersiz anahtar veya JSON, 413 1 MiB üstü, 422 alan doğrulaması veya farklı gövdeli anahtar. Hız sınırı 429'larının bir kısmı Notification'ın 16 göndericisinin burst hizalanmasından gelir; yeniden deneme bunları kapatır.

## D-082: Tıklama takibi (T-044): imzalı token ve Analytics yönlendirme ucu + nginx yedeği, tekil CTR, SMS'e kodla kısa link

- **Durum:** Kabul edildi · 2026-10-29
- **Alan:** Takip ve altyapı
- **Bağlam:** Kullanıcı T-032'yi başlatırken başarı metriği olarak tıklanma oranının (CTR) izlenmesini önerdi; örnek olarak linkin bir proxy'de aynalanıp hem asıl adrese hem bir izleyiciye gitmesini (nginx mirror) verdi. D-071 CTA URL'sini yapılandırmadan alıyor, HTML'i gönderimde Notification çiziyor ve SMS'te URL'yi yasaklıyordu. Kullanıcı tıklama takibini T-032'den ayrı, yeni bir görev (T-044) olarak istedi.
- **Karar:**
  - E-postadaki CTA linki `…/r/<token>` olur. Token HMAC ile imzalanır ve `delivery_id` ile `campaign_id` taşır. Analytics tıklamayı yazar ve 302 ile kampanyanın `cta_url`'ine (`utm`'li) yönlendirir; hedef linkten değil kampanyadan gelir, açık yönlendirme yoktur. Analytics hata verirse nginx `error_page` ile kampanyanın varsayılan sayfasına yönlendirir (T-034).
  - CTR = tekil tıklayan / gönderilen. Teslimat başına ilk tıklama tekil sayılır, sonrakiler toplam tıklamaya eklenir. Rapor dil ve saat dilimi kırılımı verir. Bot ve önizleme istekleri basit bir User-Agent kuralıyla ayıklanır.
  - SMS'e kod kısa bir izleme linki ekler (yaklaşık 25 karakter). Modelin yazdığı metinde URL yasağı sürer; SMS metin bütçesi link uzunluğu kadar küçülür ve `content` prompt'u buna göre güncellenir. SMS CTR'si de raporlanır.
  - Uygulama T-044'tedir; T-032 Analytics'i tıklamasız kurar.
- **Alternatifler:**
  - Tıklamayı T-032'ye almak — reddedildi: kullanıcı ayrı görev istedi.
  - Yalnız nginx mirror — reddedildi: hedef URL linkte taşınmalı, imza nginx'te doğrulanamaz, açık yönlendirme riski. Yalnız Analytics ucu — reddedildi: Analytics kapalıyken link çalışmazdı.
  - Yalnız toplam tıklama — reddedildi: tekil CTR hesaplanamazdı.
  - SMS'te takip olmaması — reddedildi: SMS kampanyalarının başarısı ölçülmezdi.
- **Sonuçlar / bedel:** Token teslimat id'sini taşır (takma adlı kişisel veri). İki yönlendirme yolu ve T-034'te ek nginx ayarı. D-071'in SMS kuralı değişir: SMS metin bütçesi küçülür, `content` prompt'unun sürümü artar, UCS-2'de segment sayısı artabilir.
- **Doğrulama:** Doğrulanmadı — henüz uygulanmadı (T-044).

## D-083: Analytics: normalize analytics_db, llm_call başına token_completed, model başına yapılandırılabilir fiyat, özet/liste/prompt sürümü raporları

- **Durum:** Kabul edildi · 2026-10-29
- **Alan:** Takip ve altyapı
- **Bağlam:** D-018 Analytics'i, D-036 kampanya özetlerinin ve token/maliyetin orada tutulmasını, D-060 `evt.proposal.token_completed.v1`'e prompt sürümünü ve Analytics `llm_usage`'a `prompt_version` sütununu, D-080 `evt.notification.completed.v1`'in içeriğini belirledi. Proposal token olayını henüz yayınlamıyordu. Şema, olayın sıklığı ve içeriği, fiyatlar ve rapor uçları açıktı (T-032; kodlama kuralı 7). D-073 Sonnet 5.5 cache okuma fiyatında çelişki not etmişti.
- **Karar:**
  - `analytics_db` (normalize): `campaign_summary(campaign_id PK, completed_at, sms_segments, delivery_cost_usd)`, `campaign_delivery_count(campaign_id, channel, status, count)` PK `(campaign_id, channel, status)`, `llm_usage(llm_call_id PK, campaign_id, purpose, model, prompt_name, prompt_version, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, outcome, latency_ms, cost_usd, created_at)`; indeksler `llm_usage (campaign_id)` ve `llm_usage (prompt_name, prompt_version)`; platform seti (`processed_event`).
  - Proposal her `llm_call` satırıyla aynı transaction'da outbox'a `evt.proposal.token_completed.v1 {llm_call_id, campaign_id, step_id, purpose, model, prompt_name, prompt_version, tokens{input, output, cache_read, cache_creation}, outcome, latency_ms, created_at}` yazar. Analytics `llm_call_id` ile idempotenttir; başarısız denemelerin maliyeti de görünür.
  - Maliyet Analytics'te, model başına yapılandırılabilir fiyatla hesaplanır. Varsayılanlar MTok başına USD: Sonnet 5.5 giriş 2, çıktı 10, cache yazma 2.5, cache okuma 0.20; Haiku 5.5 giriş 0.10, çıktı 0.50, yazma 0.125, okuma 0.01; mock 0. Sonnet cache okuma için yerel belge (0.20) ile fiyat sayfası (0.10) çelişir; yüksek olan varsayılandır ve ortam değişkeniyle değişir.
  - Rapor API'si: `GET /campaigns` (tamamlanmış kampanya özetleri, sayfalı), `GET /campaigns/{id}/summary` (teslimat sayıları, SMS segmenti, teslimat ve LLM maliyeti, amaç başına token), `GET /llm-usage?group=prompt_version` (sürüm başına çağrı, onarım oranı, ortalama gecikme, maliyet; D-060). Hatalar problem+json, belgeler swag.
  - Kullanıcı ayrıca onayladı: maliyet sütunları `numeric(14,6)` (toplamlar) ve `numeric(14,8)` (çağrı başına); durum ve sayı CHECK'leri; tablolar arasında FK yoktur, çünkü token olayı kampanya tamamlanmadan gelir. Kampanyanın LLM maliyeti saklanmaz, raporda `SUM(llm_usage.cost_usd)` ile hesaplanır. Onarım oranı = `validation_failed / çağrı` (prompt sürümü başına).
  - Kullanıcı ayrıca onayladı: bozuk olaylar (geçersiz uuid, negatif sayı, bilinmeyen durum, biçimsiz maliyet) D-078'deki gibi ERROR ile `Term` edilir. Ad değişikliğinden önce eski `estimated_cost_gbp` alanıyla yayınlanmış geliştirme olayının düşürülmesi kabul edildi.
  - Para birimi (kullanıcı onayladı): tüm maliyetler USD'dir; fiyatlar şimdilik keyfidir. Teslimat fiyatları da USD'ye geçer: D-071'deki `SMS_SEGMENT_PRICE_GBP`/`EMAIL_PRICE_GBP` → `SMS_SEGMENT_PRICE_USD`/`EMAIL_PRICE_USD`, varyant içeriğindeki `estimated_cost_gbp` → `estimated_cost_usd`, Proposal maliyet özeti `*_usd`, D-080 `evt.notification.completed.v1` alanı `estimated_cost_usd` (henüz dış tüketici olmadığı için v1 içinde ad değişikliği). Müşteri verisindeki GBP alanları (`totalSpendGbp` vb.) değişmez.
- **Alternatifler:**
  - JSON belge — reddedildi: kanal/durum kırılımı ve sürüm başına maliyet sorguları zorlaşırdı.
  - Kampanya onayında toplu token olayı — reddedildi: iptal edilen veya onaylanmayan kampanyaların maliyeti ve sürüm kırılımı kaybolurdu.
  - Sonnet cache okumada 0.10 — reddedildi: yerel belge doğruysa maliyet düşük görünürdü.
  - Yalnız kampanya özeti — reddedildi: prompt sürümü karşılaştırması yalnız SQL ile yapılabilirdi.
- **Sonuçlar / bedel:** Üç tablo ve iki indeks. Çağrı başına bir olay. Fiyat değişince yapılandırma güncellenir. Para birimi değişikliği D-071 ve D-080'deki ad ve alanları değiştirir; geliştirme veritabanlarındaki eski varyantların `estimated_cost_gbp` alanı yeni adla okunmaz (maliyet 0 görünür).
- **Doğrulama:** 2026-10-29'da (T-032): `internal/analytics/store` (migration up/down/up, her CHECK, PK, upsert); `internal/analytics/intake` NATS + Postgres ile (completed ve yerine geçen tekrarı, aynı olay id'si etkisiz, token maliyeti, tekrar gelen `llm_call_id` ilk satırı korur, bilinmeyen model WARN, başka tip ve sürümler yok sayılır, bozuk olay `Term` ve geri alma, DeliverAll akışın başından başlar); `internal/analytics/api` üç rapor ucu gerçek HTTP ile (sayfalama, tamamlanmamış kampanya, onarım oranı); `internal/analytics/pricing` (cache token'ları, yarıya yukarı yuvarlama, `LLM_PRICES` geçersiz kılma). Proposal: `token_event_integration_test.go` her `llm_call` satırı için alan alan tek bir `token_completed` olayı doğrular. Canlı yığında analytics `healthy`, `analytics-events` tüketicisi çalışıyor, uçlar yanıt veriyor; kampanya açılmadı. `make test` geçti (50 Go paketi).
- **Uygulama notu:** Fiyatlar tek bir `LLM_PRICES` JSON ortam değişkeniyle model ve alan başına geçersiz kılınır; maliyet `big.Rat` ile hesaplanır, çağrı başına 8 ondalığa yuvarlanır; bilinmeyen model 0 ve WARN. Liste `{items, limit, offset}`, `completed_at DESC` sıralı. `/llm-usage` sürümleri metin olarak sıralar (`1.10.0` `1.2.0`'dan önce). Problem tipleri `urn:coldbru:analytics:problem:<code>`.

## D-084: Web API istemcisi: servis başına /api/<servis> öneki, commit edilen spec'lerden üretilen ve commit edilen tipler, EventSource + tipli olaylar, problem+json ApiError

- **Durum:** Kabul edildi · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** D-016 TS tiplerinin ve istemcinin openapi-typescript ve openapi-fetch ile spec'ten üretilmesini, D-012 global store olmamasını, D-013 ve D-072 SSE'yi belirledi. Proposal ile Notification'ın ikisi de `/campaigns/{id}/…` yollarını kullanıyor; yol öneki, tiplerin nasıl üretilip tutulacağı, SSE tüketimi ve hata biçimi açıktı (T-033). Spec'ler OpenAPI 3.1'dir.
- **Karar:**
  - Tarayıcı servislere `/api/proposal`, `/api/notification`, `/api/analytics` önekleriyle ulaşır. nginx (T-034) öneki sıyırıp servise iletir; geliştirmede Vite proxy aynısını yapar. Backend yolları değişmez. İstemci servis başına bir openapi-fetch örneğidir.
  - `pnpm -C web generate:api` commit edilen `cmd/<servis>/apidocs/swagger.json` dosyalarından `web/src/api/<servis>/schema.d.ts` üretir (openapi-typescript). `make generate`'e eklenir, üretilen dosya commit edilir (D-051 deseni); bir test tiplerin spec'le güncel olduğunu doğrular.
  - SSE: `useEventStream(url, handlers)` composable'ı EventSource açar, olay adlarını OpenAPI'deki olay kataloğu tiplerine eşler, kopunca tarayıcının kendi yeniden bağlanmasını kullanır (sunucu yeni snapshot verir, D-072) ve bileşen kalkınca kapatır.
  - Hatalar: problem+json'dan kurulan tipli bir `ApiError` (status, code, detail, problems) ve composable'larda kullanılan bir `unwrap()` yardımcısı; ağ hatası ayrı bir türdür. Global store yoktur (D-012).
- **Alternatifler:**
  - Tek `/api` ve benzersizleşen backend yolları — reddedildi: Notification ve Analytics API'leri değişirdi.
  - Çalışan servislerden spec çekmek — reddedildi: üretim yığının ayakta olmasına bağlı olurdu.
  - fetch + ReadableStream — reddedildi: yeniden bağlanma ve SSE ayrıştırma elle yazılırdı.
  - openapi-fetch'i sarmalamadan kullanmak — reddedildi: her çağıran hata biçimini kendisi ele alırdı.
- **Sonuçlar / bedel:** Arayüz hangi servisin hangi ucu sunduğunu bilir. Go ve TS değişikliği birlikte commit edilmelidir. EventSource header gönderemez; kimlik doğrulama olmadığı için sorun değildir (D-072).
- **Doğrulama:** 2026-10-30'da (T-033): `web/src/api/__tests__` (Vitest, 38 test): `ApiError`/`unwrap` problem+json, düz metin 500, ağ hatası; istemci çağrısı `/api/proposal/campaigns`'e gider; sahte EventSource ile `useEventStream` (tipli olay, yeniden bağlanma, URL değişimi, kaldırılınca kapanma); `schema-freshness.spec.ts` üretilen tiplerin spec'le bayt bayt aynı olduğunu doğrular; `events.spec.ts` olay adlarını tip düzeyinde sabitler. Vite proxy canlı yığına karşı denendi: `/api/<servis>/healthz` 200, önek sıyrılıyor, SSE tamponsuz (snapshot hemen, ping 15 sn'de). `make generate` ikinci çalıştırmada fark bırakmadı.
- **Uygulama notu:** `openapi-fetch` 0.17.0 ve `openapi-typescript` 7.13.0 tam sürümle sabitlendi. Olay haritası üretilen `api.EventCatalog` tipinden doğrudan gelir. Üretilen şemalar eslint, oxlint ve prettier dışındadır. `unwrap` başarıyı `response.ok`'tan okur; yalnız `TypeError` `NetworkError` olur. Notification akışı `done`'dan sonra sunucu tarafından kapanır; tarayıcı yeniden bağlanmasın diye işleyici `close()` çağırmalıdır. `VITE_API_BASE` isteğe bağlı bir önektir; Vite proxy hedefleri sabittir.

## D-085: nginx gateway: web/Dockerfile (node 24 build → nginx stable-alpine), host 8080, /api/<servis> öneki, proxy_redirect, /api altında tamponsuz ve 1 sa okuma; /r/ T-044'te

- **Durum:** Kabul edildi · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** D-015 arayüzü sunan ve `/api/*`'yi servislere yönlendiren tek bir çok aşamalı nginx konteynerini, D-043 Customer'ın dışarı açılmamasını, D-084 `/api/proposal`, `/api/notification`, `/api/analytics` öneklerini, D-072 Proposal'ın `Location: /campaigns/{id}` başlığını gateway'de çözmeyi, D-082 tıklama yedeğini nginx'e verdi. İmaj, port, `Location` düzeltmesi, SSE ayarları ve `/r/` yolunun zamanı açıktı (T-034).
- **Karar:**
  - `web/Dockerfile`: `node:24-alpine` (corepack ile pnpm) aşamasında `pnpm build`, çıktı sabitlenmiş bir `nginx:<stable>-alpine` imajına kopyalanır. Compose'da `web` servisi `${WEB_HOST_PORT:-8080}:80` ve proposal, notification, analytics'e `service_healthy` ile bağlıdır. SPA için `try_files … /index.html`. Servislerin kendi host portları geliştirme için kalır; Customer dışarı açılmaz.
  - nginx her `/api/<servis>/` önekini sıyırıp ilgili servise iletir; Proposal'ın `Location` başlığı `proxy_redirect` ile `/api/proposal/…` olur.
  - `/api/` konumlarında `proxy_http_version 1.1`, `proxy_buffering off`, `proxy_cache off`, `proxy_read_timeout 1h` ve boş `Connection` başlığı; `text/event-stream` için gzip kapalıdır.
  - D-082'deki `/r/` yönlendirmesi ve nginx yedeği T-044'te, uçla birlikte kurulur.
- **Alternatifler:**
  - Servis host portlarını kapatmak — reddedildi: Scalar `/docs` ve doğrudan servis denemesi kaybolurdu.
  - Backend'in `X-Forwarded-Prefix` okuması — reddedildi: Go kodu ve testleri değişirdi.
  - Yalnız `/events` yollarında tamponsuz — reddedildi: yeni bir SSE yolu eklenince kural unutulabilirdi.
  - `/r/` için şimdiden yer tutucu — reddedildi: test edilemeyen kural.
- **Sonuçlar / bedel:** İlk build'de node imajı indirilir. Yeni bir `Location` biçimi eklenirse `proxy_redirect` kuralı da eklenmelidir. Normal JSON yanıtları da tamponsuz geçer. T-044 nginx dosyasına tekrar dokunur.
- **Doğrulama:** 2026-10-30'da (T-034): canlı yığında `web` `healthy`; `localhost:8080/` ve `/campaigns/x` index.html döner (no-cache), hash'li asset'ler `immutable`; `/api/{proposal,notification,analytics}/healthz` 200, `/api/customer/healthz` nginx 404; 404 problem+json geçer; Proposal SSE'de snapshot hemen, ping 15 sn sonra (tamponsuz, gzip yok); Notification SSE'de snapshot + done. `Location` düzeltmesi, Proposal gerçek model modunda olduğu için kampanya açılmadan, stub bir upstream ile doğrulandı: `Location: /api/proposal/campaigns/abc-123`, upstream öneksiz yolu ve sorgu dizesini aldı. `docker compose restart proposal` sonrası nginx yeniden yüklemeden 2 sn'de toparlandı. `make test` geçti.
- **Uygulama notu:** İmajlar `node:24.22.0-alpine` ve `nginx:1.30.5-alpine`; pnpm `12.8.1` Dockerfile'da corepack ile sabitlenir (`package.json`'da `packageManager` yok). Yapılandırma `web/nginx.conf`; Docker DNS'i `resolver 127.0.0.11` ve değişkenli `proxy_pass` ile, önek `rewrite` ile sıyrılır. `absolute_redirect off` (yoksa nginx `Location`'a host portsuz mutlak URL yazıyordu). Bilinmeyen `/api/` yolları SPA'ya düşmez, nginx HTML 404'ü döner. `X-Request-Id` çağıranınki yoksa nginx'in `$request_id`'si. İmaj build'i `pnpm build` (tip denetimi dahil) çalıştırır.

## D-086: Web renk paleti logo.png'den: beyaz zemin, karamel primary, krem yalnız hover/aktif, adaçayı ve espresso destek; shadcn token'ları + Tailwind marka renkleri

- **Durum:** Kabul edildi · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** shadcn-vue kurulumu (D-012) nötr gri temayla geldi. Kullanıcı arayüz renklerinin `web/public/logo.png`'deki kahverengi, beyaz, siyah ile `#F7F1DE`, `#B0BA99`, `#9D6638`, `#4E220F` destek renklerinden alınmasını, uygulamanın büyük bölümünde dinlenme durumundaki zeminin beyaz kalmasını istedi.
- **Karar:**
  - `web/src/assets/main.css` `:root`'ta marka değişkenleri: `--brand-ink #111111`, `--brand-espresso #4e220f`, `--brand-caramel #9d6638`, `--brand-sage #b0ba99`, `--brand-cream #f7f1de`; `@theme`'de `ink`, `espresso`, `caramel`, `sage`, `cream` Tailwind renkleri olarak açılır.
  - `background`, `card`, `popover`, `sidebar`, `secondary` beyaz; metin `ink`. `primary` ve `ring` karamel (üstünde beyaz metin). `accent` ve `sidebar-accent` krem + espresso metin, yani krem yalnız hover/aktif/seçili durumlarda görünür. `muted` `#faf7ee`, `muted-foreground` `#6b5d52`, `border`/`input` `#e9e2d0`. Grafikler karamel, adaçayı, espresso, `#d8c9a3`, ink. `destructive` shadcn kırmızısı kalır.
  - `.dark` aynı paletin koyu karşılığıdır (zemin `#17100c`, metin krem, primary `#c08a5a`); şu an tema anahtarı yok.
- **Alternatifler:**
  - Krem zemin — reddedildi: kullanıcı dinlenme durumunda beyaz zemin istedi.
  - Adaçayını odak halkası yapmak — reddedildi: `outline-ring/50` ile beyaz üstünde zor görünür.
  - Logo kahvesini (`#7f4a2a` civarı) primary yapmak — reddedildi: kullanıcının verdiği `#9D6638` beyazla AA'yı zaten geçiyor.
- **Sonuçlar / bedel:** Token'lar oklch yerine hex/`var()`; shadcn CLI teması yeniden yazarsa bu blok elle korunmalı. Karamel üstünde beyaz metin 4,78:1 ile AA'yı ancak geçer; küçük ince metinde karamel yerine espresso tercih edilmeli. Koyu tema görsel olarak denenmedi.
- **Doğrulama:** 2026-10-30: `pnpm build` ve `pnpm lint` geçti. Kontrast: beyaz/karamel 4,78, krem/espresso 11,92, beyaz/`#6b5d52` 6,34, beyaz/ink 18,88. Arayüzde henüz yalnız `App.vue` başlığı olduğundan bileşen üstünde görsel kontrol yapılmadı.

## D-087: Arayüz: Türkçe + vue-i18n, üç kolon (kampanyalar / olay zaman çizelgesi / adım paneli), düz metin, eylemler panelde, istemci araması, profil yok, e-posta önizleme ucu ve önizlemede düzenleme

- **Durum:** Kabul edildi · 2026-10-30; istemci araması, markdown yokluğu ve yan panel yerleşimi yerini aldı → D-088, D-089 · 2026-10-30; başlıktaki LLM modu rozeti yerini aldı → D-094 · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** Kullanıcı T-035'ten önce bir yerleşim taslağı (`web/application_draft_ui.png`) önerdi: solda logo, arama, kampanyalar, Analytics özeti ve profil; ortada markdown çizebilen, yeniden boyutlandırılabilen bir sohbet alanı; sağda adımları ve varyantları gösteren, açılıp kapanabilen bir önizleme paneli. İncelemede şunlar çıktı: backend serbest bir sohbet değil, yapılandırılmış bir adım akışı; profil bloğu D-072'deki kimlik doğrulama yokluğuyla çelişiyor; e-posta HTML'ini Go çiziyor ve bunun için bir uç yok; arama için API yok; canlı gönderim panosunun yeri tanımsız. Kullanıcı taslak yerine kod üzerinden ilerlemeyi seçti (T-035).
- **Karar:**
  - Arayüz dili Türkçedir; metinler `vue-i18n` ile `src/locales/tr.json`'dadır, sonra başka diller eklenebilir.
  - Yerleşim üç kolondur: solda kampanya listesi ve Analytics özeti; ortada bir olay zaman çizelgesi; sağda adım ve varyant paneli. Orta alan serbest bir sohbet değildir: kullanıcı mesajlarını (brief, bağlam notu, red gerekçesi) ve SSE olaylarından üretilen kartları (adım açıldı, varyant hazır, hata, durum) gösterir; giriş kutusu bağlama göre brief veya bağlam notu gönderir. Metin düz çizilir (satır sonları korunur); markdown yoktur.
  - Onay, red ve düzenleme eylemleri sağ paneldedir. Açık bir adım beklerken panel kapatılamaz, yalnız daraltılır; zaman çizelgesindeki bir kart panelde ilgili adıma odaklanır. Dar ekranda panel çekmeceye döner.
  - Kampanya araması istemci tarafındadır: yüklenen listede brief ve durumla filtreler; backend değişmez.
  - Profil bloğu yoktur. LLM modu (`/readyz`; mock ise görünür, D-072) küçük bir rozetle başlıkta gösterilir.
  - E-posta HTML'i Proposal'a eklenecek bir önizleme ucuyla `internal/content` üzerinden çizilir (T-036); önizlemede düzenleme yapılabilir ve mevcut `edit` ucuyla gönderilir. Canlı gönderim panosu ve grafikler (shadcn-vue chart) T-037'de kod üzerinden tasarlanır.
  - Kullanıcı ayrıca onayladı: shadcn-vue bileşenlerinin getirdiği `@vueuse/core` kalır. Bildirimler (toast) modül düzeyinde tek bir listededir; bunlar sunucu durumu değil geçici arayüz bildirimi olduğu için D-012'yle uyumlu sayılır. Zaman çizelgesi view'dan türetilebilenleri (brief, notlar, adımlar, varyantlar, redler, başarısız adım nedeni) her zaman, ön eleme sonucu, üretim başlangıcı ve gönderim durumu gibi olayları yalnız açık bağlantı süresince gösterir; olay geçmişi saklanmaz.
- **Alternatifler:**
  - Elle yazılmış küçük bir `t()` — reddedildi: çoğul, interpolasyon ve biçimler elle yazılırdı.
  - Markdown + DOMPurify — reddedildi: iki bağımlılık ve güvenilmeyen metin için temizleme sorumluluğu.
  - Eylemlerin zaman çizelgesi kartlarında olması — reddedildi: çok varyantlı adımda kartlar uzardı.
  - Backend araması (`GET /campaigns?q=`) — kullanıcı önce seçti, sonra istemci aramasına döndü.
  - Profil yer tutucusu — reddedildi: kimlik doğrulama yok (D-072).
  - TS'de kopya e-posta şablonu — reddedildi: Go ve TS şablonları ayrışırdı.
- **Sonuçlar / bedel:** Biçimli metin yoktur. Arama yalnız yüklenen sayfada çalışır. Dar ekranda panel çekmeceye döner. Önizleme ucu T-036'da Proposal API'sine eklenir. Yeni bağımlılık: `vue-i18n`.
- **Doğrulama:** 2026-10-30'da (T-035): `pnpm -C web lint`, `type-check`, `build` ve 107 Vitest testi (12 dosya) geçti: liste ve Türkçe harf duyarsız istemci araması, kampanya oluşturma ve 422 sorunları, zaman çizelgesi sırası ve tek SSE bağlantısı, adım paneli eylemleri (onay, gerekçeli red, kitle ve plan düzenleme, yeniden üret, yeniden aç, iptal, onayı yeniden dene), açık adım varken panelin kapanmaması, dar ekranda çekmeceler, eksik i18n anahtarında ve `tr.json` dışında Türkçe harfte kırılan i18n testi. `make test` geçti. Canlı yığında `localhost:8080`: kampanya listesi, tamamlanmış ve zamanlanmış kampanyaların adımları ve zaman çizelgesi, Europe/Istanbul ve Europe/London zamanlama yuvaları, `LLM: anthropic` rozeti tarayıcıda kontrol edildi; kampanya açılmadı.
- **Uygulama notu:** `vue-i18n` 11.4.14 (tam sürüm); shadcn-vue 2.8.2 ile `button`, `input`, `textarea`, `badge`, `dialog`, `sheet`, `label`, `separator`, `skeleton`. Plan düzenlemede yerel `<select>`. `snapshot` view'ın yerine geçer, diğer olaylar 150 ms birleştirilmiş bir yeniden okuma tetikler. 1024 px altında liste ve panel `Sheet` olur. Eski varyantlardaki `estimated_cost_gbp` kendi para birimiyle gösterilir. Ctrl/Cmd+Enter gönderir.

## D-088: Ajan araçları: soru sorma (cevapla yeniden üretim, adım başına 2), adım özetleri, ön elemede başlık; chat_message tablosu, campaign.title; ajan metni markdown

- **Durum:** Kabul edildi · 2026-10-30
- **Alan:** LLM entegrasyonu
- **Bağlam:** Kullanıcı T-035'ten sonra sohbet ağırlıklı bir arayüz ve ajanın kullanıcıyla konuşabilmesini istedi: ajan kullanıcıya soru sorabilmeli, ara adımların sonunda bir özet göndermeli, taslaklar küçük bir modelle başlık almalı ve başlıklar sohbette görünmeli. D-069 segment tool döngüsünü ve onarımı, D-070 plan ve zamanlama çağrılarını, D-071 metin çağrısını, D-087 düz metni belirlemişti. İş T-046 olarak ayrıldı.
- **Karar:**
  - Soru: segment döngüsüne `ask_user {question, options?}` aracı, plan ve zamanlama şemalarına isteğe bağlı `question` alanı eklenir; metin adımında soru yoktur. Ajan soru sorunca adım "cevap bekliyor" olur ve soru sohbette görünür. Kullanıcı cevaplayınca soru ve cevap bağlam notu olarak modele gider ve adım yeni bir çağrıyla yeniden üretilir; konuşma durumu saklanmaz, D-069'daki onarım mantığı korunur. Adım başına en fazla 2 soru; sonra ajan karar vermek zorundadır.
  - Özet: segment ajanına `send_summary {markdown}` aracı; plan, zamanlama ve metin şemalarına `summary` alanı eklenir; özet aynı çağrıda üretilir.
  - Başlık: ön eleme (Haiku) çıktısına `title` (en fazla 60 karakter, brief'in dilinde) eklenir (`screen@2.0.0`); `campaign.title` sütununda saklanır. Ek çağrı yoktur.
  - Saklama (kodlama kuralı 7, kullanıcı onayladı): `chat_message(id, campaign_id, step_id, kind question | answer | summary, body, options jsonb, created_at)`, ekleme-yalnız (D-068 trigger'ı), indeks `(campaign_id)`; `campaign.title` sütunu. Sohbet geçmişi yeniden açılışta da görünür.
  - Kullanıcı ayrıca onayladı: `chat_message.reply_to uuid NULL REFERENCES chat_message(id)`, CHECK `kind = 'answer'` ⇔ `reply_to IS NOT NULL`, `UNIQUE (reply_to)` (bir soruya tek cevap); `campaign.title` için CHECK (boş olamaz, en fazla 60 karakter). Adım başına üçüncü soru bir doğrulama hatasıdır: saklanmaz, aynı konuşmada "soru hakkın doldu, şimdi karar ver" sorunu döner ve bir onarım denemesi harcar (D-069).
  - Ajan özetleri ve soruları arayüzde markdown olarak çizilir: `markdown-it` (HTML kapalı) + DOMPurify, linkler `rel="noopener"`. Kullanıcı mesajları ve brief düz metin kalır (D-087'den kısmi sapma).
  - Değişen prompt paketlerinin sürümleri D-059'a göre artar (şema değiştiği için MAJOR).
- **Alternatifler:**
  - Her şeyi T-035'e eklemek — reddedildi: görev çok büyürdü.
  - Konuşmayı saklayıp kaldığı yerden sürdürmek — reddedildi: konuşma tablosu, thinking bloklarının saklanması ve karmaşık kurtarma.
  - Adım sonunda ayrı bir Haiku özet çağrısı — reddedildi: adım başına ek çağrı ve gecikme.
  - Ayrı başlık çağrısı — reddedildi: kampanya başına bir çağrı daha ve iki migration.
  - Düz metin — reddedildi: özetlerde liste ve vurgu olmazdı.
  - `context_note`'a `kind` sütunu — reddedildi: notlar ve ajan mesajları karışırdı.
  - Soru yalnız kitle adımında — reddedildi: zaman ve kanal belirsizliğinde soru sorulamazdı.
- **Sonuçlar / bedel:** Soru cevaplanınca adımın önceki ara çıktıları yeniden üretilir. Beş prompt paketi değişir, önbellek bir kez soğur. Ön eleme prompt'u iki iş yapar. Bir tablo ve bir sütun eklenir. İki web bağımlılığı (`markdown-it`, `dompurify`).
- **Uygulama notu (T-046, kullanıcı onayladı):**
  - Migration'lar: `00005_campaign_title.sql` (`campaign_title_check`), `00006_chat_message.sql`. D-068'in her FK'ye indeks kuralı gereği `chat_message_step_id_idx` de eklendi. `chat_message_options_check` jsonpath ile `options`'a yalnız `kind = 'question'` satırında ve yalnız dize dizisi olarak izin verir.
  - Şema biçimi: plan ve zamanlama çıktısı `{question, plan | entries, summary}`; üç alan da zorunlu ama `null` olabilir (isteğe bağlı alan yerine, `send_at` örneği). Seçenek sayısı (2–4) Go'da denetlenir. Sınırlar `llm.MaxQuestionsPerStep = 2`, `MaxTitleChars = 60`, `MaxQuestionChars = 500`, `MaxOptionChars = 100`, `MaxSummaryChars = 2000`; REST cevabı `api.MaxAnswerChars = 1000`. Soru ve özetlerde link, HTML ve başlık reddedilir.
  - Segment özeti: kabul edilen `propose_segment` turunda `send_summary` yoksa 8 tur sınırı içinde bir tur daha istenir. Bu turdaki hata öneriyi bozmaz, yalnız özet düşer. Bedeli: segment çağrısı başına pratikte bir Sonnet turu.
  - Kural dışı özet: segment, plan ve zamanlamada onarım sorunu olur; metin özeti ve geçersiz başlık sessizce düşer (log), adım başarısız olmaz.
  - Soru hakkı cevaplansın cevaplanmasın sorulan her soruyla düşer; yalnız cevaplananlar `clarifications` olarak modele gider. Cevap taslak varyantları geçersiz kılmaz, yeniden üretimi tetikler; kapalı adıma cevap `step_not_open`. Engellenen brief'e de başlık yazılır; `campaign_titled` olayı `screen_result`'tan önce gelir.
  - Uç: `POST /messages/{id}/answer` → 202; yeni sorun kodları `message_not_question`, `question_already_answered`. Olaylar: `question_asked`, `answer_given`, `summary_posted`, `campaign_titled`. `ContentGenerator.Generate` artık `flow.Generated{Content, Summary}` döner.
  - Prompt sürümleri: `screen@2.0.0`, `segment@2.0.0`, `channel_plan@2.0.0`, `schedule@3.0.0`, `content@2.0.0`.
- **Doğrulama:** `make test` geçti (50 Go paketi, web 107 test); `internal/proposal/flow/chat_integration_test.go` (`TestAudienceQuestionWaitsForTheAnswer`, `TestThirdQuestionIsBlocked`, `TestPlanAndScheduleQuestions`, `TestTitleAndSummaryChecks`, `TestAnswerDuringGenerationDropsTheStaleResult`), `internal/proposal/llm/chat_test.go`, store migration ve kısıt testleri. Canlı (geçici mock modu): soru içeren brief bekleyen soru ve iki seçenek gösterdi, cevap 202 ile kitleyi değiştirdi, ikinci cevap 409 `question_already_answered`; örnek brief başlık ve adım özetleriyle geldi.

## D-089: Sohbet ağırlıklı arayüz: sade sol menü (+ ikonu, arama/filtre yok), adımlar sohbette akordiyon, e-posta önizleme ucu ve önizlemede düzenleme

- **Durum:** Kabul edildi · 2026-10-30; sol menü liste görünümü ve sohbetteki kart/akordiyon yerleşimi yerini aldı → D-091 · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** T-035'in arayüzünü gören kullanıcı arayüzün sadeleşmesini, yan panel yerine sohbet ağırlıklı bir düzeni ve metin taslağında kullanmak için e-posta önizlemesini istedi. D-087 istemci aramasını, durum filtresini ve sağdaki adım panelini; T-036 e-posta önizleme ucunu öngörüyordu. İş T-045 olarak ayrıldı.
- **Karar:**
  - Sol menü sadeleşir: yeni kampanya bir `+` ikon düğmesidir; brief araması ve durum filtresi şimdilik kaldırılır.
  - Adım paneli sohbete taşınır: tamamlanan adımlar sohbette akordiyon olarak gösterilir, açık adım ve eylemleri sohbet akışındadır; kampanya başlıkları sohbette görünür (D-088).
  - E-posta HTML'i Proposal'a eklenen bir önizleme ucuyla `internal/content` üzerinden çizilir ve sohbette gösterilir; önizlemede metin düzenlenebilir ve mevcut `edit` ucuyla gönderilir.
- **Alternatifler:**
  - D-087'deki yan panel ve istemci araması — reddedildi: kullanıcı sohbet ağırlıklı ve sade bir arayüz istedi.
- **Sonuçlar / bedel:** Arama ve filtre yoktur. Uzun kampanyalarda sohbet uzar; akordiyon bunu sınırlar. Proposal API'sine bir önizleme ucu eklenir.
- **Uygulama notu (T-045):** Sol menüde `+` ikon düğmesi ve kampanya başlığı (yoksa brief). Sağ panel (`StepPanel.vue`) kaldırıldı; sohbet tek kolondur (`ChatStream.vue`). Onaylanan adım kendi onay kartında akordiyonla (shadcn-vue `accordion`, reka-ui) katlanır, açık adım ve eylemleri son kartın ardındadır, iptal ve yeniden dene satırı en alttadır. Başlık (`campaign_titled`) sohbet kartı değildir, üst başlığı ve liste satırını günceller. Ajan soruları ve özetleri `lib/markdown.ts` ile çizilir (`markdown-it` 15.0.3, `html: false`; `dompurify` 3.4.17, http/https/mailto linkleri, `rel="noopener noreferrer"`). Aynı andaki kartlar adım sırasıyla sıralanır. Kullanıcı subagent'ın bu yorumlarını onayladı.
- **Doğrulama:** `make test` geçti (50 Go paketi; web 13 dosya, 122 test). `web/src/views/__tests__/CampaignPage.steps.spec.ts` (sohbetteki adımlar, eylemler, soru düğmeleri ve cevap kutusu, özet markdown'ı, dar ekran), `CampaignPage.timeline.spec.ts`, `web/src/lib/__tests__/markdown.spec.ts` (XSS). Canlı (geçici mock modu, uygulama ajanı): `+` düğmesi, başlıklar, akordiyonlar, açık metin adımı, soru kartı ve cevap kutusu masaüstü uygulamasının tarayıcı panelinde görüldü.

## D-090: E-posta önizleme ucu: `POST /variants/{id}/preview` kaydedilmemiş içerikle, dile göre sabit örnek ad, sandbox iframe; önizlemede alan formu, soru kartında seçenek düğmeleri

- **Durum:** Kabul edildi · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** D-089 e-posta HTML'inin Proposal'a eklenen bir önizleme ucuyla `internal/content` üzerinden çizilmesini, sohbette gösterilmesini ve önizlemede düzenlenmesini istedi; D-088 ajan sorularını getirdi. Ucun biçimi, örnek adın kaynağı, HTML'in sayfaya nasıl gireceği ve düzenleme ile cevabın etkileşimi açıktı (T-045).
- **Karar:**
  - Uç: `POST /variants/{id}/preview`. Gövde isteğe bağlıdır: boşsa saklı içerik, doluysa kaydedilmemiş düzenlenmiş e-posta içeriği çizilir. Yanıt `{subject, html, text}`. Çizim `content.RenderEmail` ile ve Proposal'ın `CAMPAIGN_CTA_URL` ayarıyla yapılır; içerik kaydedilmez, yeni varyant oluşmaz. Yalnız e-posta varyantları içindir.
  - `{{first_name}}` yerine dile göre kodda sabit bir örnek ad konur (tr için "Ayşe", diğerleri için "Alex"); gerçek müşteri verisi önizlemeye girmez.
  - HTML sohbette `<iframe sandbox srcdoc>` içinde çizilir: script ve dış gezinme kapalıdır, e-postanın satır içi CSS'i uygulama stiline karışmaz.
  - Düzenleme önizlemenin yanında bir alan formudur (konu, ön başlık, başlık, paragraflar, CTA etiketi). Değişiklik gecikmeli (debounce) bir önizleme isteği atar; Kaydet mevcut `edit` ucuna gider.
  - Ajan sorusunun seçenekleri soru kartında düğmedir. Soru beklerken giriş kutusu serbest cevap gönderir (`POST /messages/{id}/answer`).
- **Alternatifler:**
  - `GET /variants/{id}/preview` yalnız saklı içerik — reddedildi: düzenlemenin sonucu ancak kaydedip yeni varyant oluşunca görünürdü.
  - Kitledeki en uzun ad uzunluğunda dolgu ad — reddedildi: önizleme yapay görünür, Customer'a ek çağrı gerekir. Görünür yer tutucu — reddedildi: alıcının göreceği metin tam görünmez.
  - DOMPurify + `v-html` — reddedildi: e-posta CSS'i uygulama stiliyle çakışır, güvenilmeyen HTML sayfaya girer.
  - Önizlemede `contenteditable` — reddedildi: alanları iframe'den geri ayrıştırmak gerekir, işaretleme sızabilir.
- **Sonuçlar / bedel:** Düzenleme sırasında her gecikmeli değişiklik bir istek atar. iframe yüksekliği içerikten hesaplanır. Düzenleme WYSIWYG değildir. Örnek ad uzun adlarda taşmayı göstermez; sınır backend'de zaten ölçülür (D-028). SMS önizlemesi, segment ve maliyet gösterimi ve zaman seçici T-036'da kalır.
- **Uygulama notu (T-045, kullanıcı onayladı):** `flow.Service.PreviewEmail` (`internal/proposal/flow/preview.go`), örnek adlar `SampleFirstNameTR = "Ayşe"`, `SampleFirstNameEN = "Alex"`. Düzenlenmiş içerik `edit` ile aynı doğrulamadan geçer; hata 422 `invalid_content`. E-posta olmayan varyant 409 `variant_not_email`. Onaylanmış ve reddedilmiş varyantlar da önizlenir; `{}` veya `content: null` saklı içerik sayılır. iframe `sandbox=""` (izin yok), yüksekliği ölçülemediği için sabit 28rem ve CSS ile elle büyütülebilir. Önizleme gecikmesi `PREVIEW_DEBOUNCE_MS = 400`. E-posta varyantı salt okunur görünümde çizilmiş önizlemeyle gösterilir; istek başarısız olursa metin blokları ve yeniden dene.
- **Doğrulama:** `make test` geçti. `internal/proposal/api/api_integration_test.go` `TestEmailPreviewOverREST` (saklı tr/en örnek adlarla, düzenlenmiş, geçersiz 422, SMS 409, bilinmeyen 404, view değişmez), `api_test.go` `TestPreview`, `flow/preview_test.go`, `CampaignPage.steps.spec.ts` (önizleme, düzenleme, 422, kaydet). Canlı (mock): curl ile saklı ve düzenlenmiş önizleme 200, geçersiz 422, SMS 409, bilinmeyen 404. Masaüstü uygulamasının tarayıcı panelinde sandbox iframe ekran görüntüsünde boş çıkıyor (opak köken iframe'in panel sınırlaması olduğu düşünülüyor); gerçek bir tarayıcıda gözle kontrol edilmedi.

## D-091: Sade sohbet: kenar çubuğunda durum gruplu akordiyon, satırda başlık ve tarih; sohbette yalnız varyantlar kart, adım olayları ve eylemleri adım başına akordiyon; iptal sohbet çubuğunda

- **Durum:** Kabul edildi · 2026-10-30; LLM modu rozeti yerini aldı → D-094 · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** T-045'i gören kullanıcı arayüzün daha da sadeleşmesini istedi: kenar çubuğundaki kampanyalar akordiyonla açılıp kapanmalı ve duruma göre alt akordiyonlarda ayrılmalı, satırlarda yalnız başlık ve satır içi tarih olmalı; sohbette mesaj ve varyantlar dışındaki olaylar ve eylemler akordiyonda durmalı, bu bileşenlerde ve ajan cevaplarında kart kullanılmamalı, iç içe kart olmamalı. D-089 adımları onay kartında akordiyon olarak gösteriyordu (T-047).
- **Karar:**
  - Kenar çubuğu: dış akordiyon "Kampanyalar"; içinde beş durum grubu (Taslak, Zamanlanmış, Gönderiliyor, Tamamlandı, İptal). Boş gruplar gizlenir; seçili kampanyanın grubu ve Taslak açık gelir. Satırda yalnız başlık (yoksa brief) ve satır içi kısa tarih vardır; durum rozeti yoktur. LLM modu rozeti başlıkta kalır (D-072).
  - Sohbet: mesajlar (brief, bağlam notu, ajan sorusu, cevap, özet) kartsız düz satırlardır. Varyantlar tek kart katmanıdır, kart içinde kart yoktur. Her adımın olayları (açıldı, üretiliyor, hata, onaylandı) ve eylemleri (onayla, reddet, düzenle, yeniden üret) o adımın tek akordiyonundadır; açık adımın akordiyonu açık, onaylananlarınki kapalı gelir.
  - Onaylanan adımda adım kartı yerine yalnız onaylanan varyant(lar) kart olarak görünür; reddedilen ve yerini alan varyantlar adımın akordiyonunda kartsız listelenir.
  - İptal düğmesi sohbet çubuğundadır. Onaylanmış bir adımı yeniden açma düğmesi o adımın akordiyonundadır; onayı yeniden dene ve yeniden başlat en sondaki "Kampanya eylemleri" akordiyonundadır.
  - Kampanya sayfasının başlığında durum rozeti yoktur; durum başlığın yanında soluk düz metindir.
- **Alternatifler:**
  - Üç kova (Aktif, Gönderim, Geçmiş) — reddedildi: zamanlanmış ve gönderiliyor ayrılmazdı.
  - Ardışık olayların tek akordiyonda toplanması — reddedildi: eylemlerin hangi adıma ait olduğu belirsizleşirdi.
  - Onaylanan adımda tüm varyantların kart kalması — reddedildi: sohbet uzardı.
- **Sonuçlar / bedel:** Liste sayfalı olduğu için gruplar yalnız yüklenen kampanyaları gösterir. Durum yalnız grup başlığından okunur. Olaylar varsayılan olarak gizlidir, bir tıkla açılır.
- **Uygulama notu (T-047, kullanıcı onayladı):** İlk kararda yeniden aç da sondaki akordiyondaydı; uygulamayı gören kullanıcı onu adımın akordiyonuna aldı ve başlık rozetini düz metne çevirdi. Kullanıcı ve ajan mesajları sola hizalıdır, fark etiket rengindedir (`data-author`). Kenar çubuğu satırında yalnız gün tarihi (`i18n.ts` `date` biçimi), grup başlığında kampanya sayısı vardır. Kampanya düzeyindeki olaylar (ön eleme, durum, onay, iptal) soluk düz satırdır. Kartların içi sol çizgiyle (`border-l-2`) düzleşti; e-posta iframe'inin çerçevesi kalktı; `InlineProblems` kutusuz kırmızı metindir. Adıma odaklanma tıklaması kalktı. Kart yalnız açık veya onaylanmış adımdaki taslak/onaylı varyantlardır (`data-slot="card"`).
- **Doğrulama:** `pnpm -C web lint`, `type-check`, `build` ve 151 Vitest testi (14 dosya) geçti: `web/src/__tests__/App.spec.ts` (durum grupları, boş grupların gizlenmesi, varsayılan açık gruplar, satırda yalnız başlık ve tarih), `web/src/views/__tests__/CampaignPage.steps.spec.ts` ("plain chat (D-091)": kartsız mesajlar, adım başına tek akordiyon, onaylanan adımda yalnız onaylanan varyant kartı, iç içe kart yok, iptal sohbet çubuğunda, yeniden aç adım akordiyonunda, son akordiyon), `features/campaigns/__tests__/chat.spec.ts`. `make test` Go tarafında yük kaynaklı testcontainers zaman aşımlarıyla düştü (customer `audience`/`consent`/`graph`, notification `intake`); bu paketler tek başına sırayla koşunca geçti. Canlı: `localhost:8080`'de kenar çubuğu grupları, satır linklerinde `text-decoration-line: none`, rozetsiz başlık ve sohbet tarayıcıda kontrol edildi.

## D-092: Kitle ve plan dağılımları shadcn-vue chart pasta grafikleriyle; sayılar lejantta, tablolar kalkar

- **Durum:** Kabul edildi · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** Kullanıcı kitle analizi gibi adımlarda kapsama dağılımının kanal, dil ve yerel ayar için pasta grafikle gösterilmesini istedi. Kitle varyantı kanal kapsamasını (`both`, `email_only`, `sms_only`, `unreachable`), dil ve yerel ayar (dil + saat dilimi, D-076) dağılımını; plan varyantı kanal başına alıcıları tablo olarak gösteriyordu. D-087 grafikler için shadcn-vue chart'ı T-037'ye bırakmıştı (T-048).
- **Karar:**
  - Grafikler shadcn-vue chart bileşeniyle çizilir (`@unovis/vue` + `@unovis/ts`, tam sürüm sabitlenir); renkler D-086 paletinin token'larından gelir. T-037'nin panosu da aynı bileşeni kullanır.
  - Kitle adımında üç pasta: kanal kapsaması, dil, yerel ayar (dil + saat dilimi). Plan adımında bir pasta: kanal başına alıcı.
  - Sayılar pastanın lejantında durur; bu dağılımların tabloları kalkar.
- **Alternatifler:**
  - Elle SVG pasta — reddedildi: ipucu, erişilebilirlik ve animasyon elle yazılırdı; T-037'de yine kütüphane gerekirdi.
  - `vue-chartjs` + Chart.js — reddedildi: canvas tabanlı, shadcn temasından ayrı stil, iki bağımlılık.
  - Pastaların tablolarla birlikte durması — reddedildi: varyant kartı uzardı. Yalnız kitle adımı — reddedildi: plan dağılımı tabloda kalırdı.
- **Sonuçlar / bedel:** İki yeni web bağımlılığı. Ayrıntılı tablo görünümü kaybolur; sayılar yalnız lejantta okunur.
- **Uygulama notu (T-048, kullanıcı onayladı):** `@unovis/vue` 1.7.1 ve `@unovis/ts` 1.7.1 (tam sürüm); shadcn-vue `chart` bileşeni (`src/components/ui/chart/`, CLI çıktısı olduğu gibi, yalnız bu klasöre dar bir lint istisnası) ile `VisSingleContainer` + `VisDonut`. Ortak bileşen `src/components/DistributionPie.vue`; yardımcılar `src/features/campaigns/distribution.ts`. Beşten fazla dilimde en büyük dördü kalır, kalanlar nötr "Diğer" diliminde toplanır. Kanallar her pastada aynı renktedir (e-posta `--chart-1`, SMS `--chart-2`, ikisi `--chart-3`, ulaşılamayan nötr); dil ve yerel ayar renkleri büyüklük sırasına göredir. Halka ortasında toplam yoktur; yüzdeler dilim toplamına göredir (`percent` biçimi, `%45,8`). Kapsama pastası ulaşılamayanları da içerdiği için toplamı kitle sayısından (yalnız ulaşılabilenler) büyük olabilir. Pasta bileşeni `defineAsyncComponent` ile ayrı parçada geç yüklenir.
- **Doğrulama:** `make test` geçti (50 Go paketi; web 18 dosya, 173 test; geç yükleme öncesi). Geç yüklemeden sonra `pnpm -C web lint`, `type-check`, `build` ve 175 Vitest testi (19 dosya) geçti: `src/components/__tests__/DistributionPie.spec.ts`, `ChartPlaceholder.spec.ts`, `features/campaigns/__tests__/distribution.spec.ts`, `AudienceBody.spec.ts`, `PlanBody.spec.ts`. Build: unovis ayrı parçada (`DistributionPie-*.js`, 111 kB), ana parça 612 kB'tan 503 kB'a indi. Canlı: `localhost:8080`'de onaylı bir kampanyanın kitle kartında üç pasta ve plan kartında kanal pastası tarayıcıda görüldü (yalnız açık tema).

## D-093: Metin varyantları tek yatay kaydırıcıda (shadcn-vue carousel); onaydan sonra sıradaki bekleyen slayt, köşede kalan yuva sayısı

- **Durum:** Kabul edildi · 2026-10-30; tek kaydırıcı ve tüm metinlerde ortak sıra yerini aldı → D-095 · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** Metin adımında her dil, saat dilimi ve kanal yuvası için birer varyant kartı alt alta diziliyor, sohbet uzuyordu. Kullanıcı e-posta ve SMS varyantlarının gruplanıp oklarla yatay gezilmesini, tek tek onaylanmasını ve kaç onay kaldığının kartın köşesinde yuvarlak bir sayıyla gösterilmesini istedi (T-049).
- **Karar:**
  - Açık metin adımının e-posta ve SMS varyantları tek bir yatay kaydırıcıdadır (shadcn-vue `carousel`, `embla-carousel-vue` tam sürüm sabitlenir); bir anda bir varyant görünür, oklar ve klavye ile gezilir.
  - Bir varyant onaylanınca kaydırıcı onay bekleyen sonraki slayta kendisi geçer.
  - Kartın köşesindeki yuvarlak rozet, henüz onaylı varyantı olmayan yuvaların (dil + saat dilimi + kanal) sayısıdır; 0 olunca kaybolur. Bu rozet D-091'in kenar çubuğu için koyduğu rozetsiz satır kuralının dışındadır.
- **Alternatifler:**
  - Elle yazılmış slayt — reddedildi: dokunmatik kaydırma ve geçiş elle yazılırdı.
  - Kanal başına ayrı kaydırıcı — reddedildi: kullanıcı tek kaydırıcı istedi.
  - Onaydan sonra yerinde kalmak — reddedildi: her onaydan sonra elle ilerlemek gerekirdi. Rozetin taslak varyant sayması — reddedildi: yeniden üretimle sayı yapılacak onaydan büyük görünürdü.
- **Sonuçlar / bedel:** Yeni web bağımlılığı `embla-carousel-vue`. Varyantlar yan yana karşılaştırılamaz. Otomatik geçiş kullanıcıyı şaşırtabilir. E-posta ve SMS aynı sırada karışık gezilir.
- **Uygulama notu (T-049, kullanıcı onayladı):** `embla-carousel-vue` 8.6.0 (tam sürüm); shadcn-vue `carousel` (`src/components/ui/carousel/`). Yardımcılar `src/features/campaigns/carousel.ts` (`pendingSlotCount`, `nextPendingIndex`), bileşen `VariantCarousel.vue`. Slaytlar önce kanala (e-posta, sonra SMS), sonra dile ve saat dilimine göre sıralanır. Oklar ve "3 / 6" yazısı kartın üstündedir, tek slaytta gizlidir. Otomatik geçiş yalnız görünen slayt onaylanınca olur ve onaylı yuvaya ait taslakları atlar. Rozet yalnız reddedilmiş ve henüz üretilmemiş (`missing`) yuvaları da sayar. Görünmeyen slaytlar `inert` olur ve yerleşim oturunca çöker; e-posta formundaki ok tuşları slayt değiştirmez.
- **Doğrulama:** `make test`: Go'da yalnız seed'li Postgres container'ının 60 sn içinde hazır olmamasından düşen `proposal/confirm`, `proposal/customer`, `proposal/flow` tek başına sırayla koşunca geçti; web 211 Vitest testi (21 dosya), `lint`, `type-check`, `build` geçti. `src/features/campaigns/__tests__/carousel.spec.ts`, `src/views/__tests__/CampaignPage.carousel.spec.ts` (tek slayt ve konum yazısı, oklar ve klavye, rozetin yuva sayması, sıradaki bekleyen yuvaya geçiş, onaylı adımda rozetsiz kaydırıcı, tek kart katmanı). Canlı: açık metin adımında konum yazısı ve köşe rozeti tarayıcıda görüldü.

## D-094: Masaüstünde üst başlık yok, LLM modu rozeti arayüzden kalktı; mobil üst çubukta menü ve logo

- **Durum:** Kabul edildi · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** Kullanıcı `13f2fe7` commit'iyle (`web/src/layouts/AppShell.vue`) masaüstündeki üst başlığı ve LLM modu rozetini kaldırdı; mobilde üst çubuk menü düğmesi ve logoyu gösteriyor. D-087 ve D-091 rozetin başlıkta kalmasını öngörüyordu. Bu sapma T-049 sırasında soruldu; kullanıcı rozetin kaldırılmış kalmasını seçti.
- **Karar:**
  - Masaüstünde üst başlık yoktur; logo kenar çubuğundadır. Mobilde üst çubuk menü düğmesi ve logodan (`app.logoAlt`) oluşur.
  - LLM modu arayüzde gösterilmez; mod log'da ve `/readyz` kontrolünde görünür kalır (D-072).
- **Alternatifler:**
  - Rozetin kenar çubuğuna taşınması — reddedildi: kullanıcı sade kabuğu tercih etti.
  - Rozetin yalnız mock modda gösterilmesi — reddedildi: aynı nedenle.
- **Sonuçlar / bedel:** Mock modda çalışıldığı arayüzden anlaşılmaz; `/readyz` veya log'a bakmak gerekir.
- **Doğrulama:** `pnpm -C web lint` geçti (kullanılmayan `LlmModeBadge` ve `mode` kalktı; `LlmModeBadge.vue`, `useLlmMode.ts` ve `getLlmMode()` silindi). `src/__tests__/App.spec.ts`: masaüstünde üst başlık ve `h1` yok, logo ve "+" kenar çubuğunda; dar ekranda menü düğmesi ve logolu üst çubuk.

## D-095: Metin adımında kanal başına ayrı kaydırıcı (önce e-posta, sonra SMS); zamanlama adımı da kaydırıcıda

- **Durum:** Kabul edildi · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** D-093 e-posta ve SMS varyantlarını tek kaydırıcıya koydu. Uygulamayı gören kullanıcı kanalların ayrılmasını ve aynı kaydırıcının zamanlama adımında da kullanılmasını istedi (T-049).
- **Karar:**
  - Açık veya onaylanmış metin adımında iki kaydırıcı alt alta durur: önce E-posta, sonra SMS (varyantı olmayan kanal gösterilmez). Her birinin kendi "1 / N" yazısı, kendi kalan yuva rozeti ve kendi otomatik geçişi vardır; geçiş yalnız aynı kanal içindedir. Kanal içinde sıra dil, sonra saat dilimidir.
  - Zamanlama adımı da aynı kaydırıcıdadır: her slayt bir (dil, saat dilimi) yuvasının zamanıdır, sıra dil sonra saat dilimidir, köşedeki rozet onaylı varyantı olmayan yuvaları sayar, onaydan sonra sıradaki bekleyen yuvaya geçilir. Onaylanmış zamanlama da kaydırıcıda, rozetsiz gösterilir.
  - D-093'ün kütüphane, rozet ve otomatik geçiş kuralları aynen geçerlidir.
- **Alternatifler:**
  - Tek alanda E-posta / SMS sekmeleri — reddedildi: bir kanalın bekleyen onayı sekme değiştirmeden görünmezdi.
  - Zamanlamada rozetsiz ve otomatik geçişsiz kaydırıcı — reddedildi: metin adımıyla davranış farkı olurdu.
- **Sonuçlar / bedel:** Metin adımı sohbette iki kart boyu yer kaplar. Kanallar arası otomatik geçiş yoktur.
- **Uygulama notu (T-049, kullanıcı onayladı):** `carouselGroups` (`src/features/campaigns/carousel.ts`) metin adımında kanal başına, zamanlamada tek grup üretir; `pendingSlotCount(step, channel)` kanal başına sayar. Her grup kendi `VariantCarousel` örneğidir (`data-carousel` = `email`, `sms`, `schedule`). Kanal kaydırıcılarının görünür başlığı vardır; zamanlama kaydırıcısı başlıksızdır (adımın akordiyon başlığı zaten "Zamanlama"). Gösterilecek kartı olmayan ama bekleyen yuvası (yalnız reddedilmiş veya üretilmemiş) olan grup başlığı, rozeti ve "bekleyen taslak yok" satırıyla görünür; hiç yuvası olmayan grup gizlenir.
- **Doğrulama:** `make test`: Go'da yalnız seed'li Postgres container'ının 60 sn içinde hazır olmamasından düşen `proposal/confirm`, `proposal/customer`, `proposal/flow` tek başına sırayla koşunca geçti; web 211 Vitest testi (21 dosya), `lint`, `type-check`, `build` geçti. `CampaignPage.carousel.spec.ts`: iki kanal kaydırıcısı ve ayrı rozetler, kanal içinde kalan otomatik geçiş, zamanlama kaydırıcısı (sıra, rozet, geçiş, onaylıda rozetsiz), boş kanal satırı (yalnız reddedilmiş, üretilmemiş, yuvasız gizli). Canlı: açık metin adımında E-posta ("1 / 2", rozet 2) ve SMS (rozet 1) kaydırıcıları, zamanlanmış kampanyada zamanlama kaydırıcısı görüldü; açık zamanlama adımı ve boş kanal yalnız testle doğrulandı.

## D-096: Kitle adımında kural (DSL) ve kitle düzenleme arayüzden kalkar; API değişmez

- **Durum:** Kabul edildi · 2026-10-30
- **Alan:** Takip ve altyapı
- **Bağlam:** Kitle varyant kartı segment kuralını (DSL) gösteriyor ve elle düzenleme formu sunuyordu (D-070'in elle düzenleme kuralı, D-087'nin kitle düzenleme eylemi). Kullanıcı T-049 sırasında sorgunun ve düzenlemenin arayüzden kaldırılmasını istedi.
- **Karar:**
  - Kitle kartında DSL gösterilmez; kişi sayısı ve dağılım pastaları (D-092) kalır.
  - Kitle varyantı için düzenleme eylemi ve formu arayüzde yoktur. Pazarlamacı kitleyi bağlam notu, red gerekçesi veya ajan sorusunun cevabıyla yönlendirir. Plan ve metin düzenlemesi kalır.
  - Proposal API'si değişmez: kitle `edit` ucu ve DSL alanı backend'de durur.
- **Alternatifler:**
  - DSL'in yalnız akordiyonda gösterilmesi — reddedildi: kullanıcı arayüzden tamamen kaldırılmasını istedi.
  - Kitle `edit` ucunun da kaldırılması — reddedildi: istek arayüzle sınırlı; API ve testleri değişmeden kalır.
- **Sonuçlar / bedel:** Pazarlamacı uygulanan kuralı göremez ve doğrudan düzeltemez; yalnız sohbetle yönlendirir. Kullanılmayan bir API ucu kalır.
- **Doğrulama:** `AudienceBody.spec.ts` (DSL ve `pre` yok), `CampaignPage.steps.spec.ts` ("offers no edit for the audience": Onayla ve Reddet var, Düzenle ve form yok); `AudienceEditForm.vue` silindi. Canlı: kitle kartında `pre` yok, üç pasta duruyor.

## D-097: `make test-go` paket paralelliğini `-p 2` ile sınırlar; `TestNATSScheduling` sunucu saatiyle ölçer

- **Durum:** Kabul edildi · 2026-10-30
- **Alan:** Test ve kapsam
- **Bağlam:** `make test` (D-051) birkaç görevdir yalnız paralel yükte kırılıyordu; kırılan paketler tek başına `-p 1` ile geçiyordu. Her entegrasyon paketi TestMain'de kendi konteynerlerini açar: 9 paket `pgtest` ile seed'li Postgres (initdb + `stub/` CSV'leri, tek başına ~9 sn), ~12 paket düz Postgres, ~8 paket NATS, `mockprovider` Mailpit ve MockServer. Varsayılan `-p` CPU sayısıdır (8); Docker VM'i 8 CPU / 4 GB, compose stack'i de açık. Bu yükte seed'li Postgres `BasicWaitStrategies`'in 60 sn penceresinde ikinci "ready to accept connections" satırını basamadı, MockServer da açılamadı; intake zamanlama testleri ve consent `QUERY_TIMEOUT` aynı CPU açlığından etkilendi.
- **Karar:**
  - `Makefile`'da `GO_TEST_P ?= 2`; `test-go` `go test -race -count=1 -p $(GO_TEST_P) -tags=integration ./...` çalıştırır. Daha güçlü veya zayıf makinede `make test GO_TEST_P=…` ile değiştirilir. Test yardımcıları, wait süreleri ve seed değişmez.
  - Önce `-p 4` seçildi. Sessiz makinede geçti (206 sn), ama makinede başka iş yükü varken (load average 15–24, koşu 480+ sn) iki koşu farklı nedenle kırıldı: `proposal/confirm`'de seed'li Postgres 60 sn'yi kaçırdı; `TestSend_RateLimitedToAHundredPerSecond` bir saniyelik pencerede 118 istek saydı (sınır `RatePerSecond+Burst+5` = 115; toplam hız 99/s doğruydu, istekler sahte provider'a CPU açlığında kümelenerek vardı). Kullanıcı varsayılanı 2'ye indirdi.
  - `-p 4` ile ilk koşuda ayrı bir flake çıktı: `TestNATSScheduling` hedef mesajı planlanan zamandan 4 ms önce gördü, çünkü host `time.Now()`'u NATS zamanlayıcısının Docker VM saatiyle karşılaştırıyordu. Kontrol artık hedef mesajın sunucuda saklanan zamanıyla (`jetstream.RawStreamMsg.Time`) yapılır, yani zamanlayıcıyla aynı saatle; "erken değil" ve "2 sn içinde" garantileri gevşemez (kullanıcı onayladı). `TestIntake_ScheduledScheduleFiresAtItsTime` host saatinde kalır: kontrolden önce `e.idle` beklediği için payı vardır.
- **Alternatifler:**
  - `-p 4` — ilk seçimdi, yerini `-p 2` aldı: paylaşılan makinede pay yetmedi (yukarıda).
  - `-p 4` ile birlikte seed'li beklemeyi 180 sn'ye çıkarmak — reddedildi: açılış zaman aşımını kaldırır ama rate testi gibi zamanlamaya duyarlı testler yükte kırılmaya devam eder; gerçek bir takılma 3 dk sonra fark edilir.
  - Tek seed'li Postgres'i süreçler arası paylaşıp paket başına template database — reddedildi: konteyner reuse ve Ryuk ömrü süreçler arasında kırılgan, çok daha fazla kod.
  - Yalnız wait timeout'larını artırmak — reddedildi: açılışı maskeler, zamanlamaya duyarlı testler yükte kırılmaya devam eder.
  - Saat kayması için host saatinde 250 ms tolerans — reddedildi: 250 ms'ye kadar erken ateşlemeyi yakalayamaz. Testi bilinen flake olarak bırakmak — reddedildi: `make test` ara sıra kırmızı kalır.
- **Sonuçlar / bedel:** Suite yavaşlar. Dış yük yine de bir sınırdır: başka oturumlar aynı anda konteyner açarsa zamanlamaya duyarlı testler kırılabilir. Paralellik makineden bağımsız sabit; çok çekirdekli bir CI'da suite gereğinden yavaş kalabilir. `-p` derleme paralelliğini de sınırlar. Paket sayısı büyüdükçe değer yeniden ölçülmeli. `TestNATSScheduling` zamanı host saatinde değil sunucu saatinde ölçer.
- **Doğrulama:** Aynı makinede (8 CPU, Docker VM 8 CPU / 4 GB, compose stack açık), varsayılan `-p` ile `go test -race -count=1 -tags=integration ./...`: 9 paket başarısız (`customer/{audience,consent,graph}`, `notification/sender`, `proposal/{api,confirm,customer,flow}`, `mockprovider`), 330 sn. `-p 4` ile: hepsi geçti, 206 sn; seed'li paketler 17–27 sn sürdü. `-p 4` ile `make test` iki kez yük altında kırıldı (yukarıda). `-p 2` ile ilk `make test` de kırıldı (692 sn, load average ~12): `notification/sender` 312 sn sürdü, `intake.Start` NATS consumer oluştururken `context deadline exceeded` aldı. Bu koşularda host'ta `kernel_task` ~%110 CPU kullanıyordu; makine sessiz `-p 4` koşusuna göre ~3 kat yavaştı. `TestNATSScheduling` sunucu saatiyle tek başına üç kez geçti. Üç ardışık yeşil `make test` henüz sağlanmadı: Doğrulanmadı.

---

## Açık konular

Seçenekler, kaynaklar ve öneriler için bkz. [docs/research/](docs/research/README.md) (2026-10-27). Oradaki öneriler karar değildir.

- Sipariş kalemi ve ürün alanlarının ilişki filtrelerine girip girmeyeceği (D-042)
- Olay listesi ve `data` alanlarının kesinleşmesi (taslak: `docs/domain-model.md`) (D-035)
- Üç onarım denemesi başarısız olunca kullanıcıya ne gösterilir (D-027); ön eleme işaretlerse akış durur ve gerekçe gösterilir (D-069), arayüzü T-035'te
- Bütçenin anlamı: kampanya harcama tavanı ve/veya ajan token tavanı
- promptfoo sürümü, `eval/cases/` vaka biçimi ve `make eval-record` akışı (D-061)
- Seed: 1000 kullanıcının "birkaç bin"e nasıl çıkarılacağı; kampanya stub'larının ne zaman ve hangi DB'ye yükleneceği
