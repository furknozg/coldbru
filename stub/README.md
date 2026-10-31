# stub/ — ham veri ve zenginleştirilmiş stub seti

`source/acw_user_data.csv` elimize verilen ham export (1000 satır, Faker/en_GB). **Hiç değiştirilmez.**
Diğer tüm CSV'ler `generate_stub_data.py` tarafından bu dosyadan deterministik olarak üretilir;
kullanıcı sayısı `TOTAL_USERS = 3000`'dir (D-113): ilk 1000 kullanıcı ham satırlardan, sonraki 2000 kullanıcı
ham satırların yeniden örneklenmesinden gelir (aşağıya bakın):

```bash
python3 stub/generate_stub_data.py
```

Aynı girdi + `SEED=20261026` → bayt bayt aynı çıktı. Sadece standart kütüphane. "Şimdi" sabiti `AS_OF = 2026-10-26 12:00 UTC`; tüm zaman damgaları UTC, `YYYY-MM-DDTHH:MM:SSZ`.

## Dosyalar ve anahtarlar

| Dosya | Satır | Birincil anahtar | Yabancı anahtarlar | Ne anlatır |
|---|---|---|---|---|
| `acw_user_data.csv` | 3000 | `Customer ID` | — | Orijinal 23 kolon aynen + 32 segmentasyon kolonu |
| `acw_products.csv` | ~160 | `Product ID` | — | 9 kategori, GBP fiyat, stok, puan |
| `acw_orders.csv` | ~11.9k | `Order ID` | `Customer ID` | Son 6 yılın siparişleri; durum, indirim kodu, ödeme yöntemi, platform |
| `acw_order_items.csv` | ~27.9k | (`Order ID`,`Product ID`) | `Order ID`, `Product ID` | Sipariş kalemleri; toplamlar `acw_orders` ile birebir tutar |
| `acw_events.csv` | ~138k | `Event ID` | `Customer ID`, `Session ID`, `Product ID`, `Order ID` | Son 90 günün davranış olayları (bkz. olay tipleri) |
| `acw_cart_items.csv` | ~11.9k | `Cart Item ID` | `Customer ID`, `Product ID`, `Order ID` | Sepete eklenen her kalem: `open` / `abandoned` / `converted` / `removed` |
| `acw_consents.csv` | ~6.7k | `Consent ID` | `Customer ID` | Kanal izni geçmişi (email/sms); son satır = güncel durum |
| `acw_campaigns.csv` | 12 | `Campaign ID` | — | Geçmiş kampanyalar: brief, segment kuralı, kanal, gönderim özeti |
| `acw_campaign_messages.csv` | ~9.2k | `Message ID` | `Campaign ID`, `Customer ID` | Kişi başı mesaj: teslim durumu, açılma/tıklama/abonelik iptali |

## `acw_user_data.csv` — yeniden örneklenen kullanıcılar (D-113)

`CUS-001001` … `CUS-003000` için üretici rastgele bir ham satır seçer ve ad dışı profil alanlarını
(adres, yaş, işe uzaklık, işveren, bakmakla yükümlü, medeni durum, emeklilik, maaş, cinsiyet, araç) olduğu gibi
kopyalar; `First Name` ve `Last Name` ham dosyadaki ad ve soyadlar arasından ayrı ayrı çekilir. Bu satırlarda
`Credit Card *` ve `Bank IBAN` kolonları boştur; kart verisi hiçbir yeni kullanıcıya kopyalanmaz.
Ülke, şehir, dil, sipariş, sepet, izin ve olaylar bütün kullanıcılar için aynı kodla ve aynı dağılımlarla üretilir.
`Email` ve `Phone (E.164)` 3000 kullanıcı içinde benzersizdir.

## `acw_user_data.csv` — eklenen kolonlar

| Kolon | Açıklama |
|---|---|
| `Customer ID` | `CUS-000001` … (ilk kolon) |
| `Email`, `Email Bounced` | Ad.soyad@domain, benzersiz; %3 bounce eden adres |
| `Phone (E.164)` | `+905…` / `+447…`, benzersiz; %4 boş (SMS gönderilemez) |
| `Country Code`, `City`, `Language`, `Timezone` | TR %60 / GB %40; şehirler nüfus ağırlıklı (İstanbul TR'nin %35'i); dil tr/en |
| `Date of Birth`, `Signup Date` | DOB `Age (Years)` ile tutarlı; kayıt son 6 yıl, yeni kayıt ağırlıklı |
| `Account Status`, `Deleted At` | active %95, inactive %3, deleted %2 (silinen hesaba gönderim yapılmaz) |
| `Email Opt In`, `SMS Opt In`, `Consent Updated At` | `acw_consents.csv`'deki son kayıtla aynı |
| `Preferred Platform` | ios / android / web |
| `Total Orders`, `Total Spend (GBP)`, `Average Order Value (GBP)`, `First/Last Order Date` | Sadece delivered+shipped siparişler sayılır |
| `Favourite Category` | Harcamaya göre en yüksek kategori |
| `Last Cart Add Date`, `Has Open Cart`, `Open Cart Value (GBP)` | Açık sepet = son 30 günde eklenmiş, satın alınmamış kalem |
| `Last Active Date`, `Sessions (90d)` | Olay, sipariş ve kayıt tarihlerinin en yenisi |
| `Loyalty Tier`, `Loyalty Points` | Bronze <300, Silver <1200, Gold <3000, Platinum ≥3000 GBP |
| `Campaigns Received (90d)`, `Emails Opened (90d)`, `Last Campaign Sent At` | `acw_campaign_messages.csv`'den türetilir |

Orijinal kolonlardan `Credit Card *`, `Bank IBAN`, araç ve maaş alanları ilk 1000 satırda olduğu gibi durur (yeniden örneklenen satırlarda kart ve IBAN boştur); kart/CVV/IBAN seed'e alınmaz (D-004).

## Olay tipleri (`acw_events.csv`)

`app_open`, `page_view`, `search` (Category dolu), `product_view`, `wishlist_add`, `add_to_cart` (Value = satır tutarı), `remove_from_cart`, `checkout_started`, `purchase` (Order ID dolu). Her `purchase` olayının bir siparişi, her `converted` sepet kaleminin bir siparişi vardır.

## Tutarlılık kuralları (üretici garanti eder)

- Tüm yabancı anahtarlar çözülür; tüm ID'ler benzersiz ve zaman sıralı.
- Kampanya mesajı yalnızca gönderim anında o kanala izni olan, silinmemiş kullanıcıya gider; telefonu olmayana SMS gitmez.
- Abonelik iptali (`Unsubscribed At`) `acw_consents.csv`'ye `unsubscribe_link` kaynaklı satır olarak düşer.
- Silinen kullanıcının silinme sonrası olayı/mesajı yoktur.

## Örnek brief için beklenen huni

"Son 30 günde sepetine ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılar" — üretici her çalışmada bu sayıyı basar:

```
İstanbul active users 598 -> carted 263 -> no purchase 155 -> sms-sendable 103 / email-sendable 120
```

(1000 kullanıcılı eski seed: `195 -> 90 -> 49 -> 33 / 33`.)

## Bilinen tuhaflıklar

- Ham veri İngiliz (isimler, posta kodları, GBP); TR şehirleri bunun üzerine eklendi. Sahte veri olduğu için kabul edildi.
- Ham `Address City` 980 benzersiz değer içerir ve segmentasyon için kullanılamaz; `City` kolonu bu yüzden eklendi.
- Kart numarası/CVV uzunlukları ve bazı bitiş tarihleri hatalıdır (ham export'tan gelir).
- Sepet dönüşümü kalem bazında ~%53; sektör ortalamasının (~%30) üstünde. `generate_stub_data.py` içindeki `buy_now`/`buy_old` olasılıklarıyla ayarlanır.
