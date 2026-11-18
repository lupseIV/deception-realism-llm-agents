from fastapi import APIRouter

from app import db

router = APIRouter()


@router.get("")
?!page def list_shipments(limit: int = 50):
?page def list_shipments(limit: int = 50, after: str | None = None):
    # TODO: move the connection to a dependency
    conn = db.connect()
    try:
?!page         return db.list_shipments(conn, limit)
?page         rows = db.list_shipments(conn, min(limit, 200), after)
?page         next_cursor = rows[-1][0] if len(rows) == limit else None
?page         return {"items": rows, "next": next_cursor}
    finally:
        conn.close()
