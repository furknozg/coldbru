# Araştırma 4 — Veri platformu: migration, servisler arası iletişim, canlı durum, Türkçe eşleşme, seed, gateway

> **Durum:** Araştırma bulgusu, karar değil. Öneriler DECISIONS.md'ye ancak kullanıcı onayıyla girer.
> **Tarih:** 2026-10-27 · **Kaynak sorular:** THOUGHTS.md GOALS (gRPC, compiled SQL vs ORM, LISTEN/NOTIFY, canlı durum için WebSocket/event stream, p95 200 ms), diyagram (nginx apigw, "inter service comms grpc mi, bir haftaya fazla teknik borç mu?"), DECISIONS açık konuları (migration aracı, Türkçe şehir eşleşmesi, 1000 → birkaç bin kullanıcı).
> Ortam: Postgres 18.6 (alpine/musl), yerelde Go 1.27.0. Sürümler 2026-10-27'de GitHub release API'si veya dokümanlardan.

## Özet öneri

| Konu | Öneri |
|---|---|
| Migration | **goose v3** + `embed.FS`, servis başına dizin, servis açılışında advisory lock altında |
| Sürücü / SQL | **pgx v5** (`pgxpool`) + statik sorgular için **sqlc**; dinamik segment `WHERE`'i elle derleyici |
| Servisler arası | **REST/JSON** (stdlib `net/http`), gRPC değil |
| Canlı durum | **SSE** + `Last-Event-ID`, outbox tablosu + `LISTEN/NOTIFY` uyandırması, 1–2 sn yedek poll |
| Türkçe şehir | Kapalı sözlük + uygulama tarafı kanonik anahtar; SQL `city = $1` olarak kalır |
| Seed | Üreticiye `STUB_SCALE` çarpanı (varsayılan 1), demo için 3 |
| Gateway | Tek nginx: Vue statik + `/api/*` yönlendirme + SSE konumu; p95 200 ms gerçekçi |

## 1. Migration, sürücü, compiled SQL vs ORM

| Araç | Durum / uygunluk |
|---|---|
| **goose** (önerilen) | v3.29.0 (2026-10-11), ~aylık sürüm, en az Go 1.26. `NewProvider(dialect, *sql.DB, fs.FS, …)` `embed.FS` kabul eder; `WithSessionLocker` kilit açar; `-- +goose Down` dosya başına isteğe bağlı. `database/sql` ister; yalnızca migration bağlantısı için `pgx/v5/stdlib`. |
| golang-migrate | v4.20.1 (2026-09-09), ama önceki sürümle arası 284 gün, ~315 açık issue. Çalışır; daha çok kalıp kod, yavaş tempo. |
| tern (jackc) | v2.4.3, pgx-yerli, yalnızca Postgres; küçük topluluk. |
| Atlas | v1.3.0; güçlü diff araçları ama lisans karışık (Community Apache 2.0, varsayılan ikili EULA/MSA), Community'de `migrate down` yok. Bir hafta için ağır. |

- **Compose:** servisler, Postgres `healthy` olduktan sonra açılışta `Up` çağırır. Replika ölçeklenirse tek seferlik `migrate` işi; goose kilidi eşzamanlı açılışı güvenli yapar (`lock` paketinin imzası doğrulanmadı).
- **initdb ile ilişkisi (D-005):** servis rolü `CREATE DATABASE` çalıştıramaz. Bu yüzden initdb, boş `campaign_db` / `notification_db` / `analytics_db` veritabanlarını ve sahip rollerini de oluşturmalı. `customer_db` salt-okunur DWH olduğu için şeması ve seed'i initdb'de kalır; goose yeni DB'ler içindir.
- **pgx v5** v5.11.0 (2026-09-07). `database/sql` + lib/pq yalnızca zorunlu kalırsa.
- **sqlc** v1.31.1: `sql_package: pgx/v5`, `schema: <migrations dizini>`; goose dosyalarını ayrıştırır.
- **Compiled SQL vs ORM (THOUGHTS GOALS §3):** sqlc + düz SQL. İndeksler, stored procedure'ler ve NOTIFY trigger'ları migration SQL'inde yaşar. GORM yok; segment sorgusu zaten özel SQL.
- **Dinamik WHERE:** sqlc statiktir. `sqlc.narg` + `coalesce` yalnızca sabit opsiyonel filtreleri karşılar. AST için ~150 satırlık Go derleyicisi (beyaz liste, `$n` argümanlar, pgxpool). Sabit parçalar (izin, durum, `phone IS NOT NULL`) ve CRUD sqlc'de kalır.

## 2. Servisler arası iletişim

**Öneri: senkron REST/JSON** (Go 1.22+ metot desenleriyle stdlib `net/http`), paylaşılan Go struct'ları veya bir OpenAPI dosyası.

| Seçenek | Değerlendirme |
|---|---|
| **REST/JSON** | Sıfır araç zinciri. Kural AST'si zaten JSON, olduğu gibi geçer. `curl` ile hata ayıklanır. |
| gRPC / connect-go | connect-go v2.0.2 (2026-10-21); tek handler gRPC, gRPC-Web ve Connect JSON sunar. Bedel: `buf`, `protoc-gen-go`, `protoc-gen-connect-go`, Docker/CI'da codegen; özyinelemeli AST `oneof` veya `Struct` ile modellenmeli. **Bir haftalık yapıma uymuyor**; birkaç bin satırda tipli sözleşme ve streaming kazancı küçük. |
| Yalnızca GraphQL | Tarayıcı tarafı için iyi (Araştırma 2); iç çağrılarda sorgu maliyet kontrolü ve AST için JSON scalar gerekir. |
| Olay güdümlü okuma modeli / mantıksal replikasyon | **Reddet:** gönderim anında bayat izin, PII'nin `campaign_db`'ye kopyalanması, CDC kurulumu (`wal_level=logical`), customer service sınırının ihlali. |

**Customer service uç noktaları:**

| Uç nokta | Gövde / yanıt |
|---|---|
| `POST /v1/segments/count` | kural → `{total, email_sendable, sms_sendable}` |
| `POST /v1/segments/preview` | kural + limit → ~10 örnek |
| `POST /v1/segments/resolve` | gönderimde keyset sayfalı (`after`, `limit` 1000) → yalnızca o an izinli, aktif, bounce etmemiş müşteriler için `{customer_id, first_name, language, email\|phone}` |
| `POST /v1/consents/check` | `{customer_ids, channel}`; her worker batch'inde gönderimden hemen önce |

- **İzin:** tek doğruluk kaynağı customer service. `campaign_db` `customer_id`, kural anlık görüntüsü ve `rule_version` tutar; PII veya izin bayrağı kopyalanmaz.
- **Boyut/hız:** 4k alıcı × ~500 B ≈ 2 MB; 1000'lik sayfalar önemsiz. Sayım ve önizleme indeksli tek sorgu, onlarca ms.
- İleride `consent.changed` olayı kuyruğa eklenebilir; sözleşme REST olarak kalır.
- İç çağrılar nginx'ten geçmez. Handler'lar ince tutulursa gRPC'ye sonradan geçmek ucuz.

## 3. Canlı durum: teklif ilerlemesi ve gönderim

**Öneri: SSE**, `Last-Event-ID` ile sürdürme; besleme outbox tablosu + `LISTEN/NOTIFY`; 1–2 sn yedek poll.

| Seçenek | Bedel |
|---|---|
| **SSE** | Tek yönlü akış ajan adımlarına, kitle sayılarına, metin parçalarına ve teslim sayılarına uyar. Düz HTTP, otomatik yeniden bağlanma, nginx'te basit. `EventSource` yalnızca GET ve özel header yok; cookie veya kısa ömürlü token. |
| WebSocket | Yalnızca istemciden sunucuya mesaj gerekiyorsa. Sürdürme ve ping'leri elle kurarsın; nginx'te Upgrade ayarı. |
| Toplu durumu poll etmek | En basit, 1–2 sn'de iyi bir yedek; daha yüksek gecikme ve yük. |

- **Go:** `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no`. `rc := http.NewResponseController(w)`; `rc.SetWriteDeadline(time.Time{})` ve her olaydan sonra `rc.Flush()`. ~15 sn'de bir `: ping` yorumu, `retry: 2000`, `r.Context().Done()`'da çık, gzip middleware'ini atla.
- **Sürdürme:** her olay `id: <outbox bigserial>` taşır; yeniden bağlanınca `Last-Event-ID` okunur, `WHERE id > $1` tekrar oynatılır, sonra canlıya geçilir.
- **Besleme:** iş transaction'ı outbox satırı ekler ve `pg_notify('events', id::text)` çağırır (yük yalnızca id; sınır 8000 bayt). Bildirimler commit anında, commit sırasıyla gelir; aynı transaction'daki aynı yükler birleşir. `WaitForNotification` için havuzdan değil ayrı bir `pgx.Conn`; süreç içinde dağıtılır. NOTIFY veritabanı başınadır; her servis kendi DB'sinden akıtır, servisler arası sayılar kuyruktan gelir.
- **Birleştirme:** kanal başına 100 msg/s'de mesaj başına olay değil, 250–500 ms'de bir toplu anlık görüntü. `copy_delta` olayları hemen gider.
- **Vue:** VueUse `useEventSource(url, ['step','audience','copy','delivery'], {autoReconnect: {retries: -1, delay: 1000}})` üzerine `useCampaignStream(id)` composable'ı. POST tarzı akış (metin üretimi) için `fetch` + `ReadableStream`.
- **Bağlantı sınırı:** HTTP/1.1'de alan başına ~6 SSE bağlantısı; sayfa başına tek çoğullanmış `/events` akışı.
- **nginx (SSE):**
  ```nginx
  location /api/stream/ {
      proxy_pass http://proposal:8080;
      proxy_buffering off; proxy_cache off; gzip off;
      proxy_set_header Connection "";
      proxy_read_timeout 1h;
  }
  ```

## 4. Türkçe büyük/küçük harf duyarsız şehir eşleşmesi

`customer_db` üzerinde salt-okunur sorgu sonuçları:

| Kontrol | Sonuç |
|---|---|
| `datcollate, datctype, datlocprovider` | `en_US.utf8`, `en_US.utf8`, `c` (libc) |
| `lower('İSTANBUL')` | `istanbul` |
| `upper('istanbul')` | `ISTANBUL` |
| `'İstanbul' ILIKE 'istanbul'` | **true** |
| `'ISPARTA' ILIKE 'ısparta'` (noktasız ı) | false |
| `lower('I' COLLATE "tr-x-icu")` | `ı` |
| `upper('i' COLLATE "tr-x-icu")` | `İ` |
| `'Istanbul' = 'İstanbul' COLLATE "tr-x-icu"` | false |
| `casefold('İSTANBUL' COLLATE pg_unicode_fast)` | `i̇stanbul` (i + U+0307 birleşen nokta) |
| `casefold('İSTANBUL' COLLATE "tr-x-icu")` | `istanbul` |
| `İstanbul` sayısı | 207 (`=`, `ILIKE`, `lower()=`, `tr-x-icu` aynı) |
| Mevcut collation'lar | `pg_c_utf8`, `pg_unicode_fast`, `tr-x-icu`, `und-x-icu` (toplam 908 ICU) |
| Eklentiler | `citext`, `unaccent` mevcut, kurulu değil |

- `docs/database-architecture.md`'deki "İstanbul / istanbul eşleşmez" varsayımı **burada yanlış çıktı**: Alpine'in musl libc'inde gerçek `en_US` locale'i yok, `datlocprovider = 'c'`. Bu davranış kırılgan ve glibc imajında denenmedi.
- Gerçek Türkçe kurallar, ASCII yazılmış "Istanbul"u (LLM'den veya kullanıcıdan gelmesi çok olası) `ıstanbul`'a çevirir ve eşleşmez. Bu yüzden `tr-x-icu` collation'ı bu girdi için **yanlış**.

**Öneri: kapalı sözlük + uygulama tarafı kanonik anahtar; DB collation'ına güvenme.**

1. Customer service açılışta `SELECT DISTINCT city`'yi (~33 değer) `key(city) → kanonik şehir` haritasına yükler.
2. `key()`: `ı İ I` → `i`, sonra NFD, birleşen işaretleri at, küçült. Dilden bağımsız ve simetrik. Go'da `x/text` ile doğrulandı: `İstanbul`, `Istanbul`, `istanbul`, `ISTANBUL`, `ıstanbul` hepsi `istanbul`; `ESKISEHIR` = `Eskişehir`; `DIYARBAKIR` = `Diyarbakır`.
3. Kural doğrulayıcı AST'deki şehir değerini kanonik değere çevirir ya da "bunu mu demek istediniz" ile reddeder. SQL `city = $1` olarak kalır: tam, indeks dostu, collation'dan bağımsız.

**Tuzak:** `cases.Lower(language.Turkish)` "Istanbul" için `ıstanbul` verir; `cases.Fold()` Türkçe işlemez (`i̇stanbul`). Kendi eşleyicini kullan.

Alternatifler: mevcut libc ile `ILIKE` (İ'de çalışır, ı'da çalışmaz, taşınabilir değil); ICU nondeterministic collation (B-tree tekilleştiremez, performans bedeli, `CREATE COLLATION` gerekir, ı/I sorunu sürer); `citext` (`LC_CTYPE`'a göre küçültür, Unicode açısından gerçekten duyarsız değil); PG18 `casefold()` / `pg_unicode_fast` (tam Unicode eşleme, Türkçe uyarlaması yok).

## 5. Seed'i 1000'den birkaç bine çıkarmak

**Öneri:** `generate_stub_data.py`'ye `STUB_SCALE` ortam değişkeni (varsayılan 1); demo için 3 (~3000 kullanıcı). Araştırmacı bir karalama kopyasında prototipledi:

- `range(n_src * SCALE)` döngüsü, `row = src_rows[idx % n_src]`, kimlikler `CUS-001001`'den. Klonlarda soyadı başka bir kaynak satırdan (`src_rows[(idx*7919+13) % n_src]`).
- `SCALE=1`'de 9 CSV'nin hepsi bayt bayt aynı kaldı (`cmp`), huni 195 → 90 → 49 → 33.
- **Prototipteki hata:** e-posta slug'ı değişti ama `Last Name` kolonu ham kaldı; klonlar için satır kopyalanıp kolon güncellenmeli.
- Klonlar kart/IBAN/CVV'yi de çoğaltır; DB'ye yüklenmese de klonlarda boşaltılmalı.
- `SCALE > 1`'de tek RNG akışı kaydığı için ilk 1000 kullanıcının ayrıntıları da değişir; D-001'deki sayılar güncellenmeli. İlk 1000'i sabit tutmak kohort başına RNG ister (daha büyük refactor).

| Tablo | ×1 | ×3 | ×5 |
|---|---|---|---|
| dim_customer | 1000 | 3000 | 5000 |
| fact_order | 3923 | 11.992 | 20.262 |
| fact_order_item | 9170 | 28.108 | 47.582 |
| fact_event | 44.114 | 137.866 | 230.598 |
| fact_cart_item | 3840 | 12.064 | 19.840 |
| fact_consent | 2219 | 6693 | 11.195 |
| Örnek brief hunisi (SMS / email) | 33 / 33 | 109 / 117 | 160 / 175 |

- Üretim ×3'te 1.7 sn. Yükleme ölçülmedi; doğrusal tahmin ×3'te ~8–10 sn, ×5'te ~15 sn. Healthcheck `start_period` 60 sn yeterli.
- Olay CSV'si ×3'te ~12 MB, ×5'te ~20 MB. ×3 commit edilebilir ya da `make seed` ile üretilebilir.

## 6. nginx gateway ve p95 ≈ 200 ms

- Tek nginx konteyneri: `/` Vue build'i, `/api/proposals/` → proposal, `/graphql` → customer (açılırsa), `/api/stream/` → §3'teki SSE konumu.
- Servisler arası trafik compose DNS adlarıyla gider, nginx'ten geçmez. Kullanıcı yolu: tarayıcı → nginx → proposal → customer → Postgres.
- nginx'in varsayılan `proxy_read_timeout`'u 60 sn. LLM uç noktaları 202 dönüp ilerlemeyi stream etmeli, isteği açık tutmamalı.
- **p95 200 ms gerçekçi mi?** LLM dışı uç noktalarda evet: nginx yerelde 1 ms'nin çok altında ekler; 3–5k satırda indeksli sayım/önizleme tek ila onlarca ms. Riskler: soğuk havuz, servisler arası N+1 çağrılar, mesaj başına izin kontrolü (batch yapılmalı).
- **Ölçüm:** k6 eşiği `http_req_duration: [{threshold: 'p(95)<200', abortOnFail: true, delayAbortEval: '10s'}]`, uç nokta etiketleriyle; servis başına Prometheus histogramı `http_request_duration_seconds` (5 ms–1 sn kovalar); nginx log alanları `$request_time`, `$upstream_response_time`. SSE için ilk olaya kadar geçen süre ayrı ölçülür.

## Doğrulanamayanlar

- DDL izni olmadığı için nondeterministic collation, `unaccent`, `citext` denenmedi; ICU ı/İ iddiası Unicode collation bilgisine dayanıyor.
- goose Postgres session locker API'si, connect-go v1/v2 önerisi, squirrel/goqu sürümleri doğrulanmadı; nginx yapılandırmaları denenmedi.
- Seed yükleme süresi ölçüm değil, tahmin. Alpine/musl davranışı Docker Hub notu ve sorgu sonuçlarından çıkarım; glibc imajı denenmedi.
- Atlas lisans metni v1.2.0 ile v1.3.0 arasında değişti.

## Kaynaklar

- Migration: https://api.github.com/repos/pressly/goose/releases · https://pkg.go.dev/github.com/pressly/goose/v3 · https://api.github.com/repos/golang-migrate/migrate/releases · https://github.com/jackc/tern · https://api.github.com/repos/ariga/atlas/releases · https://www.atlasgo.io/community-edition
- pgx / sqlc: https://api.github.com/repos/jackc/pgx/releases · https://api.github.com/repos/sqlc-dev/sqlc/releases · https://docs.sqlc.dev/en/latest/howto/ddl.html · https://docs.sqlc.dev/en/latest/reference/config.html · https://docs.sqlc.dev/en/latest/howto/named_parameters.html
- RPC: https://connectrpc.com/docs/go/getting-started · https://api.github.com/repos/connectrpc/connect-go/releases · https://nginx.org/en/docs/http/ngx_http_grpc_module.html
- SSE / NOTIFY: https://html.spec.whatwg.org/multipage/server-sent-events.html · https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events · https://www.postgresql.org/docs/18/sql-notify.html · https://pkg.go.dev/github.com/jackc/pgx/v5#Conn.WaitForNotification · https://vueuse.org/core/useEventSource/
- nginx: https://nginx.org/en/docs/http/ngx_http_proxy_module.html · https://nginx.org/en/docs/http/websocket.html
- Collation: https://www.postgresql.org/docs/18/release-18.html · https://www.postgresql.org/docs/18/collation.html · https://www.postgresql.org/docs/18/citext.html · https://pkg.go.dev/golang.org/x/text/cases · https://hub.docker.com/_/postgres
- k6: https://grafana.com/docs/k6/latest/using-k6/thresholds/
