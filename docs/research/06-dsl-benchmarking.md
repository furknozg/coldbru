# Araştırma 6 — DSL yaklaşımını "modelin doğrudan GraphQL filtre JSON'u yazması"na karşı ölçmek

> **Durum:** Araştırma bulgusu, karar değil. Öneriler DECISIONS.md'ye ancak kullanıcı onayıyla girer (CLAUDE.md Kural 1). Bu belge hiçbir kod yazmaz; yalnız ne, nasıl ve nerede ölçüleceğini anlatır.
> **Tarih:** 2026-10-28 · **Girdi:** [D-007](../../DECISIONS.md), D-026, D-027, D-038, D-041, D-059–D-061, D-065–D-071, [coldbru-gqdsl.md](../coldbru-gqdsl.md), [Araştırma 2](02-segment-query-gql-mcp-dsl.md) §4, [Araştırma 3](03-llm-agent-flow.md), [Araştırma 5](05-prompt-versioning.md); kod: `internal/proposal/{dsl,catalog,compile,customer,llm,prompts}`, `internal/customer/audience`, `internal/platform/{httpserver,metrics}`, `Makefile`; Anthropic dokümanları (token sayma, fiyatlar), Go SDK v2.4.0 kaynağı (salt okunur).
> **Not:** `[D]` kodda veya çekilen dokümanda doğrulandı, `[?]` doğrulanamadı veya varsayım. Bu araştırma sırasında Anthropic API'si çağrılmadı ve `ANTHROPIC_API_KEY` okunmadı; token sayıları ve LLM maliyetleri bu yüzden **ölçüm değil**, hesaplanabilir ifadelerdir.

## Özet öneri

| Konu | Öneri |
|---|---|
| Karşılaştırma ekseni | Altı ayrı, birbirine karıştırılmayan eksen: (1) token maliyeti, (2) Proposal tarafı CPU/gecikme, (3) Customer tarafı SQL, (4) HTTP yükü, (5) LLM kalitesi (ilk denemede geçerlilik, onarım), (6) sistem geneli maliyet kalemleri. Yalnız (4)'ün bir kısmı ve (5) gerçek LLM çağrısı ister. |
| Token maliyeti, **şimdi** | Üretim yok. Aynı metinleri iki biçimde sayarız: DSL metni ve ondan `compile.Compile` ile çıkan filtre JSON'u. Sayaç: (a) çevrimdışı vekiller (bayt, rune, sözcüksel token) her zaman; (b) `count_tokens` ile gerçek token sayısı, bir kez, **anahtar ister ama ücretsizdir** ve sonuç repoda önbelleğe alınır. Resmi çevrimdışı Claude tokenizer'ı yok. |
| Girdi/çıktı ayrımı | Çıktı fiyatı girdinin 5 katı (Sonnet 5.5 ve Haiku 5.5). Filtre metni **çıktıdır**; sistem prompt'u (gramer + katalog veya SDL) **önbellekli girdidir**. İkisi ayrı satır olarak raporlanır; tek "toplam token" sayısı verilmez. |
| CPU/gecikme | `testing.B` mikro benchmark'lar (`dsl.Parse`, `catalog.Check`, `compile.Compile`, JSON doğrulama alternatifleri), `-cpuprofile`/`-memprofile`, `benchstat`. Beklenen sonuç: mikrosaniye mertebesi; LLM çağrısı saniye mertebesi. Bu yüzden amaç "hangisi hızlı" değil "fark önemsiz mi"yi kanıtlamak. |
| SQL | İki yaklaşım **aynı** `CustomerFilter`'ı Customer'a gönderir; SQL ve Postgres süresi yaklaşıma değil filtre şekline bağlıdır. Yaklaşım karşılaştırması için ayırt edici değildir; yalnız "model farklı/gereksiz karmaşık filtre yazıyor mu" sorusuna bakılır. |
| Yük | `vegeta` (MIT, açık model, sabit hız) + gerekirse küçük bir Go SSE istemcisi. LLM'siz yük için mock Messages sunucusu gerekir. `k6` yalnız senaryo gerekirse. |
| LLM düzeyi | `live` etiketli, elle çalışan `make bench-live`/`make eval-live` (D-061): iki kol (DSL ve GraphQL-JSON-string), en az ~40 brief × 5 tekrar. Tahmini maliyet 10–40 $ aralığında `[?]`. |
| Önce karar gereken | Bölüm 7'de 12 karar; en önemlileri: sayaç anahtarı politikası (K1), GraphQL-JSON kolunun biçimi (K6), canlı deney bütçesi (K8). |

## 0. Ne karşılaştırılıyor? Adil karşılaştırmanın kuralları

**Kol A (mevcut):** model `propose_segment{dsl: "<DSL>"}` çağırır; Proposal `dsl.Parse` → `catalog.Check` → `compile.Compile` ile `CustomerFilter` değişkenini üretir ([D-065–D-067]).

**Kol B (alternatif):** model filtreyi doğrudan `CustomerFilter` JSON'u olarak yazar; Proposal onu doğrular ve olduğu gibi GraphQL değişkeni olarak yollar. D-007'nin son maddesi bunu zaten bir "opt-in" olarak ister: *DSL kapatılıp ham GraphQL doğrulaması ve gönderimi mümkün kılınmalı*. Bu araştırma o seçeneğin ne kazandıracağını ölçmenin yoludur.

Adil olmak için şunlar eşit tutulmalı:

1. **Aynı bilgi.** Şu an prompt'taki `RenderCatalog` çıktısı alan açıklamalarını, işleçleri ve kapalı değer listelerini (`loyaltyTier`, `sex`, ...) taşır. Customer SDL'i bunları taşımaz: `schema.graphqls` yalnız alan adlarını ve filtre tiplerini verir; açıklamalar ve kapalı listeler `catalog/metadata.yaml`'dadır (D-066). Kol B yalnız SDL'i verirse model değer listelerini bilmez ve kalite düşer; bu "GraphQL kaybetti" değil "bilgi eksikti" sonucudur. Kol B'nin prompt'u **SDL + aynı katalog metni** olmalı; fark yalnız `grammar.txt` yerine SDL parçasıdır.
2. **Aynı doğrulama güvencesi.** Bugün model hataları Proposal'da konumlu (satır/sütun, `near`, öneri) hata olarak döner. Kol B'de aynı güvenceyi vermek için Proposal tarafında JSON'u katalogla denetleyen bir yürüyücü gerekir (alan var mı, işleç tipe uyuyor mu, değer kapalı listede mi, sınırlar 8/100/100). Bu kod Kol A'da `catalog.Check`'in karşılığıdır; "Kol B kod yazmaz" varsayımı yanlıştır. Alternatif: Proposal'da doğrulamayı atlayıp Customer'a bırakmak (aşağıda 2.3).
3. **Aynı taşıma.** D-027: strict tool şeması özyinelemeli yapıyı ve `maxLength`'i desteklemez. Bu yüzden Kol B'nin tool'u ya (a) `filter` alanı serbest `object` olan **strict olmayan** tool ya da (b) JSON'u **string** içinde taşıyan strict tool olabilir. (b) DSL ile simetriktir: iki kolda da çıktı bir JSON string alanıdır, fark yalnız sözdizimidir. Önerilen birincil kol (b), ikincil (a) (K6).
4. **Aynı model ve parametreler.** `claude-sonnet-5-5`, `effort: medium`, adaptive thinking, `max_tokens` 4096 (D-069); `temperature` gönderilemez (D-026), dolayısıyla örnekleme gürültüsü tekrarla azaltılır, kontrol edilemez.
5. **Aynı ölçüt.** "Geçerli JSON" ile "doğru kitle" aynı şey değildir: kalite, referans filtrenin döndürdüğü alıcı kümesiyle karşılaştırılarak ölçülür (Bölüm 4).

Araştırma 2 §4'teki rakamlar (DSL ~35–45, JSON AST ~90–110, GraphQL ~130–170 çıktı token'ı) ve D-007'deki "~3 kat çıktı token'ı" **karakter sayısından yapılmış tahminlerdi, ölçüm değildi** [D, belgede öyle yazıyor]. Örnek brief için bugün bayt sayıları: DSL 115 bayt, `compile.Compile` çıktısı (kompakt JSON) 150 bayt [D, ölçüldü]; yani JSON, DSL'in yaklaşık 1,3 katı bayttır, 3 katı değil. Token oranı bundan farklı olabilir (aşağıda), ama "3 kat" varsayımı sınanmadan alıntılanmamalı. Bu, ucuz maliyet benchmark'ının ilk bulgusu olabilir.

## 1. Token maliyeti

### 1.1 Neyi sayıyoruz

| Bileşen | Tarafı | Fiyat sınıfı | Kol A | Kol B |
|---|---|---|---|---|
| Filtre metni (modelin yazdığı) | Çıktı | çıktı (5× girdi) | DSL, `{"dsl":"..."}` tool girdisi içinde | `CustomerFilter` JSON'u |
| Sistem prompt'u: yönerge | Girdi (önbellek) | yazma 1,25×, okuma 0,05× (Sonnet 5.5) | `system.txt` (4.899 bayt) | aynı, DSL örnekleri GraphQL örnekleriyle değişir |
| Sistem prompt'u: dil tanımı | Girdi (önbellek) | aynı | `grammar.txt` (2.681 bayt) | SDL parçası (~4.9–6.2 KB, aşağıya bakın) |
| Katalog | Girdi (önbellek) | aynı | `RenderCatalog` çıktısı | aynı (adillik kuralı 1) |
| Tool tanımları | Girdi (önbellek) | aynı | `tools.json` (1.411 bayt) | benzer, `dsl` yerine `filter` |
| Onarım turları | Girdi + çıktı | karışık | Bölüm 1.6 | Bölüm 1.6 |

Bayt sayıları [D] repodan; token karşılıkları `[?]` (sayılmadı).

**Girdi mi çıktı mı baskın?** Segment çağrısında sistem öneki yaklaşık 16–18 KB metindir (system.txt + grammar.txt + tools.json = 8.991 bayt, artı katalog; katalog çıktısının boyutu bu araştırmada üretilmedi, `catalog_gen.go` 12,5 KB ve `metadata.yaml` 8,3 KB olduğu için 6–9 KB varsayıldı `[?]`). Bayt/token ≈ 3,5 sezgisiyle `[?]` bu yaklaşık 4,5–5 bin token eder. Çıktı tarafında ise filtrenin kendisi kısa (onlarca token) ama **adaptive thinking** token'ları da çıktı olarak faturalanır ve filtreden büyük olması beklenir `[?]`. Dolayısıyla DSL-JSON farkı yalnız filtre metnindeki farktır ve thinking'in yanında küçük kalabilir. Bu bir tahmin; gerçek çalıştırmada `usage.output_tokens_details.thinking_tokens` ayrıca kaydedilmeli [D, claude-api skill: cost-optimization.md; SDK alanının adı `[?]`, `llm.Usage` bugün yalnız `InputTokens`, `OutputTokens`, `CacheReadTokens`, `CacheCreationTokens` tutuyor].

### 1.2 Fiyatlar (girdi/çıktı ayrımı için)

Kaynak: Anthropic fiyat sayfası (çekildi 2026-10-28) [D]:

| Model | Girdi | 5 dk önbellek yazma | 1 sa yazma | Önbellek okuma | Çıktı |
|---|---|---|---|---|---|
| `claude-sonnet-5-5` | $2 / MTok | $2,50 | $4 | **$0,10** | $10 / MTok |
| `claude-haiku-5-5` (≤100K token'lık prompt) | $0,10 / MTok | $0,125 | $0,20 | $0,01 | $0,50 / MTok |

**Çelişki:** claude-api skill'indeki `shared/models.md` Sonnet 5.5 için önbellek okumasını $0,20 yazıyor; fiyat sayfası ise 0,05× = $0,10 diyor ve dipnotu Sonnet 5.5'i açıkça sayıyor. Skill'in kendi kuralı "sayfa kazanır" olduğu için tablo $0,10 kullanır; kodda fiyat tablosuna girmeden önce fiyat sayfası yeniden kontrol edilmeli (K10). Batch API her iki yönde %50 indirim [D], ama çok turlu tool döngüsünde kullanılamaz (her tur öncekine bağlı); yalnız tek turlu "ilk deneme geçerliliği" ölçümünde kullanılabilir `[?]`.

**Haiku 5.5 tokenizer uyarısı [D]:** Token sayma dokümanı, Claude 4.7 ve sonrası modellerin yeni tokenizer kullandığını ve aynı metnin eski modellere göre yaklaşık %30 daha fazla token ürettiğini söylüyor; Haiku 5.5 için models.md de aynısını söylüyor. Sonnet 5.5'in tokenizer'ı Sonnet 5 ile aynı denilmiş; Sonnet 5.5 ile Haiku 5.5'in **birbirine** göre aynı tokenizer'ı kullanıp kullanmadığı doğrulanmadı `[?]`. Sonuç: token sayısı **modele özgü** ölçülmeli (`model` parametresi); bir modelde ölçülen oran diğerine taşınmaz. Bu karşılaştırma yalnız Sonnet 5.5 (segment) için gerekir; Haiku çağrıları (ön eleme, plan, zamanlama) filtre içermez.

**Hesap şablonu (varsayımlarla, ölçüm değil):**

```
çağrı_maliyeti = girdi_önbelleksiz × p_in
               + önbellek_yazma × 1,25 × p_in
               + önbellek_okuma × 0,05 × p_in      (Sonnet 5.5)
               + çıktı × p_out                      (p_out = 5 × p_in)
```

Örnek: filtre çıktısı DSL'de 45, JSON'da 65 token ise (varsayım `[?]`) fark 20 token × $10/MTok = **$0,0002 / çağrı**; 1.000 kampanyada 0,20 $. Aynı çağrıda 5 bin token'lık önek önbellekten okunursa 0,0005 $, önbelleksizse 0,01 $, ilk yazmada 0,0125 $. Yani maliyet kaleminde önbellek isabeti ve thinking, filtre biçimi farkından büyük olabilir; ölçüm bunu doğrulamalı ya da çürütmelidir.

### 1.3 Token sayma seçenekleri

| Seçenek | Doğruluk | Anahtar | Maliyet / sınır | Artı | Eksi |
|---|---|---|---|---|---|
| **`POST /v1/messages/count_tokens`** | Gerçek tokenizer; yine de "tahmin", küçük sapma olabilir; sistem eklemeli token'lar dahil olabilir ama faturalanmaz [D] | **Gerekir** (`x-api-key`) | **Ücretsiz**; dakikada istek sınırı: Start 5.000, Build 10.000, Scale 20.000 RPM; mesaj oluşturma sınırından bağımsız [D] | Modele özgü, `system`/`tools`/`output_config`/`thinking` kabul eder, üretim yok | Ağ + anahtar; yalnız **girdi** tokenı döner; tool_use'u hatasız sayabilmek için ayrıntı gerekir `[?]` |
| Çevrimdışı resmi tokenizer | — | — | — | — | **Yok.** Claude 3 ve sonrası için Anthropic çevrimdışı tokenizer yayınlamıyor; üçüncü taraf kaynaklar bunu söylüyor (ikincil, doğrulanamadı) `[?]`. Claude 4.7+ yeni tokenizer kullanıyor [D] |
| tiktoken ve benzerleri | Claude'da eksik sayar (skill: ~%15–20, kod ve Türkçe için çok daha fazla) [D] | Yok | Yok | Bağımlılıksız | Mutlak sayı yanlış; **göreli** oran için bile yanıltıcı olabilir |
| Bayt/rune/karakter oranı | Kaba; ~3,5 karakter/token sezgisi ikincil kaynaktan `[?]` | Yok | Yok | Her zaman çalışır, deterministik | Noktalama ağırlıklı JSON ve çok baytlı Türkçe harflerde (İ, ş) sapar |
| Sözcüksel token (DSL `lexer`, `encoding/json.Decoder.Token`) | Yapısal vekil | Yok | Yok | DSL'in 282 satırlık lexer'ı zaten var | BPE `":{"` gibi noktalama dizilerini birleştirir; JSON'u fazla sayar `[?]` |
| Gerçek çalıştırmadaki `usage` | Fatura gerçeği | Gerekir | Üretim ücretli | Tek kesin çıktı sayımı | Bölüm 4 |

**Go SDK [D]:** `anthropic-sdk-go/v2@v2.4.0`, `message.go:115`:

```go
func (r *MessageService) CountTokens(ctx context.Context, params MessageCountTokensParams, opts ...option.RequestOption) (*MessageTokensCount, error)
```

`MessageCountTokensParams` alanları: `Model`, `Messages`, `System` (`MessageCountTokensParamsSystemUnion`), `Tools` (`[]MessageCountTokensToolUnionParam`, istemci tool'u için `OfTool: &anthropic.ToolParam{...}`), `Thinking`, `ToolChoice`, `OutputConfig`, `CacheControl`, `Speed`. Sonuç `MessageTokensCount.InputTokens int64`. Model sabitleri: `anthropic.ModelClaudeSonnet5_5`, `anthropic.ModelClaudeHaiku5_5`. Yol: `v1/messages/count_tokens`. Beta ağacında ayrı bir `Beta.Messages.CountTokens` var; gerekmez. Üretim kodundaki istemci (`internal/proposal/llm/anthropic`) bu yöntemi bugün çağırmıyor; saymak için ayrı bir araç yazılır, üretim `LLM` arayüzüne (D-069) eklenmez.

**`count_tokens` ile *çıktı* metni nasıl sayılır?** Uç nokta yalnız girdi sayar. Bir metnin çıktı token'ı, o metnin token sayısına eşittir (aynı sözlük); bu nedenle metni bir user mesajı olarak sayıp sabit çerçeve ek yükünü çıkarırız: `tokens(metin) = count([user: metin]) − count([user: "."])`. Çerçeve sabiti iki biçim arasındaki farkta zaten yok olur. Küçük bir sapma sorunu: modelin gerçekte örneklediği token sınırları kanonik kodlamadan biraz farklı olabilir `[?]`; bu, gerçek çalıştırmadaki `usage.output_tokens` ile Bölüm 4'te çapraz kontrol edilir. Her metin için bir istek gerekir (toplu sayma yok); 300 metin × 4 biçim ≈ 1.200 istek, 20 istek/sn ile bir dakika civarı, ücretsiz.

**Önbellekleme:** Çıktı `bench/tokens/cache.json` gibi bir dosyada `sha256(model + "\x00" + biçim + "\x00" + metin) → token` olarak saklanır ve commit edilir (D-059'un `prompts.lock.json` modelini izler). Anahtarsız `make bench-tokens` yalnız önbelleği okur, eksik anahtar için çevrimdışı vekilleri raporlar ve "n eksik" der. Yalnız `make bench-tokens-refresh` anahtarı okur. Böylece anahtar CI'da gerekmez ve D-061'in "anahtarı yalnız canlı test okur" ilkesi korunur (K1).

### 1.4 Sistem prompt'u boyutu

Ölçülecek (her iki kolda, her model için `count_tokens` ile `system` + `tools` + sabit bir kullanıcı mesajı):

| Parça | Kol A | Kol B |
|---|---|---|
| Yönerge | `prompts/segment/system.txt` 4.899 bayt | Kol B'ye uyarlanmış; DSL'e özgü "TYPICAL MISTAKES" maddeleri (`===`, çıplak boolean, lambda alanları) yerine GraphQL'e özgü hatalar |
| Dil tanımı | `grammar.txt` 2.681 bayt | `schema.graphqls` satır 56–280 (filtre girdi tipleri + `CustomerFilter`): 6.229 bayt yorumlarla, 4.882 bayt yorumsuz [D, `wc -c`]; üstüne `TimeValue` ve skalerler (satır 9–60) |
| Katalog | `RenderCatalog(root)` | aynı |
| Tool'lar | `tools.json` 1.411 bayt | benzer |

Kol B'nin dil parçası Kol A'dan bayt olarak ~2,2–3,5 KB büyük görünüyor [D bayt, token `[?]`]; önbellekli girdi olduğu için bu fark okuma başına çok küçük (≈ 800 token × $0,10/MTok ≈ $0,00008), ilk yazmada ≈ $0,002. Yönerge metni de değişir (örneğin "birden çok koşul için `and: [...]` kullan", `@oneOf` zaman değeri kuralı `{"ago": "30d"}`); Kol B'nin prompt'u ilk haliyle taslak olduğundan, bu karşılaştırma "en iyi Kol B prompt'u" değil "makul Kol B prompt'u" ile yapılır; bunun sınırı raporda yazılmalı.

SDL'in modele **doğal** gelmesi (GraphQL eğitim verisinde yaygın) Kol B lehine, DSL'in modele **yeni** olması Kol A aleyhine bir etkendir; bu etki token sayısında değil kalite ölçümünde (Bölüm 4) görünür.

### 1.5 Önbellek etkisi

- Minimum önbellek öneki modele bağlı; Sonnet 5.5 için değer skill tablosunda; DSL önekinin (≈ 4–5 bin token `[?]`) bunu aştığı doğrulanmalı. `TestRepeatedCallsSendIdenticalPrefixBytes` (D-026) önekin brief'ten bağımsızlığını zaten doğruluyor; `live` testi iki çağrı arasında `CacheReadTokens > 0` bekliyor (`internal/proposal/llm/anthropic/live_test.go`).
- Önekteki fark yalnız **prompt sürümü başına bir kez soğuk yazmada** (D-059 sonuçları) ve her okumada hissedilir: iki kolun önek büyüklüğü farkı × 1,25 × $2/MTok (yazma), × $0,10/MTok (okuma).
- 5 dakikalık TTL, aynı önekli iki isteğin başlangıçları arası 5 dakikadan kısaysa taze kalır [D, skill]. Sistemin gerçek trafiği (kampanya oluşturma seyrek olabilir) önbellek isabetini belirler; bu, benchmark'ta değil **üretim kullanım günlüğünden** (`llm_call.cache_read_tokens`/`cache_creation_tokens`) ölçülür.
- `count_tokens` önbelleği kullanmaz (cache_control kabul eder ama yok sayar) [D]; önbellek etkisi yalnız `usage` ile görülür.

### 1.6 Onarım döngüsünün token maliyeti

D-069: onarım aynı konuşmada, `is_error` tool_result ile; toplam en fazla 3 `propose_segment`, en fazla 8 tur. Her ek tur öneki (önbellekten okunur), geçmiş mesajları ve düşünme bloklarını yeniden gönderir ve yeni çıktı üretir. Dolayısıyla onarım maliyeti ≈ `(1 − p_ilk_deneme) × tur_başına_maliyet`. Kol A ve Kol B için `p_ilk_deneme` ve hata mesajı uzunluğu farklıdır: DSL hataları konumlu ve öneri içerir (`UNKNOWN_VALUE` + öneriler); Kol B'de hata kaynağı `gqlparser`/gqlgen veya Customer `filterPath` olur ve metni başka uzunluktadır. **Bunu gerçek çalıştırma olmadan ölçemeyiz.** Şimdi yapılabilecek tek şey, *hata mesajı uzunluğunu* sabit bir hata kümesi için iki kolda saymaktır (örneğin yanlış şehir, yanlış alan adı, tip uyumsuzluğu): bu, bir onarım turunun *girdi* ek yükünü sınırlar ama modelin düzeltme başarı olasılığını söylemez.

### 1.7 Korpus

| Kaynak | Yaklaşık adet | Not |
|---|---|---|
| `system.txt` içindeki 3 örnek | 3 | Prompt'a giren gerçek örnekler |
| `docs/coldbru-gqdsl.md` §8.2 ve §13 | 8 | Örnek brief dahil |
| `dsl/parser_test.go` `examples` | ~20 | Test dosyasında export edilmemiş değişken; korpus dosyasına kopyalanır (testin kendisi korpusu okur hale getirilebilir, K5) |
| `compile/compile_test.go` `TestCompileGolden` | 27 | DSL + beklenen JSON çifti hazır |
| `catalog/check_test.go` `TestCheckValid` | ~35 | Yalnız geçerli olanlar |
| Gramer tabanlı üretici | 200+ | Katalogdan rastgele yürüyüş; derinlik ≤ 8, düğüm ≤ 100; `math/rand/v2` PCG sabit tohum; `compile.Compile` hatasızsa alınır; düğüm sayısına göre katmanlı (1, 3, 5, 10, 20, 50) → eğim görünür |
| Gerçek model çıktıları (sonra) | — | `llm_call.response` alanında saklanan `propose_segment` girdileri (D-060): canlı kampanyalar oldukları anda bedava korpus olur |

**Go-yerel fuzz korpusu (`testdata/fuzz`) uygun değildir:** `FuzzParse`/`FuzzCheck` tohumları çoğunlukla geçersiz girdidir; rastgele baytlar nadiren geçerli DSL olur. Üretici ayrı yazılmalı. Üretilen ifadelerin `Print` (kanonik) ve model-benzeri (çok satırlı, girintili) iki yazımı da ölçülür.

### 1.8 Karşılaştırılacak biçimler

| Kod | Biçim | Not |
|---|---|---|
| D1 | DSL, modelin yazdığı gibi (korpus metni) | Çok satırlı olabilir |
| D2 | DSL, `dsl.Print` kanonik | Tek boşluk, çift tırnak |
| D3 | `{"dsl":"..."}` tool girdisi, tırnaklar `\"` kaçışlı | **Gerçekte çıktıya giren budur**; DSL'deki çift tırnaklar JSON string içinde kaçış maliyeti taşır |
| D3' | Aynısı, DSL'de tek tırnak (`'İstanbul'`, lexer izin veriyor [D, D-065]) | Prompt'a "tek tırnak kullan" demenin tasarruf edip etmediğini gösterir |
| J1 | `compile.Compile` çıktısı, kompakt, anahtar sıralı, birleştirmesiz | Üst sınır (D-067: birleştirme/sadeleştirme yok) |
| J1m | Bench içinde yazılacak normalleştirici: ayrık anahtarlı `and` listesini tek nesneye katla (`{city:{eq}, cartItems:{...}}`) | Bir modelin doğrudan yazacağı daha kısa biçim; alt sınır |
| J2 | J1 girintili (`json.MarshalIndent`) | Modelin yazma eğilimi; boşluk ve satır sonu token'ları |
| J3 | `{"filter": J1}` veya string içinde J1 (`{"filter_json":"{\"and\":..."}`) | Taşıma simetrisi (kol B(b)); iç içe kaçışlar |
| G | İsteğe bağlı: değişken yerine satır içi GraphQL belgesi | Yalnız D-007 "ham GraphQL" seçeneği tamamen açılırsa |

Rapor tablosu: biçim × model × (bayt, rune, sözcüksel token, `count_tokens` token) ve oranlar `J/D` (D3'e göre J3, D1'e göre J1...). Girdi-çıktı ayrımı için sütun "faturalanan taraf": D3/J3 **çıktı**, SDL/gramer **girdi**.

**Öneri:** Aşama 0 (çevrimdışı) hemen; Aşama 1 (`count_tokens`) kullanıcı anahtar kullanımına izin verince, bir kez, sonuç commit edilir. Ayrıntı: "Hemen başlanabilecek maliyet benchmark'ı".

## 2. Gecikme / CPU

### 2.1 Mikro benchmark'lar (Go `testing.B`)

Hepsi anahtarsız, Docker'sız; `make test`'e dokunmaz (`go test` benchmark çalıştırmaz, yalnız derler).

| Benchmark | Ölçülen | Yer |
|---|---|---|
| `BenchmarkParse` | `dsl.Parse(src)` | `internal/proposal/dsl/bench_test.go` |
| `BenchmarkPrint` | `dsl.Print(ast)` | aynı |
| `BenchmarkCheck` | `catalog.Check(src, ast)` (önceden ayrıştırılmış AST) | `internal/proposal/catalog/bench_test.go` |
| `BenchmarkCompile` | `compile.Compile(src)` (Parse + Check + oluşturma) | `internal/proposal/compile/bench_test.go` |
| `BenchmarkMarshal` | `Filter.MarshalJSON` | aynı |
| `BenchmarkJSONUnmarshal` | `json.Unmarshal(J1, &map[string]any)` — Kol B'nin girişi | `bench/` (kol karşılaştırması) |
| `BenchmarkJSONValidate_*` | Kol B doğrulama alternatifleri (aşağıda) | `bench/` |
| `BenchmarkAudienceCompile` | `audience.Compiler.Compile(*model.CustomerFilter)` (Customer'ın SQL derleyicisi, yansıma tabanlı) | `internal/customer/audience/bench_test.go` |
| `BenchmarkEndToEndNoNetwork` | Parse → Check → Compile → `audience.Compile` | `bench/` |

Giriş boyutları tablo-tabanlı alt benchmark'larla (`b.Run("nodes=5", ...)`): korpusun düğüm katmanları (1, 5, 20, 100) ve derinlik 8. `for b.Loop()` (Go 1.24'ten beri) ve `b.ReportAllocs()`; `b.SetBytes(len(src))` ile MB/sn da görülür.

**Kol B'nin Proposal tarafı doğrulama seçenekleri (karşılaştırma için):**

| Seçenek | Artı | Eksi |
|---|---|---|
| (1) Hiç doğrulama, Customer'a bırak | Sıfır Proposal CPU, sıfır yeni kod | Her hata bir HTTP/GraphQL gidiş-dönüşü (ms); hata mesajı konumsuz (`filterPath`); sınırlar ve kapalı liste hataları Customer'dan döner (D-062 zaten hepsini denetliyor) |
| (2) `gqlparser` `validator.VariableValues(schema, op, vars)` | Bağımlılık zaten var (`vektah/gqlparser/v2 v2.5.37`, `schema_test.go` kullanıyor) | `@oneOf` (TimeValue) zorlamasını yapıp yapmadığı doğrulanmadı `[?]` (`vars.go` içinde `oneOf` geçmiyor, `validator/schema.go`/`rules`'da var); özel katalog kuralları (kapalı liste, düğüm sınırı) ayrıca kod ister |
| (3) JSON Schema (SDL'den üretilmiş) | Standart, ayrıştırılabilir hata yolları | Yeni bağımlılık (`santhosh-tekuri/jsonschema` vb. go.mod'da yok; `invopop/jsonschema` dolaylı bağımlılık, yalnızca üretici); `@oneOf` ve katalog kuralları yine ayrıca |
| (4) El yazması yürüyücü (`map[string]any` üzerinde `catalog.Check` eşi) | Konumlu mesajlar, Kol A ile eşdeğer güvence | En çok kod; DSL yerine JSON sözdizimi için bakım |
| (5) gqlgen'in üretilmiş `UnmarshalInputCustomerFilter`'ı | Customer'la birebir aynı tip denetimi | Proposal'ın Customer'ın `internal/customer/graph/model` paketine bağımlılığı (katman ihlali; bench'te sorun değil, üretimde değil) |

Ölçüm yalnız CPU/ayırma değil, **hata kalitesi**: sabit bir hata kümesi için (yanlış alan, tip, kapalı liste dışı değer) her seçeneğin döndürdüğü mesajın uzunluğu ve konum bilgisi. Yani Bölüm 1.6 ile birleşir.

### 2.2 Profil çıkarma

```sh
go test -run '^$' -bench 'Compile|Parse|Check' -benchmem -count 10 \
    -cpuprofile cpu.out -memprofile mem.out ./internal/proposal/compile/
go tool pprof -http=:8080 cpu.out         # Flame Graph: View menüsü
go tool pprof -sample_index=alloc_space -http=:8081 mem.out
```

- `-cpuprofile`/`-memprofile` yalnız **tek paket** için çalışır (`go test` kısıtı); çok paketli `./...` ile kullanılamaz, bu yüzden hedefler paket başınadır.
- `benchstat` (golang.org/x/perf/cmd/benchstat) iki çalıştırmanın (ör. değişiklik öncesi/sonrası) istatistiksel karşılaştırması için; `-count 10` ve laptop'ta başka iş açıkken yapılmamalı. `go tool` olarak eklenebilir (go.mod'daki `tool` yönergesiyle, `sqlc`/`swag`/`gqlgen` gibi, D-051/D-057 yöntemi) `[?]`.
- Mikro benchmark profilleri çok kısa süreli (µs) olduğundan, `-benchtime 3s` ve büyük korpus döngüsüyle örnek sayısı artırılmalı; aksi halde flamegraph'ta runtime/GC gürültüsü görünür.
- **`net/http/pprof`:** repoda yok [D, `grep pprof` boş]. Canlı servis profili için bayrakla açılan (`PPROF_ADDR=127.0.0.1:6060`) ayrı bir dinleyici önerilir; chi yönlendiricisine (`httpserver.NewRouter`) **bağlanmaz** (dışa açılırdı, ve `RequestLogger`/metrik middleware'inden geçerdi). Compose'ta host'a yayımlanmaz; `docker exec` veya `docker run --network` ile erişilir (Customer'da olduğu gibi). `go tool pprof http://host:6060/debug/pprof/profile?seconds=30` yük testi sırasında.
- **Sürekli profilleme:** Grafana Pyroscope (SDK veya Alloy ile pull) ve Parca (eBPF) seçenekleri var `[?]` (ikincil kaynaklar, lisanslar doğrulanmadı). Bu proje için fazla: tek geliştirici, tek makine, compose yığınında Prometheus bile yok. **Öneri:** sürekli profilleme yok; yük testi sırasında isteğe bağlı pprof.

### 2.3 Neyi gösterir, neyi göstermez; gürültü

- Beklenen büyüklük sırası: `Parse+Check+Compile` bir filtre için µs–on µs `[?]`; Customer'ın `audience.Compile`'ı yansımayla benzer mertebede; Postgres `audiencePreview` ms; LLM turu saniye. Farkın hangi tarafta olursa olsun LLM çağrısının 3–5 mertebe altında kalması beklenir. Sonuç "DSL CPU'su sorun değil" ya da "JSON doğrulaması da sorun değil" olur; **karar CPU'ya değil token ve kaliteye bağlanır**. Benchmark'ın değeri bunu rakamla yazılı hale getirmektir.
- Gürültü kaynakları: Apple Silicon güç/termal durumu, GC, `-race` (benchmark'ı yavaşlatır; `make test-go`'nun `-race`'i benchmark için kullanılmaz), önbellek sıcaklığı, tekrar eden aynı girdinin dallanma tahmincisini ısıtması (korpus döngüsü ile azaltılır), `fmt`/hata oluşturma yollarında geçersiz girdinin farklı maliyeti (geçerli ve geçersiz girdi ayrı raporlanır).
- Gürültü olmayan sinyal: ayırma sayısı (`allocs/op`) ve bayt/op, çalıştırmadan çalıştırmaya sabittir; CPU zamanından daha güvenilir bir regresyon göstergesidir.

### 2.4 Customer tarafı: filtre → SQL ve Postgres

- **Derleme:** `audience.Compiler.Compile` saf CPU; `cities.go:NewCities([]string)` ile sözlük kurulup benchmark edilir (veritabanı gerekmez). `previewSQL`/`recipientsSQL` paket içi olduğundan (`reach.go:26,40`) SQL metni benchmark'ı `package audience` içinde olmalıdır.
- **Yürütme:** `integration` etiketiyle (Docker gerekir), mevcut `TestMain` düzeneğini (testcontainers, seed'li Postgres, `integration_test.go`) kullanır. İki ölçüm: `BenchmarkPreview*` (Go tarafı, uçtan uca pgx) ve sunucu tarafı `EXPLAIN (ANALYZE, BUFFERS)` — derlenen `Where` + `Args` ile (parametreli, `PREPARE` gibi). Not: sorgular `statement_timeout = 5s`'lik salt okunur transaction içinde çalışır (D-062); `EXPLAIN ANALYZE` aynı transaction'da yapılmalı.
- **`pg_stat_statements` [D]:** `shared_preload_libraries = 'pg_stat_statements'`, `compute_query_id = on`, `CREATE EXTENSION` her veritabanında; sunucu yeniden başlatması gerekir. Compose'taki `postgres:18-alpine` varsayılan olarak yüklemez; `infra/postgres/initdb` içine bir betik ve `command:` ile `-c shared_preload_libraries=...` bayrağı gerekir (şema/altyapı değişikliği → kodlama kuralı 7, onay ister, K9). Anahtar sütunlar: `calls`, `total_exec_time`, `mean_exec_time`, `stddev_exec_time`, `rows`, `shared_blks_hit/read`; plan süresi için `pg_stat_statements.track_planning = on` (yüksek eşzamanlılıkta yük bindirebilir, dokümana göre) [D]. Sıfırlama: `pg_stat_statements_reset()`.
- **Önemli sınırlama:** seed `dim_customer` 1.000 satırlık [D, `database-architecture.md`] (stub ölçeklemesi "açık" konu, Araştırma 4). Bu boyutta SQL süreleri µs–ms ve gürültü ağır; plan farkları görünmez (sıralı tarama yeter). Anlamlı SQL ölçümü için ölçeklenmiş bir veri kümesi (ör. 100 bin müşteri) gerekir; bu, seed ölçeklemesi kararıdır (K9 ile birleşir).
- **Yaklaşım-bağımsızlık:** iki kol aynı `CustomerFilter`'ı gönderir; aynı filtre aynı SQL. Fark yalnız modelin *farklı* (örneğin birleştirilmemiş `and`/gereksiz `or`/fazla `not`) filtre yazması durumunda doğar; bu `EXPLAIN` düğüm sayısı ve süre farkı olarak canlı koşuda (Bölüm 4) kaydedilir.

### 2.5 Uçtan uca: Proposal → Customer önizleme gecikmesi

`internal/proposal/customer.Client.Preview` zaten gerçek Customer handler'ı + seed'li Postgres ile entegrasyon testinde çalışıyor (`TestSampleBriefPreview`). Bunun üzerine `BenchmarkClientPreview` (integration etiketi): aynı sürecin içinde `httptest` sunucusu — ağ yığını gerçek ama konteyner sınırı yok. Ayrıca konteynerler arası gerçek ölçüm için Bölüm 3. Ayrıştırma: istemci tarafı süre (`audience_preview_ms` = Parse+Check+Compile + HTTP + yanıt ayrıştırma) ve Customer tarafı `http_request_duration_seconds{route="/graphql"}` farkı → ağ+serileştirme payı. İstemci 3 deneme/10 sn zaman aşımı (D-067) benchmark'ta kapatılmalı ya da ayrı sayılmalı, yoksa hata yolu gecikmeyi şişirir.

## 3. Yük testi

### 3.1 Araçlar

| Araç | Sürüm (bakıldı) | Lisans | Model | Not |
|---|---|---|---|---|
| **vegeta** | v12.13.0 (31 Eki 2025) | MIT | **Açık model**: `-rate` ile sabit hız, `-duration`; `-max-workers` tavanı | Go; `-format json` hedef dosyası (gövde base64); `report -type hist[...]`/`json`; `vegeta/lib` kütüphanesi Go içinden kullanılabilir; HTTP/2 var (`-http2`, `-h2c`) |
| **k6** | v2.3.0 (21 Eyl, yıl sayfada görünmüyor) `[?]` | **AGPL-3.0** | Senaryo + sanal kullanıcı; `constant-arrival-rate` yürütücüsü açık model verir | JS betikleri, eşikler (`thresholds`); yerleşik SSE yok; topluluk uzantısı `xk6-sse` (özel binary için `xk6`) veya `k6/experimental/streams` `[?]` |
| **hey** | sürüm sayfası boş; son etiket eski `[?]` | Apache-2.0 | Kapalı model (`-c` eşzamanlı işçi); `-q` işçi başına hız sınırı | Bakımı zayıf görünüyor `[?]`; kapalı model "coordinated omission" nedeniyle doygunlukta gecikmeyi eksik gösterir |
| **oha** | v1.16.0 (23 Ağu, yıl görünmüyor) `[?]` | MIT | Kapalı model + `-q` hız sınırı | Rust, TUI, `--output-format json`, HTTP/2; hızlı keşif için |

AGPL notu `[?]`, hukuki görüş değil: k6'yı ayrı bir araç/konteyner olarak çalıştırmak depoya kod bağlamaz; yine de depoya k6 kopyalamak/dağıtmak yerine betik (JS) commit edip `docker run grafana/k6` kullanmak lisans sorusunu açmaz.

**Öneri: vegeta.** Açık model (sabit hızda gecikme ölçer; doygunlukta gerçek kuyruklanmayı gösterir), MIT, Go (aynı araç zinciri), tek hedef dosyası, çıktısı `benchstat` benzeri karşılaştırma için JSON. k6 yalnız çok adımlı senaryo (kampanya oluştur → SSE dinle → onayla) gerekirse. `hey` ve `oha` kapalı model olduğu için birincil değil.

### 3.2 Hedefler

**Customer `audiencePreview` (GraphQL, LLM'siz):**

```sh
# vegeta hedefi (JSON): bench/load/customer-preview.json  (gövde base64)
# POST http://customer:8080/graphql
# Authorization: Bearer <CUSTOMER_TOKEN_PROPOSAL>   (kapsam audience:read, D-063)
# gövde: {"query":"query AudiencePreview($filter: CustomerFilter!) {...}","variables":{"filter": <korpus J1>}}
vegeta attack -format json -rate 50 -duration 30s -targets customer-preview.json | vegeta report
```

- **Customer host'a yayımlanmıyor** (compose yorum satırı: "customer is reachable only inside the compose network" [D]). Yük aracı compose ağı içinde çalışmalı (`make consent`'in `docker run --network coldbru_default` kalıbı) veya `bench` profili/override ile geçici bir host portu açılmalı (K9: compose değişikliği).
- Sorgu metni `internal/proposal/customer/operations.go`'daki `previewQuery`; yük betiği bunu kopyalamak yerine, ondan üretilmeli (aksi halde sapar; `TestQueriesMatchSchema` yalnız üretim metnini korur). Korpus filtreleri farklı olmalı (tek filtre tekrarı Postgres önbelleğini ısıtır, planlayıcı sonucunu değil tampon önbelleğini).
- Sınırlar: pgx havuzu varsayılanı `max(4, CPU)` `[?]` (`db.Open` yalnız `ParseConfig` çağırıyor; `pool_max_conns` URL'den); `statement_timeout` 5 sn → aşılırsa `QUERY_TIMEOUT`. Hız basamakları (10, 25, 50, 100, 200 rps) ve "diz" noktası aranır.

**Proposal uç noktaları (LLM'siz):** T-025 (REST+SSE) bu araştırma sırasında **bitmemiş** (`feature/proposal-rest-sse` dalı; rotalar kesinleşmedi `[?]`), bu yüzden bu bölüm rotasız tasarımdır. LLM'i çıkarmanın iki yolu:

| Yol | Artı | Eksi |
|---|---|---|
| (a) `ANTHROPIC_BASE_URL` (D-069 uygulama notu) bir **mock Messages sunucusuna** işaret eder | Üretim kodu yoluna dokunmaz; SDK, akış, onarım döngüsü, `llm_call` yazımı gerçek çalışır | `llmtest.MessagesServer` `testing.TB` ister (testten bağımsız çalışmaz); bağımsız bir `cmd/mockllm` gerekir (yeni ikili → karar). Mock'un gecikme dağılımı gerçekçi olmalı (örn. log-normal, medyan birkaç sn; turlar arası); aksi halde yük testi yalnız Go'yu ölçer |
| (b) Süreç içi `llmtest.Scripted` (D-025) ile bir Go yük harness'ı | Küçük, Docker'sız, `RunParallel` ile | HTTP yığınını, nginx'i, gerçek pgx havuzunu atlar; yalnız `flow.Service` iç kapasitesini ölçer |

**Öneri:** (a) uçtan uca için, (b) `flow` iç kapasitesi için. Gözlenecekler: kampanya/dk verimi, advisory lock kuyruğu (D-070: kampanya başına kilit, çalışan sayısı havuzun yarısı ile sınırlı), `llm_call` ekleme gecikmesi, goroutine sayısı, havuz doygunluğu.

**SSE yayılımı (fan-out):** hedefler: N istemci × M kampanya. Basit ve kontrollü: `bench/ssebench` altında küçük bir Go istemcisi (`net/http` + `bufio.Scanner`, `Content-Type: text/event-stream`), her olayda sunucu zaman damgası ile alım zamanı farkı (olay gecikmesi), bağlantı kurma süresi, kopma sayısı; k6 uzantısı yerine tercih edilir (özel binary yok). Doğrudan Proposal ve nginx üzerinden (D-015: gateway tamponlaması; `proxy_buffering off` gerekir) ayrı ayrı. İşletim sistemi sınırları: `ulimit -n`, süreç başına goroutine (her bağlantı ≥ 1 goroutine). Süreç içi olay dağıtıcı (D-070) çoklu replikada paylaşılmaz; yük testi tek replika içindir.

### 3.3 Toplanacak metrikler

- **Zaten var [D]:** `http_requests_total{method,route,status}` ve `http_request_duration_seconds{method,route}` (`internal/platform/metrics`), Go ve süreç toplayıcıları (goroutine, bellek, GC, açık fd). Kovalar `prometheus.DefBuckets` (5 ms … 10 sn). Bu kovalar HTTP için uygun; **LLM çağrıları 10 sn'yi aşabilir**, kova tavanı yetmez (K11). Çok kısa işlemler (µs) için histogram değil `testing.B` kullanılır.
- **Yok [D, `grep` ile doğrulandı]:** LLM metrikleri (çağrı süresi, token, sonuç), DSL hata kodu sayaçları, Customer `QUERY_TIMEOUT` sayacı, pgx havuz istatistikleri, SSE abone sayısı. Önerilen minimum: `llm_call_duration_seconds{purpose,model,outcome}` (kovalar 0,5…60 sn), `llm_tokens_total{purpose,model,type}`, `dsl_errors_total{code}`, `sse_clients`, `pgxpool_*` (pgxpool.Stat). Etiket kardinalitesi: `prompt_name` evet, `prompt_version` hayır (D-060 kuralı).
- **Prometheus sunucusu compose'ta yok.** Seçenekler: (1) yük öncesi/sonrası `curl /metrics` ve histogram farkı alan küçük betik (sıfır altyapı; yüzde dilimleri kova çözünürlüğünde), (2) `bench` profiliyle Prometheus konteyneri. **Öneri (1)**; birincil gecikme sayısı vegeta'nın istemci tarafı ölçümü, sunucu histogramı çapraz kontrol (K9).
- İstemci tarafı ölçüm birincil: sunucu histogramı kuyruk bekleyişini görmez (istek handler'a girmeden önceki süre).

## 4. LLM düzeyinde karşılaştırma (gerçek çağrı ister)

Burada yapılabilecek **hiçbir şey** anahtarsız bir sonuç vermez; yalnız altyapı hazırlığı anahtarsızdır (mock sunucuyla çalışan harness).

### 4.1 Deney tasarımı

- **Kollar:** A = mevcut `segment@1.0.0` (DSL). B1 = GraphQL filtre JSON'u **string alanda** (`filter_json`), strict tool. İsteğe bağlı B2 = serbest `object` alanlı strict olmayan tool. Her kolda aynı `count_audience` yoklama aracı (JSON'u alan; DSL kolu DSL alan). Ekranlama (Haiku) kapatılır; yalnız segment çağrısı ölçülür.
- **Prompt:** B kolunun paketi `segment-gql@x.y.z`. Yeri: ya `internal/proposal/prompts/segment_gql/` (D-059'un kilidine girer, `prompts.lock.json` ve `make generate-prompts` etkilenir, üretime sızma riski) ya da `eval/prompts/` altında deney paketi (kilit dışı, hash'i sonuç dosyasına yazılır). **Öneri:** eval paketi, kilit dışı (K7).
- **Vakalar:** `eval/cases/` (D-061'de planlı; repoda henüz yok [D, dizin yok]). ≥ 40 brief, TR/EN karışık; kolaylık katmanları: tek koşul; çoklu `and`; `or`/`not`; ilişki niceleyicileri (`some`/`none`/`every`); zaman (`ago`, `date`); kapalı liste; şehir yazım hatası; katalogda olmayan istek (yok sayılmalı); belirsiz brief. Her vaka için **referans DSL** → `compile.Compile` → referans alıcı kümesi (seed'de Customer'dan).
- **Ölçütler (hepsi kodla, LLM hakem yok, D-061):**
  1. İlk `propose_segment` geçerli mi (Kol A: `Parse+Check+Customer`; Kol B: JSON ayrıştırma + doğrulama + Customer).
  2. En fazla 3 deneme içinde kabul (D-027/D-069) oranı; ortalama onarım sayısı; hata kodu dağılımı.
  3. **Anlamsal doğruluk:** kabul edilen filtrenin alıcı kümesi referansla eşit mi (kimlik kümesi eşitliği; sayı eşitliği yetmez). Geçerli ama yanlış filtre en kötü hatadır; yalnız geçerlilik ölçmek DSL/JSON farkını gizleyebilir.
  4. Tur sayısı, `count_audience` yoklama sayısı, `EMPTY_AUDIENCE` oranı.
  5. Girdi/çıktı/önbellek token'ları (`llm.Usage`), thinking token'ı ayrı `[?]`; `usage.output_tokens` ile `count_tokens` tahminini karşılaştır (Bölüm 1.3 çapraz kontrol).
  6. Tur başına gecikme: ilk token süresi (TTFT) ve toplam, `llm.WithRecorder` ile; `llm_call` benzeri kayıt dosyaya (veritabanına yazmadan).
  7. Filtre karmaşıklığı: düğüm sayısı, derinlik (model gereksiz karmaşık filtre yazıyor mu), `EXPLAIN` düğüm sayısı.
- **Çalıştırma:** `//go:build live` etiketi ve `ANTHROPIC_API_KEY` (D-061, `live_test.go` kalıbı); `make eval-live` var olacak hedefin bir bayrağıyla (`VARIANT=dsl|gql`) ya da ayrı `make bench-live`. Sonuç JSONL: `eval/results/<tarih>-<model>-<kol>.jsonl` (gitignored; özet PR'a eklenir, D-061).

### 4.2 Örneklem ve maliyet

- Hedef: "ilk denemede geçerlilik %90 mı %80 mi" gibi 10 puanlık fark, %80 güç, %5 anlamlılık için iki oranlı karşılaştırmada kol başına ≈ 200 örnek gerekir (hesap: ((1,96·√(2·0,85·0,15) + 0,84·√(0,9·0,1+0,8·0,2))² / 0,1² ≈ 199). Vakalar arası bağımlılık (aynı brief tekrarları) etkin örneklemi düşürür; eşleştirilmiş tasarım (aynı brief iki kolda) ve brief düzeyinde bootstrap güven aralığı önerilir `[?]` (istatistik yöntemi karar).
- ≈ 40 brief × 5 tekrar = kol başına 200 çalıştırma. `temperature` sabitlenemediği için tekrar şart.
- **Maliyet tahmini (varsayımlarla, ölçüm değil `[?]`):** çalıştırma başına: önbellekli girdi ~5 bin token × $0,10/MTok = $0,0005; yeni girdi ~0,5 bin × $2/MTok = $0,001; çıktı (thinking + tool) ~1,2 bin × $10/MTok = $0,012 → tur başına ≈ $0,015; ortalama 2,5 tur (yoklama ve onarım) → **≈ $0,04**. İki kol 400 çalıştırma ≈ **$16**, üç kol ≈ $24; kol başına bir kez soğuk yazma ve fiyat/çıktı belirsizliği ile **$10–40 aralığı**. D-061'deki "tur başına yaklaşık 0,3 $" tahmini (araştırma 5'ten, ölçülmedi) bir "tur"un tüm vaka kümesi olduğu varsayımıyla uyumlu: 40 vaka × 0,04 ≈ 1,6 $, bu eski tahminden düşük; çelişki gerçek koşuyla çözülür.
- İlk adım olarak **küçük pilot**: 10 brief × 2 kol × 3 tekrar = 60 çalıştırma ≈ $2–3; çıktı token dağılımını ve gerçek maliyeti öğrenip tam deneyi buna göre kurmak için. Batch API (%50) yalnız tek turlu ilk deneme geçerliliği için uygulanabilir `[?]`.
- Haiku 5.5'i segment için ayrıca denemek (JSON yolu daha ucuz bir modelde yeterli mi) kapsam dışıdır; D-026 Sonnet kararı geçerli kalır.

## 5. Sistemde ölçülmeye değer diğer şeyler

| Kalem | Neden | Nasıl |
|---|---|---|
| **Kampanya başına LLM maliyeti** | Brief'in "maliyet" beklentisi; D-060 `llm_call` ve Analytics `llm_usage` zaten token'ı taşıyor | `llm_call`'dan `purpose` (screen/segment/content/channel_plan/schedule) × model × 4 token türü; fiyat tablosu yapılandırmada (fiyat sayfası değişir). Çıktı: kampanya başına $ ve amaç payları. K10 |
| **Haiku çağrıları** (ön eleme, plan, zamanlama) | 5.5 Haiku $0,10/$0,50; ama 3 çağrı/kampanya. Haiku'nun yeni tokenizer'ı aynı metni ~%30 fazla sayar [D] | `llm_call`'dan amaç başına ortalama token ve gecikme; ön eleme TTFT'i kullanıcıya görünen bekleme |
| **Metin onarım oranı** (D-071: 1 + 2) | Metin kanal sınırlarına uymak zorunda: SMS ≤ 160 (en uzun ad ile), konu ≤ 80, yalnız `{{first_name}}` | `llm_call.outcome` (`accepted`/`validation_failed`) metin amacı için; canlı koşuda ilk denemede geçme oranı |
| **SMS segment maliyeti** | Türkçe neredeyse her zaman UCS-2: tek SMS 70, birleşik 67 karakter/segment (D-028, Araştırma 3 §5). Alıcı başına segment × alıcı sayısı çoğu zaman LLM maliyetinden büyük kalem olur `[?]` (sağlayıcı fiyatı bilinmiyor) | Üretilen SMS'lerin kodlama ve segment dağılımı; `kampanya_maliyeti = alıcı × segment × birim_fiyat` formülü; fiyat mock/yapılandırma |
| **Önbellek isabet oranı** | Önek sürüm başına bir kez soğur (D-059); gerçek trafikte TTL boşlukları isabeti düşürür | `cache_read / (cache_read + cache_creation + input)` prompt sürümü ve zaman penceresi başına |
| **TTFT (metin akışı)** | Kullanıcının algıladığı bekleme (D-013 SSE) | İlk delta'ya kadar süre, delta sayısı, akış toplam süre; mock ve canlı ayrı |
| **Outbox/relay gecikmesi** | LISTEN/NOTIFY + 1 sn poll (D-054) | olay yazımı → NATS yayını farkı |
| **Gönderim hız sınırlayıcı doğruluğu** | Kanal başına 100 msg/s (D-024) | gerçek oran vs 100; kuyrukta bekleme p95; mock provider 429 oranının etkisi (T-029/T-031 sonrası) |
| **`audienceRecipients` sayfalama verimi** | Onayda alıcı listesi sayfa başına ≤ 1000 (D-058) | sayfa başına süre, toplam süre / alıcı; büyük kitle |
| **`consentStatus` toplu sorgusu** | Gönderim anı izin kontrolü, ≤ 1000 kimlik (D-011) | gecikme ve devre kesici tetiklenmesi |
| **Veritabanı** | `QUERY_TIMEOUT` ve havuz | Customer'da sayaç (yok); `pg_stat_statements` (Bölüm 2.4) |
| **Hata kodu dağılımı** | Hangi DSL/JSON hataları sık | canlı koşuda ve üretimde `dsl_errors_total{code}` |

## 6. Kodun nerede yaşayacağı ve hangi hedefte çalışacağı

| Parça | Yer | Hedef | `make test`'te mi? |
|---|---|---|---|
| Paket mikro benchmark'ları | `internal/proposal/{dsl,catalog,compile}/bench_test.go`, `internal/customer/audience/bench_test.go` | `make bench` (yeni): `go test -run '^$' -bench . -benchmem -count 10` paket başına, çıktı `bench/results/` (gitignored) | Hayır (benchmark çalışmaz, yalnız derlenir). Çürümeyi önlemek için isteğe bağlı `make bench-smoke`: `-benchtime 1x` ile her benchmark bir kez (K4) |
| Kol karşılaştırması (JSON doğrulama alternatifleri, uçtan uca CPU) | `bench/` Go paketi (`bench/compare_bench_test.go`) | `make bench` | Hayır |
| Korpus + üretici | `bench/corpus/` (dosyalar: `.dsl`), `bench/corpusgen/` (üretici, `math/rand/v2` PCG) | — | Üreticinin kendi birim testi evet (üretilen her ifade `compile.Compile` geçmeli) |
| Token aracı | `cmd/tokenbench` (çevrimdışı varsayılan; `-count-tokens` bayrağı anahtar ister) + `bench/tokens/cache.json` | `make bench-tokens` (anahtarsız, önbellekten), `make bench-tokens-refresh` (anahtar) | Önbellek tazelik testi evet olabilir (korpusta olup önbellekte olmayan girdi uyarısı), anahtarsız |
| Entegrasyon benchmark'ları (Customer SQL, `Client.Preview`) | ilgili paket `*_bench_test.go`, `//go:build integration` | `make bench-integration` (Docker) | Hayır |
| Yük betikleri | `bench/load/` (vegeta hedefleri, SSE istemcisi `bench/ssebench`) | `make bench-load` (compose ayakta olmalı) | Hayır |
| Mock LLM sunucusu | `cmd/mockllm` veya `internal/proposal/llm/llmtest` içinde `testing.TB`siz yapıcı | `make bench-load` | Yeniden kullanım yalnızca `llmtest` içi refactor ise testler etkilenir |
| Canlı LLM deneyi | `eval/` + `//go:build live` test/komut | `make bench-live` (veya `make eval-live VARIANT=gql`) | Hayır, anahtar ister |

Mevcut `Makefile` hedefleri: `test`, `test-go`, `test-unit`, `test-web`, `coverage`, `generate*`, `consent`. `bench*` hiçbiri yok [D]; `.PHONY` listesine yeni hedefler eklenmeli. `eval` ve `eval-live` D-061'de planlı ama Makefile'da henüz yok [D].

## 7. Kullanıcıdan beklenen kararlar

Her biri için öneri ilk sırada.

**K1. Token sayacı ve anahtar politikası.**
(a) **Önerilen:** iki aşama. Çevrimdışı vekiller her zaman; `count_tokens` (ücretsiz, anahtar ister) yalnız `make bench-tokens-refresh` ile, sonuçlar `bench/tokens/cache.json` olarak commit edilir; `cmd/tokenbench` `ANTHROPIC_API_KEY`'i yalnız bu hedefte okur. (b) Yalnız çevrimdışı vekiller, hiç `count_tokens` yok (anahtarsız, ama oran yanıltıcı olabilir). (c) Sayımı canlı koşulara bırak (`usage`), ucuz benchmark yok.

**K2. Karşılaştırılacak biçimler.**
(a) **Önerilen:** D1, D2, D3, D3', J1, J1m, J2, J3 (Bölüm 1.8). (b) Yalnız D3 ve J1 (en az iş, "3 kat" iddiasını sınar). (c) Ek olarak G (satır içi GraphQL belgesi).

**K3. Hangi modeller için token sayılsın.**
(a) **Önerilen:** yalnız `claude-sonnet-5-5` (filtreyi yazan model, D-026). (b) Sonnet 5.5 + Haiku 5.5 (tokenizer farkını da gösterir). (c) Ek Opus 5.5 (alternatif model değerlendirmesi için).

**K4. Benchmark'ların `make test` ile ilişkisi.**
(a) **Önerilen:** `make test` benchmark çalıştırmaz; ayrı `make bench`; ek olarak `make bench-smoke` (`-benchtime 1x`) PR öncesi elle/CI'da. (b) Yalnız `make bench`. (c) `bench-smoke` `make test`'e dahil (yavaşlatır, ama çürümeyi yakalar).

**K5. Korpusun kaynağı.**
(a) **Önerilen:** mevcut test örnekleri `bench/corpus/*.dsl` dosyalarına taşınır ve hem testler hem benchmark oradan okur (tek doğruluk kaynağı) + gramer tabanlı üretici. (b) Kopyala, testlere dokunma. (c) Yalnız üretici.

**K6. GraphQL-JSON kolunun biçimi (canlı deney).**
(a) **Önerilen:** B1 = JSON bir string alanda (simetrik, strict tool). (b) B2 = serbest `object` alanlı, strict olmayan tool (modelin gerçekte yapacağına yakın, ama taşıma farkı karışır). (c) İkisi birden (maliyet ≈ %50 artar).

**K7. B kolunun prompt paketi nerede yaşar.**
(a) **Önerilen:** `eval/prompts/` altında, `prompts.lock.json` dışı; hash sonuç dosyasına yazılır. (b) `internal/proposal/prompts/segment_gql/` (D-059 kilidine girer, `requires_dsl` anlamı belirsiz). (c) Deney süresince geçici dal, sonra silinir.

**K8. Canlı deney bütçesi ve ölçeği.**
(a) **Önerilen:** önce pilot (10 brief × 2 kol × 3 tekrar ≈ $2–3), sonra pilot verisine göre tam koşu (40 × 5 × 2 ≈ $16, üst sınır $40 `[?]`). (b) Doğrudan tam koşu. (c) Canlı deney yok; yalnız ucuz benchmark'lar.

**K9. Altyapı değişiklikleri (kodlama kuralı 7 onayı).**
(a) **Önerilen:** compose'ta `bench` profili (Customer'ı host'a açan override ve isteğe bağlı `pg_stat_statements`'lı Postgres komutu), Prometheus konteyneri **yok**; sunucu metrikleri `curl /metrics` farkıyla. (b) Compose'a dokunma; yük aracı her zaman `docker run --network coldbru_default` ile, `pg_stat_statements` yok. (c) Tam gözlem yığını (Prometheus + Grafana).
Not: ölçekli seed (SQL ölçümünün anlam kazanması için) bu karardan ayrı, D-005/D-017 ve Araştırma 4'teki "seed ölçeği" açık konusuyla birleşir.

**K10. Metrik/maliyet izleme kapsamı ve fiyat tablosu.**
(a) **Önerilen:** `llm_call`'dan kampanya başına maliyet sorgusu/raporu; fiyat tablosu yapılandırma dosyasında, değerler fiyat sayfasından (Sonnet 5.5 önbellek okuması $0,10; `models.md` ile $0,20 çelişkisi giderilince). (b) Yalnız raporlama betiği, kodda fiyat yok. (c) Analytics'in `llm_usage` modeline fiyat sütunu.

**K11. Yeni Prometheus metrikleri (LLM, DSL, SSE).**
(a) **Önerilen:** `llm_call_duration_seconds` (0,5–60 sn kovaları), `llm_tokens_total`, `dsl_errors_total{code}`, `sse_clients` eklenir; `DefBuckets` HTTP için kalır. (b) Yalnız `llm_*`. (c) Metrik yok; benchmark tek seferlik dışarıdan.

**K12. Profilleme.**
(a) **Önerilen:** mikro benchmark profilleri + bayrakla açılan, yalnız 127.0.0.1'e bağlı `net/http/pprof` dinleyicisi (`PPROF_ADDR`), sürekli profilleme yok. (b) Yalnız mikro benchmark profilleri, servise pprof eklenmez. (c) Pyroscope/Parca ile sürekli profilleme.

## 8. Hemen başlanabilecek maliyet benchmark'ı

**Amaç:** üretim çağrısı yapmadan, DSL metni ile onun derlendiği GraphQL filtre JSON'u arasındaki boyut ve (isteğe bağlı) token farkını ölçmek; böylece D-007'nin "çıktı token'ını azaltır" hipotezi ve Araştırma 2'nin 35–45 / 90–110 / 130–170 tahminleri sayıya bağlanır.

**Adımlar:**

1. **Korpus (anahtarsız).** Bölüm 1.7'deki kaynaklardan en az 100–300 DSL ifadesi: prompt örnekleri, belge örnekleri, `examples`/golden/valid test girdileri, gramer tabanlı üretici (düğüm sayısına göre katmanlı).
2. **Her ifade için biçimleri üret (anahtarsız).** D1, D2, D3, D3' DSL tarafı; J1 (`json.Marshal(compile.Compile(src).Variable())`), J1m, J2, J3 JSON tarafı. Her biri için `compile.Compile` hatasız olmalı (aksi halde korpustan atılır).
3. **Çevrimdışı sayaçlar (anahtarsız):** bayt, rune, sözcüksel token (DSL lexer ve `json.Decoder.Token`), tablo ve oranlar. Çıktı: biçim × katman (düğüm sayısı) için ortalama/medyan/p95 ve `J/D` oranı. Beklenti (örnek brief ölçümü): J1/D ≈ 1,3 bayt [D]; sözcüksel ve token oranı bundan farklı olabilir.
4. **Sistem prompt'u karşılaştırması (anahtarsız, bayt):** `system.txt + grammar.txt + RenderCatalog + tools.json` ile `SDL parçası + RenderCatalog + tools_gql.json` bayt/rune.
5. **Maliyet modeli (anahtarsız):** Bölüm 1.2'deki formül bir elektronik tablo/Go fonksiyonu olarak; parametreler (ort. filtre token'ı, thinking token'ı, tur sayısı, önbellek isabet oranı) kaydırılabilir; "farkın dolar karşılığı" ve "hangi parametrede biçim farkı maliyeti belirler" duyarlılık tablosu.
6. **Gerçek token (anahtar ister, tek seferlik, ücretsiz):** `make bench-tokens-refresh` ile D1–J3 ve sistem prompt'u bileşenleri için `count_tokens` (`claude-sonnet-5-5`; K3'e göre Haiku da), `bench/tokens/cache.json` yazılır. Gerekli: Console'dan standart bir API anahtarı ve istek sınırı ihtiyacı: ~1.200–2.000 istek, 5.000 RPM (Start) sınırının çok altında. **Üretim (mesaj oluşturma) çağrısı yok; ücret yok** [D, token sayma dokümanı: "free to use"]. Alınan sayılar `[D, tarih ve model ile]` olarak raporlanır.
7. **Rapor:** `bench/results/tokens.md` (veya `make bench-tokens` çıktısı): tablo + "J/D" oranları + sistem önek farkı + duyarlılık tablosu. Rakamlar alıntılanmadan önce modeli ve tarihi taşımalı (tokenizer modele özgü).

**Anahtar gerekir mi?**

| Adım | Anahtar |
|---|---|
| 1–5 (korpus, bayt/rune/sözcüksel sayaç, maliyet modeli) | **Gerekmez.** Bugün başlanabilir. |
| 6 (`count_tokens`) | **Gerekir** (standart API anahtarı, `.env`'de `ANTHROPIC_API_KEY`), üretim yok, ücretsiz. Sonuçlar commit edilen önbellekte kalır; sonraki çalıştırmalar anahtarsız. |
| Çıktı token'ının *gerçek* örneklenmiş değeri, thinking payı, onarım oranı, ilk denemede geçerlilik | **Gerekir ve üretim çağrısı ister** (Bölüm 4); bu benchmark'ın dışında. |

**Neyi göstermez:** modelin DSL'i ya da JSON'u daha doğru yazıp yazmadığını, thinking token'larını ve onarım turlarını; bunlar "gerçek çalıştırma" ister. Bu benchmark yalnız "aynı filtrenin iki gösteriminin boyutu" sorusunu cevaplar.

## Doğrulanamayanlar

- Sonnet 5.5 ve Haiku 5.5'in tokenizer'larının birbirine göre aynı olup olmadığı; dokümanda yalnız "4.7 ve sonrası yeni tokenizer, ~%30 fazla token" ve "Sonnet 5.5 = Sonnet 5 tokenizer'ı" yazıyor.
- `count_tokens`'ın assistant/tool_use bloklu mesajlarda tool_result zorunluluğu; metni user mesajı olarak saymak yeterli görüldü.
- Modelin çıktı olarak örneklediği token sınırlarının kanonik kodlamayla birebir aynı olup olmadığı.
- `usage.output_tokens_details.thinking_tokens` alanının Go SDK v2.4.0'daki adı ve `llm.Usage`'a eklenmesi (skill dokümanı alanı veriyor, SDK kaynağında aranmadı).
- `gqlparser v2.5.37 validator.VariableValues`'in `@oneOf` girdi nesnelerini zorlayıp zorlamadığı (kaynakta `vars.go`'da `oneOf` geçmiyor).
- Segment öneki ve katalog çıktısının token sayısı (katalog çıktısı bu araştırmada üretilmedi).
- k6 ve oha sürüm tarihlerinin yılı; hey'in son sürümü; Pyroscope ve Parca lisansları; k6 `xk6-sse` olgunluğu.
- Postgres 18 alpine imajında `pg_stat_statements` modülünün contrib paketiyle geldiği (alpine imajlarında `postgresql-contrib` gerekebilir).
- pgx havuzu varsayılan `MaxConns` değeri (`max(4, CPU)` hatırlanan davranış).
- Çalıştırma başına çıktı token'ı (~1,2 bin) ve tur sayısı (~2,5) varsayımları; maliyet tahmini bunlara bağlı.
- Batch API'nin Sonnet 5.5 için tam %50 indirimi (sayfa genel olarak %50 diyor, model satırı okunmadı).

## Kaynaklar

- Token sayma (ücretsiz, RPM sınırları, tokenizer notu): https://platform.claude.com/docs/en/build-with-claude/token-counting · API referansı: https://platform.claude.com/docs/en/api/messages/count_tokens
- Fiyatlar: https://platform.claude.com/docs/en/about-claude/pricing
- Yerel claude-api skill dokümanları: `shared/token-counting.md`, `shared/models.md`, `shared/prompt-caching.md`, `shared/cost-optimization.md` (`/private/tmp/claude-501/bundled-skills/2.1.308/56faceda9a01cd9172d07f172bf49d8d/claude-api/`)
- Go SDK v2.4.0 (yerel kaynak): `github.com/anthropics/anthropic-sdk-go/v2@v2.4.0/message.go` (`CountTokens`, `MessageCountTokensParams`, `MessageTokensCount`) · https://github.com/anthropics/anthropic-sdk-go
- Çevrimdışı tokenizer yokluğu (ikincil): https://blog.gopenai.com/counting-claude-tokens-without-a-tokenizer-e767f2b6e632 · https://github.com/javirandor/anthropic-tokenizer
- Go: `testing` benchmark'ları, `b.Loop`, `-cpuprofile`/`-memprofile`: https://pkg.go.dev/testing · `go tool pprof`: https://pkg.go.dev/net/http/pprof · benchstat: https://pkg.go.dev/golang.org/x/perf/cmd/benchstat
- PostgreSQL `pg_stat_statements` (18): https://www.postgresql.org/docs/18/pgstatstatements.html
- Yük araçları: https://github.com/tsenart/vegeta · https://github.com/grafana/k6 · https://github.com/rakyll/hey · https://github.com/hatoo/oha · k6 uzantıları: https://grafana.com/docs/k6/next/extensions/explore/ · xk6-sse: https://github.com/phymbert/xk6-sse
- Sürekli profilleme (ikincil): https://devopsboys.com/blog/pyroscope-vs-parca-vs-grafana-pyroscope-profiling-2026 · https://betterstack.com/community/comparisons/parca-alternatives/
- Repo: `DECISIONS.md` D-007, D-026, D-027, D-038, D-041, D-059–D-061, D-062, D-065–D-071 · `docs/coldbru-gqdsl.md` §8.2, §13 · `docs/research/02`, `03`, `05` · `internal/proposal/{dsl,catalog,compile,customer,llm,prompts}` · `internal/customer/audience` · `internal/customer/graph/schema.graphqls` · `internal/platform/{httpserver,metrics}` · `Makefile` · `docker-compose.yml`
