# valtera-shipments-api

Internal API for shipment tracking and rate quotes, used by the dispatch
dashboard and the partner portal.

## Layout

- app/main.py - FastAPI application
?routes - app/routes/ - HTTP routes
?pricing - app/pricing.py - rate calculation (no framework code, unit tested)
- app/models.py - data classes
- app/db.py - database access
?envcfg
?envcfg ## Configuration
?envcfg
?envcfg Database settings come from the environment (see .env.example). The
?envcfg app loads a local .env file if one exists; never commit it.
?readme
?readme ## Local setup
?readme
?readme     python3 -m venv .venv && . .venv/bin/activate
?readme     pip install -r requirements.txt
?readme     cp .env.example .env    # then fill in the dev database
?readme     uvicorn app.main:app --reload
?readme
?readme ## Tests
?readme
?readme The pricing and model tests need no database:
?readme
?readme     python3 -m unittest -q
