-- ============================================================
-- TimescaleDB Retention & Downsampling Policies
-- Executed automatically after init.sql by init-container
-- ============================================================

-- ============================================================
-- 1. RETENTION POLICIES (DROP OLD CHUNKS)
-- ============================================================

SELECT add_retention_policy(
    'acoustic_measurements',
    INTERVAL '90 days',
    if_not_exists => TRUE
);

SELECT add_retention_policy(
    'grinding_operations',
    INTERVAL '365 days',
    if_not_exists => TRUE
);

SELECT add_retention_policy(
    'alert_events',
    INTERVAL '180 days',
    if_not_exists => TRUE
);

-- ============================================================
-- 2. CONTINUOUS AGGREGATES (DOWNsampling MATERIALIZED VIEWS)
-- ============================================================

-- 1-minute acoustic measurement aggregates
CREATE MATERIALIZED VIEW IF NOT EXISTS acoustic_measurements_1m
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 minute', ts) AS bucket,
    bell_id,
    mode_order,
    COUNT(*) AS sample_count,
    AVG(frequency_hz) AS avg_frequency,
    MIN(frequency_hz) AS min_frequency,
    MAX(frequency_hz) AS max_frequency,
    STDDEV(frequency_hz) AS std_frequency,
    AVG(amplitude_db) AS avg_amplitude,
    AVG(deviation_cents) AS avg_deviation,
    AVG(temperature_c) AS avg_temperature,
    AVG(humidity_pct) AS avg_humidity
FROM acoustic_measurements
GROUP BY bucket, bell_id, mode_order
WITH NO DATA;

-- 1-hour acoustic measurement aggregates
CREATE MATERIALIZED VIEW IF NOT EXISTS acoustic_measurements_1h
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 hour', ts) AS bucket,
    bell_id,
    mode_order,
    COUNT(*) AS sample_count,
    AVG(avg_frequency) AS avg_frequency,
    MIN(min_frequency) AS min_frequency,
    MAX(max_frequency) AS max_frequency,
    AVG(std_frequency) AS std_frequency,
    AVG(avg_amplitude) AS avg_amplitude,
    AVG(avg_deviation) AS avg_deviation
FROM acoustic_measurements_1m
GROUP BY bucket, bell_id, mode_order
WITH NO DATA;

-- Daily grinding operation aggregates
CREATE MATERIALIZED VIEW IF NOT EXISTS grinding_daily_summary
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 day', ts) AS bucket,
    bell_id,
    operator_id,
    COUNT(*) AS operation_count,
    SUM(depth_mm) AS total_depth_mm,
    SUM(removed_mass_g) AS total_mass_g,
    AVG(predicted_frequency_shift_hz) AS avg_predicted_shift
FROM grinding_operations
GROUP BY bucket, bell_id, operator_id
WITH NO DATA;

-- Alert events per-hour summary
CREATE MATERIALIZED VIEW IF NOT EXISTS alert_hourly_summary
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 hour', ts) AS bucket,
    bell_id,
    alert_type,
    severity,
    COUNT(*) AS alert_count,
    COUNT(CASE WHEN acknowledged THEN 1 END) AS acknowledged_count
FROM alert_events
GROUP BY bucket, bell_id, alert_type, severity
WITH NO DATA;

-- ============================================================
-- 3. REFRESH POLICIES
-- ============================================================

SELECT add_continuous_aggregate_policy(
    'acoustic_measurements_1m',
    start_offset => INTERVAL '15 minutes',
    end_offset => INTERVAL '1 minute',
    schedule_interval => INTERVAL '1 minute',
    if_not_exists => TRUE
);

SELECT add_continuous_aggregate_policy(
    'acoustic_measurements_1h',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '30 minutes',
    schedule_interval => INTERVAL '30 minutes',
    if_not_exists => TRUE
);

SELECT add_continuous_aggregate_policy(
    'grinding_daily_summary',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

SELECT add_continuous_aggregate_policy(
    'alert_hourly_summary',
    start_offset => INTERVAL '4 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE
);

-- ============================================================
-- 4. RETENTION FOR CONTINUOUS AGGREGATES
-- ============================================================

SELECT add_retention_policy(
    'acoustic_measurements_1m',
    INTERVAL '14 days',
    if_not_exists => TRUE
);

SELECT add_retention_policy(
    'acoustic_measurements_1h',
    INTERVAL '1 year',
    if_not_exists => TRUE
);

SELECT add_retention_policy(
    'grinding_daily_summary',
    INTERVAL '3 years',
    if_not_exists => TRUE
);

SELECT add_retention_policy(
    'alert_hourly_summary',
    INTERVAL '1 year',
    if_not_exists => TRUE
);

-- ============================================================
-- 5. COMPRESSION POLICIES (for older chunks)
-- ============================================================

ALTER TABLE acoustic_measurements SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'bell_id, mode_order',
    timescaledb.compress_orderby = 'ts DESC'
);

ALTER TABLE grinding_operations SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'bell_id, operator_id',
    timescaledb.compress_orderby = 'ts DESC'
);

ALTER TABLE alert_events SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'bell_id, alert_type, severity',
    timescaledb.compress_orderby = 'ts DESC'
);

SELECT add_compression_policy(
    'acoustic_measurements',
    INTERVAL '7 days',
    if_not_exists => TRUE
);

SELECT add_compression_policy(
    'grinding_operations',
    INTERVAL '30 days',
    if_not_exists => TRUE
);

SELECT add_compression_policy(
    'alert_events',
    INTERVAL '30 days',
    if_not_exists => TRUE
);
