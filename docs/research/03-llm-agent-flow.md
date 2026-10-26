# Araştırma 3 — LLM akışı: model, yapılandırılmış çıktı, doğrulama, SMS/HTML, enjeksiyon, mock LLM

> **Durum:** Araştırma bulgusu, karar değil. Öneriler DECISIONS.md'ye ancak kullanıcı onayıyla girer.
> **Tarih:** 2026-10-27 · **Kaynak sorular:** THOUGHTS.md §1–7 (mesaj + bütçe, teklif/onay/red döngüsü, kanal kapsaması, HTML email + SMS), DECISIONS açık konuları (model, yapılandırılmış çıktı, segment DSL'i, doğrulama, karakter limiti, streaming, prompt injection, mock LLM).
> **Not:** `[D]` araştırmacının çektiği dokümanda doğrulandı, `[?]` doğrulanamadı.

## Özet öneri

| Konu | Öneri |
|---|---|
| Model | Segment ve metin için `claude-sonnet-5-5`; ucuz ön eleme ve yedek için `claude-haiku-5-5` |
| Yapılandırılmış çıktı | Native structured outputs (`output_config.format`, `json_schema`) + Go'da tam doğrulama |
| Durum | Sunucu tarafı teklif kayıtları; her LLM çağrısı durumsuz |
| SMS limiti | Genişletilmiş en kötü durum metni üzerinde 160 karakter tavanı + kodlama ve segment sayısı gösterimi |
| HTML email | Model yapılandırılmış bloklar döner, Go sabit bir şablona `html/template` ile yerleştirir |
| Güvenlik | İzin, hesap durumu, erişilebilirlik ve kitle tavanı kodda zorlanır; brief güvenilmeyen veridir |
| Test | Kendi `LLM` arayüzü + senaryolu sahte + altın dosyalar + Messages API'yi taklit eden `httptest` sunucusu |

## 1. Model seçimi `[D]`

| Model | Fiyat / MTok (girdi / çıktı) | Gecikme |
|---|---|---|
| `claude-fable-5-1` | $10 / $50 | Yavaş |
| `claude-opus-5-5` | $4 / $20 | Orta |
| `claude-sonnet-5-5` | $2 / $10 | Hızlı |
| `claude-haiku-5-5` | $0.10 / $0.50 (100K token'a kadar) | En hızlı |

- **Segment çağrısı:** Sonnet 5.5, `effort: "medium"`. Yanlış kural gerçek müşterilere dokunur; burada token harcamak doğru.
- **Metin çağrısı:** Sonnet 5.5, `effort: "low"` veya `"medium"`, streaming. Türkçe ton kalitesi önemli.
- **Haiku 5.5:** brief enjeksiyon ön elemesi. Kendi güvenlik sınıflandırıcısı `stop_reason: "refusal"` dönebilir; bu "işaretlendi" sayılır.
- Uzun sistem prompt'u ve katalog prompt caching ile önbelleğe alınır (Sonnet 5.5 önbellek okuması $0.10/MTok; en küçük önbelleklenebilir prompt 512 token).
- **5.x tuzakları:** `temperature`, `top_p`, `top_k` göndermek 400 döner (API sürümü 2026-10-21). Asistan prefill'i 400 döner. Thinking token'ları `max_tokens`'a sayılır; pay bırak. Thinking metni varsayılan olarak gelmez, arayüz ilk token'dan önce bir duraklama görür.

## 2. Yapılandırılmış çıktı `[D]`

- **Native structured outputs** genel kullanımda (beta header yok), dilbilgisi kısıtlı örnekleme kullanır; Sonnet 5.5, Haiku 5.5 ve Opus 5.5'te destekli. Streaming'de JSON normal `text_delta` olarak gelir.
- **Tool use:** Sonnet 5.5, Opus 5.5 ve Fable 5.1'de zorlanmış `tool_choice` (`any` / `tool`) 400 döner; yalnızca `auto` ve `none`. Tool döngüsü (bkz. Araştırma 2) `auto` + `strict: true` + prompt talimatıyla çalışır. Haiku 5.5 zorlanmış seçimi hâlâ kabul eder ama o zaman thinking bloğu dönmez.
- **Şema sınırları:** `minLength`/`maxLength`, `minimum`/`maximum`, özyinelemeli şemalar ve `false` dışı `additionalProperties` desteklenmez. `minItems` yalnızca 0 veya 1. İstekte en fazla 24 opsiyonel ve 16 union parametre.
- **Sonuç:** iki şema tutulur. Claude'a giden "tel" şeması sade; Go'da çalışan tam doğrulama şeması `maxLength`, aralıklar vb. içerir.
- Şemada `sms` `email`'den, `subject` `body`'den önce gelsin: zorunlu alanlar şema sırasıyla üretilir, arayüz yukarıdan aşağı dolar.
- `refusal` ve `max_tokens` durumlarında JSON geçersiz olabilir; ayrıştırmadan önce `stop_reason`'a bakılır. Enum harf büyüklüğü garanti değil; büyük/küçük harf duyarsız karşılaştır. Yeni şemalı ilk istek yavaştır, derlenmiş dilbilgisi 24 saat önbellekte kalır.
- **Go SDK:** `github.com/anthropics/anthropic-sdk-go/v2`, son sürüm v2.3.0 (2026-10-23). v2.0.0 kırıcı bir sürümdü; modül yolu `/v2` oldu, Go 1.25+ ister. Eski örnekler v1 yolunu gösterir. `Messages.NewStreaming` + `Accumulate`, `OutputConfigParam{Format: JSONOutputFormatParam{...}, Effort: ...}`, `ToolParam{Strict, EagerInputStreaming}`, `option.WithBaseURL`, `option.WithMaxRetries` (varsayılan 2; 408/409/429/5xx). SDK `ANTHROPIC_API_KEY`'i ortamdan okur; `.env`'i önce kendin yükle.

## 3. Segment kuralının biçimi

Bu araştırmacı **sabit derinlikli JSON AST** önerdi (kök grup → grup → yaprak; yaprak değerleri `values: string[]`, tipine göre kodda ayrıştırılır; `cart_abandoned{within_days}` gibi makrolar sabit `EXISTS` şablonlarına açılır; derinlik ≤ 3, ≤ 20 düğüm, `within_days` 1–365; boş kural reddedilir). Araştırma 2 ise **DSL string** önerdi. Uzlaştırma [README](README.md#çelişkiler-ve-uzlaştırma) içinde.

Ortak noktalar: alan → (kolon, tip, operatör, aralık/enum) beyaz listesi Go haritasında; tanımlayıcı asla Claude string'inden gelmez; değerler `$n` parametre; tarihler sunucuda `now()` ile hesaplanır; `LIKE` joker karakterleri kaçırılır. squirrel bakım modunda, goqu daha ağır, sqlc dinamik kural için uygun değil; ~150 satırlık elle derleyici yeterli.

## 4. Doğrulama hattı ve onarım döngüsü

1. Go struct'ına `DisallowUnknownFields` ile çöz.
2. Tam JSON Schema doğrulaması: **`santhosh-tekuri/jsonschema/v6`** (draft 04 → 2020-12, gezilebilir hatalar). `kaptinlin/jsonschema` kabul edilebilir; `xeipuuv/gojsonschema` yalnızca eski draft'lar ve bakımı belirsiz, kaçın.
3. Anlamsal kontroller, her biri makine okunur hata üretir: alan/operatör beyaz listede, şehir DB'de var (Türkçe duyarlı, bkz. Araştırma 4), enum geçerli, tarih penceresi makul, `COUNT(*)` 0'dan büyük ve tavanın altında, SMS limiti tutuyor.
4. **Onarım:** en fazla 2 onarım (3 deneme), SDK'nın taşıma retry'larından ayrı. Her onarım brief + geçersiz çıktı + `{path, rule, got}` listesiyle yeni, durumsuz bir çağrıdır; thinking blokları tekrar oynatılmaz. `refusal`'da yeniden deneme yok; `max_tokens`'ta bir kez daha yüksek limitle. Her deneme token'larıyla DB'ye kaydedilir.
5. **Son başarısızlık:** kullanıcıya düz Türkçeyle hangi parçanın tutmadığı söylenir; "brief'i düzenle" veya elle kural kurucu önerilir. Asla otomatik gönderim yok; başarısız taslak denetim için salt-okunur saklanır.

## 5. SMS uzunluğu: Türkçe neredeyse her zaman UCS-2

- **GSM-7 temel set** ö, ü, Ö, Ü ve büyük Ç'yi içerir; **küçük ç, ş, Ş, ğ, Ğ, ı, İ yoktur**. Uzantı karakterleri (€ [ ] { } \ ^ ~ |) 2 septet sayılır.
- Tek bir böyle karakter bütün mesajı UCS-2'ye çevirir: tek SMS **70** karakter, birleştirilmiş mesajda segment başına **67** (GSM-7'de 160 / 153).
- 3GPP TS 23.038'deki Türkçe ulusal dil kaydırma tabloları ş Ş ğ Ğ ı İ ç'yi kapsar, ama UDH ister (kapasite ~155 veya ~152) ve çoğu toplayıcı desteklemez `[?]`.
- **Öneri:**
  1. `{{first_name}}` şablonda yer tutucu olarak kalır.
  2. Kitledeki en uzun ada göre doğrula (`max(length(first_name))`, ~20 karakterle sınırlı).
  3. Aynı kontrol gönderimden hemen önce alıcı başına tekrar.
  4. UCS-2 için UTF-16 kod birimi say (emoji 2).
  5. Teklifte segment sayısı ve maliyet gösterilir.
- **Alternatifler (ürün kararı):** tek UCS-2 segmenti için 70 tavanı (Türkçe için çok kısa) veya ş→s gibi ASCII katlamayla GSM-7'de 160 (ucuz ama metni değiştirir).
- Brief'teki "SMS en fazla 160 karakter" kuralı bu yüzden Türkçe metinde genellikle **3 UCS-2 segmenti** demek; bütçe hesabını doğrudan etkiler.

## 6. Streaming

- Yalnızca **metin çağrısı** stream edilir. Segment çağrısı stream'siz çalışır veya sadece durum yayınlar.
- Structured outputs `text_delta` olarak, tool girdisi `input_json_delta` olarak akar (`partial_json` parçaları birleştirilip `content_block_stop`'ta ayrıştırılır). `eager_input_streaming: true` doğrulanmamış parça gönderir; ayrıştırma korunmalı. Akış ortasında `ping` ve `error` (ör. `overloaded_error`) gelebilir.
- Go: `stream.Next()` + `Accumulate`; metin parçaları hoşgörülü bir artımlı çıkarıcıya gider, `{field: "sms.text", delta}` olayları üretir. SSE olayları: `delta`, `final` (doğrulanmış JSON + usage), `error` (yapılandırılmış kod). Taşıma ayrıntısı Araştırma 4'te.
- Vue: POST gerektiği için `EventSource` yerine `fetch` + `ReadableStream`. Kısmi alanlar taslak önizlemeye çizilir, `final` veya `error` gelince değiştirilir.
- Kısmi veriyle asla işlem yapılmaz; doğrulama öncesi SMS sayacı yalnızca tahmindir.

## 7. Prompt injection

- Her derlenmiş sorguya kodda eklenir: `account_status = 'active'` ve kanal izni + iletişim bilgisi. Gönderim anında da tekrar uygulanır.
- AST'de bu koşulları kapatabilecek alan veya operatör yoktur. "İzinden bağımsız herkese gönder" doğrulamada düşer.
- Kitle başına en fazla alıcı ve bütçe tavanı; gönderim öncesi insan onayı; tavanı aşan her şey açık onay ister.
- Brief ayrılmış, JSON kodlu bir blokta "veri" olarak verilir. Segment ve metin çağrıları ayrıdır. **Metin çağrısı yalnızca doğrulanmış segment özetini görür, müşteri satırlarını asla** (aynı zamanda veri minimizasyonu).
- Çıktı kontrolleri: değişken beyaz listesi, URL beyaz listesi (CTA URL'si kampanya yapılandırmasından gelir, LLM'den değil), içerik lint'i.
- İsteğe bağlı Haiku 5.5 enjeksiyon ön elemesi. OWASP'a göre kusursuz önlem yok; katmanlı savunma şart. "Kuralları yok say" gibi düşmanca brief'lerle test.

## 8. Mock LLM

```go
type LLM interface {
    GenerateSegment(ctx context.Context, in SegmentIn) (SegmentOut, Usage, error)
    StreamCopy(ctx context.Context, in CopyIn, on func(Delta)) (CopyOut, Usage, error)
}
```

- **Senaryolu sahte:** çağrı başına yanıt kuyruğu (önce geçersiz, sonra geçerli); onarım döngüsünü ve başarısızlık deneyimini test eder.
- **Altın dosyalar:** kaydedilmiş gerçek yanıtlar diskten oynatılır; canlı değerlendirme build tag arkasında.
- **`httptest` sunucusu:** Messages SSE olaylarını sırasıyla yayar (`message_start` → `content_block_start` → `ping` → `content_block_delta` → `content_block_stop` → `message_delta` (usage) → `message_stop`). Gerçek adaptör `option.WithBaseURL(srv.URL)` + `option.WithAPIKey("test")` ile ona bağlanır. 429/529, `max_tokens`, `refusal`, kesik JSON ve akış ortası `error` vakaları.
- Testler asla `ANTHROPIC_API_KEY` okumaz.

## 9. Çok adımlı, insan onaylı akış (THOUGHTS §1–8)

**Durum sunucu tarafı teklif kayıtlarında tutulur; her LLM çağrısı durumsuzdur.**

```text
agent_proposal(id, campaign_id, kind [segment|channels|copy|schedule],
               status [proposed|approved|rejected|superseded], parent_id,
               payload jsonb, reject_reason, model, prompt_version)
llm_call(id, proposal_id, model, input_tokens, output_tokens,
         cache_creation_tokens, cache_read_tokens, stop_reason, attempt, created_at)
```

- Her Claude çağrısı brief'i, onaylanmış önceki ürünleri ve önceki red sebeplerini veri olarak alır.
- **Neden konuşma geçmişi değil:** thinking blokları modele ve konuşmaya bağlıdır; geçmişi düzenlemek tekrar oynatılan blokları geçersiz kılar ve 2026-08-31 sonrası açılan hesaplarda 400 döner.
- **Adım 2–3:** AST derleyiciye gider, `COUNT` ve örnek teklif olarak döner. Alıcı listesi yalnızca son onayda dondurulur; izin gönderimde yeniden uygulanır.
- **Adım 5:** kanal kapsaması (`email_only`, `sms_only`, `both`, `neither`) LLM ile değil SQL ile hesaplanır. "Azalt" ya da "diğer kanaldan ulaş" kararı bir mi iki mi metin teklifi olacağını belirler.
- **Red:** önceki adıma döner, teklif `rejected` olur.
- **Bütçe iki ayrı şeydir:**
  - **Kampanya harcama tavanı** (kodda): `max_recipients = floor(budget / unit_cost)`. SMS birim maliyeti = segment fiyatı × segment sayısı; UCS-2 etkisi maliyeti doğrudan büyütür. Tutara Claude karar vermez.
  - **Ajan token tavanı:** kampanya başına `llm_call` toplamı; oturum başına sınır ve en fazla çağrı sayısı.
- **Token kaydı:** deneme başına bir `llm_call` satırı. Faturalama yanıttaki `usage`'dan; thinking çıktı olarak faturalanır. Streaming'de girdi `message_start`'tan, kümülatif çıktı `message_delta`'dan. `count_tokens` ücretsiz bir ön tahmindir, faturadan biraz sapar. Analytics'in `token.completed.v1` olayı (Araştırma 1) bu satırlardan beslenir.

## 10. HTML email (THOUGHTS §6.1)

**Öneri: yuvalı şablon.**

- Claude yapılandırılmış bloklar döner: `subject`, `preheader`, `headline`, `paragraphs[]`, `cta_label`.
- Go bunları sabit, tablo tabanlı, inline-CSS bir şablona `html/template` otomatik kaçışıyla yerleştirir. CTA URL'si yapılandırmadan gelir.
- Yer tutucular (`{{first_name}}`) beyaz listeli ve varsayılan değerli kendi değişimimizle doldurulur; LLM metni `text/template`'ten geçirilmez.
- Düz metin alternatifi aynı bloklardan üretilir. Abonelikten çıkma alt bilgisi kodda eklenir; `List-Unsubscribe` header'ı `[?]`.
- **Alternatif, serbest HTML:** bluemonday `NewPolicy` allowlist, `AllowURLSchemes("https", "mailto")`, `RequireParseableURLs`, `style` özniteliğine izin verme. bluemonday CSS'i temizlemez ve stil silmek email düzenini bozar; yuvalı şablon bu yüzden daha iyi.

## Doğrulanamayanlar

- Zorlanmış `tool_choice` kısıtı yalnızca Haiku 5.5 ve Sonnet 5.5 için doğrudan okundu; Opus 5.5 ve Fable 5.1 için claude-api skill metnine dayanıyor.
- Braze ve Insider segment dokümanları kontrol edilmedi.
- gojsonschema, kaptinlin/jsonschema, bluemonday, squirrel bakım durumları görünmedi.
- Türkçe kaydırma tablosu kod noktaları ve UDH kapasite rakamları Wikipedia ve bir blogdan; 3GPP metni değil. Provider desteği kontrol edilmedi.
- Go SDK v2 yardımcıları kodla denenmedi; Türkçe metin işleme için kısa bir spike önerilir.
- Araştırmacı `cases.Lower(language.Turkish)` önerdi; Araştırma 4'te bunun `Istanbul` için `ıstanbul` ürettiği Go'da doğrulandı. Şehir eşleşmesi için Araştırma 4'teki yöntem geçerlidir.

## Kaynaklar

- Modeller: https://platform.claude.com/docs/en/models/overview · https://platform.claude.com/docs/en/models/sonnet-5-5/overview · https://platform.claude.com/docs/en/models/sonnet-5-5/whats-new-sonnet-5-5 · https://platform.claude.com/docs/en/models/haiku-5-5/migration-guide · https://platform.claude.com/docs/en/api/migration-2026-10-21
- Yapılandırılmış çıktı ve tool: https://platform.claude.com/docs/en/build-with-claude/structured-outputs · https://platform.claude.com/docs/en/agents-and-tools/tool-use/strict-tool-use · https://platform.claude.com/docs/en/agents-and-tools/tool-use/fine-grained-tool-streaming
- Streaming ve token: https://platform.claude.com/docs/en/build-with-claude/streaming · https://platform.claude.com/docs/en/build-with-claude/token-counting
- Go SDK: https://platform.claude.com/docs/en/cli-sdks-libraries/sdks/go · https://github.com/anthropics/anthropic-sdk-go/releases
- DSL ve sorgu kurucular: https://jsonlogic.com/ · https://docs.customer.io/journeys/data-driven-segments · https://github.com/Masterminds/squirrel · https://github.com/doug-martin/goqu
- JSON Schema: https://github.com/santhosh-tekuri/jsonschema · https://github.com/kaptinlin/jsonschema · https://github.com/xeipuuv/gojsonschema
- SMS: https://en.wikipedia.org/wiki/GSM_03.38 · https://www.twilio.com/docs/glossary/what-sms-character-limit.md · https://nowsms.com/shift-tables-national-language-sms-in-160-characters-without-unicode
- Güvenlik: https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/mitigate-jailbreaks · https://genai.owasp.org/llmrisk/llm01-prompt-injection/
- HTML: https://github.com/microcosm-cc/bluemonday
