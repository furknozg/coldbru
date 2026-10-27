# Araştırma 5 — Prompt sürümleme altyapısı: dosya, DB kayıt defteri, harici araçlar, değerlendirme

> **Durum:** Araştırma bulgusu, karar değil. Öneriler DECISIONS.md'ye ancak kullanıcı onayıyla girer.
> **Tarih:** 2026-10-27 · **Soru:** Bundan sonra LLM prompt'ları hangi altyapıyla sürümlensin?
> **Girdi:** D-002, D-009, D-022, D-025, D-026, D-027, D-035, D-038–D-047, D-051, D-052, D-054; [03-llm-agent-flow.md](03-llm-agent-flow.md); [domain-model.md](../domain-model.md) §3, §6.
> **Not:** `[D]` birincil kaynakta (resmi doküman, repo) doğrulandı, `[İ]` ikincil kaynaktan (haber, blog, karşılaştırma sitesi), `[?]` doğrulanamadı.

## Özet öneri

| Konu | Öneri |
|---|---|
| Kaynak gerçek | Prompt'lar repoda düz dosya, `go:embed` ile binary'ye gömülür; PR'da incelenir, git geçmişi sürüm geçmişidir |
| Biçim | Düz metin (şablon motoru yok); dinamik veri sistem prompt'una değil kullanıcı mesajına girer |
| Sürüm kimliği | `ad@semver` (elle) + `sha256` (otomatik, istek önekinden); hash değişip semver artmadıysa test kırılır (commit'lenen kilit dosyası) |
| Kayıt | Her `llm_call` satırı `prompt_name`, `prompt_version`, `prompt_sha256` taşır; `proposal.token_completed` olayı ve `llm_usage` aynı alanları alır |
| DB kayıt defteri | Şimdilik hayır. İsteğe bağlı ikinci adım: açılışta doldurulan ekleme-yalnız `prompt_snapshot` tablosu |
| Harici araç (Langfuse, PromptLayer, Braintrust …) | Bu case için hayır: `docker compose up` ve anahtarsız test koşullarını bozar, Go desteği zayıf veya ücretli |
| Değerlendirme | Üç katman: `make test` (anahtarsız; kilit + altın dosya tazeliği), `make eval` (kayıtlı yanıt tekrarı), `make eval-live` (`live` tag, elle) |
| promptfoo | Zorunlu değil; Go doğrulayıcılarını yeniden yazdırır. Go içinde küçük bir eval koşucusu daha uyumlu |
| Arayüz | Teklif ayrıntısında "AI notları" altında `segment@1.2.0` rozeti; yalnız okunur |

## 1. Depo içi dosyalar ve `go:embed`

**Mekanik `[D]`:** `//go:embed` dosyaları derleme anında `string`, `[]byte` veya `embed.FS`'ye alır. `embed.FS` salt okunurdur, eşzamanlı kullanıma güvenlidir. Desenler paket dizinine göredir; `.` ve `_` ile başlayan adlar `all:` olmadan atlanır, eşleşmeyen desen derlemeyi kırar. Prompt değişikliği = yeniden derleme + yeniden dağıtım; docker compose demosunda sorun değil.

**Önerilen yerleşim (taslak):**

```text
internal/proposal/prompts/
  prompts.go                 // //go:embed, Bundle yükleyici, hash hesabı
  prompts.lock.json          // üretilir, commit'lenir (D-051 ile aynı ilke)
  segment/prompt.md          // sistem metni
  segment/tool.json          // propose_segment / count_audience tel şeması
  segment/meta.yaml          // version: 1.2.0, requires_dsl: "0.1", model, effort
  copy/{prompt.md,schema.json,meta.yaml}
  screen/{prompt.md,meta.yaml}        // Haiku 5.5 ön eleme (D-026)
  repair/prompt.md                    // onarım denemesi (D-027)
  fragments/dsl-grammar.md            // docs/coldbru-gqdsl.md'den, segment'e eklenir
```

### 1.1 Şablon motoru gerekmez, zararlı bile olabilir

- Prompt'un modele **öğretmesi gereken** metinde `{{first_name}}` gibi yer tutucular var (D-009 kişiselleştirme). Go `text/template` aynı sınırlayıcıyı kullanır; bu makinedeki Go 1.27.0 ile denendi `[D]`: `Parse("Merhaba {{first_name}}")` → `function "first_name" not defined`. Her yer tutucu `{{"{{"}}first_name{{"}}"}}` gibi kaçırılmalı; hata kaynağı.
- Önbellek (§1.3) için sistem metni **bayt bayt sabit** olmalı. Değişkensiz düz dosya bunu doğal olarak sağlar. Brief, bağlam notları, onaylı önceki adımlar, hedef dil ve kanal zaten çağrıya özgü veri; 03 §7'deki gibi JSON kodlu bir "veri" bloğu olarak **kullanıcı mesajına** girer ve `encoding/json` ile kurulur.
- Tek istisna kataloğu ve DSL dilbilgisini prompt'a katmak; bunlar derleme zamanı birleştirmesi (`fragments/` + sabit sıra) ve açılışta bir kez yapılır. `strings.Join` yeter; `html/template`/`text/template` gerekmez. D-044'e göre katalog build'de üretilir, yani prompt hash'ine otomatik girer: Customer şeması değişirse prompt sürümü de değişmek zorunda kalır.

### 1.2 Sürüm kimliği: semver mi, hash mi?

| Seçenek | Artı | Bedel |
|---|---|---|
| Yalnız içerik hash'i | Otomatik, unutulamaz, tekrarlanabilirlik kanıtı | İnsan okuyamaz; "1.2'den 1.3'e ne değişti" sorusu cevapsız |
| Yalnız elle semver | Okunur, release notu bağlanır | Unutulur; metin değişir sürüm değişmez, kayıt yalan söyler |
| **İkisi birden (öneri)**: `segment@1.2.0` + `sha256:ab12…` | Okunur ad + değiştirilemez kanıt; tutarsızlık testle yakalanır | Küçük bir kilit dosyası ve test |

**Kilit dosyası mekaniği:** `prompts.lock.json` her paket için `{name, version, sha256}` tutar. `make generate` (D-051'deki hedef) yeniden üretir. `TestPromptLockMatchesEmbedded` hash'i yeniden hesaplar: hash kilitle uyuşmuyorsa "çalıştır `make generate`" der; hash değişti ama `meta.yaml` `version` aynıysa "sürümü artır" der. PR'da kilit diff'i prompt değişikliğini görünür kılar.

**Semver anlamı (öneri):** MAJOR = çıktı sözleşmesi değişti (tool/JSON şeması, DSL sürümü; kod ve altın dosyalar da değişir). MINOR = davranış değişikliği (talimat, örnek). PATCH = davranışı etkilemeyen düzeltme (yazım); hash yine değişir, bu yüzden bump yine zorunlu. `requires_dsl: "0.1"` (D-045) ayrıştırıcının sabitiyle testte karşılaştırılır; DSL 0.2 geldiğinde eski prompt sessizce kalmaz.

**Hash neyin üzerinden:** gerçekten gönderilen önek: `system` + `tools` + `output_config.format` şeması, kanonik biçimde (anahtar sıralı JSON) + `model` + `effort`. Böylece "aynı hash = aynı önbellek önekine uyumlu istek" olur (§1.3). Kullanıcı mesajı hash'e girmez.

### 1.3 Prompt caching ile etkileşim `[D]`

- Önek sırası `tools → system → messages`; bir düzeydeki değişiklik o düzeyi ve sonrasını geçersiz kılar. Araç tanımı (ad, açıklama, parametre) değişirse **tüm** önbellek düşer; sistem değişirse sistem + mesajlar; `tool_choice` değişirse mesajlar.
- `output_config.format` değişikliği o konuşma dizisinin prompt önbelleğini geçersiz kılar; ayrıca derlenmiş dilbilgisi önbelleği 24 saat tutulur, yapı değişince ilk istek yavaştır. Yalnız `name`/`description` değişikliği dilbilgisi önbelleğini bozmaz, prompt önbelleğine etkisi dokümanda belirsiz.
- Sonuç: **her prompt sürüm artışı bir kez soğuk önbellek demektir.** Sonnet 5.5 5 dk yazma $2,50/MTok, okuma $0,10/MTok (D-026'daki fiyatlarla tutarlı); yani sürüm başına bir kez önek boyu kadar yazma maliyeti, ihmal edilebilir. Asıl risk **istemeden** önek değiştirmek: sistem metnine zaman damgası/kampanya kimliği koymak ya da araç şemasını belirsiz sırayla serileştirmek. Bu yüzden: (a) dinamik veri yalnız kullanıcı mesajında (D-039 zaten tarih aritmetiğini LLM'e bırakmıyor, prompt'ta tarih gerekmez); (b) `tool.json`/`schema.json` bayt olarak gömülür, kanonikleştirilmiş hali hem hash'e hem isteğe girer; (c) tek bir çağrı yolu (`prompts.Bundle` → istek kurucu).
- En az önbelleklenebilir uzunluk Sonnet 5.5 ve Haiku 5.5 için 512 token; altında önbellek sessizce çalışmaz, hata yoktur. Segment prompt'u (dilbilgisi + katalog) bunu rahat aşar; `screen` ve belki `copy` aşmayabilir. `usage.cache_creation_input_tokens` ve `cache_read_input_tokens` ikisi de 0 ise önbelleklenmemiştir: **`llm_call` bu iki sütunu da tutmalı** (domain-model taslağında yalnız `cache_read_tokens` var, `cache_creation_tokens` yok; 03 §9'da vardı).
- **Önbellek tanılama (cache diagnostics) GA `[D]`:** isteğe `diagnostics.previous_message_id` verilince yanıt, önek ayrışırsa `system_changed`/`tools_changed`/`messages_changed` döndürür. `live` testte aynı sürümün iki ardışık çağrısında ayrışma olmadığını doğrulamak için uygun. Doküman Go örneğini `Beta.Messages` altında gösteriyor; D-047'deki v2.3.0'da düz `Messages` üzerinde olup olmadığı `[?]`.

### 1.4 Dosya yaklaşımının sınırları

Prompt'u kod değiştirmeden canlıda düzenlemek yok; A/B (bonus) için iki sürüm gömülür, seçim yapılandırma veya `campaign_id` hash'iyle yapılır ve `llm_call`'a yazılır. Teknik olmayan biri prompt düzenlemeyecek; bu case'te tek ekip var.

## 2. proposal_db içinde DB kayıt defteri

Taslak: `prompt_version(id, name, version, body, status, created_at)` + `prompt_active(name → version_id)`; çağrıda aktif sürüm okunur, `llm_call.prompt_version_id` FK.

| Boyut | Dosya + `go:embed` | DB kayıt defteri |
|---|---|---|
| PR'da inceleme, diff | Doğal (git) | Yok; ayrıca "prompt değişikliği onayı" akışı kurulmalı |
| Kodla birlikte tutarlılık | Prompt, ayrıştırıcı (D-038), tool şeması ve katalog aynı commit'te | Prompt DB'de, DSL dilbilgisi kodda; bağlanma kontrolü yok, sürümler birbirinden kayar |
| Tekrarlanabilirlik | Commit + hash | Satır değişmezse iyi; "aktif" işaretçisi değişkendir, çağrıya sürüm kimliği yazmak şart |
| Dağıtım bağı | Değişiklik = yeniden derleme | Yeniden dağıtımsız değişim, canlı kampanya sırasında davranış kayar |
| Test | `make test` anahtarsız, DB'siz | Prompt tohumlanmalı; testcontainers gerekir (D-025/D-051) |
| Kademeli açma / A-B | Elle, yapılandırmayla | Doğal (ağırlık, yüzde) |
| Maliyet | Sıfır tablo | Yeni tablolar: CLAUDE.md kural 7 gereği şema kararı + goose migration (D-020) |

Case'in puanlama ölçütü (CLAUDE.md): doğrulama hattı ve AI çıktısını nasıl doğruladığın. Çalışma zamanında düzenlenebilir prompt bunu zayıflatır; dosya yaklaşımı güçlendirir. **Melez (isteğe bağlı, sonra):** kaynak gerçek dosya kalır, açılışta `prompt_snapshot(sha256 PK, name, version, body, first_seen_at)` ekleme-yalnız tablosuna `INSERT … ON CONFLICT DO NOTHING` yapılır; `llm_call.prompt_sha256` buna bağlanır. Kazanç: eski bir çağrının tam metnini git'e bakmadan (ve dosya silinse bile) gösterebilmek; `context_note`/`llm_call` ekleme-yalnız ilkesiyle (D-022) uyumlu. Bedel: bir tablo + migration kararı. İlk sürüm için şart değil.

## 3. Harici prompt yönetim araçları

| Araç | Barındırma / lisans | Go | Maliyet | Docker-compose demosu | Kilitlenme |
|---|---|---|---|---|---|
| **Langfuse** | Açık kaynak, MIT (`ee` klasörleri hariç) `[D]`; Ocak 2026'dan beri ClickHouse'un parçası `[D]`; self-host: Web + Worker + Postgres + ClickHouse + Redis + S3 `[D]` | Resmi Go SDK yok; Python/JS var. REST `GET /api/public/v2/prompts/{ad}` (label/version) `[D]`; OTLP ile iz + `langfuse.prompt.name/version` özellikleri `[D]`; topluluk istemcileri (ör. `optible/langfuse-go` v1.3.2) `[İ]` | Self-host ücretsiz; `ee` özellikleri lisans anahtarı ister | Mümkün ama altı ek servis; compose için "tek VM, HA/yedek yok" notu `[D]`; 4 servis + Postgres + NATS yığınının üstüne ağır | Düşük (açık kaynak, dışa aktarma) |
| **PromptLayer** | SaaS; self-host/tek kiracı yalnız Enterprise `[İ]` | Doküman örnekleri Python/JS `[D]` | Ücretsiz: 5 kullanıcı, 2,5 bin istek/ay; Pro $49 `[İ]` | Hayır (SaaS, anahtar gerekir) | Orta |
| **Braintrust** | SaaS; self-host Enterprise `[İ]` | Go: `client.LoadPrompt()`; Go'da her çağrıda sunucudan çeker, önbellek yok `[D]` | Starter ücretsiz, Pro ~$249 `[İ]` | Hayır | Orta |
| **Humanloop** | Eylül 2025'te kapandı `[İ]` | — | — | — | Seçenek değil |
| **Helicone** | Mintlify satın aldı (Mart 2026), bakım modu `[İ]` | Proxy | — | — | Seçenek değil |
| **Anthropic Console** | Workbench'te paylaşılan prompt'ların revizyon geçmişi ve geri yükleme `[D]`; Evaluate aracı: yan yana karşılaştırma, 5'li puan, prompt sürümü `[D]` | Yalnız arayüz. API referans dizininde (llms.txt) kayıtlı-prompt kaynağı bulunmadı `[D]` | Normal API kullanımı | Hayır (hesap + giriş) | — |

**Değerlendirme:** (1) Brief "testler gerçek API key olmadan çalışmalı" ve "tek komut kurulum" diyor; prompt'u çalışma zamanında dış servisten almak ya bu koşulu bozar ya da yedek (gömülü kopya) gerektirir; gömülü kopya zaten dosya yaklaşımıdır. (2) Go için ya topluluk istemcisi ya OTel yolu var; resmi, bakımı garanti bir prompt istemcisi yok. (3) İki araç kapanmış/bakım modunda; piyasa kaymış. (4) Langfuse'un OTel özellik adları (`langfuse.prompt.name`, `langfuse.prompt.version`) adlandırma için ödünç alınabilir; sonradan geçiş ucuz kalır. Anthropic Console, prompt'u **keşfetmek** (Workbench, Evaluate) için kullanılabilir ama kaynak gerçek olamaz: sürümü git'te değil, API ile çekilemiyor.

## 4. Değerlendirme/regresyon: sürümleme ancak ölçülebilirse anlamlı

**Katman 1 — `make test` (her zaman, anahtarsız):**
- `TestPromptLockMatchesEmbedded` (§1.2), `requires_dsl` uyumu, `tool.json`/`schema.json` geçerli JSON Schema mı, prompt'ta bilinmeyen yer tutucu yok mu.
- Senaryolu sahte + `httptest` Messages sunucusu (D-025) prompt metnini kontrol etmez; **gelen istekte** `system`/`tools` baytlarının `prompts.Bundle` ile aynı olduğunu ve `cache_control` konumunu doğrulayan bir test eklenir (yanlışlıkla dinamik veri sızmasını yakalar).
- Altın dosyalar (03 §8): kayıtlı **gerçek** yanıtlar tekrar oynatılır; her dosya başlığında kaydedildiği `prompt_sha256` bulunur. Dosyadaki hash güncel prompt'unkiyle uyuşmuyorsa `TestGoldenFreshness` başarısız olur ve "yeniden kaydet: `make eval-record`" der (kayıt `live` tag ister). Dürüst sınır: tekrar oynatma yeni prompt'un davranışını kanıtlamaz, yalnız ayrıştırıcı/doğrulayıcı hattını gerçek çıktıya karşı korur; bu yüzden güncellik kontrolü şart.

**Katman 2 — `make eval` (anahtarsız, kayıtlı):** `eval/cases/*.yaml` (brief → beklenen DSL ya da beklenen sayım; örnek brief 33 kişi, D-039) kayıtlı yanıtlarla tüm doğrulama hattından geçer.

**Katman 3 — `make eval-live` (`live` tag, `ANTHROPIC_API_KEY` ister, elle):** aynı vakalar gerçek modele gider; her vaka N kez; kontrol **kodla**: DSL ayrıştırılıyor mu, katalogda mı, SQL sayımı beklenenle uyuşuyor mu, SMS ≤ 160 / konu ≤ 80 (D-028), yer tutucular beyaz listede mi, enjeksiyon brief'i reddediliyor mu. Sonuç `eval/results/<ad>@<sürüm>.json` olarak yazılır ve prompt sürüm artışı olan PR'a eklenir. Kaba maliyet tahmini (varsayımla: ~6 bin token önbellekli önek, ~1 bin token çıktı, 30 vaka): okuma $0,10/MTok + çıktı $10/MTok ≈ $0,3 / tur. Tahmin, ölçüm değil.

**promptfoo:** MIT; Mart 2026'da OpenAI satın aldı, lisansın değişmeyeceği bildirildi `[İ]`; son sürüm 0.125.1 (2026-10-25) `[D]`; CI için Node ≥ 22.22 (24 LTS önerilir), `PROMPTFOO_CACHE_PATH` önbelleği, `--fail-on-error`/jq ile eşik `[D]`; `echo` sağlayıcısı API çağırmaz ama `llm-rubric` gibi model değerlendirmeli kontroller yine sağlayıcı ister `[D]`. Makefile'dan `npx promptfoo eval` ile çalışabilir (repoda `web/` için Node var). **Uyumsuzluk:** promptfoo prompt+model'i test eder, Go'daki DSL ayrıştırıcıyı/katalog doğrulamasını/SQL sayımını çalıştıramaz (özel sağlayıcı/HTTP köprüsü doğrulanmadı `[?]`); kurallar YAML'da yeniden yazılır, iki doğruluk kaynağı olur. Brief'in vurguladığı "backend doğrulaması"nı doğrudan sınayan Go içi koşucu daha tutarlı. promptfoo, yalnız ton/çok-model karşılaştırması için isteğe bağlı eklenebilir.

**LLM-as-judge:** yalnız öznel ölçüt için ("samimi ama kısa"). Anthropic dokümanı üretenden farklı model ile notlamayı en iyi uygulama olarak yazıyor `[D]`; Haiku 5.5 hakem olabilir. Katı kurallar (uzunluk, DSL geçerliliği) hakeme bırakılmaz. Önerilen: ilk sürümde yok, bonus.

**Kapı:** CI henüz yok (bonus). Şimdilik kural: `prompts/**` değişen commit/PR'da (a) `make test` yeşil, (b) kilit güncel, (c) `make eval-live` sonucu eklendi ve önceki sürümden kötü değil, (d) gerekçe prompt dizinindeki `CHANGELOG.md` satırı veya DECISIONS kaydı olarak yazıldı. CI gelirse (a)–(b) otomatik, (c) manuel iş akışı.

## 5. Sürüm sistemde nasıl akar

| Katman | Değişiklik (öneri) |
|---|---|
| `llm_call` (proposal_db) | `prompt_name`, `prompt_version`, `prompt_sha256` (NOT NULL); ayrıca `cache_creation_tokens`. `purpose` ile birlikte onarım denemesi kendi paketini (`repair@x`) yazar. Ekleme-yalnız kalır (D-022). Şema değişikliği olduğu için CLAUDE.md kural 7 → kullanıcı kararı |
| `proposal_variant` | Yeni sütun yok. Varyantın hangi prompt'la üretildiği `llm_call.variant_id` üzerinden bulunur (kabul edilen son deneme). "Kabul edilen deneme" işareti için `llm_call.outcome` (`accepted`/`validation_failed`/`refusal`) gerekebilir (açık nokta) |
| `evt.proposal.token_completed.v1` | `data`'ya `prompt_name`, `prompt_version` eklenir. Sürüm içinde yalnız alan eklenir kuralı (domain-model §6, D-035) bunu `.v1`'de mümkün kılar; Analytics bilinmeyen alanı yok sayabilir |
| Analytics (`llm_usage`) | `prompt_version` sütunu → sürüm başına maliyet, önbellek isabeti (`cache_read/(cache_read+cache_creation+input)`), onarım oranı. "Hangi prompt sürümü daha ucuz/az hata" panosu için asıl kazanç burada |
| OpenAPI / UI | Teklif ayrıntı yanıtında `generated_by: [{purpose, prompt_version, model}]`; Vue'da kart altında küçük "AI notları" açılır alanı, yalnız okunur. Düzenleme yok |
| Günlük/metrik | `slog` alanı `prompt_version`; Prometheus sayacına etiket olarak **sürüm değil** `prompt_name` (kardinalite) |
| İnceleme görüşmesi | `ai-sessions/` + DECISIONS: her `prompts/**` değişikliği için neden, eval sonucu ve hash. `llm_call` satırından geriye doğru: sürüm → git commit → eval sonucu zinciri anlatılabilir |

## Doğrulanamayanlar

- Anthropic'in kayıtlı/şablon prompt'u API'den çağırma özelliği: `llms.txt` dizininde yok, Console'da var olabilecek bir şey için erişimim yok. Yokluğu kesin değil.
- Console Evaluate aracının güncel ayrıntıları (yalnız arama özeti ve 2024 duyurusu); Humanloop'un ne olduğu (kaynaklar çelişiyor: ekip Anthropic'e geçti, varlıkların devri belirsiz).
- Helicone bakım modu, PromptLayer ve Braintrust fiyat/self-host bilgileri ikincil kaynaklardan (rakip ve toplayıcı siteler); Langfuse sürüm numarası ve kaynak ihtiyacı okunamadı; topluluk Go istemcilerinin bakım durumu bakılmadı.
- promptfoo'nun özel/exec sağlayıcısı ve HTTP köprüsüyle Go hattını çağırması; OpenAI satın almasının ayrıntıları haberlerden.
- Cache diagnostics'in Go SDK v2.3.0'da düz `Messages` altında olup olmadığı; `output_config.format` içinde yalnız `description` değişikliğinin prompt önbelleğini düşürüp düşürmediği (doküman belirsiz).
- Hash belirlenimciliği (`go:embed` + kanonik JSON) ve `text/template` dışındaki her şey kodla denenmedi; eval maliyeti varsayımsal.

## Kaynaklar

- Prompt caching, araç + önbellek, tanılama: https://platform.claude.com/docs/en/build-with-claude/prompt-caching · https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-use-with-prompt-caching · https://platform.claude.com/docs/en/build-with-claude/cache-diagnostics
- Yapılandırılmış çıktı ve önbellek: https://platform.claude.com/docs/en/build-with-claude/structured-outputs
- Anthropic değerlendirme: https://platform.claude.com/docs/en/test-and-evaluate/develop-tests · https://platform.claude.com/docs/en/test-and-evaluate/eval-tool · https://claude.com/blog/evaluate-prompts · Console paylaşımı: https://support.anthropic.com/en/articles/10722177-sharing-prompts-in-the-api-console · API dizini: https://platform.claude.com/llms.txt
- Go `embed`: https://pkg.go.dev/embed
- Langfuse: https://langfuse.com/docs/prompt-management/get-started · https://langfuse.com/self-hosting · https://langfuse.com/docs/opentelemetry/get-started · https://github.com/langfuse/langfuse · https://pkg.go.dev/github.com/optible/langfuse-go
- PromptLayer: https://docs.promptlayer.com/features/prompt-registry/overview · https://promptlayer.com/pricing
- Braintrust: https://www.braintrust.dev/docs/guides/prompts · https://www.zenml.io/blog/braintrust-pricing
- Humanloop: https://wandb.ai/wandb_fc/product-announcements-fc/reports/Humanloop-is-sunsetting-Migrate-to-Weights-Biases-as-an-alternative--VmlldzoxMzk4ODc1Nw
- Helicone: https://chatforest.com/reviews/helicone-llm-observability-gateway/ · https://dev.to/spanlens/helicone-was-acquired-by-mintlify-here-is-a-migration-checklist-if-you-are-moving-off-41m3
- promptfoo: https://www.promptfoo.dev/docs/configuration/reference/ · https://www.promptfoo.dev/docs/integrations/ci-cd/ · https://www.promptfoo.dev/docs/providers/echo/ · https://github.com/promptfoo/promptfoo/releases · https://finance.yahoo.com/news/openai-acquires-promptfoo-secure-ai-174904138.html

## Kullanıcı kararı bekleyen konular

Aşağıdakiler öneridir, karar değildir.

| Konu | Önerilen | Seçenekler |
|---|---|---|
| Prompt kaynağı | Repoda düz dosya + `go:embed` | DB kayıt defteri; harici araç (Langfuse vb.) |
| Biçim | Düz metin, şablon motoru yok; dinamik veri kullanıcı mesajında | `text/template` |
| Sürüm kimliği | `ad@semver` + otomatik `sha256`, kilit dosyası testi | Yalnız hash; yalnız semver; tam sayı revizyon |
| `llm_call` şeması | `prompt_name`, `prompt_version`, `prompt_sha256`, `cache_creation_tokens` ekle (şema kararı, kural 7) | Yalnız `prompt_version`; hiç ekleme |
| Olay/Analytics | `token_completed.v1`'e `prompt_name/version`, `llm_usage.prompt_version` | Yalnız `llm_call`'da tut |
| `prompt_snapshot` tablosu | Şimdilik yok; gerekirse sonra | Baştan ekle |
| Değerlendirme | 3 katman; Go içi koşucu, `eval-live` elle | promptfoo ekle; yalnız `make test` |
| LLM-as-judge | İlk sürümde yok | Haiku 5.5 hakem (ton için) |
| Arayüzde gösterim | Yalnız okunur "AI notları" rozeti | Hiç; yönetici sayfası |
| Önbellek tanılama testi | `live` etiketli testte kullan (SDK desteği doğrulanınca) | Kullanma |
