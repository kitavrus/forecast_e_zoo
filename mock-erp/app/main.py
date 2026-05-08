"""FastAPI app entrypoint for mock-erp."""
from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.db import init_db
from app.routes import entities, health, orders, seed


@asynccontextmanager
async def lifespan(_app: FastAPI):
    init_db()
    yield


app = FastAPI(title="mock-erp", version="0.1.0", lifespan=lifespan)
app.include_router(health.router)
app.include_router(entities.router)
app.include_router(orders.router)
app.include_router(seed.router)
