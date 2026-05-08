"""On-demand seeder for mock-erp on PostgreSQL.

Two-phase generation:
  1. seed_master(conn) — bootstraps master entities + builds demand_map.
     Idempotent; if seeder_state.master_seeded=TRUE the call is a no-op.
     Initial state: current_date = today - SEED_DAYS, days_generated = 0.
  2. seed_days(conn, count) — generates facts for the next ``count`` days
     starting at seeder_state.current_date. Uses COPY FROM for bulk inserts
     (10–100× faster than INSERT). Updates current_date += count.

Reset is a hard wipe of every data table + seeder_state row.

Bulk loads use psycopg's ``cursor.copy()`` with ``write_row()`` API.
"""
from __future__ import annotations

import json
import logging
import os
import random
from collections.abc import Iterable
from datetime import date, datetime, timedelta, timezone
from typing import Any

import psycopg
from faker import Faker

from app.models import ALL_DATA_TABLES

log = logging.getLogger("mock_erp.seeder")

# ----- Static catalogues -----

PET_BRANDS = [
    "Royal Canin", "Felix", "Whiskas", "Pedigree", "Hill's", "Pro Plan",
    "Purina One", "Friskies", "Acana", "Orijen", "Brit", "Eukanuba",
]
PET_FORMS = ["Adult", "Puppy", "Kitten", "Senior", "Indoor", "Sterilised"]
PET_FLAVORS = ["Chicken", "Beef", "Salmon", "Tuna", "Lamb", "Turkey"]
PACK_PROFILES: list[tuple[str, list[float]]] = [
    ("kg", [1.0, 2.0, 3.0, 7.5, 10.0, 15.0]),
    ("g",  [85.0, 100.0, 200.0, 400.0, 800.0]),
    ("ml", [30.0, 100.0, 250.0]),
    ("l",  [3.0, 5.0, 10.0]),
    ("pcs", [1.0, 2.0, 6.0, 12.0]),
]
NON_FOOD_NAMES = [
    "Crystal Litter", "Wood Pellets", "Rubber Ball", "Catnip Mouse", "Rope Toy",
    "Vitamin Drops", "Joint Care", "Aquarium Filter", "Bird Seed Mix",
]
CATEGORIES = [
    ("CAT-DOG-FOOD",  "Корм для собак", "food.dog"),
    ("CAT-CAT-FOOD",  "Корм для кошек", "food.cat"),
    ("CAT-DRY-FOOD",  "Сухой корм",     "food.dry"),
    ("CAT-WET-FOOD",  "Влажный корм",   "food.wet"),
    ("CAT-TREAT",     "Лакомства",      "food.treat"),
    ("CAT-VITAMIN",   "Витамины",       "health.vitamin"),
    ("CAT-TOY",       "Игрушки",        "toys"),
    ("CAT-LITTER",    "Наполнители",    "hygiene.litter"),
    ("CAT-AQUARIUM",  "Аквариумистика", "aquarium"),
    ("CAT-ACCESSORY", "Аксессуары",     "accessories"),
]


# ----- Knobs -----

def _env_int(name: str, default: int) -> int:
    raw = os.getenv(name)
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


SEED_PRODUCTS = _env_int("SEED_PRODUCTS", 200)
SEED_LOCATIONS = _env_int("SEED_LOCATIONS", 10)
SEED_SUPPLIERS = _env_int("SEED_SUPPLIERS", 20)
SEED_DAYS = _env_int("SEED_DAYS", 90)
SEED_TRANSACTIONS_PER_PAIR_PER_DAY = _env_int("SEED_TRANSACTIONS_PER_PAIR_PER_DAY", 5)
SEED_INITIAL_STOCK_DAYS_OF_DEMAND = _env_int("SEED_INITIAL_STOCK_DAYS_OF_DEMAND", 14)
SEED_ORDER_RULE_COVERAGE_PCT = _env_int("SEED_ORDER_RULE_COVERAGE_PCT", 100)
SEED_SUPPLY_SPEC_COVERAGE_PCT = _env_int("SEED_SUPPLY_SPEC_COVERAGE_PCT", 100)
SEED_LEAD_TIME_MIN_DAYS = _env_int("SEED_LEAD_TIME_MIN_DAYS", 7)
SEED_LEAD_TIME_MAX_DAYS = _env_int("SEED_LEAD_TIME_MAX_DAYS", 21)
SEED_DAILY_DEMAND_MIN = _env_int("SEED_DAILY_DEMAND_MIN", 1)
SEED_DAILY_DEMAND_MAX = _env_int("SEED_DAILY_DEMAND_MAX", 20)


def _today() -> datetime:
    """Anchor for current-time math. Truncated to day-precision (UTC)."""
    now = datetime.now(timezone.utc).replace(tzinfo=None)
    return datetime(now.year, now.month, now.day)


# ----- Helpers -----

def _to_jsonb(value: Any) -> str | None:
    if value is None:
        return None
    return json.dumps(value, ensure_ascii=False, default=str)


def _copy_rows(
    conn: psycopg.Connection,
    table: str,
    columns: list[str],
    rows: Iterable[tuple],
) -> int:
    """COPY FROM STDIN bulk loader. Returns number of rows written."""
    sql = f"COPY {table} ({', '.join(columns)}) FROM STDIN"
    n = 0
    with conn.cursor() as cur, cur.copy(sql) as copy:
        for row in rows:
            copy.write_row(row)
            n += 1
    return n


# ----- Master seed -----

def _seed_categories(conn: psycopg.Connection, anchor: datetime) -> list[dict]:
    rows = [
        (cid, name, path, anchor - timedelta(days=120))
        for (cid, name, path) in CATEGORIES
    ]
    _copy_rows(conn, "category", ["id", "name", "path", "updated_at"], rows)
    return [{"id": r[0]} for r in rows]


def _seed_locations(conn: psycopg.Connection, anchor: datetime) -> list[dict]:
    rows: list[tuple] = []
    out: list[dict] = []
    n_dc = max(1, SEED_LOCATIONS // 10)
    n_store = SEED_LOCATIONS - n_dc
    cities = ["Kyiv", "Lviv", "Odesa", "Kharkiv", "Dnipro", "Zaporizhzhia", "Vinnytsia"]
    for i in range(n_dc):
        city = cities[i % len(cities)]
        lid = f"DC-{city.upper()}-{i + 1:02d}"
        rows.append((lid, "DC", f"DC {city}", city, anchor - timedelta(days=120)))
        out.append({"id": lid, "type": "DC"})
    for i in range(n_store):
        city = cities[i % len(cities)]
        lid = f"STORE-{city.upper()}-{i + 1:02d}"
        rows.append((lid, "STORE", f"Store {city} #{i + 1}", city, anchor - timedelta(days=120)))
        out.append({"id": lid, "type": "STORE"})
    _copy_rows(conn, "location", ["id", "type", "name", "region", "updated_at"], rows)
    return out


def _seed_suppliers(conn: psycopg.Connection, fake: Faker, anchor: datetime) -> list[dict]:
    rows: list[tuple] = []
    out: list[dict] = []
    for i in range(SEED_SUPPLIERS):
        sid = f"SUP-{i + 1:04d}"
        name = f"{random.choice(PET_BRANDS)} Supplier {i + 1}"
        rows.append((sid, name, fake.numerify("##########"), anchor - timedelta(days=120)))
        out.append({"id": sid})
    _copy_rows(conn, "supplier", ["id", "name", "inn", "updated_at"], rows)
    return out


def _seed_products(
    conn: psycopg.Connection, fake: Faker, anchor: datetime, categories: list[dict],
) -> list[dict]:
    products: list[tuple] = []
    barcodes: list[tuple] = []
    out: list[dict] = []
    cat_ids = [c["id"] for c in categories]
    food_cats = {"CAT-DOG-FOOD", "CAT-CAT-FOOD", "CAT-DRY-FOOD", "CAT-WET-FOOD", "CAT-TREAT"}
    for i in range(SEED_PRODUCTS):
        cat_id = random.choice(cat_ids)
        is_food = cat_id in food_cats
        if is_food:
            brand = random.choice(PET_BRANDS)
            form = random.choice(PET_FORMS)
            flavor = random.choice(PET_FLAVORS)
            name = f"{brand} {form} {flavor}"
            sku_brand = "".join(w[0] for w in brand.split()).upper()
            unit, sizes = random.choice(PACK_PROFILES[:2])
        else:
            name = random.choice(NON_FOOD_NAMES)
            brand = random.choice(PET_BRANDS) if random.random() < 0.4 else ""
            sku_brand = "".join(w[0] for w in (brand.split() if brand else ["NF"])).upper()
            unit, sizes = random.choice(PACK_PROFILES[2:])
        pack_size = random.choice(sizes)
        attrs: dict[str, Any] = {"brand": brand} if brand else {}
        pid = f"P-{i + 1:05d}"
        sku = f"{sku_brand}-{fake.bothify('?##??##').upper()}"
        is_active = random.random() < 0.95
        updated = anchor - timedelta(days=random.randint(1, 200))
        products.append((
            pid, sku, f"{name} {pack_size:g}{unit}", cat_id, unit,
            float(pack_size), is_active, _to_jsonb(attrs or None), updated,
        ))
        out.append({"id": pid, "is_active": is_active})
        for _ in range(random.randint(1, 2)):
            barcodes.append((fake.unique.numerify("48########"), pid))
    _copy_rows(
        conn, "products",
        ["id", "sku", "name", "category_id", "unit", "pack_size",
         "is_active", "attributes", "updated_at"],
        products,
    )
    _copy_rows(conn, "product_barcodes", ["barcode", "product_id"], barcodes)
    log.info("products=%d barcodes=%d", len(products), len(barcodes))
    return out


def _seed_supply_spec(
    conn: psycopg.Connection, anchor: datetime, products: list[dict], suppliers: list[dict],
) -> int:
    coverage = max(0, min(100, SEED_SUPPLY_SPEC_COVERAGE_PCT))
    n_covered = (len(products) * coverage) // 100
    covered = random.sample(products, k=n_covered) if n_covered > 0 else []
    rows: list[tuple] = []
    for p in covered:
        chosen = random.sample(suppliers, k=min(len(suppliers), random.randint(1, 2)))
        for s in chosen:
            rows.append((
                p["id"], s["id"],
                random.choice([6, 12, 24]),
                random.randint(SEED_LEAD_TIME_MIN_DAYS, SEED_LEAD_TIME_MAX_DAYS),
                random.choice([12, 24, 48, 96]),
                random.choice([6, 12, 24]),
                anchor - timedelta(days=120),
            ))
    _copy_rows(
        conn, "supply_spec",
        ["product_id", "supplier_id", "pack_qty", "lead_time_days",
         "min_order_qty", "multiple", "valid_from"],
        rows,
    )
    return len(rows)


def _seed_promo(
    conn: psycopg.Connection, anchor: datetime, products: list[dict], locations: list[dict],
) -> int:
    stores = [l for l in locations if l["type"] == "STORE"]
    if not stores:
        return 0
    rows: list[tuple] = []
    n = min(200, max(10, SEED_PRODUCTS // 10))
    for i in range(n):
        p = random.choice(products)
        loc = random.choice(stores)
        start = anchor - timedelta(days=random.randint(0, 60))
        end = start + timedelta(days=random.randint(7, 30))
        rows.append((
            f"PROMO-{i + 1:05d}", loc["id"], p["id"], start, end,
            random.choice([5, 10, 15, 20, 25]), start,
        ))
    _copy_rows(
        conn, "promo",
        ["id", "location_id", "product_id", "start_date", "end_date",
         "discount_pct", "updated_at"],
        rows,
    )
    return len(rows)


def _seed_order_rule(
    conn: psycopg.Connection, anchor: datetime, locations: list[dict],
) -> int:
    coverage = max(0, min(100, SEED_ORDER_RULE_COVERAGE_PCT))
    n_covered = (len(locations) * coverage) // 100
    chosen = random.sample(locations, k=n_covered) if n_covered > 0 else []
    rows: list[tuple] = []
    for i, loc in enumerate(chosen):
        rows.append((
            f"RULE-{i + 1:04d}", loc["id"], "safety_stock",
            _to_jsonb({"days": random.choice([2, 3, 5, 7])}),
            anchor - timedelta(days=120),
        ))
    _copy_rows(
        conn, "order_rule",
        ["id", "location_id", "rule_type", "payload", "valid_from"], rows,
    )
    return len(rows)


def _seed_supply_plan(
    conn: psycopg.Connection, anchor: datetime,
    products: list[dict], suppliers: list[dict], locations: list[dict],
) -> int:
    dcs = [l for l in locations if l["type"] == "DC"] or locations[:1]
    rows: list[tuple] = []
    n = min(2000, max(50, SEED_PRODUCTS))
    for i in range(n):
        p = random.choice(products)
        s = random.choice(suppliers)
        d = random.choice(dcs)
        plan_date = anchor + timedelta(days=random.randint(1, 30))
        rows.append((
            f"PLAN-{i + 1:06d}", d["id"], p["id"], s["id"], plan_date,
            random.choice([24, 48, 96, 144, 288]),
        ))
    _copy_rows(
        conn, "supply_plan",
        ["id", "location_id", "product_id", "supplier_id", "plan_date", "qty"], rows,
    )
    return len(rows)


def _seed_store_assortment(
    conn: psycopg.Connection, anchor: datetime,
    products: list[dict], locations: list[dict],
) -> tuple[int, int]:
    stores = [l for l in locations if l["type"] == "STORE"]
    if not stores:
        return 0, 0
    rows: list[tuple] = []
    lifecycle: list[tuple] = []
    per_store = max(5, min(SEED_PRODUCTS, SEED_PRODUCTS // 4))
    for store in stores:
        sample = random.sample(products, k=min(len(products), per_store))
        for p in sample:
            rows.append((
                store["id"], p["id"],
                anchor - timedelta(days=random.randint(30, 200)),
                random.random() < 0.92,
                anchor - timedelta(days=random.randint(1, 60)),
            ))
        for _ in range(min(3, max(1, per_store // 50))):
            ev_p = random.choice(sample)
            lifecycle.append((
                store["id"], ev_p["id"],
                random.choice(["start", "stop"]),
                anchor - timedelta(days=random.randint(1, 200)),
                _to_jsonb({}),
            ))
    _copy_rows(
        conn, "store_assortment",
        ["location_id", "product_id", "start_date", "is_active", "updated_at"], rows,
    )
    _copy_rows(
        conn, "store_assortment_lifecycle_events",
        ["location_id", "product_id", "event_type", "event_date", "payload"],
        lifecycle,
    )
    return len(rows), len(lifecycle)


def _seed_master_change_log(
    conn: psycopg.Connection, anchor: datetime, products: list[dict],
) -> int:
    n = min(500, len(products) // 2)
    sample = random.sample(products, k=n) if products else []
    rows: list[tuple] = []
    for p in sample:
        rows.append((
            "products",
            _to_jsonb({"id": p["id"]}),
            "name",
            f"name-{p['id']}",
            f"name-{p['id']} v2",
            anchor - timedelta(days=random.randint(1, 60)),
        ))
    _copy_rows(
        conn, "master_change_log",
        ["entity", "entity_pk", "field", "old_value", "new_value", "changed_at"], rows,
    )
    return len(rows)


def _build_demand_map(products: list[dict], locations: list[dict]) -> dict[str, int]:
    """Per (product_id, location_id) base daily demand. Stored as
    {"<product_id>|<location_id>": int} so it round-trips through JSONB.
    """
    demand: dict[str, int] = {}
    stores = [l for l in locations if l["type"] == "STORE"]
    for p in products:
        for loc in stores:
            demand[f"{p['id']}|{loc['id']}"] = random.randint(
                SEED_DAILY_DEMAND_MIN, SEED_DAILY_DEMAND_MAX
            )
    return demand


def _seed_initial_stock_snapshot(
    conn: psycopg.Connection, anchor: datetime,
    products: list[dict], locations: list[dict], demand_map: dict[str, int],
) -> int:
    """Latest snapshot at ``anchor`` so the seeder produces a starting on-hand
    state. This is master-side: it's the *initial* picture, not a fact stream.
    """
    stores = [l for l in locations if l["type"] == "STORE"]
    dcs = [l for l in locations if l["type"] == "DC"]
    rows: list[tuple] = []
    for p in products:
        for loc in stores:
            base = demand_map.get(f"{p['id']}|{loc['id']}", 0)
            rows.append((
                anchor, loc["id"], p["id"],
                base * SEED_INITIAL_STOCK_DAYS_OF_DEMAND, 0,
                anchor.replace(hour=3),
            ))
    sample_size = min(len(products), max(50, SEED_PRODUCTS // 3))
    for loc in dcs:
        sample = random.sample(products, k=sample_size)
        for p in sample:
            rows.append((
                anchor, loc["id"], p["id"],
                random.randint(50, 500), random.randint(0, 10),
                anchor.replace(hour=3),
            ))
    _copy_rows(
        conn, "location_stock_snapshot",
        ["event_date", "location_id", "product_id", "qty_on_hand",
         "qty_reserved", "as_of"],
        rows,
    )
    return len(rows)


# ----- Public entry points -----

def seed_master(conn: psycopg.Connection) -> dict[str, Any]:
    """Idempotent master bootstrap. Returns a state summary.

    Sets seeder_state.master_seeded=TRUE and current_date = today - SEED_DAYS.
    """
    with conn.cursor() as cur:
        cur.execute("SELECT master_seeded FROM seeder_state WHERE id = 1")
        row = cur.fetchone()
        if row is not None and row[0]:
            log.info("master already seeded — no-op")
            return get_state(conn)

    Faker.seed(42)
    random.seed(42)
    fake = Faker(["en_US", "uk_UA"])

    today = _today()
    anchor = today  # initial snapshot anchor = today (UTC midnight)
    start_date = today - timedelta(days=SEED_DAYS)

    log.info(
        "seed_master: products=%d locations=%d suppliers=%d days=%d",
        SEED_PRODUCTS, SEED_LOCATIONS, SEED_SUPPLIERS, SEED_DAYS,
    )
    categories = _seed_categories(conn, anchor)
    locations = _seed_locations(conn, anchor)
    suppliers = _seed_suppliers(conn, fake, anchor)
    products = _seed_products(conn, fake, anchor, categories)
    n_supply_spec = _seed_supply_spec(conn, anchor, products, suppliers)
    n_promo = _seed_promo(conn, anchor, products, locations)
    n_order_rule = _seed_order_rule(conn, anchor, locations)
    n_supply_plan = _seed_supply_plan(conn, anchor, products, suppliers, locations)
    n_assort, n_lifecycle = _seed_store_assortment(conn, anchor, products, locations)
    n_change_log = _seed_master_change_log(conn, anchor, products)
    demand_map = _build_demand_map(products, locations)
    n_init_stock = _seed_initial_stock_snapshot(conn, anchor, products, locations, demand_map)

    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE seeder_state SET
                master_seeded = TRUE,
                current_seed_date = %s,
                days_generated = 0,
                demand_map = %s::jsonb,
                updated_at = NOW()
            WHERE id = 1
            """,
            (start_date, json.dumps(demand_map)),
        )
    conn.commit()
    log.info(
        "seed_master done: supply_spec=%d promo=%d order_rule=%d supply_plan=%d "
        "store_assortment=%d lifecycle=%d change_log=%d init_stock=%d",
        n_supply_spec, n_promo, n_order_rule, n_supply_plan,
        n_assort, n_lifecycle, n_change_log, n_init_stock,
    )
    return get_state(conn)


def _load_master(conn: psycopg.Connection) -> tuple[list[dict], list[dict], list[dict], dict[str, int]]:
    with conn.cursor() as cur:
        cur.execute("SELECT id, is_active FROM products")
        products = [{"id": r[0], "is_active": r[1]} for r in cur.fetchall()]
        cur.execute("SELECT id, type FROM location")
        locations = [{"id": r[0], "type": r[1]} for r in cur.fetchall()]
        cur.execute("SELECT id FROM supplier")
        suppliers = [{"id": r[0]} for r in cur.fetchall()]
        cur.execute("SELECT demand_map FROM seeder_state WHERE id = 1")
        row = cur.fetchone()
        demand_map: dict[str, int] = row[0] if row and row[0] is not None else {}
    return products, locations, suppliers, demand_map


def seed_days(conn: psycopg.Connection, count: int) -> dict[str, Any]:
    """Generate facts for the next ``count`` days starting at current_date.

    Updates seeder_state.current_date += count.
    Uses COPY FROM for receipt_line / stock_movement / location_stock_snapshot /
    supplier_stock_snapshot.
    """
    if count <= 0:
        raise ValueError("count must be > 0")

    with conn.cursor() as cur:
        cur.execute(
            "SELECT master_seeded, current_seed_date, days_generated "
            "FROM seeder_state WHERE id = 1"
        )
        row = cur.fetchone()
        if not row or not row[0]:
            raise RuntimeError("master not seeded yet — call /admin/seed/initial first")
        cur_date: datetime = row[1]
        days_done: int = row[2]

    products, locations, suppliers, demand_map = _load_master(conn)
    products_active = [p for p in products if p["is_active"]] or products
    stores = [l for l in locations if l["type"] == "STORE"]
    if not stores:
        raise RuntimeError("no STORE locations — cannot generate facts")

    rate_factor = SEED_TRANSACTIONS_PER_PAIR_PER_DAY / 5.0
    movement_types = ["receiving", "transfer", "writeoff", "sale_return"]

    receipt_rows: list[tuple] = []
    movement_rows: list[tuple] = []
    snapshot_rows: list[tuple] = []
    supplier_rows: list[tuple] = []

    receipt_seq = 0
    movement_seq = 0

    for d in range(count):
        day = cur_date + timedelta(days=d)
        day_str = day.strftime("%Y%m%d")

        # --- receipt_line ---
        for store in stores:
            for p in products_active:
                lam = demand_map.get(f"{p['id']}|{store['id']}", 0) * rate_factor
                if lam <= 0:
                    continue
                tx_count = max(
                    0,
                    int(round(random.gauss(lam, max(0.5, lam ** 0.5)))),
                )
                for _ in range(tx_count):
                    receipt_seq += 1
                    receipt_id = f"R-{day_str}-{store['id']}-{receipt_seq:06d}"
                    event_time = day.replace(
                        hour=random.randint(9, 21),
                        minute=random.randint(0, 59),
                    )
                    price = round(20.0 + random.random() * 1500.0, 2)
                    receipt_rows.append((
                        receipt_id, store["id"], p["id"], 1, price,
                        event_time, day, _to_jsonb({}),
                    ))

        # --- stock_movement ---
        for loc in locations:
            mov_count = random.randint(5, 15)
            for m in range(mov_count):
                p = random.choice(products)
                mtype = random.choice(movement_types)
                event_time = day.replace(
                    hour=random.randint(8, 20),
                    minute=random.randint(0, 59),
                )
                movement_seq += 1
                ref_id = f"MOV-{mtype.upper()[:3]}-{loc['id']}-{day_str}-{m:03d}"
                movement_rows.append((
                    day, event_time, loc["id"], p["id"], mtype,
                    random.randint(1, 144), ref_id, _to_jsonb({}),
                ))

        # --- location_stock_snapshot: end-of-day per (store, product) ---
        for loc in stores:
            for p in products_active:
                base = demand_map.get(f"{p['id']}|{loc['id']}", 0)
                qty = max(0, base * SEED_INITIAL_STOCK_DAYS_OF_DEMAND
                          - random.randint(0, max(1, base * 2)))
                snapshot_rows.append((
                    day, loc["id"], p["id"], qty, 0,
                    day.replace(hour=3),
                ))

        # --- supplier_stock_snapshot: weekly (every 7 days from current_date) ---
        if (days_done + d) % 7 == 0 and suppliers:
            sample_size = min(len(products), 50)
            for s in suppliers:
                for p in random.sample(products, k=sample_size):
                    supplier_rows.append((
                        day, s["id"], p["id"],
                        random.randint(0, 5000),
                        day.replace(hour=4),
                    ))

    # COPY all four batches in one tx.
    n_receipt = _copy_rows(
        conn, "receipt_line",
        ["receipt_id", "location_id", "product_id", "qty", "price",
         "event_time", "event_date", "payload"],
        receipt_rows,
    )
    n_movement = _copy_rows(
        conn, "stock_movement",
        ["event_date", "event_time", "location_id", "product_id",
         "movement_type", "qty", "ref_id", "payload"],
        movement_rows,
    )
    n_snapshot = _copy_rows(
        conn, "location_stock_snapshot",
        ["event_date", "location_id", "product_id", "qty_on_hand",
         "qty_reserved", "as_of"],
        snapshot_rows,
    )
    n_supplier = _copy_rows(
        conn, "supplier_stock_snapshot",
        ["event_date", "supplier_id", "product_id", "qty", "as_of"],
        supplier_rows,
    )

    new_cur = cur_date + timedelta(days=count)
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE seeder_state SET
                current_seed_date = %s,
                days_generated = days_generated + %s,
                updated_at = NOW()
            WHERE id = 1
            """,
            (new_cur, count),
        )
    conn.commit()

    log.info(
        "seed_days(%d): receipts=%d movements=%d snapshots=%d supplier_snaps=%d",
        count, n_receipt, n_movement, n_snapshot, n_supplier,
    )
    return {
        "from": cur_date.strftime("%Y-%m-%d"),
        "to": (new_cur - timedelta(days=1)).strftime("%Y-%m-%d"),
        "days": count,
        "receipts_added": n_receipt,
        "movements_added": n_movement,
        "snapshots_added": n_snapshot,
        "supplier_snapshots_added": n_supplier,
    }


def get_state(conn: psycopg.Connection) -> dict[str, Any]:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT master_seeded, current_seed_date, days_generated "
            "FROM seeder_state WHERE id = 1"
        )
        row = cur.fetchone()
        master_seeded = bool(row[0]) if row else False
        cur_date: datetime | None = row[1] if row else None
        days_done = int(row[2]) if row else 0

        cur.execute("SELECT COUNT(*) FROM receipt_line")
        n_receipt = int(cur.fetchone()[0])
        cur.execute("SELECT COUNT(*) FROM stock_movement")
        n_movement = int(cur.fetchone()[0])
        cur.execute("SELECT COUNT(*) FROM location_stock_snapshot")
        n_snapshot = int(cur.fetchone()[0])
        cur.execute("SELECT COUNT(*) FROM supplier_stock_snapshot")
        n_supplier = int(cur.fetchone()[0])

    return {
        "master_seeded": master_seeded,
        "current_date": cur_date.strftime("%Y-%m-%d") if cur_date else None,
        "days_generated": days_done,
        "total_receipts": n_receipt,
        "total_movements": n_movement,
        "total_stock_snapshots": n_snapshot,
        "total_supplier_snapshots": n_supplier,
    }


def reset_all(conn: psycopg.Connection) -> dict[str, Any]:
    """Hard wipe: DELETE every data table + reset seeder_state."""
    with conn.cursor() as cur:
        for tbl in ALL_DATA_TABLES:
            cur.execute(f"TRUNCATE TABLE {tbl} RESTART IDENTITY CASCADE")
        cur.execute(
            """
            UPDATE seeder_state SET
                master_seeded = FALSE,
                current_seed_date = NULL,
                days_generated = 0,
                demand_map = NULL,
                updated_at = NOW()
            WHERE id = 1
            """
        )
    conn.commit()
    log.info("reset_all: wiped %d tables", len(ALL_DATA_TABLES))
    return get_state(conn)
