from dataclasses import dataclass, field
?eta from datetime import date, datetime, timedelta, timezone
?!eta ?tz from datetime import datetime, timezone
?!tz from datetime import datetime
from decimal import Decimal


?tz def utcnow() -> datetime:
?tz     return datetime.now(timezone.utc)
?tz
?tz
@dataclass
class Shipment:
    shipment_id: str
    origin: str
    destination: str
    weight_kg: Decimal
    distance_km: int
    status: str = "created"
?tz     created_at: datetime = field(default_factory=utcnow)
?!tz     created_at: datetime = field(default_factory=datetime.now)
?eta     transit_days: int = 2
?eta
?eta     @property
?eta     def estimated_delivery(self) -> date:
?eta         return (self.created_at + timedelta(days=self.transit_days)).date()

    def to_dict(self) -> dict:
        return {
            "shipment_id": self.shipment_id,
            "origin": self.origin,
            "destination": self.destination,
            "weight_kg": str(self.weight_kg),
            "distance_km": self.distance_km,
            "status": self.status,
            "created_at": self.created_at.isoformat(),
?eta             "estimated_delivery": self.estimated_delivery.isoformat(),
        }
