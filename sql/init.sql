-- ============================================
-- 古代编钟调音磨锉声学仿真与音高修正系统
-- TimescaleDB 初始化脚本
-- ============================================

CREATE EXTENSION IF NOT EXISTS timescaledb;

-- 编钟基本信息表
CREATE TABLE IF NOT EXISTS bells (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    serial_number VARCHAR(50) UNIQUE NOT NULL,
    material VARCHAR(50) DEFAULT 'bronze',
    mass_kg DOUBLE PRECISION,
    height_cm DOUBLE PRECISION,
    diameter_cm DOUBLE PRECISION,
    thickness_mm DOUBLE PRECISION,
    target_frequency DOUBLE PRECISION NOT NULL,
    tolerance_cents DOUBLE PRECISION DEFAULT 5.0,
    max_grinding_depth_mm DOUBLE PRECISION DEFAULT 3.0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    description TEXT
);

-- 声学测量数据表 (时序超表)
CREATE TABLE IF NOT EXISTS acoustic_measurements (
    time TIMESTAMPTZ NOT NULL,
    bell_id INTEGER NOT NULL REFERENCES bells(id),
    fundamental_freq DOUBLE PRECISION NOT NULL,
    overtone_freqs DOUBLE PRECISION[] NOT NULL,
    overtone_amplitudes DOUBLE PRECISION[] NOT NULL,
    temperature DOUBLE PRECISION,
    humidity DOUBLE PRECISION,
    sensor_id VARCHAR(50),
    deviation_cents DOUBLE PRECISION
);

SELECT create_hypertable('acoustic_measurements', 'time', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_acoustic_measurements_bell_id ON acoustic_measurements(bell_id, time DESC);

-- 磨锉操作记录表
CREATE TABLE IF NOT EXISTS grinding_operations (
    id SERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    bell_id INTEGER NOT NULL REFERENCES bells(id),
    position_x DOUBLE PRECISION NOT NULL,
    position_y DOUBLE PRECISION NOT NULL,
    position_z DOUBLE PRECISION NOT NULL,
    grinding_depth_mm DOUBLE PRECISION NOT NULL,
    grinding_area DOUBLE PRECISION,
    operator_id VARCHAR(50),
    before_frequency DOUBLE PRECISION,
    after_frequency DOUBLE PRECISION,
    predicted_frequency DOUBLE PRECISION,
    notes TEXT
);

SELECT create_hypertable('grinding_operations', 'time', if_not_exists => TRUE);

-- 音高修正建议表
CREATE TABLE IF NOT EXISTS pitch_corrections (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    bell_id INTEGER NOT NULL REFERENCES bells(id),
    current_frequency DOUBLE PRECISION NOT NULL,
    target_frequency DOUBLE PRECISION NOT NULL,
    deviation_cents DOUBLE PRECISION NOT NULL,
    recommended_positions JSONB NOT NULL,
    estimated_result_freq DOUBLE PRECISION,
    iterations INTEGER,
    algorithm VARCHAR(50) DEFAULT 'gradient_descent',
    status VARCHAR(20) DEFAULT 'pending'
);

-- 告警事件表
CREATE TABLE IF NOT EXISTS alert_events (
    id SERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    bell_id INTEGER NOT NULL REFERENCES bells(id),
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    details JSONB,
    acknowledged BOOLEAN DEFAULT FALSE,
    mqtt_topic VARCHAR(100),
    mqtt_delivered BOOLEAN DEFAULT FALSE
);

SELECT create_hypertable('alert_events', 'time', if_not_exists => TRUE);

-- 仿真结果表
CREATE TABLE IF NOT EXISTS simulation_results (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    bell_id INTEGER NOT NULL REFERENCES bells(id),
    simulation_type VARCHAR(50) NOT NULL,
    parameters JSONB NOT NULL,
    eigenfrequencies DOUBLE PRECISION[],
    mode_shapes JSONB,
    stress_distribution JSONB,
    computation_time_ms INTEGER
);

-- 插入示例编钟数据 (曾侯乙编钟风格)
INSERT INTO bells (name, serial_number, material, mass_kg, height_cm, diameter_cm, thickness_mm, target_frequency, tolerance_cents, max_grinding_depth_mm, description) VALUES
('镈钟·低音C2', 'B-ZEY-001', 'bronze', 203.8, 152.3, 80.5, 28.5, 65.41, 5.0, 3.5, '曾侯乙编钟下层第一组'),
('甬钟·G2', 'B-ZEY-002', 'bronze', 119.3, 112.8, 58.2, 22.3, 98.00, 5.0, 3.0, '曾侯乙编钟中层第一组'),
('甬钟·C3', 'B-ZEY-003', 'bronze', 84.5, 87.6, 45.3, 18.7, 130.81, 5.0, 2.5, '曾侯乙编钟中层第二组'),
('甬钟·E3', 'B-ZEY-004', 'bronze', 62.1, 68.9, 38.1, 15.2, 164.81, 5.0, 2.5, '曾侯乙编钟中层第三组'),
('钮钟·G3', 'B-ZEY-005', 'bronze', 38.7, 52.3, 28.5, 12.1, 196.00, 5.0, 2.0, '曾侯乙编钟上层第一组'),
('钮钟·C4', 'B-ZEY-006', 'bronze', 24.3, 39.8, 21.2, 9.8, 261.63, 5.0, 2.0, '曾侯乙编钟上层第二组'),
('钮钟·E4', 'B-ZEY-007', 'bronze', 18.5, 32.1, 17.3, 7.6, 329.63, 5.0, 1.5, '曾侯乙编钟上层第三组'),
('钮钟·G4', 'B-ZEY-008', 'bronze', 12.8, 25.6, 13.8, 6.2, 392.00, 5.0, 1.5, '曾侯乙编钟上层第四组')
ON CONFLICT (serial_number) DO NOTHING;
