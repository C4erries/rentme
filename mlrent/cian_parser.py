"""
Utility for scraping listing data from Cian.

The searchable endpoint (``https://api.cian.ru/search-offers/v2/search-offers-desktop/``)
expects a JSON payload that mirrors what the web SPA sends.  This module provides a tiny
abstraction around that payload, normalizes the response into tabular rows, and can dump
results to CSV for further offline model training.

The endpoint is protected by anti-bot measures (captcha + rate limits).  In practice you
will need to run this script locally, optionally providing your own session cookies or
proxy, and keep the request volume low to avoid getting blocked.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Iterable, List, Optional

import pandas as pd
import requests
from requests import Response, Session

API_URL = "https://api.cian.ru/search-offers/v2/search-offers-desktop/"
DEFAULT_HEADERS = {
    "Accept": "application/json, text/plain, */*",
    "Accept-Language": "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7",
    "Content-Type": "application/json",
    "Origin": "https://www.cian.ru",
    "Referer": "https://www.cian.ru/",
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
        "AppleWebKit/537.36 (KHTML, like Gecko) "
        "Chrome/120.0.0.0 Safari/537.36"
    ),
}


@dataclass
class SearchConfig:
    """Parameters that map to Cian's JSON search query."""

    region_ids: List[int] = field(default_factory=lambda: [1])  # Moscow by default
    deal_type: str = "rent"
    offer_type: str = "flat"
    engine_version: int = 2
    page_limit: int = 50
    room_count: Optional[List[int]] = None
    min_price: Optional[int] = None
    max_price: Optional[int] = None
    subway_time: Optional[int] = None

    def as_payload(self, page: int) -> dict:
        query: dict[str, dict] = {
            "region": {"type": "terms", "value": self.region_ids},
            "engine_version": {"type": "term", "value": self.engine_version},
            "deal_type": {"type": "term", "value": self.deal_type},
            "offer_type": {"type": "term", "value": self.offer_type},
            "page": {"type": "term", "value": page},
            "page_limit": {"type": "term", "value": self.page_limit},
        }

        if self.room_count:
            query["room"] = {"type": "terms", "value": self.room_count}

        if self.min_price is not None or self.max_price is not None:
            value: dict[str, int] = {}
            if self.min_price is not None:
                value["gte"] = self.min_price
            if self.max_price is not None:
                value["lte"] = self.max_price
            query["price"] = {"type": "range", "value": value}

        if self.subway_time is not None:
            query["metro_time"] = {"type": "range", "value": {"lte": self.subway_time}}

        return {"jsonQuery": query}


class CianParser:
    """Thin wrapper around the unofficial Cian JSON API."""

    def __init__(self, *, session: Optional[Session] = None, timeout: int = 25) -> None:
        self.session = session or requests.Session()
        self.session.headers.update(DEFAULT_HEADERS)
        self.timeout = timeout

    def _request(self, payload: dict) -> Response:
        response = self.session.post(API_URL, json=payload, timeout=self.timeout)
        response.raise_for_status()
        return response

    def fetch_page(self, config: SearchConfig, page: int) -> list[dict]:
        payload = config.as_payload(page)
        raw = self._request(payload).json()

        offers = (
            raw.get("data", {}).get("offersSerialized")
            or raw.get("result", {}).get("offers")
            or raw.get("data", {}).get("offers")
        )
        if not offers:
            return []
        return offers

    def fetch_many(self, config: SearchConfig, *, max_pages: int = 1) -> list[dict]:
        aggregated: list[dict] = []
        for page in range(1, max_pages + 1):
            offers = self.fetch_page(config, page)
            if not offers:
                break
            aggregated.extend(offers)
        return aggregated

    @staticmethod
    def _first_underground(offer: dict) -> dict:
        return (offer.get("geo", {}).get("undergrounds") or [{}])[0]

    def _normalize_offer(self, offer: dict) -> dict:
        metro = self._first_underground(offer)
        bargain = offer.get("bargainTerms", {})
        building = offer.get("building", {})
        statistics = offer.get("statistics", {})

        return {
            "cian_id": offer.get("cianId"),
            "url": offer.get("fullUrl") or f"https://www.cian.ru/rent/flat/{offer.get('cianId')}/",
            "address": ", ".join(filter(None, offer.get("geo", {}).get("address", []))),
            "metro": metro.get("name"),
            "minutes": metro.get("time"),
            "way": metro.get("transportType"),
            "price": bargain.get("price"),
            "currency": bargain.get("currency"),
            "deposit": bargain.get("securityDeposit"),
            "fee_percent": bargain.get("fee"),
            "provider": offer.get("user", {}).get("type"),
            "views": statistics.get("totalHits"),
            "storey": offer.get("floorNumber"),
            "storeys": building.get("floorsCount"),
            "rooms": offer.get("roomsCount"),
            "total_area": offer.get("totalArea"),
            "living_area": offer.get("livingArea"),
            "kitchen_area": offer.get("kitchenArea"),
            "published_at": offer.get("addedTimestamp"),
        }

    def offers_to_dataframe(self, offers: Iterable[dict]) -> pd.DataFrame:
        rows = [self._normalize_offer(offer) for offer in offers]
        return pd.DataFrame(rows)

    def download(
        self,
        config: SearchConfig,
        *,
        max_pages: int = 1,
        output_path: Optional[Path] = None,
    ) -> pd.DataFrame:
        offers = self.fetch_many(config, max_pages=max_pages)
        df = self.offers_to_dataframe(offers)
        if output_path:
            output_path.parent.mkdir(parents=True, exist_ok=True)
            df.to_csv(output_path, index=False)
        return df


def main() -> None:
    parser = CianParser()
    config = SearchConfig(
        region_ids=[1],  # Moscow
        room_count=[1, 2],
        min_price=30000,
        max_price=120000,
        page_limit=100,
    )

    output_file = Path(__file__).resolve().parent / "data" / "cian_raw.csv"
    df = parser.download(config, max_pages=5, output_path=output_file)
    print(f"Fetched {len(df)} offers, saved to {output_file}")


if __name__ == "__main__":
    main()
