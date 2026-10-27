Smart Campaign Composer

Pazarlamacının doğal dille yazdığı bir kampanya isteğini Claude ile hedef kitleye ve SMS ve email mesajlarına çeviren; onaya sunan ve gönderip durumunu canlı gösteren uçtan uca bir sistem kuracaksın.
Frontend Vue.js Backend Go veya PHP LLM Claude API Tech Stacki kullanmakla kısıtlsın

    İş bağlamı
    Kullanıcı akışı
    Fonksiyonel gereksinimler
    AI gereksinimleri
    Kaynaklar
    Teknoloji
    Teslim edilecekler
    Bonus
    AI araçları kullanımı
    Değerlendirme
    Sonraki adım

İş bağlamı

Müşterilerimiz için her gün milyonlarca kişiye SMS ve email gönderiyor. Bir kampanya hazırlamak için kitleyi tanımlamak, her kanal için metin yazmak ve karakter limitlerine uymak gerekiyor. Bu case'te pazarlamacının yalnızca ne istediğini yazdığı, gerisini Claude'un önerdiği ve son kararı insanın verdiği bir araç kurmakla görevliyiz.
Kullanıcı akışı
Örnek brief

“Son 30 günde sepetine ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılara %30 indirim hatırlatması gönder. Samimi ama kısa olsun.”

    Kitle çıkarılır
    Claude brief'ten bir segment kuralı üretir, arayüz kaç kişiye gideceğini gösterir.
    Mesajlar üretilir
    SMS ve email içeriği ekrana stream edilerek akar.
    İnsan onaylar
    Pazarlamacı önizler, gerekirse düzenler ve onaylar.
    Gönderilir
    Mesajlar kuyruk üzerinden mock provider'a gönderilir.
    Canlı izlenir
    Dashboard gönderim durumunu gösterir.

Gerçek kimseye mesaj gitmez. Alıcılar senin oluşturacağın sahte kullanıcılardır, gönderim de senin yazacağın mock provider'a yapılır.
Fonksiyonel gereksinimler
Kitle

    Claude'un ürettiği segment kuralı backend'de doğrulanır; LLM çıktısı doğrudan SQL olarak çalıştırılmaz.
    İzni olmayan kullanıcıya o kanaldan mesaj gönderilmez.

İçerik

    İçerik kanal limitlerine uymalı: SMS en fazla 160 karakter, email konusu en fazla 80 karakter.
    {{first_name}} gibi kişiselleştirme değişkenleri desteklenir.

Gönderim

    Asenkron kuyruk, kanal başına saniyede en fazla 100 mesaj.
    Aynı kişiye aynı mesaj iki kez gitmez. Retry stratejisini sen tasarla.

Takip

    Canlı gönderim durumu, metrics ve health check endpoint'leri.

AI gereksinimleri

Case'in en önemli kısmı. Claude'u çağırmak yeterli değil; çıktısını güvenilir hale getirmeni bekliyoruz.

    Doğrulama: Claude'un çıktısı yapılandırılmış (JSON) olmalı ve backend'de kontrol edilmeli.
    Test edilebilirlik: testler gerçek API key olmadan, mock bir LLM ile çalışmalı.

Kaynaklar

Claude API key: Uygulama key'i .env dosyasındaki ANTHROPIC_API_KEY değişkeninden okumalı; repoya bir .env.example ekle. Geliştirme sırasında mock bir LLM ile çalışabilir, gerçek modelle denemek istersen Claude Console'dan kendi API key'ini alabilirsin.

Aşağıdakileri de kendin hazırlayacaksın:

    Sahte kullanıcı verisi: birkaç bin kullanıcılık bir seed. Hangi alanlara ihtiyaç olduğuna (ad, email, telefon, şehir, alışveriş geçmişi, kanal izinleri gibi) sen karar ver.
    Mock SMS/email provider: gerçek bir servisi taklit eden basit bir servis. Retry ve rate limit mantığını gerçekten test edebilmek için zaman zaman hata, gecikme ve 429 döndürmeli.

Teknoloji

Frontend Vue.js, backend Go veya PHP. Veritabanı, kuyruk ve kütüphaneler senin seçimin.
Teslim edilecekler

    GitHub veya GitLab repository
    docker compose up ile tek komutla kurulum (seed verisi ve mock provider dahil)
    make test ile çalışan testler
    README.md: kurulum ve kısa mimari özeti
    DECISIONS.md: önemli kararların ve nedenleri
    ai-sessions/: Claude session export'ların (aşağıya bakın)

Kapsam notu: Her şeyi yapmak yerine çekirdek akışı sağlam yapmanı tercih ederiz. Yetiştiremediklerini README'de belirtmen yeterli.
Bonus (isteğe bağlı)

    Zamanlanmış gönderim
    A/B varyantları
    Kullanıcının diline göre içerik
    CI pipeline

AI araçları kullanımı

Claude Code, Cursor gibi araçları kullanman serbest, hatta bekleniyor. Önemli olan üretilen kodu anlaman ve sahiplenmen.

Case boyunca Claude ile yaptığın tüm çalışma session'larının export'larını repodaki ai-sessions/ klasörüne ekle (Claude Code'da /export, claude.ai'da paylaşım linki). Göndermeden önce API key ve kişisel bilgileri temizle.
Neden istiyoruz: Kayıtları ne kadar AI kullandığına göre puanlamıyoruz. Problemi nasıl ele aldığını ve AI çıktısını nasıl doğruladığını görmek istiyoruz.
Değerlendirme
LLM entegrasyonu%25
Mimari ve kararlar%20
Backend güvenilirliği%20
Frontend%20
Test ve kod kalitesi%15
Sonraki adım

Projen incelenip olumlu bulunduğu takdirde seninle 30 dakikalık bir review görüşmesi yapacağız. Görüşmede çözümünü ve verdiğin kararları birlikte konuşacağız. Soruların olursa veya ek süreye ihtiyacın olursa bize yazman yeterli.
Insider One · AI Developer Case Study · 2026

---

# Karar kuralları (ZORUNLU)

Bu bölüm bu repoda çalışan her Claude oturumu için bağlayıcıdır. Yukarıdaki case metni neyin yapılacağını, bu bölüm kararların nasıl alınacağını belirler.

## 1. Önemli kararları kullanıcı verir

Aşağıdakilerden biri gündeme geldiğinde **koda geçmeden önce dur ve kullanıcıya `AskUserQuestion` ile sor**:

- Dil, framework, veritabanı, kuyruk, kütüphane veya harici servis seçimi
- Şema tasarımı: DB tabloları, segment kuralı DSL'i, LLM çıktısının JSON şeması, API sözleşmesi
- Brief'teki bir gereksinimin yorumu (idempotency anahtarı, retry politikası, rate limit yöntemi, izin kontrolünün yeri, karakter limitinin ölçülme şekli)
- LLM entegrasyonu: model, yapılandırılmış çıktı yöntemi, doğrulama/yeniden deneme davranışı, prompt sorumlulukları
- Mock provider davranışı (hata/gecikme/429 oranları), seed içeriği
- Bir şeyi kapsam dışı bırakmak, basitleştirmek veya bonus özelliğe girmek
- DECISIONS.md'de `Kabul edildi` durumundaki bir karardan sapmak

Soru kuralları:
- 2–4 seçenek sun; önerin ilk seçenek olsun ve etiketinde `(Önerilen)` yazsın.
- Her seçeneğin `description`'ında artısını ve bedelini tek cümlede yaz.
- Birbirine bağlı olmayan kararları tek `AskUserQuestion` çağrısında topla (en fazla 4 soru); kullanıcıyı ardı ardına prompt'larla bölme.
- Cevap gelene kadar o karara bağlı kodu yazma. Karardan bağımsız işler sürebilir.
- Değişken adı, dosya yeri, küçük refactor gibi geri dönüşü ucuz ve sonucu olmayan seçimler için sorma; makul olanı yap.

Emin değilsen sor. Yanlışlıkla sorulmuş bir soru bir tıklama, sorulmadan alınmış yanlış bir karar review görüşmesinde savunulamayan bir kod demektir.

## 2. Her kararı DECISIONS.md'ye işle

- Kullanıcı bir kararı onayladığında, aynı turda `documenting-decisions` skill'ini kullanarak DECISIONS.md'ye `Kabul edildi` durumunda kayıt aç.
- Kullanıcının onaylamadığı hiçbir karar `Kabul edildi` olarak yazılmaz; senin önerin `Önerildi` olarak kalır.
- Kayıttaki her somut iddia (parametre, tablo, test adı) kodla eşleşmeli. Kod ile DECISIONS.md çelişirse düzeltmeden önce kullanıcıya söyle.

## 3. Mevcut kararlara uy

- Herhangi bir göreve başlamadan önce DECISIONS.md'yi oku. `Kabul edildi` kayıtlar bu repo için kuraldır.
- Bir kararın yanlış veya yetersiz olduğunu düşünüyorsan, sessizce farklı uygulama: gerekçeyle birlikte kullanıcıya sor (Kural 1). Onay gelirse eski kaydı `Yerini aldı → D-0XX` yap ve yeni kayıt aç.

## 4. Görev kapanış kontrolü

Kod değiştiren her görevi bitirmeden önce:
1. Bu görevde Kural 1 kapsamına giren ama sorulmadan verilmiş bir seçim var mı? Varsa şimdi sor.
2. Onaylanan her karar DECISIONS.md'de var mı ve indeks tablosu güncel mi?
3. Yapılan iş `Kabul edildi` bir kayıtla çelişiyor mu?

Bu üç kontrolden biri başarısızsa görev bitmiş sayılmaz.


# Kodlama kuralları (ZORUNLU):

1. Branch naming conventionunda suffixler belirtilen şekildedir: feature,doc,refactor,fix (suffix/geliştirme-adı)
2. Codegraph skilli (/codegraph veya .claude/skills/codegraph) kurulum fazı kullanımı kuvvetle önerilir
3. Kod inceleme, endeksli arama yardımcısı codegraph kullanımı ile gerçekleştirilmesi kuvvetle önerilir (desteklenmeyen diller durumunda normal yollara atlanabilir)
4. Commit mesajları bütün featureları açıklamak için gerekmemekte, merge ve pull requestler bu ihtiyacı karşılamaktadır
5. Olmasa bile branch kontrolü varmış gibi davranmak, bir değişiklik commit edilecekse checkout -b çalıştırıp commitlemek zorunludur. Merge için kullanıcı onayı istenebilir.
6. Customer veritabanı haricinde, herhangi bir migration uygulanacaksa, mevcut migration managerlar ile uygulanmalı.
7. Migration managerı ile uygulanan değişiklikler karar kurallarına tabiidir. endeks,şema değişikliği,stored procedure,listen/notify, RLS uygulanması kararları vb. kullanıcı onayı verilmelidir.
8. Kodlama görevlerinde /caveman skilli ile subagent kullanılır (skill agent cevaplarını küçültür, yazımları değil), subagent modeli birden fazla dosyayı ilgilendiriyor ise sonnet-5 kullanılır aşmıyorsa haiku, karar çıkarsa alt ajan durur, soruyu ana oturuma döndürür.

# Task Takibi ve öngörü (Zorunlu):
1. Tasklar /task skilli tetiklenerek incelenebilir, bu skill tetiklenmeden tasks.csv ile işlem yapılmamalı
2. Task kullanıcı onayı alınmadan csv de done olarak işaretlenmez
