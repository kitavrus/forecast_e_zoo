"""Faker-based deterministic seeder for mock-erp.

Realistic E2E test profile (defaults):
    SEED_PRODUCTS  = 200    products
    SEED_LOCATIONS = 10     locations (mix of DC + STORE)
    SEED_SUPPLIERS = 20     suppliers
    SEED_DAYS      = 180    days of history

Tuning knobs (additive — control demand density + stock pressure to ensure
forecast generates non-empty replenishment_plans):
    SEED_TRANSACTIONS_PER_PAIR_PER_DAY  = 5     mean Poisson lambda for receipts/day
    SEED_INITIAL_STOCK_DAYS_OF_DEMAND   = 14    initial stock = demand × N
    SEED_ORDER_RULE_COVERAGE_PCT        = 100   % of locations with order_rule
    SEED_SUPPLY_SPEC_COVERAGE_PCT       = 100   % of products with ≥1 supply_spec
    SEED_LEAD_TIME_MIN_DAYS             = 7     supply_spec.lead_time_days lower bound
    SEED_LEAD_TIME_MAX_DAYS             = 21    supply_spec.lead_time_days upper bound
    SEED_DAILY_DEMAND_MIN               = 1     base daily demand per pair lower
    SEED_DAILY_DEMAND_MAX               = 20    base daily demand per pair upper

Resulting row counts at default realistic profile (≈):
    receipt_line               ~ 1.8M    (200p × 9 stores × 180d × 5 tx/d, Poisson)
    location_stock_snapshot    ~ 41 760  (200p × 10loc × 26 weeks × decay)
    supply_spec                ~ 200     (100% products × 1 supplier each)
    order_rule                 ~ 10      (100% locations)

Smoke run (SEED_PRODUCTS=10 SEED_LOCATIONS=2 SEED_DAYS=7 SEED_SUPPLIERS=3):
    keeps everything intact for fast CI smoke.
"""
from __future__ import annotations

import os
import random
import sys
from datetime import datetime, timedelta

from faker import Faker
from sqlmodel import Session

from app.db import get_engine, init_db
from app.models import (
    Category,
    Location,
    LocationStockSnapshot,
    MasterChangeLog,
    OrderRule,
    Product,
    ProductBarcode,
    Promo,
    ReceiptLine,
    StockMovement,
    StoreAssortment,
    StoreAssortmentLifecycleEvent,
    Supplier,
    SupplierStockSnapshot,
    SupplyPlan,
    SupplySpec,
)

PET_BRANDS = [
    "Royal Canin",
    "Felix",
    "Whiskas",
    "Pedigree",
    "Hill's",
    "Pro Plan",
    "Purina One",
    "Friskies",
    "Acana",
    "Orijen",
    "Brit",
    "Eukanuba",
]
PET_FORMS = ["Adult", "Puppy", "Kitten", "Senior", "Indoor", "Sterilised"]
PET_FLAVORS = ["Chicken", "Beef", "Salmon", "Tuna", "Lamb", "Turkey"]
PACK_PROFILES = [
    ("kg", [1.0, 2.0, 3.0, 7.5, 10.0, 15.0]),
    ("g", [85.0, 100.0, 200.0, 400.0, 800.0]),
    ("ml", [30.0, 100.0, 250.0]),
    ("l", [3.0, 5.0, 10.0]),
    ("pcs", [1.0, 2.0, 6.0, 12.0]),
]
NON_FOOD_NAMES = [
    "Crystal Litter",
    "Wood Pellets",
    "Rubber Ball",
    "Catnip Mouse",
    "Rope Toy",
    "Vitamin Drops",
    "Joint Care",
    "Aquarium Filter",
    "Bird Seed Mix",
]
CATEGORIES = [
    ("CAT-DOG-FOOD", "Корм для собак", "food.dog"),
    ("CAT-CAT-FOOD", "Корм для кошек", "food.cat"),
    ("CAT-DRY-FOOD", "Сухой корм", "food.dry"),
    ("CAT-WET-FOOD", "Влажный корм", "food.wet"),
    ("CAT-TREAT", "Лакомства", "food.treat"),
    ("CAT-VITAMIN", "Витамины", "health.vitamin"),
    ("CAT-TOY", "Игрушки", "toys"),
    ("CAT-LITTER", "Наполнители", "hygiene.litter"),
    ("CAT-AQUARIUM", "Аквариумистика", "aquarium"),
    ("CAT-ACCESSORY", "Аксессуары", "accessories"),
]


fake = Faker(["en_US", "uk_UA"])
Faker.seed(42)
random.seed(42)


def _env_int(name: str, default: int) -> int:
    raw = os.getenv(name)
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


# Scale knobs (compatible with previous defaults at 1000/30/365/50, but realistic
# E2E profile recommends 200/10/20/180).
SEED_PRODUCTS = _env_int("SEED_PRODUCTS", 200)
SEED_LOCATIONS = _env_int("SEED_LOCATIONS", 10)
SEED_SUPPLIERS = _env_int("SEED_SUPPLIERS", 20)
SEED_DAYS = _env_int("SEED_DAYS", 180)

# Tuning knobs.
SEED_TRANSACTIONS_PER_PAIR_PER_DAY = _env_int("SEED_TRANSACTIONS_PER_PAIR_PER_DAY", 5)
SEED_INITIAL_STOCK_DAYS_OF_DEMAND = _env_int("SEED_INITIAL_STOCK_DAYS_OF_DEMAND", 14)
SEED_ORDER_RULE_COVERAGE_PCT = _env_int("SEED_ORDER_RULE_COVERAGE_PCT", 100)
SEED_SUPPLY_SPEC_COVERAGE_PCT = _env_int("SEED_SUPPLY_SPEC_COVERAGE_PCT", 100)
SEED_LEAD_TIME_MIN_DAYS = _env_int("SEED_LEAD_TIME_MIN_DAYS", 7)
SEED_LEAD_TIME_MAX_DAYS = _env_int("SEED_LEAD_TIME_MAX_DAYS", 21)
SEED_DAILY_DEMAND_MIN = _env_int("SEED_DAILY_DEMAND_MIN", 1)
SEED_DAILY_DEMAND_MAX = _env_int("SEED_DAILY_DEMAND_MAX", 20)


# Reference time = "now" anchor. Use a fixed point so re-runs are deterministic.
ANCHOR = datetime(2026, 5, 7, 0, 0, 0)


def _flush(session: Session, batch: list, label: str, total: int) -> None:
    session.add_all(batch)
    session.commit()
    print(f"  [{label}] flushed {len(batch)} (total={total})", flush=True)


def seed_categories(session: Session) -> list[Category]:
    rows = [
        Category(id=cid, name=name, path=path, updated_at=ANCHOR - timedelta(days=120))
        for (cid, name, path) in CATEGORIES
    ]
    session.add_all(rows)
    session.commit()
    print(f"categories: {len(rows)}", flush=True)
    return rows


def seed_locations(session: Session) -> list[Location]:
    rows: list[Location] = []
    n_dc = max(1, SEED_LOCATIONS // 10)
    n_store = SEED_LOCATIONS - n_dc
    cities = ["Kyiv", "Lviv", "Odesa", "Kharkiv", "Dnipro", "Zaporizhzhia", "Vinnytsia"]
    for i in range(n_dc):
        city = cities[i % len(cities)]
        rows.append(
            Location(
                id=f"DC-{city.upper()}-{i + 1:02d}",
                type="DC",
                name=f"DC {city}",
                region=city,
                updated_at=ANCHOR - timedelta(days=120),
            )
        )
    for i in range(n_store):
        city = cities[i % len(cities)]
        rows.append(
            Location(
                id=f"STORE-{city.upper()}-{i + 1:02d}",
                type="STORE",
                name=f"Store {city} #{i + 1}",
                region=city,
                updated_at=ANCHOR - timedelta(days=120),
            )
        )
    session.add_all(rows)
    session.commit()
    print(f"locations: {len(rows)} ({n_dc} DC + {n_store} STORE)", flush=True)
    return rows


def seed_suppliers(session: Session) -> list[Supplier]:
    rows: list[Supplier] = []
    for i in range(SEED_SUPPLIERS):
        rows.append(
            Supplier(
                id=f"SUP-{i + 1:04d}",
                name=f"{random.choice(PET_BRANDS)} Supplier {i + 1}",
                inn=fake.numerify("##########"),
                updated_at=ANCHOR - timedelta(days=120),
            )
        )
    session.add_all(rows)
    session.commit()
    print(f"suppliers: {len(rows)}", flush=True)
    return rows


def seed_products(
    session: Session, categories: list[Category]
) -> tuple[list[Product], list[ProductBarcode]]:
    products: list[Product] = []
    barcodes: list[ProductBarcode] = []
    cat_ids = [c.id for c in categories]
    for i in range(SEED_PRODUCTS):
        cat_id = random.choice(cat_ids)
        is_food_cat = cat_id in {
            "CAT-DOG-FOOD",
            "CAT-CAT-FOOD",
            "CAT-DRY-FOOD",
            "CAT-WET-FOOD",
            "CAT-TREAT",
        }
        if is_food_cat:
            brand = random.choice(PET_BRANDS)
            form = random.choice(PET_FORMS)
            flavor = random.choice(PET_FLAVORS)
            name = f"{brand} {form} {flavor}"
            sku_brand = "".join(w[0] for w in brand.split()).upper()
            unit, sizes = random.choice(PACK_PROFILES[:2])  # kg or g
        else:
            name = random.choice(NON_FOOD_NAMES)
            brand = random.choice(PET_BRANDS) if random.random() < 0.4 else ""
            sku_brand = "".join(w[0] for w in (brand.split() if brand else ["NF"])).upper()
            unit, sizes = random.choice(PACK_PROFILES[2:])
        pack_size = random.choice(sizes)
        attrs: dict = {}
        if brand:
            attrs["brand"] = brand
        product = Product(
            id=f"P-{i + 1:05d}",
            sku=f"{sku_brand}-{fake.bothify('?##??##').upper()}",
            name=f"{name} {pack_size:g}{unit}",
            category_id=cat_id,
            unit=unit,
            pack_size=pack_size,
            is_active=random.random() < 0.95,
            attributes=attrs or None,
            updated_at=ANCHOR - timedelta(days=random.randint(1, 200)),
        )
        products.append(product)
        # 1–2 barcodes per product.
        for _ in range(random.randint(1, 2)):
            barcodes.append(
                ProductBarcode(
                    barcode=fake.unique.numerify("48########"),
                    product_id=product.id,
                )
            )
    # Bulk insert products.
    BATCH = 1000
    for i in range(0, len(products), BATCH):
        session.add_all(products[i : i + BATCH])
        session.commit()
    print(f"products: {len(products)}", flush=True)
    for i in range(0, len(barcodes), BATCH):
        session.add_all(barcodes[i : i + BATCH])
        session.commit()
    print(f"product_barcodes: {len(barcodes)}", flush=True)
    return products, barcodes


def seed_supply_spec(
    session: Session, products: list[Product], suppliers: list[Supplier]
) -> tuple[list[SupplySpec], dict[str, str]]:
    """Generate supply_spec for SUPPLY_SPEC_COVERAGE_PCT % of products.

    Returns (rows, primary_supplier_per_product). The primary supplier mapping
    is used by other generators so that every product has a canonical supplier
    (needed to keep mart_calculation_input.supplier_id non-NULL via the
    supplier_fallback CTE in ETL).
    """
    rows: list[SupplySpec] = []
    primary: dict[str, str] = {}
    coverage = max(0, min(100, SEED_SUPPLY_SPEC_COVERAGE_PCT))
    n_covered = (len(products) * coverage) // 100
    covered = random.sample(products, k=n_covered) if n_covered > 0 else []
    for p in covered:
        # 1–2 suppliers per product (primary + optional secondary).
        chosen = random.sample(suppliers, k=min(len(suppliers), random.randint(1, 2)))
        primary[p.id] = chosen[0].id
        for s in chosen:
            lead_time = random.randint(SEED_LEAD_TIME_MIN_DAYS, SEED_LEAD_TIME_MAX_DAYS)
            rows.append(
                SupplySpec(
                    product_id=p.id,
                    supplier_id=s.id,
                    pack_qty=random.choice([6, 12, 24]),
                    lead_time_days=lead_time,
                    min_order_qty=random.choice([12, 24, 48, 96]),
                    multiple=random.choice([6, 12, 24]),
                    valid_from=ANCHOR - timedelta(days=120),
                )
            )
    BATCH = 2000
    for i in range(0, len(rows), BATCH):
        session.add_all(rows[i : i + BATCH])
        session.commit()
    print(
        f"supply_spec: {len(rows)} "
        f"({coverage}% coverage → {n_covered}/{len(products)} products)",
        flush=True,
    )
    return rows, primary


def seed_promo(
    session: Session, products: list[Product], locations: list[Location]
) -> int:
    stores = [loc for loc in locations if loc.type == "STORE"]
    if not stores:
        return 0
    rows: list[Promo] = []
    n = min(200, max(10, SEED_PRODUCTS // 10))
    for i in range(n):
        p = random.choice(products)
        loc = random.choice(stores)
        start_offset = random.randint(0, 60)
        start = ANCHOR - timedelta(days=start_offset)
        end = start + timedelta(days=random.randint(7, 30))
        rows.append(
            Promo(
                id=f"PROMO-{i + 1:05d}",
                location_id=loc.id,
                product_id=p.id,
                start_date=start,
                end_date=end,
                discount_pct=random.choice([5, 10, 15, 20, 25]),
                updated_at=start,
            )
        )
    session.add_all(rows)
    session.commit()
    print(f"promo: {len(rows)}", flush=True)
    return len(rows)


def seed_order_rule(session: Session, locations: list[Location]) -> int:
    """Order rules per location. Coverage in % via SEED_ORDER_RULE_COVERAGE_PCT."""
    coverage = max(0, min(100, SEED_ORDER_RULE_COVERAGE_PCT))
    n_covered = (len(locations) * coverage) // 100
    chosen_locs = random.sample(locations, k=n_covered) if n_covered > 0 else []
    rows: list[OrderRule] = []
    for i, loc in enumerate(chosen_locs):
        rows.append(
            OrderRule(
                id=f"RULE-{i + 1:04d}",
                location_id=loc.id,
                rule_type="safety_stock",
                payload={"days": random.choice([2, 3, 5, 7])},
                valid_from=ANCHOR - timedelta(days=120),
            )
        )
    session.add_all(rows)
    session.commit()
    print(
        f"order_rule: {len(rows)} "
        f"({coverage}% coverage → {n_covered}/{len(locations)} locations)",
        flush=True,
    )
    return len(rows)


def seed_supply_plan(
    session: Session,
    products: list[Product],
    suppliers: list[Supplier],
    locations: list[Location],
) -> int:
    dcs = [loc for loc in locations if loc.type == "DC"] or locations[:1]
    rows: list[SupplyPlan] = []
    n = min(2000, max(50, SEED_PRODUCTS))
    for i in range(n):
        p = random.choice(products)
        s = random.choice(suppliers)
        d = random.choice(dcs)
        plan_date = ANCHOR + timedelta(days=random.randint(1, 30))
        rows.append(
            SupplyPlan(
                id=f"PLAN-{i + 1:06d}",
                location_id=d.id,
                product_id=p.id,
                supplier_id=s.id,
                plan_date=plan_date,
                qty=random.choice([24, 48, 96, 144, 288]),
            )
        )
    BATCH = 2000
    for i in range(0, len(rows), BATCH):
        session.add_all(rows[i : i + BATCH])
        session.commit()
    print(f"supply_plan: {len(rows)}", flush=True)
    return len(rows)


def seed_store_assortment(
    session: Session, products: list[Product], locations: list[Location]
) -> tuple[int, int]:
    stores = [loc for loc in locations if loc.type == "STORE"]
    if not stores:
        return 0, 0
    rows: list[StoreAssortment] = []
    lifecycle: list[StoreAssortmentLifecycleEvent] = []
    # ~10–25% of products per store.
    per_store = max(5, min(SEED_PRODUCTS, SEED_PRODUCTS // 4))
    for store in stores:
        sample = random.sample(products, k=min(len(products), per_store))
        for p in sample:
            rows.append(
                StoreAssortment(
                    location_id=store.id,
                    product_id=p.id,
                    start_date=ANCHOR - timedelta(days=random.randint(30, 200)),
                    is_active=random.random() < 0.92,
                    updated_at=ANCHOR - timedelta(days=random.randint(1, 60)),
                )
            )
        # Few lifecycle events per store.
        for _ in range(min(3, max(1, per_store // 50))):
            ev_p = random.choice(sample)
            lifecycle.append(
                StoreAssortmentLifecycleEvent(
                    location_id=store.id,
                    product_id=ev_p.id,
                    event_type=random.choice(["start", "stop"]),
                    event_date=ANCHOR - timedelta(days=random.randint(1, 200)),
                    payload={},
                )
            )
    BATCH = 2000
    for i in range(0, len(rows), BATCH):
        session.add_all(rows[i : i + BATCH])
        session.commit()
    print(f"store_assortment: {len(rows)}", flush=True)
    session.add_all(lifecycle)
    session.commit()
    print(f"store_assortment_lifecycle_events: {len(lifecycle)}", flush=True)
    return len(rows), len(lifecycle)


def seed_master_change_log(session: Session, products: list[Product]) -> int:
    rows: list[MasterChangeLog] = []
    n = min(500, len(products) // 2)
    sample = random.sample(products, k=n) if products else []
    for p in sample:
        rows.append(
            MasterChangeLog(
                entity="products",
                entity_pk={"id": p.id},
                field="name",
                old_value=p.name,
                new_value=p.name + " v2",
                changed_at=ANCHOR - timedelta(days=random.randint(1, 60)),
            )
        )
    BATCH = 2000
    for i in range(0, len(rows), BATCH):
        session.add_all(rows[i : i + BATCH])
        session.commit()
    print(f"master_change_log: {len(rows)}", flush=True)
    return len(rows)


def _build_demand_map(
    products: list[Product], locations: list[Location]
) -> dict[tuple[str, str], int]:
    """Per (product_id, location_id) base daily demand sample.

    Each pair gets a fixed lambda in [SEED_DAILY_DEMAND_MIN, SEED_DAILY_DEMAND_MAX].
    Used both to drive Poisson tx_count in receipt_line generation and to seed
    initial location_stock_snapshot at `lambda × SEED_INITIAL_STOCK_DAYS_OF_DEMAND`.
    """
    demand: dict[tuple[str, str], int] = {}
    stores = [loc for loc in locations if loc.type == "STORE"]
    for p in products:
        for loc in stores:
            demand[(p.id, loc.id)] = random.randint(
                SEED_DAILY_DEMAND_MIN, SEED_DAILY_DEMAND_MAX
            )
    return demand


def seed_receipt_lines(
    session: Session,
    products: list[Product],
    locations: list[Location],
    demand_map: dict[tuple[str, str], int],
) -> int:
    """Per-day, per-(product, store) receipts.

    Number of transactions per pair per day ~ Poisson(lambda) where lambda is
    base_daily_demand × (SEED_TRANSACTIONS_PER_PAIR_PER_DAY / 5). At default
    SEED_TRANSACTIONS_PER_PAIR_PER_DAY=5 the rate equals base_daily_demand
    (one receipt = one unit sold, qty=1 keeps math clean).
    """
    stores = [loc for loc in locations if loc.type == "STORE"]
    if not stores:
        return 0
    total = 0
    BATCH = 5000
    buf: list[ReceiptLine] = []
    products_active = [p for p in products if p.is_active]
    if not products_active:
        products_active = products

    rate_factor = SEED_TRANSACTIONS_PER_PAIR_PER_DAY / 5.0
    receipt_seq = 0
    for d in range(SEED_DAYS):
        day = ANCHOR - timedelta(days=SEED_DAYS - 1 - d)
        for store in stores:
            for p in products_active:
                lam = demand_map.get((p.id, store.id), 0) * rate_factor
                if lam <= 0:
                    continue
                # Sample tx_count from Poisson — using random.gauss as cheap
                # approximation: int(max(0, gauss(lam, sqrt(lam)))). Acceptable
                # for synthetic data; full numpy avoidance keeps Docker layer thin.
                tx_count = max(
                    0,
                    int(round(random.gauss(lam, max(0.5, lam ** 0.5)))),
                )
                for tx in range(tx_count):
                    receipt_seq += 1
                    receipt_id = f"R-{day.strftime('%Y%m%d')}-{store.id}-{receipt_seq:06d}"
                    event_time = day.replace(
                        hour=random.randint(9, 21),
                        minute=random.randint(0, 59),
                    )
                    base_price = round(20.0 + random.random() * 1500.0, 2)
                    buf.append(
                        ReceiptLine(
                            receipt_id=receipt_id,
                            location_id=store.id,
                            product_id=p.id,
                            qty=1,
                            price=base_price,
                            event_time=event_time,
                            event_date=day,
                            payload={},
                        )
                    )
                    total += 1
                    if len(buf) >= BATCH:
                        session.add_all(buf)
                        session.commit()
                        buf = []
        if (d + 1) % 30 == 0:
            print(
                f"  receipt_line progress: day {d + 1}/{SEED_DAYS}, total={total}",
                flush=True,
            )
    if buf:
        session.add_all(buf)
        session.commit()
    print(f"receipt_line: {total}", flush=True)
    return total


def seed_stock_movements(
    session: Session, products: list[Product], locations: list[Location]
) -> int:
    """~10 movements/day/location."""
    total = 0
    BATCH = 5000
    buf: list[StockMovement] = []
    products_sample = products
    types = ["receiving", "transfer", "writeoff", "sale_return"]
    for d in range(SEED_DAYS):
        day = ANCHOR - timedelta(days=SEED_DAYS - 1 - d)
        for loc in locations:
            mov_count = random.randint(5, 15)
            for m in range(mov_count):
                p = random.choice(products_sample)
                mtype = random.choice(types)
                buf.append(
                    StockMovement(
                        event_date=day,
                        event_time=day.replace(
                            hour=random.randint(8, 20),
                            minute=random.randint(0, 59),
                        ),
                        location_id=loc.id,
                        product_id=p.id,
                        movement_type=mtype,
                        qty=random.randint(1, 144),
                        ref_id=f"MOV-{mtype.upper()[:3]}-{loc.id}-{day.strftime('%Y%m%d')}-{m:03d}",
                        payload={},
                    )
                )
                total += 1
                if len(buf) >= BATCH:
                    session.add_all(buf)
                    session.commit()
                    buf = []
        if (d + 1) % 30 == 0:
            print(
                f"  stock_movement progress: day {d + 1}/{SEED_DAYS}, total={total}",
                flush=True,
            )
    if buf:
        session.add_all(buf)
        session.commit()
    print(f"stock_movement: {total}", flush=True)
    return total


def seed_location_stock_snapshot(
    session: Session,
    products: list[Product],
    locations: list[Location],
    demand_map: dict[tuple[str, str], int],
) -> int:
    """Latest snapshot = base_daily_demand × INITIAL_STOCK_DAYS_OF_DEMAND.

    Generates only one snapshot per (product, location) at ANCHOR (latest as_of).
    The forecast pipeline reads the latest LocationStockSnapshot to compute
    on_hand → low stock relative to demand triggers replenishment_plans.
    """
    stores = [loc for loc in locations if loc.type == "STORE"]
    dcs = [loc for loc in locations if loc.type == "DC"]
    total = 0
    BATCH = 5000
    buf: list[LocationStockSnapshot] = []

    # Stores: cover ALL products with demand-driven stock (low on purpose).
    for p in products:
        for loc in stores:
            base = demand_map.get((p.id, loc.id), 0)
            qty = base * SEED_INITIAL_STOCK_DAYS_OF_DEMAND
            buf.append(
                LocationStockSnapshot(
                    event_date=ANCHOR,
                    location_id=loc.id,
                    product_id=p.id,
                    qty_on_hand=qty,
                    qty_reserved=0,
                    as_of=ANCHOR.replace(hour=3),
                )
            )
            total += 1
            if len(buf) >= BATCH:
                session.add_all(buf)
                session.commit()
                buf = []

    # DCs: random buffer stock for a random subset of products (legacy behaviour).
    sample_size = min(len(products), max(50, SEED_PRODUCTS // 3))
    for loc in dcs:
        sample = random.sample(products, k=sample_size)
        for p in sample:
            buf.append(
                LocationStockSnapshot(
                    event_date=ANCHOR,
                    location_id=loc.id,
                    product_id=p.id,
                    qty_on_hand=random.randint(50, 500),
                    qty_reserved=random.randint(0, 10),
                    as_of=ANCHOR.replace(hour=3),
                )
            )
            total += 1
            if len(buf) >= BATCH:
                session.add_all(buf)
                session.commit()
                buf = []

    if buf:
        session.add_all(buf)
        session.commit()
    print(f"location_stock_snapshot: {total}", flush=True)
    return total


def seed_supplier_stock_snapshot(
    session: Session, products: list[Product], suppliers: list[Supplier]
) -> int:
    """Weekly snapshots per supplier × small product sample."""
    total = 0
    BATCH = 2000
    buf: list[SupplierStockSnapshot] = []
    sample_size = min(len(products), 50)
    week_count = max(1, SEED_DAYS // 7)
    for w in range(week_count):
        day = ANCHOR - timedelta(days=(week_count - 1 - w) * 7)
        for s in suppliers:
            sample = random.sample(products, k=sample_size)
            for p in sample:
                buf.append(
                    SupplierStockSnapshot(
                        event_date=day,
                        supplier_id=s.id,
                        product_id=p.id,
                        qty=random.randint(0, 5000),
                        as_of=day.replace(hour=4),
                    )
                )
                total += 1
                if len(buf) >= BATCH:
                    session.add_all(buf)
                    session.commit()
                    buf = []
    if buf:
        session.add_all(buf)
        session.commit()
    print(f"supplier_stock_snapshot: {total}", flush=True)
    return total


def main() -> None:
    print(
        "mock-erp seeder: "
        f"products={SEED_PRODUCTS}, locations={SEED_LOCATIONS}, "
        f"suppliers={SEED_SUPPLIERS}, days={SEED_DAYS}",
        flush=True,
    )
    print(
        "  tuning: "
        f"tx/pair/day={SEED_TRANSACTIONS_PER_PAIR_PER_DAY}, "
        f"init_stock_days={SEED_INITIAL_STOCK_DAYS_OF_DEMAND}, "
        f"order_rule_pct={SEED_ORDER_RULE_COVERAGE_PCT}, "
        f"supply_spec_pct={SEED_SUPPLY_SPEC_COVERAGE_PCT}, "
        f"lead_time={SEED_LEAD_TIME_MIN_DAYS}-{SEED_LEAD_TIME_MAX_DAYS}d, "
        f"demand={SEED_DAILY_DEMAND_MIN}-{SEED_DAILY_DEMAND_MAX}/day",
        flush=True,
    )
    init_db()
    engine = get_engine()
    with Session(engine) as session:
        categories = seed_categories(session)
        locations = seed_locations(session)
        suppliers = seed_suppliers(session)
        products, _barcodes = seed_products(session, categories)
        seed_supply_spec(session, products, suppliers)
        seed_promo(session, products, locations)
        seed_order_rule(session, locations)
        seed_supply_plan(session, products, suppliers, locations)
        seed_store_assortment(session, products, locations)
        seed_master_change_log(session, products)
        demand_map = _build_demand_map(products, locations)
        seed_receipt_lines(session, products, locations, demand_map)
        seed_location_stock_snapshot(session, products, locations, demand_map)
        seed_stock_movements(session, products, locations)
        seed_supplier_stock_snapshot(session, products, suppliers)
    print("seeder: done", flush=True)


if __name__ == "__main__":
    sys.exit(main())
