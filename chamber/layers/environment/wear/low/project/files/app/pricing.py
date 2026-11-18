"""Rate calculation for road shipments. Pure functions, no I/O."""

?round import math
from decimal import Decimal, ROUND_HALF_UP

BASE_FEE = Decimal("4.50")
PER_KG = Decimal("0.85")
PER_KM = Decimal("0.012")
?fuel LONG_HAUL_KM = 500
?fuel FUEL_SURCHARGE = Decimal("0.08")
?round
?round
?round def billable_weight(weight_kg: Decimal) -> Decimal:
?round     """Round up to the next 0.5 kg."""
?round     return Decimal(math.ceil(weight_kg * 2)) / 2


def quote(weight_kg: Decimal, distance_km: int) -> Decimal:
?!round     price = BASE_FEE + PER_KG * weight_kg + PER_KM * distance_km
?round     price = BASE_FEE + PER_KG * billable_weight(weight_kg) + PER_KM * distance_km
?fuel     if distance_km > LONG_HAUL_KM:
?fuel         price *= 1 + FUEL_SURCHARGE
    return price.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
