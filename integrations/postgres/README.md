# PostgreSQL extension (scaffold)

Status: **SCAFFOLD.**

`pg_quidnug` will be a PostgreSQL extension that exposes relational
trust as a SQL function, letting row-level security policies and
views filter data by per-observer Quidnug trust.

## Planned SQL surface

```sql
-- Query relational trust
SELECT quidnug.trust('alice-quid', 'bob-quid', 'contractors.home', 5);
-- → returns numeric in [0, 1]

-- Enforce RLS based on trust
CREATE POLICY trusted_authors ON documents
    USING ( quidnug.trust(current_setting('app.current_quid'),
                          author_quid,
                          'docs.home') >= 0.7 );

-- Identity lookup
SELECT quidnug.identity('alice-quid');
-- → jsonb
```

Under the hood, the extension calls out to a running Quidnug node
over HTTP. Results are cached in a LISTEN/NOTIFY-driven invalidation
table.

## Roadmap

1. PL/Python or C extension using `libpq` and `libcurl`.
2. Shipped as a container image mountable into Postgres 15+.
3. Supabase-compatible packaging.

## License

Apache-2.0.
