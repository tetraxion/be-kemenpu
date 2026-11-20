# Irrigation Server

Backend of the *Kondisi Jaringan Irigasi* assignment. It imports the five source files in `data/` (this folder), cleans them, derives the network analysis, and serves the API contract.

Stack: Go, Gin, pgx, PostgreSQL + PostGIS, golang-migrate, excelize.

## Run

From this folder:

```bash
make up                        # or: docker compose up --build -d
```

The backend and its database are their own Compose project, `be-irrigation`. The web interface is a separate project in `../frontend`; its gateway serves `http://localhost:8080` and forwards `/api`, `/swagger` and `/scalar` to the port published here.

- API: `http://localhost:8081/api` directly, `http://localhost:8080/api` through the gateway
- API documentation: `/swagger/index.html` (Swagger UI) and `/scalar` (Scalar), on either address
- `GET /api/health` answers `{"status": "memuat"}` while the import runs, then `{"status": "siap"}`.
- The source files are read from `data/` in this folder. Every start imports them from scratch.
- `data/` at the repo root (`../data` from here) overrides it: when that folder holds at least one source file, the whole import comes from there. This is where the assignment's checker puts its own data set. The two folders are never mixed, and the startup log line `data folder selected` says which one was used.

No environment variable is required. `.env.example` lists the defaults; copy it to `.env` to change one, for example `HTTP_PORT` or `DATA_OVERRIDE_PATH`.

### Migrations and seeding

The server applies pending migrations and imports the data folder by itself on every start, so neither command below is needed to run the application. They exist for working on the database separately.

| Command | Binary | Does |
| --- | --- | --- |
| `make migrate-up` | `cmd/migrate` | applies the migrations in `migrations/` |
| `make migrate-down` | `cmd/migrate -down 1` | rolls back the last step |
| `make seed` | `cmd/seed` | replaces the schema content with what the data folder describes |
| `make seed ARGS="-follow-up"` | `cmd/seed -follow-up` | the same, then imports `laporan_susulan.xlsx` |

The `make` targets run the binaries inside the backend image on the compose network, because the database port is not published to the host. With a PostgreSQL reachable from the host, `go run ./cmd/migrate` and `go run ./cmd/seed` work too, reading `DATABASE_URL`, `DATA_DIR` and `DATA_OVERRIDE_DIR` from the environment.

There is no separate seed data: the only data of this application is the five source files. A running server re-imports them on its next start, which undoes a `-follow-up` seed.

### API documentation

Every handler carries swaggo annotations; the specification is generated into `docs/` and compiled into the binary.

| Address | Shows |
| --- | --- |
| `/swagger/index.html` | Swagger UI, with "Try it out" |
| `/scalar` | Scalar API Reference (loads its script from a CDN, so it needs internet access) |
| `/swagger/doc.json` | The OpenAPI specification |

After changing an annotation or a DTO, run `make swag` and commit `docs/`. Those files are generated; do not edit them by hand. A unit test compares the registered routes with the specification, so a route without documentation fails the tests.

Set `DOCS_ENABLED=false` to turn the documentation pages off.

### Without Docker

Needs a PostgreSQL with PostGIS reachable through `DATABASE_URL`.

```bash
cp .env.example .env
make run
```

### Commands

| Command | Does |
| --- | --- |
| `make up` | Build and start the stack in the background (`docker compose up -d --build --wait`) |
| `make down` | Stop and remove the containers |
| `make logs` | Follow the backend log |
| `make db-up` | Start only PostgreSQL/PostGIS |
| `make migrate-up` | Apply every pending migration |
| `make migrate-down` | Roll back the last migration (drops the imported data) |
| `make seed` | Fill the database from the data folder; `make seed ARGS="-follow-up"` also imports the follow-up file |
| `make swag` | Regenerate the OpenAPI documentation into `docs/` |
| `make test` | Unit tests (no database needed) |
| `make test-race` | Unit tests under the race detector |
| `make test-integration` | Integration tests against PostGIS (needs `TEST_DATABASE_URL`) |
| `make vet` | Static checks |
| `make fmt` | Format the code |
| `make build` | Build the binary into `bin/` |
| `make docker-build` | Build the container image |

### Tests

| Kind | Build tag | Covers |
| --- | --- | --- |
| Unit | (none) | normalisation, projection, network graph, cleaning rules, services with a fake repository, the API contract through the router |
| Integration | `integration` | migrations up → down → up, polygon repair in PostGIS, latest-condition and duplicate rules in SQL, the sample data end to end, follow-up import repeated |

The integration tests own the `irrigation` schema of the database they are given and rewrite it:

```bash
docker run -d --name irrigation-itest -e POSTGRES_USER=test -e POSTGRES_PASSWORD=test -e POSTGRES_DB=test \n  -p 127.0.0.1:55432:5432 postgis/postgis:16-3.4
TEST_DATABASE_URL="postgres://test:test@127.0.0.1:55432/test?sslmode=disable" make test-integration
docker rm -f irrigation-itest
```

## Layout

```text
cmd/server/            wiring: startup, router, readiness gate
cmd/migrate/           apply or roll back migrations
cmd/seed/              fill the database from the data folder
internal/features/<f>/ one feature = one package
internal/pkg/          cross-feature code (no feature logic)
  network/             canal graph and flow direction from the weir
  datafile/            CSV, Excel and GeoJSON readers
  geo/ normalize/      coordinate conversion, spelling normalization
  config/ database/ ...  configuration, connection, errors, middleware
migrations/            ordered SQL, embedded in the binary
docs/                  generated OpenAPI specification (swaggo)
```

Files inside a feature follow `<f>.{domain,dto,service,repository,handler,route}.go`.

| Layer | Responsibility |
| --- | --- |
| `domain` | types, pure rules, repository interface, error codes |
| `dto` | request and response types, conversions to the contract shape |
| `handler` | bind the request, call the service, write the response |
| `service` | orchestration and rules; no SQL, no `gin.Context` |
| `repository` | SQL and transactions |

| Feature | Serves |
| --- | --- |
| `ingest` | import, cleaning rules, impact analysis, follow-up import. The only feature that writes to the database |
| `asset` | structures, report history, downstream impact, maintenance priority |
| `canal` | canal length recap, canal map layer |
| `plot` | tertiary plot map layer |
| `quality` | data-quality summary and findings |
| `health` | import status |

Features never import each other. A feature that needs another feature's data reads it through SQL; `cmd/server` is the only place that knows every feature.

## Conventions

- Identifiers, comments and logs are in English.
- JSON property names, condition values (`rusak_ringan`, ...) and finding categories keep the exact Indonesian names fixed by the assignment.
- Text shown to maintenance staff (finding descriptions, error messages) is in Indonesian.
- Success responses are not wrapped in an envelope, because the assignment fixes each response shape. Errors use `{"error": {"code", "message", "request_id"}}`.
- Configuration is read only in `internal/pkg/config`.

## Error handling

Every error answers the same envelope, with a stable `code` for clients and a `request_id` that also appears in the server log:

```json
{"error": {"code": "ASSET_NOT_FOUND", "message": "bangunan tidak ditemukan", "request_id": "0192b3a4-..."}}
```

| Status | Code | When |
| --- | --- | --- |
| 400 | `ASSET_INVALID_KIND`, `ASSET_INVALID_CONDITION` | `jenis` or `kondisi` is not a known value. An unknown value is rejected instead of answering an empty list |
| 400 | `ASSET_INVALID_LIMIT` | `limit` is not a positive integer. There is no upper bound: a large limit returns every row |
| 404 | `ASSET_NOT_FOUND` | no structure with that `kode_aset` |
| 404 | `ROUTE_NOT_FOUND` | unknown address |
| 404 | `FOLLOW_UP_FILE_NOT_FOUND` | `laporan_susulan.xlsx` is not in the data folder |
| 405 | `METHOD_NOT_ALLOWED` | known address, wrong method (e.g. `GET /api/impor/susulan`) |
| 422 | `FOLLOW_UP_FILE_UNREADABLE` | the follow-up file exists but is not a readable workbook |
| 400 | `UPLOAD_FILE_MISSING` | an upload without a file in the `berkas` field |
| 400 | `UPLOAD_FILE_TOO_LARGE` | an uploaded file larger than 10 MB |
| 422 | `UPLOAD_FILE_UNREADABLE` | the uploaded file is not a readable workbook |
| 422 | `UPLOAD_FILE_NOT_REPORTS` | the uploaded workbook has no `id_laporan` column |

`POST /api/impor/susulan` reads the data folder when the request has no body, as the contract describes. As an addition outside the contract, a `multipart/form-data` request with an `.xlsx` in the field `berkas` imports that file instead, with the same cleaning rules and the same response.
| 503 | `IMPORT_IN_PROGRESS` | the initial import has not finished |
| 503 | `IMPORT_FAILED` | the initial import failed; `GET /api/health` answers `gagal` |
| 503 | `DATABASE_UNAVAILABLE` | PostgreSQL cannot be reached; retry later |
| 504 | `REQUEST_TIMEOUT` | the request exceeded `REQUEST_TIMEOUT` (default 60s) |
| 500 | `INTERNAL_ERROR` | anything unexpected. The cause goes to the log only, never to the client |

During the import:

- A source file that is missing or cannot be parsed is skipped and recorded as a finding (`berkas_tidak_terbaca`); the other files are still imported and the status still becomes `siap`.
- A row or feature that cannot be used (no identifier, unreadable geometry, invalid date) is dropped or stored without that value, and recorded as a finding. It never stops the import.
- A panic inside the import, a file reader or a polygon worker is recovered. It costs that item, or marks the import as failed, but does not take the server down.

## How the import works

The import runs on its own goroutine, so the HTTP server answers `GET /api/health` with `memuat` from the first second. Inside it:

- The four files are read and parsed concurrently; they do not depend on each other.
- Plot polygons are validated in PostGIS by a bounded set of workers (`repairWorkers`); each plot is an independent query.
- The cleaning rules then run in a fixed order on one goroutine, because each step needs the previous one and because "keep the first of a duplicate" depends on file order. Results of the concurrent steps are stored by index, so the outcome never depends on which goroutine finishes first.

The steps:

1. **Canals** — repair coordinates, measure length on EPSG:32749, drop segments whose geometry repeats (in either drawing direction).
2. **Assets** — normalise attributes, keep the newest row of a repeated `kode_aset`, then flag each location by its distance to the nearest canal.
3. **Plots** — repair coordinates, close open rings, let PostGIS validate the polygon and measure its area, then settle the unit and plausibility of `luas_baku`.
4. **Analysis** — build the canal graph, direct it away from the weir, place each structure on it, and collect the plots downstream of each structure.
5. **Reports** — normalise, then match to a structure by nomenclature or by distance (30 m).
6. **Recompute** — duplicate reports, latest condition of every asset, and the findings that depend on reports are derived again from the tables.

Step 6 also runs after a follow-up import. Because it depends only on the table content, importing the same follow-up file again changes nothing.

The coordinate system and the axis order are not stated per row. They are inferred from value ranges: a value above 180 can only be metres, and a latitude never exceeds 90.

## Compliance with the assignment

Checked against the running service with the sample data.

### Part A — minimum requirements

| # | Requirement | Status |
| --- | --- | --- |
| 1 | `docker compose up --build` from the repo root runs everything | Done. The `docker-compose.yml` at the repo root includes this project and `../frontend`; both stay separate Compose projects that can also be started from their own folder |
| 2 | UI on `http://localhost:8080`, API on `/api` | Done once both projects run, through the gateway in `../frontend` |
| 3 | Reads `./data` on start and imports automatically | Done. `./data` at the repo root is imported when it holds source files; otherwise the copy in `backend/data` is |
| 4 | `GET /api/health`: `memuat`, then `siap` within 10 minutes | Done (a few seconds) |
| 5 | `README.md` with run steps, assumptions, data problems, unfinished parts | Done (repo root) |
| 6 | `.env.example`; runs with its defaults | Done (`backend/.env.example`) |

### Part B — integration and cleaning

| Rule | Status |
| --- | --- |
| Nomenclature to `B.KW.1` / `B.KW.6A` | Done |
| Condition to four canonical values | Done |
| Dates to `YYYY-MM-DD`, ambiguous read day first | Done |
| Decimal comma | Done |
| UTM 49S to WGS84 | Done |
| Swapped latitude/longitude | Done |
| Empty, `0,0`, or more than 2 km from a canal: no geometry, `koordinat_tidak_valid` | Done |
| More than 50 m from any canal: `jauh_dari_saluran`, left out of the impact analysis | Done |
| Duplicate asset: newest `tgl_update` | Done |
| Duplicate canal segment (same geometry) | Done |
| Small gap between segment ends | Done, with 10 m (part D) instead of 5 m (part B) |
| Invalid plot polygon repaired | Done |
| Duplicate plot | Done |
| `luas_baku` in m² converted to hectares | Done |
| Area differing more than 20%: geometry area, `luas_tidak_wajar` | Done |
| Duplicate report (asset, date, condition) | Done |
| Report matching: nomenclature, then nearest within 30 m, else `laporan_tanpa_aset` | Done |
| Latest condition: newest of inventory and active reports, report wins a tie | Done |
| Geometry stored and served in WGS84; length and area on EPSG:32749 | Done |
| Every action recorded | Done (`/api/kualitas-data/temuan`) |

### Part C — API contract

| Endpoint | Status |
| --- | --- |
| `GET /api/health` | Done |
| `GET /api/aset` with `jenis`, `kondisi`, `saluran` | Done |
| `GET /api/aset/{kode_aset}` with `riwayat` (cancelled reports included) | Done |
| `GET /api/aset/{kode_aset}/dampak` | Done |
| `GET /api/rekap/saluran` | Done |
| `GET /api/prioritas` with `limit` (default 20) | Done |
| `GET /api/kualitas-data` with the eleven required categories | Done |
| `POST /api/impor/susulan` | Done |

### Part D — downstream impact and priority

| Rule | Status |
| --- | --- |
| Position = nearest point on a canal, within 50 m | Done |
| Segment ends within 10 m are connected | Done |
| Flow direction from the weir, not from the drawing direction | Done |
| Plot served by `nomenklatur_sadap`, else nearest offtake within 200 m, else `tanpa_sadap` | Done |
| Impacted plots include those tapped at the structure itself | Done |
| Impacted area in hectares, two decimals | Done |
| Score = condition weight × impacted area; only above 0; ties by `kode_aset` | Done |
| Length recap per level and condition, kilometres, two decimals | Done |

The worked example of the assignment (B.KW.2 → 150 ha) is reproduced as a unit test.

### Part E — interface

Not built.

### Part F — follow-up data

| Rule | Status |
| --- | --- |
| Imported only on `POST /api/impor/susulan` | Done |
| New, updated and cancelled reports | Done |
| Part B cleaning applies | Done |
| Latest condition and priorities reflect the new data | Done |
| Safe to repeat, no duplicated data | Done |

With the sample data the first call answers `14 / 6 / 3 / 2` (added, updated, cancelled, unchanged) and every later call answers `0 / 0 / 0 / 25`.

### Open points

1. **Interface (part E)** lives in `../frontend`. When the backend runs on its own, `http://localhost:8080/` answers a small JSON index of the API instead of a web page.
2. **Answer key.** The numbers were checked for internal consistency and against PostGIS, not against the checker's answer key, which is not available.
