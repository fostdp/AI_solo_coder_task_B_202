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

-- ============================================
-- 调音工艺表
-- ============================================
CREATE TABLE IF NOT EXISTS tuning_processes (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    process_type VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    harmonicity_factor DOUBLE PRECISION DEFAULT 1.0,
    frequency_shift_factor DOUBLE PRECISION DEFAULT 1.0,
    reversibility BOOLEAN DEFAULT FALSE,
    complexity INTEGER DEFAULT 1,
    historical_era VARCHAR(50) DEFAULT 'Spring and Autumn',
    advantages JSONB,
    disadvantages JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO tuning_processes (name, process_type, description, harmonicity_factor, frequency_shift_factor, reversibility, complexity, historical_era, advantages, disadvantages) VALUES
('磨锉调音', 'grinding', '春秋战国时期主流调音工艺，通过磨锉钟壁内侧调整音高。磨锉使钟壁变薄，频率升高。', 1.0, 1.12, false, 2, 'Spring and Autumn',
    '["工艺简单，成本低", "频率升高效果明显", "历史悠久，经验丰富", "对钟体结构影响小"]',
    '["不可逆，磨错无法恢复", "可能过度磨锉导致钟体过薄", "只能升高频率，无法降低", "需要多次反复测量"]'),
('铸镶调音', 'casting_inlay', '在钟壁特定位置镶嵌不同材质（如铅块）增加局部质量，降低音高。', 0.88, -0.08, true, 4, 'Warring States',
    '["可逆，可通过去除镶块恢复", "可精确微调频率", "能降低频率，补充磨锉的不足", "不损坏钟体结构"]',
    '["工艺复杂，需铸造技术", "可能影响音色和谐性", "材料成本高", "镶块可能松动脱落"]'),
('焊补调音', 'welding_repair', '在钟壁特定位置焊补青铜材料，增加厚度降低音高，同时修复裂纹。', 0.95, -0.06, true, 5, 'Modern',
    '["可修复钟体裂纹", "可精确调整频率", "可逆，可通过磨锉去除焊料", "增强钟体结构强度"]',
    '["工艺复杂，需专业技能", "可能产生焊接应力", "焊接区域音色变化", "材料成本高"]')
ON CONFLICT (process_type) DO NOTHING;

-- ============================================
-- 工艺对比结果表
-- ============================================
CREATE TABLE IF NOT EXISTS process_comparisons (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    bell_id INTEGER NOT NULL REFERENCES bells(id),
    process_types VARCHAR(50)[],
    results JSONB,
    best_process VARCHAR(50),
    confidence_score DOUBLE PRECISION
);

-- ============================================
-- 经验法则表
-- ============================================
CREATE TABLE IF NOT EXISTS empirical_rules (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    rule_text TEXT NOT NULL,
    source VARCHAR(200),
    historical_era VARCHAR(50),
    formula TEXT,
    variables VARCHAR(100)[],
    description TEXT
);

INSERT INTO empirical_rules (name, rule_text, source, historical_era, formula, variables, description) VALUES
('壁厚与频率关系', '钟壁厚薄与音之高下，厚则声高，薄则声下', '《考工记·凫氏》', 'Spring and Autumn', 'f = 0.6 * sqrt(t) / (d * 0.01)',
    '{"thickness_mm", "diameter_cm"}',
    '春秋时期《考工记》记载的经验公式，描述钟壁厚度与音高的关系。验证表明该法则在±10%误差范围内准确。'),
('质量与基频估算', '大钟声宏，小钟声清，轻重为节', '《吕氏春秋·古乐》', 'Warring States', 'f = 0.45 * sqrt(m) / h^0.75',
    '{"mass_kg", "height_cm"}',
    '《吕氏春秋》记载的编钟基频估算公式，根据质量和高度估算基频。'),
('磨锉深度预估', '磨一分，声增五音分', '民间匠人口传', 'Qing Dynasty', 'f_new = f_current * (1 + 0.15 * d / t)',
    '{"current_freq", "grind_depth_mm", "thickness_mm"}',
    '清代工匠经验，每磨1分深度约升高5音分。现代验证显示该经验在合理范围内准确。'),
('直径与频率反比', '径大者声宏，径小者声清', '《梦溪笔谈》', 'Song Dynasty', 'f = 8000 / diameter',
    '{"diameter_cm"}',
    '北宋沈括《梦溪笔谈》记载，钟的直径与频率成反比关系。'),
('相邻钟频率比', '相邻两钟，频率比为半音', '曾侯乙编钟考古', 'Warring States', 'f_next = f_current * 2^(1/12)',
    '{"lower_freq"}',
    '曾侯乙编钟考古发现，相邻编钟的频率比约为半音（2^(1/12) ≈ 1.0595）。')
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- 跨时代对比文章表
-- ============================================
CREATE TABLE IF NOT EXISTS comparison_articles (
    id SERIAL PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    category VARCHAR(50) NOT NULL,
    bianzhong JSONB,
    piano JSONB,
    conclusion TEXT,
    references TEXT[],
    created_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO comparison_articles (title, category, bianzhong, piano, conclusion, "references") VALUES
('调音原理对比', 'principle',
    '{"name":"编钟调音","year":"公元前5世纪","method":"磨锉/铸镶/焊补","material":"青铜合金","adjustment":"壁厚调整","tolerance":"±5音分","harmonics":"非整数倍泛音","complexity":"每口钟独立调音"}',
    '{"name":"钢琴调律","year":"18世纪","method":"弦锤击弦","material":"钢弦+铸铁架","adjustment":"弦张力调整","tolerance":"±2音分","harmonics":"整数倍泛音","complexity":"88键统一调律"}',
    '编钟调音通过壁厚变化调整固有频率，钢琴通过弦张力调整。两者都是机械振动系统，但实现方式截然不同。编钟每口钟独立调音，复杂度更高。',
    '{"《考工记·凫氏》","赫尔姆霍茨《论音调的感觉》","曾侯乙编钟考古报告"}'),

('调律精度对比', 'accuracy',
    '{"target_accuracy":"±5音分","measurement_method":"人耳+声学仪器（现代）","stability":"数百年不变","environmental_factor":"温湿度影响小","maintenance_interval":"50-100年"}',
    '{"target_accuracy":"±2音分","measurement_method":"电子调音器","stability":"数月漂移","environmental_factor":"温湿度影响大","maintenance_interval":"3-6个月"}',
    '钢琴调律精度更高，但稳定性差，需要定期维护。编钟调音精度稍低，但一次调音可保持数百年。',
    '{"国际钢琴技师协会标准","文物保护技术规范"}'),

('泛音结构对比', 'harmonics',
    '{"harmonic_count":"8+","ratios":"[1.0,2.0,3.0,4.16,5.42]","inharmonicity":"明显","tone_color":"丰富多变","decay_time":"8-15秒"}',
    '{"harmonic_count":"15+","ratios":"[1.0,2.0,3.0,4.0,5.0]","inharmonicity":"较小","tone_color":"相对单一","decay_time":"3-6秒"}',
    '编钟的非整数倍泛音是其独特音色的来源，钢琴的整数倍泛音符合十二平均律体系。',
    '{"音乐声学基础","编钟声学特性研究"}'),

('调音工具对比', 'tools',
    '{"tools":["磨石","铸模","熔炉"],"power_source":"人力","skill_level":"高，需长期训练","adjustment_range":"0.01-0.8mm","reversibility":"磨锉不可逆，铸镶/焊补可逆"}',
    '{"tools":["调音扳手","止音棉","音叉"],"power_source":"人力","skill_level":"中，需专业培训","adjustment_range":"扭力调整","reversibility":"完全可逆"}',
    '编钟调音工具更原始但精确度要求更高，钢琴调音工具更精细高效。两者都需要高度专业技能。',
    '{"中国古代科技史","钢琴调律技术"}'),

('文化意义对比', 'culture',
    '{"cultural_level":"礼器+乐器","ceremony":"祭祀、宴享","social_status":"王室专属","symbolism":"礼制等级象征","craftsmanship":"世代传承"}',
    '{"cultural_level":"乐器+家具","ceremony":"音乐会、家庭娱乐","social_status":"普及乐器","symbolism":"西方音乐代表","craftsmanship":"工业化生产"}',
    '编钟是中国古代礼乐文明的象征，钢琴是西方音乐文化的代表。两者在各自文化中都占有重要地位。',
    '{"中国音乐史","西方音乐史","曾侯乙编钟研究"}')
ON CONFLICT (id) DO NOTHING;
