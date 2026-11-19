CREATE EXTENSION IF NOT EXISTS postgis;

CREATE SCHEMA irrigation;

-- Canal segments after duplicates are removed. Geometry is WGS84; length is
-- measured on EPSG:32749 at import time.
CREATE TABLE irrigation.canal_segment (
    id           integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    segment_code text             NOT NULL,
    canal_name   text             NOT NULL,
    level        text,
    condition    text CHECK (condition IN ('baik', 'rusak_ringan', 'rusak_sedang', 'rusak_berat')),
    length_m     double precision NOT NULL CHECK (length_m >= 0),
    geom         geometry(Geometry, 4326) NOT NULL
);

-- Irrigation structures. inventory_* is what the inventory file says;
-- condition/condition_date is the latest state derived from inventory and reports.
CREATE TABLE irrigation.asset (
    code                text PRIMARY KEY,
    nomenclature        text,
    kind                text NOT NULL CHECK (kind IN
        ('bendung', 'bangunan_bagi', 'bangunan_sadap', 'bangunan_bagi_sadap', 'bangunan_pelengkap')),
    canal_name          text,
    built_year          integer,
    inventory_condition text CHECK (inventory_condition IN ('baik', 'rusak_ringan', 'rusak_sedang', 'rusak_berat')),
    inventory_date      date,
    condition           text CHECK (condition IN ('baik', 'rusak_ringan', 'rusak_sedang', 'rusak_berat')),
    condition_date      date,
    geom                geometry(Point, 4326),
    canal_distance_m    double precision,
    flags               text[]           NOT NULL DEFAULT '{}',
    impacted_area_ha    double precision NOT NULL DEFAULT 0 CHECK (impacted_area_ha >= 0)
);
CREATE INDEX asset_nomenclature_idx ON irrigation.asset (nomenclature);

-- Tertiary plots. area_ha is the area used in the analysis: the attribute
-- when it is plausible, otherwise the geometry area.
CREATE TABLE irrigation.plot (
    code                 text PRIMARY KEY,
    name                 text,
    attribute_area_ha    double precision,
    geometry_area_ha     double precision,
    area_ha              double precision NOT NULL CHECK (area_ha >= 0),
    offtake_nomenclature text,
    offtake_asset_code   text REFERENCES irrigation.asset (code),
    flags                text[] NOT NULL DEFAULT '{}',
    geom                 geometry(MultiPolygon, 4326)
);

-- Plots that lose water when an asset fails (downstream impact).
CREATE TABLE irrigation.impact (
    asset_code text NOT NULL REFERENCES irrigation.asset (code) ON DELETE CASCADE,
    plot_code  text NOT NULL REFERENCES irrigation.plot (code) ON DELETE CASCADE,
    PRIMARY KEY (asset_code, plot_code)
);

-- Field reports. Duplicates are kept but flagged, so that re-importing a
-- file always converges to the same state.
CREATE TABLE irrigation.report (
    id               text PRIMARY KEY,
    reported_on      date,
    officer          text,
    nomenclature     text,
    raw_nomenclature text,
    damage_type      text,
    condition        text CHECK (condition IN ('baik', 'rusak_ringan', 'rusak_sedang', 'rusak_berat')),
    lat              double precision,
    lon              double precision,
    status           text    NOT NULL CHECK (status IN ('aktif', 'batal')),
    note             text,
    asset_code       text REFERENCES irrigation.asset (code),
    is_duplicate     boolean NOT NULL DEFAULT false,
    source_file      text    NOT NULL
);
CREATE INDEX report_asset_idx ON irrigation.report (asset_code);

-- One row per cleaning action taken on a source row or feature.
CREATE TABLE irrigation.finding (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_file text NOT NULL,
    row_ref     text NOT NULL,
    category    text NOT NULL,
    problem     text NOT NULL,
    action      text NOT NULL
);
CREATE INDEX finding_category_idx ON irrigation.finding (category);
