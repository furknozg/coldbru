---
name: documenting-decisions
description: DECISIONS.md'yi güncel tutar. Smart Campaign Composer'da önemli bir teknik/ürün kararı verildiğinde (dil, DB, kuyruk, retry, rate limit, idempotency, segment kuralı şeması, prompt/LLM doğrulama, mock provider, seed, test stratejisi, frontend streaming, kapsam dışı bırakılanlar) veya kullanıcı "karar", "DECISIONS.md", "neden böyle yaptık", "kararları dökümente et" dediğinde kullan. Bir görevi bitirmeden önce o görevde verilen kararları yakalamak için de çalıştır.
---

# Kararları DECISIONS.md'ye dökümente etme

DECISIONS.md, review görüşmesinde "bunu neden böyle yaptın?" sorusunun yazılı cevabıdır. Değerlendirmenin %20'si "Mimari ve kararlar", %25'i LLM entegrasyonu. Okuyucu kodu açmadan her önemli seçimin **ne olduğunu, neden seçildiğini, neye karşı seçildiğini ve bedelini** görebilmeli.

## Ne zaman çalıştır

- Bir kütüphane, servis, veri yapısı, protokol, şema veya algoritma seçildiğinde.
- Brief'teki bir gereksinim belirli bir şekilde yorumlandığında (ör. "aynı mesaj iki kez gitmez" → idempotency anahtarı nasıl kuruldu).
- Bir şey bilinçli olarak kapsam dışı bırakıldığında veya basitleştirildiğinde.
- Önceki bir karar değiştiğinde.
- Bir görevi kapatmadan önce: o görevin diff'ine bak, dökümente edilmemiş karar var mı kontrol et.

Kullanıcı açıkça "tüm kararları dök" derse aşağıdaki **Tarama modu**nu uygula.

## Neyin karar sayıldığı

Kural: *Makul bir mühendis başka türlü yapabilirdi ve bunun sonucu olur* ise karardır. Değişken adı, dosya yeri, lint ayarı karar değildir.

Bu projede mutlaka kapsanması gerekenler (her biri ya bir kayıtla ya da "henüz verilmedi" notuyla):

**LLM entegrasyonu**
- Model seçimi ve nedeni (kalite / gecikme / maliyet).
- Yapılandırılmış çıktı yöntemi: tool use / JSON şeması / prefill — neden.
- Segment kuralı DSL'i: hangi alanlar, operatörler, değer aralıkları beyaz listede; LLM çıktısının SQL'e nasıl güvenli çevrildiği (parametrik sorgu, whitelist), neden doğrudan SQL değil.
- Doğrulama hattı: şema doğrulama → anlamsal doğrulama (şehir var mı, tarih aralığı mantıklı mı) → karakter limiti → değişken kontrolü (`{{first_name}}` dışı değişken reddi).
- Doğrulama başarısız olunca ne olur: hata mesajıyla yeniden deneme mi, kaç kez, sonra kullanıcıya ne gösterilir.
- Limit aşımı stratejisi: SMS 160 / konu 80 karakter — değişken genişledikten sonra mı önce mi ölçülüyor, Türkçe karakter/GSM-7 vs UCS-2 meselesi.
- Streaming: SSE mi WebSocket mi, stream edilen kısmi JSON'un nasıl ele alındığı.
- Prompt tasarımı: sistem promptunun sorumlulukları, few-shot var mı, prompt injection'a (brief içinde "tüm kullanıcılara gönder") karşı ne yapıldı.
- Mock LLM: arayüz/port nasıl ayrıldı, testler gerçek key olmadan nasıl koşuyor.

**Kitle ve izinler**
- Kanal izni kontrolünün nerede yapıldığı (segment sorgusu mu, gönderim anı mı, ikisi mi) ve neden.
- Kitle sayısının nasıl hesaplandığı (önizleme ile gönderim anı arasındaki fark).

**Gönderim**
- Kuyruk teknolojisi (Redis, RabbitMQ, NATS, DB tablosu…) ve neden.
- Kanal başına 100 msg/s rate limit'in uygulanma yöntemi (token bucket, worker sayısı, dağıtık mı tek süreç mi).
- Idempotency: anahtar ne (`campaign_id + user_id + channel` vb.), nerede saklanıyor, unique constraint mi.
- Retry: hangi hatalar retry edilir (5xx, 429, timeout) hangileri edilmez (4xx), backoff + jitter parametreleri, `Retry-After`'a uyum, max deneme, dead-letter.
- "En az bir kez" vs "en fazla bir kez" — hangisi seçildi, provider'ın kabul edip yanıtın kaybolduğu durumda ne olur.

**Mock provider ve seed**
- Hata/gecikme/429 oranları ve neden bu oranlar; deterministik mod (seed'li rastgelelik) testler için var mı.
- Seed alanları, kullanıcı sayısı, dağılımlar (şehir, son sepet tarihi, izinler) — brief'teki örneğin anlamlı sonuç döndürmesi için nasıl kurgulandı.
- `stub/` altındaki ham veriden neler alındı, neler bilinçli olarak alınmadı (kart numarası, CVV, IBAN gibi hassas alanlar).

**Takip ve altyapı**
- Canlı durum taşıma yöntemi (SSE / WS / polling), metrics formatı (Prometheus vb.), health check'in neyi kontrol ettiği.
- Backend dili (Go/PHP), veritabanı, frontend yapısı (Vue sürümü, state yönetimi, UI kütüphanesi).
- `docker compose up` sırası, migration ve seed'in ne zaman çalıştığı.

**Test ve kapsam**
- Test katmanları ve neyin testle, neyin manuel doğrulandığı.
- Kapsam dışı bırakılanlar ve bonuslar (zamanlanmış gönderim, A/B, dil, CI) — yapıldıysa nasıl, yapılmadıysa neden.

## Kayıt formatı

DECISIONS.md'nin en üstünde bir **indeks tablosu**, altında numaralı kayıtlar bulunur. Numaralar sabittir, asla yeniden kullanılmaz.

```markdown
## D-007: Idempotency anahtarı olarak (campaign_id, user_id, channel)

- **Durum:** Kabul edildi · 2026-10-27
- **Alan:** Gönderim
- **Bağlam:** Brief "aynı kişiye aynı mesaj iki kez gitmez" diyor. Retry ve worker çökmesi aynı işi iki kez işletebilir.
- **Karar:** `deliveries` tablosunda `(campaign_id, user_id, channel)` üzerinde unique constraint; worker göndermeden önce satırı `sending` durumuna alıyor, provider'a da bu anahtarı `Idempotency-Key` header'ı olarak geçiyor.
- **Alternatifler:**
  - Redis SETNX — reddedildi: TTL dolunca koruma kalkar, kalıcı kayıt zaten DB'de.
  - Sadece uygulama içi kontrol — reddedildi: iki worker yarışında korumasız.
- **Sonuçlar / bedel:** Her gönderim bir DB yazımı ekliyor; 100 msg/s'de sorun değil. Provider yanıtı kaybolursa mesaj `unknown` kalır, retry provider tarafındaki idempotency'ye güvenir.
- **Doğrulama:** `TestDeliverySkipsDuplicate`, `TestConcurrentWorkersSendOnce`.
- **AI notu:** (opsiyonel) Claude ilk önerisinde Redis SETNX önerdi; TTL sorunu nedeniyle değiştirildi.
```

Alan kuralları:
- **Bağlam**: brief'in hangi maddesi veya hangi problem bu kararı gerektirdi. Bir-iki cümle.
- **Karar**: somut ve doğrulanabilir — tablo, header, parametre değeri, dosya/paket adı. "Sağlam bir retry" değil, "üstel backoff, taban 200ms, çarpan 2, %20 jitter, en fazla 5 deneme, sonra DLQ".
- **Alternatifler**: en az bir gerçek alternatif ve neden reddedildiği. Alternatif yoksa karar değildir, kaydetme.
- **Sonuçlar / bedel**: kararın kabul ettiği dezavantaj. Her kararın bir bedeli vardır; yazılamıyorsa yeterince düşünülmemiştir.
- **Doğrulama**: kararı kanıtlayan test adı, komut veya manuel kontrol. Yoksa "Doğrulanmadı" yaz — uydurma.
- **AI notu**: yalnızca AI önerisinin reddedildiği, düzeltildiği veya doğrulandığı durumda. Case ekibi AI çıktısının nasıl doğrulandığını görmek istiyor; bu satır onu görünür kılar.

Durum değerleri: `Önerildi`, `Kabul edildi`, `Yerini aldı → D-0XX`, `Geri alındı`.

## Yazım kuralları

1. **Geçmişi silme.** Karar değişirse eski kaydı düzenleme; durumunu `Yerini aldı → D-0XX` yap ve yeni kayıt aç. Yeni kaydın bağlamında eskisinin neden yetmediğini yaz.
2. **Koddan doğrula.** Kayıttaki her somut iddia (parametre değeri, tablo adı, test adı) kodda gerçekten öyle olmalı. Yazmadan önce ilgili dosyayı oku veya `grep` ile kontrol et. Kod ile DECISIONS.md çelişirse önce kullanıcıya söyle.
3. **Henüz yapılmamışı yapılmış gibi yazma.** Planlanan ama kodlanmamış kararlar `Önerildi` durumunda kalır.
4. **Kararı kullanıcı verir.** Kullanıcının açıkça seçmediği bir şeyi `Kabul edildi` diye işaretleme. Henüz sorulmamış bir karar için CLAUDE.md "Karar kuralları" bölümündeki gibi `AskUserQuestion` ile sor (önerin ilk seçenek, `(Önerilen)` etiketiyle); cevap gelene kadar kayıt `Önerildi` kalır.
5. **Kısa tut.** Bir kayıt ekranı geçmesin. Uzun gerekçe gerekiyorsa README'deki mimari bölümüne veya koda yorum olarak bağla.
6. **İndeksi güncelle.** Her yeni/değişen kayıtta en üstteki tabloyu da güncelle.
7. **Dil:** Türkçe yaz; kod, tablo, kütüphane adları orijinal haliyle kalsın.
8. **Gizli bilgi yok.** API key, gerçek kişi verisi, `.env` içeriği asla girmez.

## Akış

1. DECISIONS.md'yi oku; son D-numarasını ve mevcut alanları bul. Dosya boşsa aşağıdaki iskeletle başlat.
2. Yakalanacak kararları belirle: mevcut görevin diff'i (`git diff`, yoksa bu oturumda değişen dosyalar) + konuşmada verilen seçimler.
3. Her aday için: zaten kayıtlı mı? Kayıtlıysa hâlâ doğru mu? Değilse güncelleme değil, yerine geçen kayıt aç.
4. Kodda doğrula (Kural 2), sonra kaydı yaz ve indeksi güncelle.
5. `Önerildi` kalan kayıtları tek bir `AskUserQuestion` çağrısında (en fazla 4 soru) kullanıcıya sor; onaylananları `Kabul edildi` yap, reddedilenleri seçilen alternatifle yeniden yaz.
6. Kullanıcıya hangi kayıtları eklediğini/değiştirdiğini tek satırlık listeyle bildir.

## Tarama modu ("tüm kararları dök")

1. Repo yapısını, `docker-compose.yml`, `go.mod`/`composer.json`, `package.json`, migration'lar, kuyruk/worker kodu, LLM istemcisi, prompt dosyaları, mock provider ve testleri tara.
2. Yukarıdaki **Neyin karar sayıldığı** listesinin her maddesini koddaki karşılığıyla eşle.
3. Koddan çıkarılabilen ama gerekçesi bilinmeyen kararlar için gerekçeyi uydurma: kaydı yaz, **Bağlam/Alternatifler**'i "Kullanıcıya sorulacak" olarak bırak ve sonunda bu soruları tek listede kullanıcıya sor.
4. Listedeki maddelerden koda yansımamış olanları DECISIONS.md'nin sonunda **Açık konular** başlığı altında topla.

## Boş dosya iskeleti

```markdown
# Kararlar

Bu dosya Smart Campaign Composer'daki önemli teknik ve ürün kararlarını, gerekçeleri ve bedelleriyle birlikte kaydeder. Kararlar silinmez; değişen karar yeni bir kayıtla yerini alır.

| No | Karar | Alan | Durum |
|----|-------|------|-------|

---

## Açık konular

- …
```
