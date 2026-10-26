# Kararlar

Bu dosya Smart Campaign Composer'daki önemli teknik ve ürün kararlarını, gerekçeleri ve bedelleriyle birlikte kaydeder. Kararlar silinmez; değişen karar yeni bir kayıtla yerini alır.

| No | Karar | Alan | Durum |
|----|-------|------|-------|
| D-001 | Ham stub'ın deterministik üreticiyle segmentasyon verisine zenginleştirilmesi | Mock provider ve seed | Önerildi |

---

## D-001: Ham stub'ın deterministik üreticiyle segmentasyon verisine zenginleştirilmesi

- **Durum:** Önerildi · 2026-10-26
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
---

## Açık konular

- Backend dili (Go / PHP) ve veritabanı
- Kuyruk teknolojisi ve kanal başına 100 msg/s rate limit yöntemi
- Idempotency anahtarı ve retry stratejisi
- LLM yapılandırılmış çıktı yöntemi ve segment kuralı DSL'i
- Mock provider hata/gecikme/429 oranları
- Seed: zenginleştirilmiş `stub/*.csv` setinden hangi tablo ve alanların DB'ye alınacağı (kart, CVV, IBAN alınmayacak); 1000 kullanıcının "birkaç bin"e nasıl çıkarılacağı
- Canlı durum taşıma yöntemi (SSE / WebSocket / polling)
