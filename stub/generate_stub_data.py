#!/usr/bin/env python3
"""Enrich the raw ACW user export into a segmentation-ready stub dataset.

Input  (never modified):  stub/source/acw_user_data.csv   - original 1000-row Faker export
Output (regenerated):     stub/acw_user_data.csv          - original 23 columns + enrichment columns;
                                                            1000 raw rows + 2000 resampled rows (D-113)
                          stub/acw_products.csv
                          stub/acw_orders.csv
                          stub/acw_order_items.csv
                          stub/acw_events.csv
                          stub/acw_cart_items.csv
                          stub/acw_consents.csv
                          stub/acw_campaigns.csv
                          stub/acw_campaign_messages.csv

Deterministic: same input + SEED => byte-identical output. Standard library only.
Run:  python3 stub/generate_stub_data.py
"""
import csv
import os
import random
import sys
from collections import Counter, defaultdict
from datetime import datetime, timedelta

SEED = 20261026
AS_OF = datetime(2026, 10, 26, 12, 0, 0)          # "now" for the whole dataset (UTC)
EVENT_WINDOW_DAYS = 90                             # behavioural events kept for this long
OPEN_CART_DAYS = 30                                # cart item older than this => abandoned
TOTAL_USERS = 3000                                 # raw rows first, then resampled rows (D-113)
# Raw columns never copied into a resampled row: card data and IBAN stay out of every output.
SENSITIVE_COLUMNS = ("Credit Card Start Date", "Credit Card Expiry Date", "Credit Card Number",
                     "Credit Card CVV", "Bank IBAN")
HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "source", "acw_user_data.csv")

# --------------------------------------------------------------------------- reference data
TR_CITIES = [("İstanbul", 35), ("Ankara", 14), ("İzmir", 10), ("Bursa", 6), ("Antalya", 6),
             ("Adana", 4), ("Konya", 3), ("Gaziantep", 3), ("Kocaeli", 3), ("Mersin", 3),
             ("Kayseri", 2), ("Eskişehir", 2), ("Samsun", 2), ("Trabzon", 2), ("Denizli", 2),
             ("Diyarbakır", 1), ("Sakarya", 1), ("Muğla", 1)]
GB_CITIES = [("London", 30), ("Manchester", 10), ("Birmingham", 10), ("Leeds", 7), ("Glasgow", 6),
             ("Liverpool", 6), ("Bristol", 5), ("Edinburgh", 5), ("Sheffield", 4), ("Newcastle", 4),
             ("Cardiff", 3), ("Nottingham", 3), ("Leicester", 3), ("Belfast", 2), ("Southampton", 2)]
COUNTRY_WEIGHTS = [("TR", 60), ("GB", 40)]
EMAIL_DOMAINS = {"TR": ["gmail.com", "hotmail.com", "yandex.com", "outlook.com.tr", "icloud.com"],
                 "GB": ["gmail.com", "outlook.com", "yahoo.co.uk", "btinternet.com", "icloud.com"]}
TIMEZONE = {"TR": "Europe/Istanbul", "GB": "Europe/London"}
PLATFORMS = {"TR": [("android", 50), ("ios", 30), ("web", 20)],
             "GB": [("ios", 45), ("android", 30), ("web", 25)]}

CATALOGUE = {
    "Electronics": (["Wireless Earbuds", "Bluetooth Speaker", "Smartwatch", "4K Monitor", "Mechanical Keyboard",
                     "Wireless Mouse", "Power Bank", "Noise-Cancelling Headphones", "Smart Plug", "Action Camera",
                     "10-inch Tablet", "USB-C Hub"], (15, 450), ["Nova", "Pulse", "Orbit", "Vertex"]),
    "Fashion": (["Slim Fit Jeans", "Cotton T-Shirt", "Hooded Sweatshirt", "Leather Jacket", "Running Trainers",
                 "Chelsea Boots", "Wool Coat", "Summer Dress", "Linen Shirt", "Denim Jacket", "Cargo Trousers",
                 "Knit Jumper"], (12, 220), ["Harbor", "Maple", "Luna", "Atlas"]),
    "Home & Living": (["Scented Candle Set", "Memory Foam Pillow", "Duvet Cover Set", "Air Fryer", "Espresso Machine",
                       "Robot Vacuum", "Floor Lamp", "Non-Stick Pan Set", "Bamboo Cutting Board", "Throw Blanket",
                       "Wall Clock", "Storage Baskets"], (8, 350), ["Hearth", "Nest", "Casa", "Loft"]),
    "Beauty": (["Vitamin C Serum", "Moisturising Cream", "Sunscreen SPF50", "Eau de Parfum", "Hair Dryer",
                "Electric Toothbrush", "Lip Balm Trio", "Shampoo & Conditioner", "Beard Trimmer", "Face Mask Pack",
                "Nail Polish Set"], (5, 160), ["Glow", "Aura", "Pure", "Velvet"]),
    "Sports & Outdoors": (["Yoga Mat", "Dumbbell Set", "Camping Tent", "Trail Backpack", "Cycling Helmet",
                           "Resistance Bands", "Running Watch", "Water Bottle 1L", "Football", "Hiking Boots",
                           "Fitness Tracker"], (8, 300), ["Peak", "Stride", "Summit", "Rapid"]),
    "Books & Media": (["Bestselling Novel", "Cookbook", "Children's Picture Book", "Self-Help Guide",
                       "History Biography", "Graphic Novel", "Puzzle Book", "Language Course", "Travel Guide",
                       "Science Book"], (5, 40), ["Page", "Inkwell", "Folio"]),
    "Toys & Kids": (["Building Blocks Set", "Plush Bear", "Remote Control Car", "Board Game", "1000pc Jigsaw Puzzle",
                     "Art Supplies Kit", "Baby Stroller", "Kids Scooter", "Dollhouse", "Science Kit"], (6, 280),
                    ["Playco", "Tiny", "Wonder"]),
    "Grocery": (["Turkish Coffee 500g", "Olive Oil 1L", "Green Tea 100 Bags", "Dark Chocolate Bar",
                 "Dried Apricots 1kg", "Organic Honey", "Pasta Selection", "Mixed Nuts 750g", "Baklava Box",
                 "Protein Bars 12pk"], (3, 35), ["Harvest", "Anadolu", "Pantry"]),
    "Automotive": (["Car Phone Mount", "Dash Cam", "Car Vacuum", "Seat Covers Set", "Tyre Inflator", "Car Wash Kit",
                    "Jump Starter", "Roof Box", "Floor Mats", "Air Freshener Pack"], (5, 320),
                   ["Drive", "Motion", "Torque"]),
}
DISCOUNT_CODES = [("WELCOME10", 10), ("SUMMER30", 30), ("CART15", 15), ("BAYRAM20", 20), ("VIP25", 25), ("FLASH40", 40)]
PAYMENT_METHODS = {"TR": [("card", 60), ("wallet", 20), ("cash_on_delivery", 12), ("bank_transfer", 8)],
                   "GB": [("card", 70), ("wallet", 25), ("bank_transfer", 5)]}

# Past campaigns: (name, brief, channels, days_before_as_of, template, reach_pct, segment summary)
CAMPAIGNS = [
    ("Spring Refresh", "Home & Living'de yeni sezon ürünlerini tüm aktif kullanıcılara duyur.", "email", 175,
     "seasonal_launch", 55, "status = active"),
    ("Welcome Series - May", "Son 30 günde kayıt olanlara %10 hoş geldin indirimi gönder.", "email,sms", 160,
     "welcome_discount", 12, "signup_date >= now-30d"),
    ("Cart Reminder - June", "Sepetinde ürün bırakanlara 24 saat içinde hatırlatma gönder.", "sms", 140,
     "cart_reminder", 18, "has_open_cart = true"),
    ("Summer Sale Kick-off", "Tüm kullanıcılara %30 yaz indirimi. Samimi ve kısa olsun.", "email,sms", 120,
     "summer_sale", 70, "status = active"),
    ("Summer Sale Last Call", "Yaz indiriminden yararlanmayan kullanıcılara son çağrı.", "email", 112,
     "summer_last_call", 50, "status = active AND orders_since(campaign) = 0"),
    ("Back to School", "Çocuklu kullanıcılara okul dönüşü ürünleri.", "email", 95,
     "back_to_school", 30, "dependants >= 1"),
    ("Istanbul Flash Deal", "İstanbul'daki kullanıcılara 48 saatlik flash indirim.", "sms", 80,
     "flash_deal_city", 22, "city = İstanbul"),
    ("Loyalty Gold Perks", "Gold ve Platinum üyelere özel erken erişim.", "email", 65,
     "loyalty_perks", 15, "loyalty_tier IN (Gold, Platinum)"),
    ("Cart Reminder - September", "Son 7 günde sepete ekleyip almayanlara %15 hatırlatma.", "email,sms", 45,
     "cart_reminder", 16, "last_cart_add >= now-7d AND no purchase"),
    ("Autumn Fashion Drop", "Moda kategorisini seven kullanıcılara yeni koleksiyon.", "email", 30,
     "fashion_drop", 35, "favourite_category = Fashion"),
    ("Win-back 60d", "60 gündür alışveriş yapmayan eski müşterilere geri dönüş teklifi.", "email,sms", 18,
     "win_back", 28, "last_order_date <= now-60d AND total_orders >= 1"),
    ("Republic Day Weekend", "29 Ekim haftasonu için tüm TR kullanıcılarına %20 indirim.", "sms", 3,
     "republic_day", 55, "country = TR"),
]
CAMPAIGN_STATUS = {"email": [("delivered", 93), ("bounced", 4), ("failed", 3)],
                   "sms": [("delivered", 95), ("undelivered", 3), ("failed", 2)]}
CONSENT_SOURCES_OUT = [("preference_centre", 60), ("support_request", 25), ("unsubscribe_link", 15)]


# --------------------------------------------------------------------------- helpers
def wchoice(rng, pairs):
    total = sum(w for _, w in pairs)
    r = rng.uniform(0, total)
    acc = 0
    for value, w in pairs:
        acc += w
        if r <= acc:
            return value
    return pairs[-1][0]


def poisson(rng, lam):
    if lam <= 0:
        return 0
    L, k, p = 2.718281828459045 ** (-lam), 0, 1.0
    while True:
        p *= rng.random()
        if p <= L:
            return k
        k += 1


def iso(dt):
    return dt.strftime("%Y-%m-%dT%H:%M:%SZ")


def day(dt):
    return dt.strftime("%Y-%m-%d")


def money(x):
    return "%.2f" % round(x + 1e-9, 2)


def rand_dt(rng, start, end):
    span = (end - start).total_seconds()
    return start + timedelta(seconds=rng.uniform(0, max(span, 1)))


def slug_name(s):
    table = str.maketrans("çğıöşüÇĞİÖŞÜ'", "cgiosucgiosu ")
    return "".join(ch for ch in s.translate(table).lower() if ch.isalnum())


def years_ago(dt, years):
    try:
        return dt.replace(year=dt.year - years)
    except ValueError:                       # 29 Feb
        return dt.replace(year=dt.year - years, day=28)


def write_csv(path, header, rows):
    with open(path, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(header)
        w.writerows(rows)
    print("  %-32s %6d rows" % (os.path.basename(path), len(rows)))


# --------------------------------------------------------------------------- main
def main():
    if not os.path.exists(SRC):
        sys.exit("missing %s (the untouched original export)" % SRC)
    rng = random.Random(SEED)
    with open(SRC, newline="", encoding="utf-8") as f:
        reader = csv.reader(f)
        src_header = next(reader)
        src_rows = list(reader)
    col = {name: i for i, name in enumerate(src_header)}

    # ---------------- products
    products = []                                         # dicts
    for category, (names, (lo, hi), brands) in CATALOGUE.items():
        for name in names:
            for brand in rng.sample(brands, k=rng.choice([1, 2, 2])):
                price = round(lo + (hi - lo) * rng.random() ** 2) - 0.01   # skew towards cheaper items
                products.append({
                    "id": "PRD-%04d" % (len(products) + 1), "name": "%s %s" % (brand, name), "brand": brand,
                    "category": category, "price": price,
                    "in_stock": rng.random() > 0.06, "rating": round(rng.uniform(3.2, 4.9), 1)})
    by_category = defaultdict(list)
    for p in products:
        by_category[p["category"]].append(p)
    categories = list(CATALOGUE.keys())

    # ---------------- users: raw rows first, then resampled rows up to TOTAL_USERS (D-113)
    # A resampled row copies one random raw row's profile (age, salary, vehicle, address, ...) and
    # takes its first and last name from two other random raw rows. Card data and IBAN are blanked.
    first_names = [r[col["First Name"]] for r in src_rows]
    last_names = [r[col["Last Name"]] for r in src_rows]
    user_rows_src = list(src_rows)
    while len(user_rows_src) < TOTAL_USERS:
        row = list(rng.choice(src_rows))
        row[col["First Name"]] = rng.choice(first_names)
        row[col["Last Name"]] = rng.choice(last_names)
        for name in SENSITIVE_COLUMNS:
            row[col[name]] = ""
        user_rows_src.append(row)

    # ---------------- users: base enrichment
    users = []
    seen_emails = set()
    seen_phones = set()
    for idx, row in enumerate(user_rows_src):
        uid = "CUS-%06d" % (idx + 1)
        first, last = row[col["First Name"]], row[col["Last Name"]]
        age = int(row[col["Age (Years)"]])
        country = wchoice(rng, COUNTRY_WEIGHTS)
        city = wchoice(rng, TR_CITIES if country == "TR" else GB_CITIES)
        language = wchoice(rng, [("tr", 90), ("en", 10)] if country == "TR" else [("en", 97), ("tr", 3)])

        dob = years_ago(AS_OF - timedelta(days=rng.randint(0, 364)), age)
        earliest = max(years_ago(dob, -16), AS_OF - timedelta(days=6 * 365))
        signup = rand_dt(rng, earliest, AS_OF - timedelta(days=1))
        if rng.random() < 0.35:                            # recency skew: more recent signups
            signup = rand_dt(rng, AS_OF - timedelta(days=365), AS_OF - timedelta(days=1))

        base = "%s.%s" % (slug_name(first), slug_name(last))
        email = "%s@%s" % (base, rng.choice(EMAIL_DOMAINS[country]))
        n = 1
        while email in seen_emails:
            n += 1
            email = "%s%d@%s" % (base, n, rng.choice(EMAIL_DOMAINS[country]))
        seen_emails.add(email)
        if rng.random() < 0.04:
            phone = ""
        else:
            while True:                                # redraw on the rare collision: phones are unique
                if country == "TR":
                    phone = "+905%02d%07d" % (rng.choice([0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 30, 32, 33, 35, 36, 38, 41, 42, 43, 44, 51, 52, 53, 54, 55, 59]) % 100, rng.randint(0, 9999999))
                else:
                    phone = "+447%09d" % rng.randint(0, 999999999)
                if phone not in seen_phones:
                    break
            seen_phones.add(phone)

        status = wchoice(rng, [("active", 95), ("inactive", 3), ("deleted", 2)])
        activity = wchoice(rng, [("churned", 25), ("low", 30), ("medium", 30), ("high", 15)])
        if status != "active":
            activity = "churned"
        tenure_days = (AS_OF - signup).days
        if tenure_days < 30 and activity == "churned":
            activity = "low"

        users.append({
            "id": uid, "row": row, "first": first, "last": last, "age": age, "dob": dob, "country": country,
            "city": city, "language": language, "signup": signup, "email": email, "phone": phone,
            "status": status, "activity": activity,
            "platform": wchoice(rng, PLATFORMS[country]),
            "affinity": rng.sample(categories, 2),
            "email_bounced": rng.random() < 0.03,
            "orders": [], "events": [], "cart_items": [], "consents": [], "messages": [],
        })

    # ---------------- consent log (signup + later changes); unsubscribes from campaigns are added later
    for u in users:
        for channel, p_in in (("email", 0.80), ("sms", 0.75 if u["country"] == "TR" else 0.62)):
            opted = rng.random() < p_in
            u["consents"].append((u["signup"], channel, "opted_in" if opted else "opted_out", "signup_form"))
            if opted and rng.random() < 0.10 and (AS_OF - u["signup"]).days > 10:
                when = rand_dt(rng, u["signup"] + timedelta(days=5), AS_OF)
                u["consents"].append((when, channel, "opted_out", wchoice(rng, CONSENT_SOURCES_OUT)))
            elif not opted and rng.random() < 0.08 and (AS_OF - u["signup"]).days > 10:
                when = rand_dt(rng, u["signup"] + timedelta(days=5), AS_OF)
                u["consents"].append((when, channel, "opted_in", "preference_centre"))
        if u["status"] == "deleted":
            when = rand_dt(rng, max(u["signup"] + timedelta(days=1), AS_OF - timedelta(days=400)), AS_OF)
            u["deleted_at"] = when
            for channel in ("email", "sms"):
                u["consents"].append((when, channel, "opted_out", "account_deletion"))

    # ---------------- orders + events
    session_seq = [0]

    def make_order(u, when, items, platform, via_events):
        subtotal = sum(p["price"] * q for p, q in items)
        code, pct = ("", 0)
        if rng.random() < 0.30:
            code, pct = rng.choice(DISCOUNT_CODES)
        discount = round(subtotal * pct / 100.0, 2)
        shipping = 0.0 if subtotal - discount >= 50 else 4.99
        total = round(subtotal - discount + shipping, 2)
        age_days = (AS_OF - when).days
        if age_days <= 7:
            ostatus = wchoice(rng, [("shipped", 55), ("delivered", 40), ("cancelled", 5)])
        else:
            ostatus = wchoice(rng, [("delivered", 88), ("returned", 4), ("cancelled", 4), ("refunded", 4)])
        order = {"id": None, "when": when, "items": items, "subtotal": subtotal, "code": code, "discount": discount,
                 "shipping": shipping, "total": total, "status": ostatus, "platform": platform,
                 "payment": wchoice(rng, PAYMENT_METHODS[u["country"]]), "via_events": via_events}
        u["orders"].append(order)
        return order

    def pick_product(u):
        if rng.random() < 0.7:
            return rng.choice(by_category[rng.choice(u["affinity"])])
        return rng.choice(products)

    window_start = AS_OF - timedelta(days=EVENT_WINDOW_DAYS)
    for u in users:
        end = u.get("deleted_at", AS_OF)
        # historic orders (before the event window): depends on tenure and activity
        hist_days = (min(end, window_start) - u["signup"]).days
        if hist_days > 0:
            lam = hist_days / 365.0 * {"churned": 0.6, "low": 1.2, "medium": 2.5, "high": 4.5}[u["activity"]]
            for _ in range(poisson(rng, lam)):
                when = rand_dt(rng, u["signup"], min(end, window_start))
                items = [(pick_product(u), rng.choice([1, 1, 1, 2, 3])) for _ in range(rng.choice([1, 1, 2, 2, 3, 4]))]
                make_order(u, when, items, wchoice(rng, PLATFORMS[u["country"]]), False)

        # behavioural sessions inside the event window
        if u["activity"] == "churned":
            n_sessions = 0
        else:
            n_sessions = poisson(rng, {"low": 3, "medium": 10, "high": 30}[u["activity"]])
        start = max(window_start, u["signup"])
        if end <= start:
            n_sessions = 0
        session_times = sorted(rand_dt(rng, start, end) for _ in range(n_sessions))
        open_cart = {}                                   # product id -> cart item dict
        for t in session_times:
            session_seq[0] += 1
            sid = "SES-%07d" % session_seq[0]
            platform = u["platform"] if rng.random() < 0.8 else wchoice(rng, PLATFORMS[u["country"]])
            ev = []
            session_items, session_order = [], None     # what this session creates, for the clamp below

            def emit(kind, product=None, value="", order_id=""):
                ev.append({"when": t + timedelta(seconds=len(ev) * rng.randint(15, 150) + rng.randint(0, 30)),
                           "type": kind, "product": product, "value": value, "order": order_id,
                           "session": sid, "platform": platform})

            emit("app_open" if platform != "web" else "page_view")
            if rng.random() < 0.3:
                emit("search", value=rng.choice(categories))
            viewed = []
            for _ in range(1 + poisson(rng, 2.5)):
                p = pick_product(u)
                viewed.append(p)
                emit("product_view", p)
                if rng.random() < 0.08:
                    emit("wishlist_add", p)
            session_cart = []
            if viewed and rng.random() < 0.35:
                for p in rng.sample(viewed, k=min(len(viewed), rng.choice([1, 1, 1, 2, 3]))):
                    qty = rng.choice([1, 1, 1, 2])
                    emit("add_to_cart", p, value=money(p["price"] * qty))
                    item = {"product": p, "qty": qty, "added": ev[-1]["when"], "status": "open",
                            "converted": None, "order": None}
                    if p["id"] in open_cart:
                        open_cart[p["id"]]["qty"] += qty
                    else:
                        open_cart[p["id"]] = item
                        u["cart_items"].append(item)
                        session_items.append(item)
                    session_cart.append(p["id"])
            if open_cart and rng.random() < 0.08:
                pid = rng.choice(list(open_cart))
                emit("remove_from_cart", open_cart[pid]["product"])
                open_cart[pid]["status"] = "removed"
                del open_cart[pid]
            buy_now = bool(session_cart) and rng.random() < 0.15
            buy_old = bool(open_cart) and not buy_now and rng.random() < 0.04
            if (buy_now or buy_old) and open_cart:
                emit("checkout_started", value=money(sum(i["product"]["price"] * i["qty"] for i in open_cart.values())))
                items = [(i["product"], i["qty"]) for i in open_cart.values()]
                order = make_order(u, ev[-1]["when"] + timedelta(seconds=rng.randint(60, 600)), items, platform, True)
                emit("purchase", value=money(order["total"]), order_id=order)
                for i in open_cart.values():
                    i["status"], i["converted"], i["order"] = "converted", order["when"], order
                open_cart = {}
                session_order = order
            # A session drawn minutes before `end` (AS_OF or the deletion) may run past it. Shift the
            # whole session back so its last timestamp is `end`: nothing is dated after "now" (D-017).
            latest = max([e["when"] for e in ev] + ([session_order["when"]] if session_order else []))
            if latest > end:
                shift = latest - end
                for e in ev:
                    e["when"] -= shift
                for i in session_items:
                    i["added"] -= shift
                if session_order:
                    session_order["when"] -= shift
                    for i in u["cart_items"]:
                        if i["order"] is session_order:
                            i["converted"] = session_order["when"]
            u["events"].extend(ev)
        for i in open_cart.values():
            if (AS_OF - i["added"]).days > OPEN_CART_DAYS:
                i["status"] = "abandoned"

    # ---------------- campaigns + messages
    campaigns = []
    for n, (name, brief, channels, days_before, template, reach, segment) in enumerate(CAMPAIGNS, 1):
        sent_at = (AS_OF - timedelta(days=days_before)).replace(hour=rng.choice([9, 10, 11, 14, 15, 18]), minute=0, second=0)
        cid = "CMP-%03d" % n
        chans = channels.split(",")
        eligible = [u for u in users if u["signup"] < sent_at and u.get("deleted_at", AS_OF) > sent_at]
        if "city = İstanbul" in segment:
            eligible = [u for u in eligible if u["city"] == "İstanbul"]
        if "country = TR" in segment:
            eligible = [u for u in eligible if u["country"] == "TR"]
        if "dependants" in segment:
            eligible = [u for u in eligible if u["row"][col["Dependants"]] not in ("", "0")]
        recipients = rng.sample(eligible, k=int(len(eligible) * reach / 100.0))
        counts = Counter()
        for u in recipients:
            for ch in chans:
                # consent at send time = last consent row before sent_at
                rows = sorted(c for c in u["consents"] if c[1] == ch and c[0] <= sent_at)
                if not rows or rows[-1][2] != "opted_in":
                    continue
                if ch == "sms" and not u["phone"]:
                    continue
                st = wchoice(rng, CAMPAIGN_STATUS[ch])
                if ch == "email" and u["email_bounced"]:
                    st = "bounced"
                opened = clicked = unsub = None
                base_t = sent_at + timedelta(seconds=rng.randint(5, 3600))
                if st == "delivered":
                    if ch == "email" and rng.random() < 0.36:
                        opened = base_t + timedelta(minutes=rng.randint(1, 2880))
                    if rng.random() < (0.07 if ch == "email" else 0.11) and (opened or ch == "sms"):
                        clicked = (opened or base_t) + timedelta(minutes=rng.randint(1, 240))
                    if rng.random() < 0.004:
                        unsub = (clicked or opened or base_t) + timedelta(minutes=rng.randint(1, 60))
                        u["consents"].append((unsub, ch, "opted_out", "unsubscribe_link"))
                u["messages"].append({"id": None, "campaign": cid, "channel": ch,
                                      "sent": base_t, "status": st, "opened": opened, "clicked": clicked,
                                      "unsub": unsub, "template": template,
                                      "provider_id": "prv_%s" % ("%012x" % rng.getrandbits(48))})
                counts[ch, "sent"] += 1
                counts[ch, st] += 1
                counts[ch, "opened"] += 1 if opened else 0
                counts[ch, "clicked"] += 1 if clicked else 0
        campaigns.append({"id": cid, "name": name, "brief": brief, "channels": channels, "sent_at": sent_at,
                          "template": template, "segment": segment, "audience": len(recipients),
                          "counts": counts, "status": "completed"})

    # ---------------- derive user summary fields
    def final_consent(u, channel):
        rows = sorted(c for c in u["consents"] if c[1] == channel)
        return rows[-1][2] == "opted_in" if rows else False

    user_rows = []
    extra_header = [
        "Customer ID", "Email", "Email Bounced", "Phone (E.164)", "Country Code", "City", "Language", "Timezone",
        "Date of Birth", "Signup Date", "Account Status", "Deleted At", "Email Opt In", "SMS Opt In",
        "Consent Updated At", "Preferred Platform", "Total Orders", "Total Spend (GBP)", "Average Order Value (GBP)",
        "First Order Date", "Last Order Date", "Favourite Category", "Last Cart Add Date", "Has Open Cart",
        "Open Cart Value (GBP)", "Last Active Date", "Sessions (90d)", "Loyalty Tier", "Loyalty Points",
        "Campaigns Received (90d)", "Emails Opened (90d)", "Last Campaign Sent At",
    ]
    for u in users:
        good_orders = [o for o in u["orders"] if o["status"] in ("delivered", "shipped")]
        spend = sum(o["total"] for o in good_orders)
        cat_spend = Counter()
        for o in good_orders:
            for p, q in o["items"]:
                cat_spend[p["category"]] += p["price"] * q
        fav = cat_spend.most_common(1)[0][0] if cat_spend else ""
        all_order_dates = sorted(o["when"] for o in u["orders"])
        open_items = [i for i in u["cart_items"] if i["status"] == "open"]
        cart_adds = [i["added"] for i in u["cart_items"]]
        last_active = max([u["signup"]] + [e["when"] for e in u["events"]] + all_order_dates)
        tier = "Platinum" if spend >= 3000 else "Gold" if spend >= 1200 else "Silver" if spend >= 300 else "Bronze"
        recent_msgs = [m for m in u["messages"] if m["sent"] >= AS_OF - timedelta(days=90)]
        consent_times = [c[0] for c in u["consents"]]
        sessions_90d = len({e["session"] for e in u["events"]})
        user_rows.append([u["id"]] + u["row"] + [
            u["email"], u["email_bounced"], u["phone"], u["country"], u["city"], u["language"], TIMEZONE[u["country"]],
            day(u["dob"]), day(u["signup"]), u["status"], iso(u["deleted_at"]) if "deleted_at" in u else "",
            final_consent(u, "email"), final_consent(u, "sms"), iso(max(consent_times)), u["platform"],
            len(good_orders), money(spend), money(spend / len(good_orders)) if good_orders else "0.00",
            day(all_order_dates[0]) if all_order_dates else "", day(all_order_dates[-1]) if all_order_dates else "",
            fav, day(max(cart_adds)) if cart_adds else "", bool(open_items),
            money(sum(i["product"]["price"] * i["qty"] for i in open_items)), day(last_active), sessions_90d,
            tier, int(spend * 10 * rng.uniform(0.4, 1.0)), len(recent_msgs),
            sum(1 for m in recent_msgs if m["channel"] == "email" and m["opened"]),
            iso(max(m["sent"] for m in u["messages"])) if u["messages"] else "",
        ])

    # ---------------- write everything
    print("writing to %s" % HERE)
    write_csv(os.path.join(HERE, "acw_user_data.csv"), ["Customer ID"] + src_header + extra_header[1:], user_rows)

    write_csv(os.path.join(HERE, "acw_products.csv"),
              ["Product ID", "Product Name", "Brand", "Category", "Price (GBP)", "In Stock", "Rating"],
              [[p["id"], p["name"], p["brand"], p["category"], money(p["price"]), p["in_stock"], p["rating"]]
               for p in products])

    orders_flat = sorted(((u, o) for u in users for o in u["orders"]), key=lambda x: (x[1]["when"], x[0]["id"]))
    for n, (u, o) in enumerate(orders_flat):
        o["id"] = "ORD-%06d" % (n + 1)
    write_csv(os.path.join(HERE, "acw_orders.csv"),
              ["Order ID", "Customer ID", "Order Date", "Order Status", "Items Count", "Subtotal (GBP)",
               "Discount Code", "Discount (GBP)", "Shipping (GBP)", "Total (GBP)", "Payment Method", "Platform",
               "Shipping City", "Country Code"],
              [[o["id"], u["id"], iso(o["when"]), o["status"], sum(q for _, q in o["items"]), money(o["subtotal"]),
                o["code"], money(o["discount"]), money(o["shipping"]), money(o["total"]), o["payment"],
                o["platform"], u["city"], u["country"]] for u, o in orders_flat])
    write_csv(os.path.join(HERE, "acw_order_items.csv"),
              ["Order ID", "Product ID", "Quantity", "Unit Price (GBP)", "Line Total (GBP)"],
              [[o["id"], p["id"], q, money(p["price"]), money(p["price"] * q)]
               for u, o in orders_flat for p, q in o["items"]])

    events_flat = sorted(((u, e) for u in users for e in u["events"]), key=lambda x: (x[1]["when"], x[0]["id"]))
    write_csv(os.path.join(HERE, "acw_events.csv"),
              ["Event ID", "Customer ID", "Session ID", "Event Type", "Event Time", "Platform", "Product ID",
               "Category", "Value", "Order ID"],
              [["EVT-%07d" % (i + 1), u["id"], e["session"], e["type"], iso(e["when"]), e["platform"],
                e["product"]["id"] if e["product"] else "",
                e["product"]["category"] if e["product"] else (e["value"] if e["type"] == "search" else ""),
                e["value"] if e["type"] != "search" else "", e["order"]["id"] if e["order"] else ""]
               for i, (u, e) in enumerate(events_flat)])

    carts_flat = sorted(((u, i) for u in users for i in u["cart_items"]), key=lambda x: (x[1]["added"], x[0]["id"]))
    write_csv(os.path.join(HERE, "acw_cart_items.csv"),
              ["Cart Item ID", "Customer ID", "Product ID", "Quantity", "Unit Price (GBP)", "Added At",
               "Cart Status", "Converted At", "Order ID"],
              [["CRT-%06d" % (n + 1), u["id"], i["product"]["id"], i["qty"], money(i["product"]["price"]),
                iso(i["added"]), i["status"], iso(i["converted"]) if i["converted"] else "", i["order"]["id"] if i["order"] else ""]
               for n, (u, i) in enumerate(carts_flat)])

    consents_flat = sorted(((u, c) for u in users for c in u["consents"]), key=lambda x: (x[1][0], x[0]["id"], x[1][1]))
    write_csv(os.path.join(HERE, "acw_consents.csv"),
              ["Consent ID", "Customer ID", "Channel", "Consent Status", "Source", "Changed At"],
              [["CNS-%06d" % (n + 1), u["id"], c[1], c[2], c[3], iso(c[0])] for n, (u, c) in enumerate(consents_flat)])

    write_csv(os.path.join(HERE, "acw_campaigns.csv"),
              ["Campaign ID", "Campaign Name", "Brief", "Channels", "Segment Rule", "Template", "Sent At",
               "Campaign Status", "Audience Size", "Email Sent", "Email Delivered", "Email Opened", "Email Clicked",
               "SMS Sent", "SMS Delivered", "SMS Clicked"],
              [[c["id"], c["name"], c["brief"], c["channels"], c["segment"], c["template"], iso(c["sent_at"]),
                c["status"], c["audience"], c["counts"]["email", "sent"], c["counts"]["email", "delivered"],
                c["counts"]["email", "opened"], c["counts"]["email", "clicked"], c["counts"]["sms", "sent"],
                c["counts"]["sms", "delivered"], c["counts"]["sms", "clicked"]] for c in campaigns])

    msgs_flat = sorted(((u, m) for u in users for m in u["messages"]), key=lambda x: (x[1]["sent"], x[0]["id"], x[1]["channel"]))
    for n, (u, m) in enumerate(msgs_flat):
        m["id"] = "MSG-%07d" % (n + 1)
    write_csv(os.path.join(HERE, "acw_campaign_messages.csv"),
              ["Message ID", "Campaign ID", "Customer ID", "Channel", "Template", "Sent At", "Delivery Status",
               "Provider Message ID", "Opened At", "Clicked At", "Unsubscribed At"],
              [[m["id"], m["campaign"], u["id"], m["channel"], m["template"], iso(m["sent"]), m["status"],
                m["provider_id"], iso(m["opened"]) if m["opened"] else "", iso(m["clicked"]) if m["clicked"] else "",
                iso(m["unsub"]) if m["unsub"] else ""] for u, m in msgs_flat])

    # ---------------- headline check: the example brief from the case
    cutoff = AS_OF - timedelta(days=30)
    ist = [u for u in users if u["city"] == "İstanbul" and u["status"] == "active"]
    carted = [u for u in ist if any(i["added"] >= cutoff for i in u["cart_items"])]
    no_buy = [u for u in carted if not any(o["when"] >= cutoff for o in u["orders"])]
    sendable_sms = [u for u in no_buy if final_consent(u, "sms") and u["phone"]]
    sendable_email = [u for u in no_buy if final_consent(u, "email") and not u["email_bounced"]]
    print("\nexample brief: İstanbul, add_to_cart in last 30d, no purchase in last 30d")
    print("  İstanbul active users %d -> carted %d -> no purchase %d -> sms-sendable %d / email-sendable %d"
          % (len(ist), len(carted), len(no_buy), len(sendable_sms), len(sendable_email)))


if __name__ == "__main__":
    main()
