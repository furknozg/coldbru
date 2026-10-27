---
name: task-csv
description: Smart Campaign Composer'ın tasks.csv görev listesini okur, anlamlandırır ve görev üzerinde çalışır. Kullanıcı "görevler", "tasks.csv", "sıradaki görev", "T-0XX üzerinde çalış", "görev listesi", "hangi görevler kaldı", "görevi bitir" dediğinde veya bir görev ID'si (T-001 gibi) verdiğinde kullan.
---

# tasks.csv ile çalışma

`tasks.csv` (repo kökü) projenin iş listesidir. Sütunlar: `task-id,title,description,related-docs,status`.

- `related-docs` alanı `;` ile ayrılmış dosya yollarıdır (repo köküne göre).
- `status` değerleri: `todo`, `inprogress`, `done`. Başka bir değer kullanma.
- Dosya şu an git'te izlenmiyor (`??`). Düzenlemeleri yine de dosyaya doğrudan yap, commit'i kullanıcı ister.

Dosyayı ham metin olarak `cat` etme. Açıklamalar Türkçe ve uzun, CSV tırnakları ve `;` ayraçları karışık. Her zaman yardımcı scripti kullan:

```bash
python3 .claude/skills/task-csv/scripts/tasks.py summary            # sayılar ve durum listesi
python3 .claude/skills/task-csv/scripts/tasks.py next               # ilk todo görev
python3 .claude/skills/task-csv/scripts/tasks.py show T-006         # görevin alanları + doküman var mı
python3 .claude/skills/task-csv/scripts/tasks.py validate           # şema, tekrar eden ID, geçersiz status, eksik doküman
```

## 1. Okuma ve anlamlandırma

Kullanıcı genel bir soru sorarsa (ne kaldı, neyi yapmalıyız):

1. `summary` ile durumu çıkar.
2. Görevleri ID sırasıyla gruplayıp kısa bir harita ver: tamamlananlar, sıradakiler, hangi fazın (altyapı, servis, frontend, test, dokümantasyon) ne kadarının kaldığı.
3. Açıklamalardaki bağımlılıkları oku. Örnek: "T-009 NATS" olmadan "T-012 outbox" çalışmaz. Bağımlılık açıkça yazılmıyorsa ID sırasını ve katman mantığını (altyapı → platform → servis → frontend → test) varsayım olarak söyle.
4. Kullanıcıya somut öneri ver: hangi görev şimdi, neden.

## 2. Bir görev üzerinde çalışma

Görev ID'si verilmediyse `next` ile sıradaki `todo` görevi seç ve kullanıcıya bunu söyle (onay isteme; makul varsayılan).

Sırayla:

1. **Görevi oku:** `show <ID>` çıktısındaki her doküman için dosyayı oku (`(YOK)` işaretlileri bildir). Ardından `DECISIONS.md`'yi oku; görevle ilgili `Kabul edildi` kayıtlar kuraldır.
2. **Plan:** Görevin `status` alanını `inprogress` yap. Adımları `update_plan` ile göster. Görev kodlama içeriyorsa kök `CLAUDE.md`'deki kodlama kurallarını uygula (branch, codegraph, subagent ayarları).
3. **Karar kapısı:** Görev, kök `CLAUDE.md` Kural 1 kapsamındaki bir seçimi gerektiriyorsa (şema, kütüphane, API sözleşmesi, retry/rate limit/idempotency yorumu, mock provider oranları, seed içeriği, kapsam dışı bırakma) kodu yazmadan önce `AskUserQuestion` ile sor. Birbirinden bağımsız soruları tek çağrıda topla. Kararı bağımsız kısımlar sürebilir.
4. **Uygula:** Kodu yaz. Testleri `make test` ile koş. Test koşmuyorsa nedenini söyle; "çalıştı" deme.
5. **Kapanış kontrolü** (kök `CLAUDE.md` bölüm 4): onaylanan her karar `DECISIONS.md`'de mi, indeks tablosu güncel mi, iş `Kabul edildi` bir kayıtla çelişiyor mu. Bunlardan biri başarısızsa görev bitmiş sayılmaz.
6. **Durumu güncelle:** Yalnızca iş doğrulandıysa satırın `status` alanını `done` yap. Yarım işte `inprogress` bırak ve kalanı görev bazında söyle. Kullanıcı işi bırakırsa `todo`'ya geri al.

### CSV'yi düzenlerken

- Yalnızca hedef satırı değiştir. Başlığı, sıralamayı ve diğer satırları koruma dışında bırakma.
- Alanı `Edit` ile değiştir; tüm dosyayı yeniden yazma. Açıklamada `"` veya `,` varsa satır zaten tırnaklıdır, aynı biçimi koru.
- Değişiklikten sonra `validate` çalıştır. Çıkış kodu 0 değilse düzelt.

## 3. Yeni görev eklemek

Kullanıcı yeni görev isterse ID'yi mevcut en büyük numaradan devam ettir (`T-042`...). Alanları Türkçe yaz, `related-docs` için gerçek dosya yolları ver, `status` olarak `todo` gir, sonra `validate` çalıştır. Kullanıcı açıkça istemedikçe görev silme ya da yeniden numaralandırma yapma.

## 4. Dikkat

- Görevin `related-docs` alanındaki dosyalar `(YOK)` çıkarsa bunu bir risk olarak raporla; sessizce yeni dosya uydurma.
- Kullanıcı bir görevin "bitti" olduğunu söylerse yine de `validate` ve ilgili testleri çalıştır, sonra `done` yap.
- Birden çok görev aynı dosyalara dokunuyorsa (ör. `docker-compose.yml`) bunları sırayla yap, paralel subagent'lara bölme.
