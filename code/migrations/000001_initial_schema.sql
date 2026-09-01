-- Initial schema migration for the waste-routing PostgreSQL/PostGIS model.
-- Application integration and seed/import logic are intentionally out of scope.

BEGIN;

CREATE EXTENSION IF NOT EXISTS postgis;

CREATE SCHEMA IF NOT EXISTS waste_routing;
SET LOCAL search_path TO waste_routing, public;

CREATE TABLE waste_sources (
    id                      text PRIMARY KEY,
    service_zone            text NOT NULL,
    address                 text NOT NULL,
    district                text NOT NULL,
    waste_type              text NOT NULL,
    container_type          text NOT NULL,
    container_volume_m3     double precision NOT NULL,
    container_count         integer NOT NULL,
    waste_amount            double precision NOT NULL,
    pickups_warm_period     double precision,
    pickups_cold_period     double precision,
    geom                    geometry(Point, 4326) NOT NULL,

    CONSTRAINT waste_sources_id_not_blank
        CHECK (btrim(id) <> ''),
    CONSTRAINT waste_sources_address_not_blank
        CHECK (btrim(address) <> ''),
    CONSTRAINT waste_sources_container_volume_positive
        CHECK (
            container_volume_m3 > 0
            AND container_volume_m3 < 'Infinity'::double precision
        ),
    CONSTRAINT waste_sources_container_count_positive
        CHECK (container_count > 0),
    CONSTRAINT waste_sources_waste_amount_positive
        CHECK (
            waste_amount > 0
            AND waste_amount < 'Infinity'::double precision
        ),
    CONSTRAINT waste_sources_pickups_nonnegative
        CHECK (
            (
                pickups_warm_period IS NULL
                OR (
                    pickups_warm_period >= 0
                    AND pickups_warm_period < 'Infinity'::double precision
                )
            )
            AND (
                pickups_cold_period IS NULL
                OR (
                    pickups_cold_period >= 0
                    AND pickups_cold_period < 'Infinity'::double precision
                )
            )
            AND num_nonnulls(
                pickups_warm_period,
                pickups_cold_period
            ) >= 1
            AND COALESCE(pickups_warm_period, 0)
                + COALESCE(pickups_cold_period, 0) > 0
        ),
    CONSTRAINT waste_sources_geom_not_empty
        CHECK (NOT ST_IsEmpty(geom)),
    CONSTRAINT waste_sources_geom_valid
        CHECK (ST_IsValid(geom)),
    CONSTRAINT waste_sources_geom_wgs84_bounds
        CHECK (
            ST_X(geom) BETWEEN -180 AND 180
            AND ST_Y(geom) BETWEEN -90 AND 90
        )
);

CREATE TABLE transfer_stations (
    id          text PRIMARY KEY,
    name        text NOT NULL,
    capacity    double precision NOT NULL,
    geom        geometry(Point, 4326) NOT NULL,

    CONSTRAINT transfer_stations_id_not_blank
        CHECK (btrim(id) <> ''),
    CONSTRAINT transfer_stations_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT transfer_stations_capacity_positive
        CHECK (capacity > 0 AND capacity < 'Infinity'::double precision),
    CONSTRAINT transfer_stations_geom_not_empty
        CHECK (NOT ST_IsEmpty(geom)),
    CONSTRAINT transfer_stations_geom_valid
        CHECK (ST_IsValid(geom)),
    CONSTRAINT transfer_stations_geom_wgs84_bounds
        CHECK (
            ST_X(geom) BETWEEN -180 AND 180
            AND ST_Y(geom) BETWEEN -90 AND 90
        )
);

CREATE TABLE plants (
    id          text PRIMARY KEY,
    name        text NOT NULL,
    capacity    double precision NOT NULL,
    geom        geometry(Point, 4326) NOT NULL,

    CONSTRAINT plants_id_not_blank
        CHECK (btrim(id) <> ''),
    CONSTRAINT plants_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT plants_capacity_positive
        CHECK (capacity > 0 AND capacity < 'Infinity'::double precision),
    CONSTRAINT plants_geom_not_empty
        CHECK (NOT ST_IsEmpty(geom)),
    CONSTRAINT plants_geom_valid
        CHECK (ST_IsValid(geom)),
    CONSTRAINT plants_geom_wgs84_bounds
        CHECK (
            ST_X(geom) BETWEEN -180 AND 180
            AND ST_Y(geom) BETWEEN -90 AND 90
        )
);

CREATE TABLE route_costs (
    route_cost_id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    -- Typed nullable foreign keys preserve referential integrity for the
    -- three allowed route directions without a weak polymorphic foreign key.
    source_waste_source_id         text,
    source_transfer_station_id     text,
    target_transfer_station_id     text,
    target_plant_id                text,

    source_type text GENERATED ALWAYS AS (
        CASE
            WHEN source_waste_source_id IS NOT NULL THEN 'waste_source'
            WHEN source_transfer_station_id IS NOT NULL THEN 'transfer_station'
        END
    ) STORED,
    target_type text GENERATED ALWAYS AS (
        CASE
            WHEN target_transfer_station_id IS NOT NULL THEN 'transfer_station'
            WHEN target_plant_id IS NOT NULL THEN 'plant'
        END
    ) STORED,

    costing_model          text NOT NULL DEFAULT 'auto',
    distance_km            double precision,
    travel_time_seconds    double precision,
    reachable              boolean NOT NULL,
    status                 text NOT NULL,
    error_message          text,
    calculated_at          timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT route_costs_source_waste_source_fk
        FOREIGN KEY (source_waste_source_id)
        REFERENCES waste_sources (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT route_costs_source_transfer_station_fk
        FOREIGN KEY (source_transfer_station_id)
        REFERENCES transfer_stations (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT route_costs_target_transfer_station_fk
        FOREIGN KEY (target_transfer_station_id)
        REFERENCES transfer_stations (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT route_costs_target_plant_fk
        FOREIGN KEY (target_plant_id)
        REFERENCES plants (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,

    CONSTRAINT route_costs_allowed_direction
        CHECK (
            (
                source_waste_source_id IS NOT NULL
                AND source_transfer_station_id IS NULL
                AND target_transfer_station_id IS NOT NULL
                AND target_plant_id IS NULL
            )
            OR
            (
                source_waste_source_id IS NOT NULL
                AND source_transfer_station_id IS NULL
                AND target_transfer_station_id IS NULL
                AND target_plant_id IS NOT NULL
            )
            OR
            (
                source_waste_source_id IS NULL
                AND source_transfer_station_id IS NOT NULL
                AND target_transfer_station_id IS NULL
                AND target_plant_id IS NOT NULL
            )
        ),
    CONSTRAINT route_costs_costing_model_not_blank
        CHECK (btrim(costing_model) <> ''),
    CONSTRAINT route_costs_status_allowed
        CHECK (status IN ('ok', 'unreachable', 'error')),
    CONSTRAINT route_costs_distance_nonnegative
        CHECK (
            distance_km IS NULL
            OR (
                distance_km >= 0
                AND distance_km < 'Infinity'::double precision
            )
        ),
    CONSTRAINT route_costs_travel_time_nonnegative
        CHECK (
            travel_time_seconds IS NULL
            OR (
                travel_time_seconds >= 0
                AND travel_time_seconds < 'Infinity'::double precision
            )
        ),
    CONSTRAINT route_costs_reachability_consistent
        CHECK (
            (
                reachable
                AND status = 'ok'
                AND distance_km IS NOT NULL
                AND travel_time_seconds IS NOT NULL
            )
            OR
            (
                NOT reachable
                AND status IN ('unreachable', 'error')
                AND distance_km IS NULL
                AND travel_time_seconds IS NULL
            )
        ),
    CONSTRAINT route_costs_error_message_present
        CHECK (
            status <> 'error'
            OR (error_message IS NOT NULL AND btrim(error_message) <> '')
        )
);

CREATE TABLE calculation_runs (
    run_id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    description             text,
    status                  text NOT NULL DEFAULT 'pending',
    parameters              jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at              timestamptz NOT NULL DEFAULT now(),
    started_at              timestamptz,
    finished_at             timestamptz,
    objective_value         double precision,
    solver_exit_flag        integer,
    solve_time_seconds      double precision,
    solver_metadata         jsonb NOT NULL DEFAULT '{}'::jsonb,
    error_message           text,

    CONSTRAINT calculation_runs_description_not_blank
        CHECK (description IS NULL OR btrim(description) <> ''),
    CONSTRAINT calculation_runs_status_allowed
        CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    CONSTRAINT calculation_runs_parameters_object
        CHECK (jsonb_typeof(parameters) = 'object'),
    CONSTRAINT calculation_runs_solver_metadata_object
        CHECK (jsonb_typeof(solver_metadata) = 'object'),
    CONSTRAINT calculation_runs_objective_nonnegative
        CHECK (
            objective_value IS NULL
            OR (
                objective_value >= 0
                AND objective_value < 'Infinity'::double precision
            )
        ),
    CONSTRAINT calculation_runs_solve_time_nonnegative
        CHECK (
            solve_time_seconds IS NULL
            OR (
                solve_time_seconds >= 0
                AND solve_time_seconds < 'Infinity'::double precision
            )
        ),
    CONSTRAINT calculation_runs_time_order
        CHECK (
            (started_at IS NULL OR started_at >= created_at)
            AND (
                finished_at IS NULL
                OR finished_at >= COALESCE(started_at, created_at)
            )
        ),
    CONSTRAINT calculation_runs_lifecycle_consistent
        CHECK (
            (
                status = 'pending'
                AND started_at IS NULL
                AND finished_at IS NULL
            )
            OR
            (
                status = 'running'
                AND started_at IS NOT NULL
                AND finished_at IS NULL
            )
            OR
            (
                status = 'succeeded'
                AND started_at IS NOT NULL
                AND finished_at IS NOT NULL
                AND objective_value IS NOT NULL
                AND error_message IS NULL
            )
            OR
            (
                status = 'failed'
                AND started_at IS NOT NULL
                AND finished_at IS NOT NULL
                AND error_message IS NOT NULL
                AND btrim(error_message) <> ''
            )
        )
);

CREATE TABLE source_assignments (
    run_id                              bigint NOT NULL,
    waste_source_id                     text NOT NULL,
    destination_transfer_station_id     text,
    destination_plant_id                text,
    assignment_type text GENERATED ALWAYS AS (
        CASE
            WHEN destination_transfer_station_id IS NOT NULL THEN 'transfer_station'
            WHEN destination_plant_id IS NOT NULL THEN 'plant'
        END
    ) STORED,
    waste_amount                         double precision NOT NULL,
    distance_km                          double precision NOT NULL,
    transport_cost double precision GENERATED ALWAYS AS (
        waste_amount * distance_km
    ) STORED,

    CONSTRAINT source_assignments_pk
        PRIMARY KEY (run_id, waste_source_id),
    CONSTRAINT source_assignments_run_fk
        FOREIGN KEY (run_id)
        REFERENCES calculation_runs (run_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT source_assignments_waste_source_fk
        FOREIGN KEY (waste_source_id)
        REFERENCES waste_sources (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT source_assignments_transfer_station_fk
        FOREIGN KEY (destination_transfer_station_id)
        REFERENCES transfer_stations (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT source_assignments_plant_fk
        FOREIGN KEY (destination_plant_id)
        REFERENCES plants (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,

    CONSTRAINT source_assignments_exactly_one_destination
        CHECK (
            num_nonnulls(
                destination_transfer_station_id,
                destination_plant_id
            ) = 1
        ),
    CONSTRAINT source_assignments_waste_amount_positive
        CHECK (
            waste_amount > 0
            AND waste_amount < 'Infinity'::double precision
        ),
    CONSTRAINT source_assignments_distance_nonnegative
        CHECK (
            distance_km >= 0
            AND distance_km < 'Infinity'::double precision
        ),
    CONSTRAINT source_assignments_transport_cost_finite
        CHECK (
            transport_cost >= 0
            AND transport_cost < 'Infinity'::double precision
        )
);

CREATE TABLE transfer_flows (
    run_id                  bigint NOT NULL,
    transfer_station_id     text NOT NULL,
    plant_id                text NOT NULL,
    waste_amount            double precision NOT NULL,
    distance_km             double precision NOT NULL,
    transport_cost double precision GENERATED ALWAYS AS (
        waste_amount * distance_km
    ) STORED,

    CONSTRAINT transfer_flows_pk
        PRIMARY KEY (run_id, transfer_station_id, plant_id),
    CONSTRAINT transfer_flows_run_fk
        FOREIGN KEY (run_id)
        REFERENCES calculation_runs (run_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT transfer_flows_transfer_station_fk
        FOREIGN KEY (transfer_station_id)
        REFERENCES transfer_stations (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT transfer_flows_plant_fk
        FOREIGN KEY (plant_id)
        REFERENCES plants (id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT transfer_flows_waste_amount_positive
        CHECK (
            waste_amount > 0
            AND waste_amount < 'Infinity'::double precision
        ),
    CONSTRAINT transfer_flows_distance_nonnegative
        CHECK (
            distance_km >= 0
            AND distance_km < 'Infinity'::double precision
        ),
    CONSTRAINT transfer_flows_transport_cost_finite
        CHECK (
            transport_cost >= 0
            AND transport_cost < 'Infinity'::double precision
        )
);

-- Spatial indexes used by QGIS and geographic filtering.
CREATE INDEX waste_sources_geom_gix
    ON waste_sources USING gist (geom);
CREATE INDEX transfer_stations_geom_gix
    ON transfer_stations USING gist (geom);
CREATE INDEX plants_geom_gix
    ON plants USING gist (geom);

-- Common source filters for the largest input table.
CREATE INDEX waste_sources_service_zone_district_idx
    ON waste_sources (service_zone, district);

-- Each partial unique index is both the route cache key and a lookup index
-- for one of the three route matrix directions.
CREATE UNIQUE INDEX route_costs_source_to_transfer_uq
    ON route_costs (
        source_waste_source_id,
        target_transfer_station_id,
        costing_model
    )
    WHERE source_waste_source_id IS NOT NULL
      AND target_transfer_station_id IS NOT NULL;

CREATE UNIQUE INDEX route_costs_source_to_plant_uq
    ON route_costs (
        source_waste_source_id,
        target_plant_id,
        costing_model
    )
    WHERE source_waste_source_id IS NOT NULL
      AND target_plant_id IS NOT NULL;

CREATE UNIQUE INDEX route_costs_transfer_to_plant_uq
    ON route_costs (
        source_transfer_station_id,
        target_plant_id,
        costing_model
    )
    WHERE source_transfer_station_id IS NOT NULL
      AND target_plant_id IS NOT NULL;

CREATE INDEX route_costs_matrix_lookup_idx
    ON route_costs (source_type, target_type, costing_model, reachable);
CREATE INDEX route_costs_target_transfer_station_idx
    ON route_costs (target_transfer_station_id)
    WHERE target_transfer_station_id IS NOT NULL;
CREATE INDEX route_costs_target_plant_idx
    ON route_costs (target_plant_id)
    WHERE target_plant_id IS NOT NULL;

CREATE INDEX calculation_runs_status_created_idx
    ON calculation_runs (status, created_at DESC);

-- The primary keys already optimize queries by run_id. Reverse-order
-- indexes below support FK checks and object-centric history queries.
CREATE INDEX source_assignments_waste_source_run_idx
    ON source_assignments (waste_source_id, run_id);
CREATE INDEX source_assignments_transfer_station_run_idx
    ON source_assignments (destination_transfer_station_id, run_id)
    WHERE destination_transfer_station_id IS NOT NULL;
CREATE INDEX source_assignments_plant_run_idx
    ON source_assignments (destination_plant_id, run_id)
    WHERE destination_plant_id IS NOT NULL;
CREATE INDEX source_assignments_run_type_idx
    ON source_assignments (run_id, assignment_type);

CREATE INDEX transfer_flows_transfer_station_run_idx
    ON transfer_flows (transfer_station_id, run_id);
CREATE INDEX transfer_flows_plant_run_idx
    ON transfer_flows (plant_id, run_id);

COMMENT ON TABLE waste_sources IS
    'Validated waste accumulation points used as optimization sources.';
COMMENT ON TABLE transfer_stations IS
    'Waste transfer stations with annual capacity in tonnes.';
COMMENT ON TABLE plants IS
    'Waste processing or disposal plants with annual capacity in tonnes.';
COMMENT ON TABLE route_costs IS
    'Valhalla distance, travel time and reachability for allowed object pairs.';
COMMENT ON TABLE calculation_runs IS
    'Lifecycle, parameters and solver summary for one optimization run.';
COMMENT ON TABLE source_assignments IS
    'One destination assignment per waste source and calculation run.';
COMMENT ON TABLE transfer_flows IS
    'Positive aggregate transfer-station-to-plant flow for one calculation run.';

COMMIT;
