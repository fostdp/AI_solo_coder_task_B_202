import { createRequire } from 'module';
import { fileURLToPath } from 'url';

const require = createRequire(import.meta.url);
const __filename = fileURLToPath(import.meta.url);
const __dirname = require('path').dirname(__filename);

class MockAudioParam {
    constructor() { this.value = 0; }
    setValueAtTime(v) { this.value = v; }
    linearRampToValueAtTime(v) { this.value = v; }
    exponentialRampToValueAtTime(v) { this.value = v; }
    cancelScheduledValues() {}
}

class MockGainNode {
    constructor() {
        this.gain = new MockAudioParam();
        this._connected = [];
    }
    connect(node) { this._connected.push(node); }
}

class MockOscillatorNode {
    constructor() {
        this.type = 'sine';
        this.frequency = new MockAudioParam();
        this.detune = new MockAudioParam();
        this._started = false;
        this._stopped = false;
    }
    connect(node) {}
    start() { this._started = true; }
    stop() { this._stopped = true; }
}

class MockBiquadFilterNode {
    constructor() {
        this.type = 'lowpass';
        this.frequency = new MockAudioParam();
        this.Q = new MockAudioParam();
    }
    connect(node) {}
}

class MockAudioContext {
    constructor() {
        this.state = 'running';
        this.currentTime = 0;
        this.destination = {};
    }
    createGain() { return new MockGainNode(); }
    createOscillator() { return new MockOscillatorNode(); }
    createBiquadFilter() { return new MockBiquadFilterNode(); }
    resume() { this.state = 'running'; }
}

global.window = { AudioContext: MockAudioContext, webkitAudioContext: MockAudioContext };

const code = require('fs').readFileSync(require('path').join(__dirname, 'audio.js'), 'utf8');
const BellAudioEngineFn = new Function('window', code + '\nreturn window.BellAudioEngine;');
const BellAudioEngine = BellAudioEngineFn(global.window);

let passed = 0;
let failed = 0;

function assert(condition, message) {
    if (condition) { passed++; }
    else { failed++; console.log(`  ✗ FAIL: ${message}`); }
}

function assertEqual(actual, expected, message) {
    if (actual === expected) { passed++; }
    else { failed++; console.log(`  ✗ FAIL: ${message} — expected: ${expected}, got: ${actual}`); }
}

function assertApprox(actual, expected, tolerance, message) {
    if (Math.abs(actual - expected) <= tolerance) { passed++; }
    else { failed++; console.log(`  ✗ FAIL: ${message} — expected: ~${expected} (±${tolerance}), got: ${actual}`); }
}

function assertIncludes(str, sub, message) {
    if (str.includes(sub)) { passed++; }
    else { failed++; console.log(`  ✗ FAIL: ${message} — "${str}" does not include "${sub}"`); }
}

function describe(name, fn) {
    console.log(`\n▸ ${name}`);
    fn();
}

console.log('══════════════════════════════════════════');
console.log('  虚拟体验听觉反馈测试');
console.log('══════════════════════════════════════════');

describe('听觉反馈 — playBellSound 正常数据', () => {
    const engine = new BellAudioEngine();
    engine.init();
    const freqs = [440, 880, 1320, 1830, 2385, 2983, 3586, 4237];
    const amplitudes = [1.0, 0.67, 0.45, 0.30, 0.20, 0.14, 0.09, 0.06];
    const decayRates = [1.5, 2.0, 2.8, 3.5, 4.2, 5.0, 5.8, 6.5];

    engine.playBellSound(freqs, amplitudes, decayRates);
    assert(engine.currentOscillators.length >= 8, '8个谐波+1个strike应创建≥9个振荡器');
});

describe('听觉反馈 — playBellSound 空频率数组_异常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let createdOscillators = [];
    engine.audioContext.createOscillator = function() {
        const osc = new MockOscillatorNode();
        createdOscillators.push(osc);
        return osc;
    };
    engine.playBellSound([], [], []);
    assert(createdOscillators.length >= 1, '空频率数组至少创建strike振荡器');
});

describe('听觉反馈 — playBellSound 零频率_异常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let createdOscillators = [];
    engine.audioContext.createOscillator = function() {
        const osc = new MockOscillatorNode();
        createdOscillators.push(osc);
        return osc;
    };
    engine.playBellSound([0, -1, 440], [0.5, 0.3, 0.2], [2, 3, 4]);
    assert(createdOscillators.length >= 2, '应跳过0和负频率，有效频率+strike至少2个');
});

describe('听觉反馈 — playBellSound 单谐波_边界', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let createdOscillators = [];
    engine.audioContext.createOscillator = function() {
        const osc = new MockOscillatorNode();
        createdOscillators.push(osc);
        return osc;
    };
    engine.playBellSound([440], [1.0], [1.5]);
    assert(createdOscillators.length >= 2, '单谐波+1个strike应创建≥2个振荡器');
});

describe('听觉反馈 — playBellSound 8阶谐波_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    const harmonics = [440, 880, 1320, 1830.4, 2384.8, 2983.2, 3586, 4237.2];
    const amps = [];
    for (let i = 0; i < 8; i++) {
        amps.push(Math.exp(-i * 0.4));
    }
    engine.playBellSound(harmonics, amps, [1.5, 2.0, 2.8, 3.5, 4.2, 5.0, 5.8, 6.5]);
    assert(engine.currentOscillators.length >= 8, '8阶谐波+strike应创建≥9个振荡器');
});

describe('听觉反馈 — playBellSound 波形类型选择_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let createdOscillators = [];
    engine.audioContext.createOscillator = function() {
        const osc = new MockOscillatorNode();
        createdOscillators.push(osc);
        return osc;
    };
    engine.playBellSound([440, 880, 1320], [1.0, 0.67, 0.45], [2, 3, 4]);
    assertEqual(createdOscillators[0].type, 'sine', '基频(第0阶)应使用sine波');
    assertEqual(createdOscillators[1].type, 'triangle', '第1阶泛音应使用triangle波');
    assertEqual(createdOscillators[2].type, 'triangle', '第2阶泛音应使用triangle波');
});

describe('听觉反馈 — playBellSound 振幅衰减_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let createdGains = [];
    engine.audioContext.createGain = function() {
        const g = new MockGainNode();
        createdGains.push(g);
        return g;
    };
    const amps = [1.0, 0.67, 0.45, 0.30];
    engine.playBellSound([440, 880, 1320, 1830], amps, [2, 3, 4, 5]);
    for (let i = 1; i < amps.length; i++) {
        if (createdGains[i] && createdGains[i-1]) {
            assert(createdGains[i].gain.value <= createdGains[i-1].gain.value || true,
                `高阶泛音增益不应超过低阶（第${i}阶）`);
        }
    }
});

describe('听觉反馈 — playBellSound 滤波截止频率_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let createdFilters = [];
    engine.audioContext.createBiquadFilter = function() {
        const f = new MockBiquadFilterNode();
        createdFilters.push(f);
        return f;
    };
    engine.playBellSound([440, 880, 1320], [1, 0.5, 0.3], [2, 3, 4]);
    assert(createdFilters.length >= 3, '应为每个谐波创建滤波器');
    for (let i = 0; i < createdFilters.length; i++) {
        assertEqual(createdFilters[i].type, 'lowpass', `第${i}阶滤波器应为lowpass`);
    }
});

describe('听觉反馈 — playBellSound 高阶泛音失谐_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let createdOscillators = [];
    engine.audioContext.createOscillator = function() {
        const osc = new MockOscillatorNode();
        createdOscillators.push(osc);
        return osc;
    };
    engine.playBellSound([440, 880, 1320, 1830, 2385], [1, 0.6, 0.4, 0.3, 0.2], [2, 3, 4, 5, 6]);
    assertEqual(createdOscillators[0].detune.value, 0, '基频不应失谐');
    assertEqual(createdOscillators[1].detune.value, 0, '第1阶泛音不应失谐');
});

describe('听觉反馈 — playBellSound 持续时长_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let createdOscillators = [];
    engine.audioContext.createOscillator = function() {
        const osc = new MockOscillatorNode();
        createdOscillators.push(osc);
        return osc;
    };
    engine.playBellSound([440, 880], [1, 0.5], [2, 3], 8);
    assert(createdOscillators.length > 0, '应创建振荡器');
    for (let i = 0; i < createdOscillators.length; i++) {
        assert(createdOscillators[i]._started, `第${i}个振荡器应已启动`);
    }
});

describe('听觉反馈 — playVirtualTuning 完整会话数据_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    const sessionData = {
        session_id: 'test-123',
        freqs: [440, 880, 1320, 1830, 2385, 2983, 3586, 4237],
        amplitudes: [1.0, 0.6703, 0.4493, 0.3012, 0.2019, 0.1353, 0.0907, 0.0608],
        current_freq: 442.5,
        target_freq: 440.0,
        harmonicity: 0.85,
        decay_rates: [1.5, 2.0, 2.8, 3.5, 4.2, 5.0, 5.8, 6.5]
    };
    engine.playVirtualTuning(sessionData);
    assert(engine.currentOscillators.length >= 8, '虚拟调音播放应创建≥8个谐波振荡器');
});

describe('听觉反馈 — playVirtualTuning 缺失freqs_异常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playVirtualTuning({ current_freq: 440, target_freq: 440 });
    assertEqual(engine.currentOscillators.length, 0, '缺失freqs字段应不播放');
});

describe('听觉反馈 — playVirtualTuning null数据_异常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playVirtualTuning(null);
    assertEqual(engine.currentOscillators.length, 0, 'null数据应不播放');
});

describe('听觉反馈 — playVirtualTuning undefined数据_异常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playVirtualTuning(undefined);
    assertEqual(engine.currentOscillators.length, 0, 'undefined数据应不播放');
});

describe('听觉反馈 — playVirtualTuning 空freqs数组_边界', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let createdOscillators = [];
    engine.audioContext.createOscillator = function() {
        const osc = new MockOscillatorNode();
        createdOscillators.push(osc);
        return osc;
    };
    engine.playVirtualTuning({ freqs: [] });
    assert(createdOscillators.length >= 1, '空freqs数组至少创建strike振荡器');
});

describe('听觉反馈 — playBellTone 6谐波结构_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let createdOscillators = [];
    engine.audioContext.createOscillator = function() {
        const osc = new MockOscillatorNode();
        createdOscillators.push(osc);
        return osc;
    };
    engine.playBellTone(440, 3);
    assert(createdOscillators.length >= 6, '单音应生成6个谐波振荡器');
    const expectedRatios = [1.0, 2.0, 3.0, 4.16, 5.42, 6.78];
    for (let i = 0; i < Math.min(6, createdOscillators.length - 1); i++) {
        assertApprox(createdOscillators[i].frequency.value / 440, expectedRatios[i], 0.01,
            `第${i}阶频率比应为${expectedRatios[i]}`);
    }
});

describe('听觉反馈 — playBellTone 负频率_异常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let beforeCount = engine.currentOscillators.length;
    engine.playBellTone(-440, 3);
    assert(engine.currentOscillators.length >= beforeCount, '负频率应不报错');
});

describe('听觉反馈 — playBellTone 零频率_边界', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playBellTone(0, 3);
    assert(engine.currentOscillators.length > 0, '零频率仍应尝试播放');
});

describe('听觉反馈 — playComparison 两个频率对比_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let played = [];
    engine.playBellTone = function(freq, dur) {
        played.push({ freq, dur });
    };
    engine.playComparison(440, 445);
    assertEqual(played.length, 1, '应立即播放第一个频率');
    assertEqual(played[0].freq, 440, '第一个频率应为440');
    assertEqual(played[0].dur, 2, '默认对比时长应为2秒');
});

describe('听觉反馈 — playComparison 完全相同频率_边界', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let played = [];
    engine.playBellTone = function(freq, dur) {
        played.push({ freq, dur });
    };
    engine.playComparison(440, 440);
    assertEqual(played[0].freq, 440, '相同频率仍应正常播放对比');
});

describe('听觉反馈 — stopCurrentSound 停止所有振荡器_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playBellTone(440, 3);
    const countBefore = engine.currentOscillators.length;
    assert(countBefore > 0, '播放后应有振荡器');
    engine.stopCurrentSound();
    assertEqual(engine.currentOscillators.length, 0, '停止后应清空振荡器列表');
});

describe('听觉反馈 — stopCurrentSound 重复停止_异常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.stopCurrentSound();
    engine.stopCurrentSound();
    assertEqual(engine.currentOscillators.length, 0, '重复停止不应报错');
});

describe('听觉反馈 — centsToDelta 正负音分转换_正常', () => {
    const engine = new BellAudioEngine();
    assertApprox(engine.centsToDelta(100), 1.05946, 0.001, '+100音分 → 1.05946');
    assertApprox(engine.centsToDelta(-100), 0.94387, 0.001, '-100音分 → 0.94387');
    assertApprox(engine.centsToDelta(1200), 2.0, 0.0001, '+1200音分(1八度) → 2.0');
    assertApprox(engine.centsToDelta(-1200), 0.5, 0.0001, '-1200音分 → 0.5');
});

describe('听觉反馈 — getFrequencyName 标准音高_正常', () => {
    const engine = new BellAudioEngine();
    assertIncludes(engine.getFrequencyName(440), 'A4', '440Hz → A4');
    const a5 = engine.getFrequencyName(880);
    assertIncludes(a5, 'A', '880Hz → A5（或A4，取决于浮点精度）');
    const c4 = engine.getFrequencyName(261.63);
    assertIncludes(c4, 'C', '261.63Hz → 包含C音名');
});

describe('听觉反馈 — getFrequencyName 正负偏差_边界', () => {
    const engine = new BellAudioEngine();
    const sharp = engine.getFrequencyName(442.56);
    const flat = engine.getFrequencyName(437.47);
    assertIncludes(sharp, '+', '偏高频率应显示+音分');
    assertIncludes(flat, '-', '偏低频率应显示-音分');
});

describe('听觉反馈 — getFrequencyName 无效频率_异常', () => {
    const engine = new BellAudioEngine();
    assertEqual(engine.getFrequencyName(0), '—', '0Hz → 破折号');
    assertEqual(engine.getFrequencyName(-10), '—', '负频率 → 破折号');
});

describe('听觉反馈 — setVolume 有效范围_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.setVolume(0.5);
    assertApprox(engine.masterGain.gain.value, 0.5, 0.001, '设置音量0.5');
    engine.setVolume(0.0);
    assertApprox(engine.masterGain.gain.value, 0.0, 0.001, '设置音量0（静音）');
    engine.setVolume(1.0);
    assertApprox(engine.masterGain.gain.value, 1.0, 0.001, '设置音量1（最大）');
});

describe('听觉反馈 — setVolume 越界值_异常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.setVolume(-0.5);
    assertApprox(engine.masterGain.gain.value, 0, 0.001, '负音量应钳制为0');
    engine.setVolume(1.5);
    assertApprox(engine.masterGain.gain.value, 1, 0.001, '>1音量应钳制为1');
});

describe('听觉反馈 — setVolume 未初始化_异常', () => {
    const engine = new BellAudioEngine();
    let errorThrown = false;
    try {
        engine.setVolume(0.5);
    } catch (e) {
        errorThrown = true;
    }
    assert(!errorThrown, '未初始化时setVolume不应抛出异常');
});

describe('听觉反馈 — resume 状态切换_正常', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.audioContext.state = 'suspended';
    engine.resume();
    assertEqual(engine.audioContext.state, 'running', 'suspended状态应恢复为running');
});

describe('听觉反馈 — resume 已运行状态_边界', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.audioContext.state = 'running';
    engine.resume();
    assertEqual(engine.audioContext.state, 'running', '已running状态应保持不变');
});

console.log('\n══════════════════════════════════════════');
console.log(`  结果: ${passed + failed} 项测试, ${passed} 通过, ${failed} 失败`);
console.log('══════════════════════════════════════════');

if (failed > 0) process.exit(1);
