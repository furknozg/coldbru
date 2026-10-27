#!/usr/bin/env python3
"""Okur, doğrular ve özetler: tasks.csv için yardımcı araç (yalnız standart kütüphane).

Kullanım:
  python3 tasks.py summary              durum sayıları ve faz bazlı özet
  python3 tasks.py next                 sıradaki yapılacak görev (ilk todo, bağımlılık yok varsayımıyla)
  python3 tasks.py show T-005           tek görevin tüm alanları ve var olup olmadığı kontrol edilen dokümanlar
  python3 tasks.py validate             şema, benzersiz ID, geçerli durum ve doküman yolu denetimi
"""

import csv
import os
import sys

HEADER = ["task-id", "title", "description", "related-docs", "status"]
STATUSES = {"todo", "inprogress", "done"}
REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "..", ".."))
CSV_PATH = os.path.join(REPO, "tasks.csv")


def load():
    with open(CSV_PATH, encoding="utf-8", newline="") as f:
        reader = csv.reader(f)
        header = next(reader, [])
        rows = [dict(zip(header, r)) for r in reader if r]
    return header, rows


def docs_of(row):
    return [d.strip() for d in row.get("related-docs", "").split(";") if d.strip()]


def summary(rows):
    counts = {}
    for r in rows:
        counts[r["status"]] = counts.get(r["status"], 0) + 1
    print(f"Toplam: {len(rows)}  " + "  ".join(f"{k}={v}" for k, v in sorted(counts.items())))
    for r in rows:
        mark = "x" if r["status"] == "done" else " "
        print(f"[{mark}] {r['task-id']:<6} {r['title']}")


def nxt(rows):
    for r in rows:
        if r["status"] == "todo":
            print(f"{r['task-id']}  {r['title']}")
            print(f"  {r['description']}")
            return
    print("Yapılacak görev yok.")


def show(rows, task_id):
    match = [r for r in rows if r["task-id"] == task_id]
    if not match:
        print(f"{task_id} bulunamadı.", file=sys.stderr)
        return 1
    r = match[0]
    print(f"{r['task-id']} | {r['title']} | {r['status']}")
    print(f"Açıklama: {r['description']}")
    for d in docs_of(r):
        exists = os.path.exists(os.path.join(REPO, d))
        print(f"  - {d}{'' if exists else '  (YOK)'}")
    return 0


def validate(header, rows):
    errors = []
    if header != HEADER:
        errors.append(f"Başlık beklenenden farklı: {header}")
    seen = set()
    for i, r in enumerate(rows, start=2):
        tid = r.get("task-id", "")
        if not tid:
            errors.append(f"satır {i}: task-id boş")
        if tid in seen:
            errors.append(f"satır {i}: {tid} tekrar ediyor")
        seen.add(tid)
        if not r.get("title"):
            errors.append(f"satır {i} ({tid}): title boş")
        if r.get("status") not in STATUSES:
            errors.append(f"satır {i} ({tid}): geçersiz status '{r.get('status')}'")
        for d in docs_of(r):
            if not os.path.exists(os.path.join(REPO, d)):
                errors.append(f"satır {i} ({tid}): doküman yok -> {d}")
        if None in r or len(r) != len(HEADER):
            errors.append(f"satır {i} ({tid}): alan sayısı {len(r)} != {len(HEADER)}")
    if errors:
        print("\n".join(errors))
        return 1
    print(f"Tamam: {len(rows)} görev doğrulandı.")
    return 0


def main(argv):
    if len(argv) < 2:
        print(__doc__)
        return 2
    header, rows = load()
    cmd = argv[1]
    if cmd == "summary":
        summary(rows)
        return 0
    if cmd == "next":
        nxt(rows)
        return 0
    if cmd == "show" and len(argv) == 3:
        return show(rows, argv[2])
    if cmd == "validate":
        return validate(header, rows)
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
