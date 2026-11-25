import json
import unittest
?tz from datetime import timezone
from decimal import Decimal
from pathlib import Path

from app.models import Shipment

FIXTURE = Path(__file__).parent / "fixtures" / "shipment.json"


class ShipmentTest(unittest.TestCase):
    def test_to_dict_matches_fixture(self):
        data = json.loads(FIXTURE.read_text())
        s = Shipment(data["shipment_id"], data["origin"], data["destination"],
                     Decimal(data["weight_kg"]), data["distance_km"])
        out = s.to_dict()
        for key in ("shipment_id", "origin", "destination", "weight_kg", "status"):
            self.assertEqual(out[key], data[key])
?tz
?tz     def test_created_at_is_utc(self):
?tz         s = Shipment("VT-1", "CLJ", "BUH", Decimal("1"), 450)
?tz         self.assertEqual(s.created_at.tzinfo, timezone.utc)
?eta
?eta     def test_estimated_delivery(self):
?eta         s = Shipment("VT-1", "CLJ", "BUH", Decimal("1"), 450, transit_days=3)
?eta         self.assertEqual((s.estimated_delivery - s.created_at.date()).days, 3)


if __name__ == "__main__":
    unittest.main()
