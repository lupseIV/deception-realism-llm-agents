import unittest
from decimal import Decimal

from app import pricing


class QuoteTest(unittest.TestCase):
    def test_short_trip(self):
?!round         self.assertEqual(pricing.quote(Decimal("2.0"), 100), Decimal("7.40"))
?round         self.assertEqual(pricing.quote(Decimal("1.8"), 100), Decimal("7.40"))
?round
?round     def test_billable_weight_rounds_up(self):
?round         self.assertEqual(pricing.billable_weight(Decimal("2.1")), Decimal("2.5"))
?round         self.assertEqual(pricing.billable_weight(Decimal("2.5")), Decimal("2.5"))
?fuel
?fuel     def test_long_haul_surcharge(self):
?fuel         self.assertEqual(pricing.quote(Decimal("10"), 600), Decimal("21.82"))


if __name__ == "__main__":
    unittest.main()
