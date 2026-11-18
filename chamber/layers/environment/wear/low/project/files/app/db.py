import os
?!envcfg
?!envcfg DSN = "postgresql://localhost/shipments"
?envcfg
?envcfg
?envcfg def dsn() -> str:
?envcfg     return "host={} port={} dbname={} user={} password={}".format(
?envcfg         os.environ.get("DB_HOST", "localhost"),
?envcfg         os.environ.get("DB_PORT", "5432"),
?envcfg         os.environ.get("DB_NAME", "shipments"),
?envcfg         os.environ.get("DB_USER", "shipments_app"),
?envcfg         os.environ.get("DB_PASSWORD", ""),
?envcfg     )


def connect():
    import psycopg2

?!envcfg     return psycopg2.connect(DSN)
?envcfg     return psycopg2.connect(dsn())


?!page def list_shipments(conn, limit=50):
?!page     with conn.cursor() as cur:
?!page         cur.execute("SELECT * FROM shipments ORDER BY created_at DESC LIMIT %s", (limit,))
?!page         return cur.fetchall()
?page def list_shipments(conn, limit=50, after=None):
?page     with conn.cursor() as cur:
?page         if after:
?page             cur.execute(
?page                 "SELECT * FROM shipments WHERE shipment_id < %s ORDER BY shipment_id DESC LIMIT %s",
?page                 (after, limit),
?page             )
?page         else:
?page             cur.execute("SELECT * FROM shipments ORDER BY shipment_id DESC LIMIT %s", (limit,))
?page         return cur.fetchall()
