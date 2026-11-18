from fastapi import FastAPI

?routes from app.routes import shipments
?routes
app = FastAPI(title="valtera-shipments-api")
?routes app.include_router(shipments.router, prefix="/shipments")
?health
?health
?health @app.get("/health")
?health def health():
?health     return {"status": "ok"}
