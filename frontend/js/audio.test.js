import { createRequire } from 'module';
import { fileURLToPath } from 'url';

const require = createRequire(import.meta.url);
const __filename = fileURLToPath(import.meta.url);
const __dirname = require('path').dirname(__filename);

class MockAudioParam {
    constructor() {
        this.value = 0;
    }
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
    if (condition) {
        passed++;
    } else {
        failed++;
        console.log(`  ✗ FAIL: ${message}`);
    }
}

function assertEqual(actual, expected, message) {
    if (actual === expected) {
        passed++;
    } else {
        failed++;
        console.log(`  ✗ FAIL: ${message} — expected: ${expected}, got: ${actual}`);
    }
}

function assertApprox(actual, expected, tolerance, message) {
    if (Math.abs(actual - expected) <= tolerance) {
        passed++;
    } else {
        failed++;
        console.log(`  ✗ FAIL: ${message} — expected: ~${expected} (±${tolerance}), got: ${actual}`);
    }
}

function assertIncludes(str, sub, message) {
    if (str.includes(sub)) {
        passed++;
    } else {
        failed++;
        console.log(`  ✗ FAIL: ${message} — "${str}" does not include "${sub}"`);
    }
}

function describe(name, fn) {
    console.log(`\n▸ ${name}`);
    fn();
}

console.log('══════════════════════════════════════════');
console.log('  BellAudioEngine 单元测试');
console.log('══════════════════════════════════════════');

describe('构造函数', () => {
    const engine = new BellAudioEngine();
    assertEqual(engine.audioContext, null, 'audioContext 初始为 null');
    assertEqual(engine.masterGain, null, 'masterGain 初始为 null');
    assertEqual(engine.isInitialized, false, 'isInitialized 初始为 false');
    assert(Array.isArray(engine.currentOscillators), 'currentOscillators 是数组');
    assertEqual(engine.currentOscillators.length, 0, 'currentOscillators 初始为空');
    assertEqual(engine.reverb, null, 'reverb 初始为 null');
});

describe('init()', () => {
    const engine = new BellAudioEngine();
    engine.init();
    assert(engine.audioContext !== null, 'init 后 audioContext 不为 null');
    assert(engine.masterGain !== null, 'init 后 masterGain 不为 null');
    assertEqual(engine.isInitialized, true, 'init 后 isInitialized 为 true');
    assertApprox(engine.masterGain.gain.value, 0.3, 0.001, 'masterGain 音量默认 0.3');
});

describe('init() 重复调用', () => {
    const engine = new BellAudioEngine();
    engine.init();
    const ctx = engine.audioContext;
    engine.init();
    assertEqual(engine.audioContext, ctx, '重复 init 不重建 AudioContext');
});

describe('resume()', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.audioContext.state = 'suspended';
    engine.resume();
    assertEqual(engine.audioContext.state, 'running', 'resume 恢复 suspended 状态');
});

describe('resume() 非 suspended 状态', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.resume();
    assertEqual(engine.audioContext.state, 'running', 'resume 对 running 状态无影响');
});

describe('getFrequencyName — 正常值', () => {
    const engine = new BellAudioEngine();
    const r440 = engine.getFrequencyName(440);
    assertIncludes(r440, 'A4', '440Hz → A4');
    assertIncludes(r440, '¢', '包含音分符号');

    const r261 = engine.getFrequencyName(261.63);
    assertIncludes(r261, 'C4', '261.63Hz → C4');

    const r880 = engine.getFrequencyName(880);
    assertIncludes(r880, 'A5', '880Hz → A5');

    const r220 = engine.getFrequencyName(220);
    assertIncludes(r220, 'A3', '220Hz → A3');
});

describe('getFrequencyName — 边界值', () => {
    const engine = new BellAudioEngine();
    const r0 = engine.getFrequencyName(0);
    assertEqual(r0, '—', '0Hz → —');

    const r20 = engine.getFrequencyName(20);
    assertIncludes(r20, '¢', '极低频 20Hz 返回含音分格式');

    const r20000 = engine.getFrequencyName(20000);
    assertIncludes(r20000, '¢', '极高频 20000Hz 返回含音分格式');
});

describe('getFrequencyName — 异常值', () => {
    const engine = new BellAudioEngine();
    const rNeg = engine.getFrequencyName(-1);
    assertEqual(rNeg, '—', '-1Hz → —');

    const rNeg100 = engine.getFrequencyName(-100);
    assertEqual(rNeg100, '—', '-100Hz → —');
});

describe('centsToDelta — 正常值', () => {
    const engine = new BellAudioEngine();
    assertApprox(engine.centsToDelta(1200), 2.0, 0.0001, '1200音分 → 2.0');
    assertApprox(engine.centsToDelta(-1200), 0.5, 0.0001, '-1200音分 → 0.5');
    assertApprox(engine.centsToDelta(700), 1.4983, 0.001, '700音分 ≈ 1.4983 (纯五度)');
    assertApprox(engine.centsToDelta(100), 1.0595, 0.001, '100音分 ≈ 1.0595 (半音)');
});

describe('centsToDelta — 边界值', () => {
    const engine = new BellAudioEngine();
    assertApprox(engine.centsToDelta(0), 1.0, 0.0001, '0音分 → 1.0');
});

describe('setVolume — 正常值', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.setVolume(0.5);
    assertApprox(engine.masterGain.gain.value, 0.5, 0.001, '设置音量 0.5');
    engine.setVolume(0.8);
    assertApprox(engine.masterGain.gain.value, 0.8, 0.001, '设置音量 0.8');
});

describe('setVolume — 边界值', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.setVolume(0);
    assertApprox(engine.masterGain.gain.value, 0, 0.001, '音量 0');
    engine.setVolume(1);
    assertApprox(engine.masterGain.gain.value, 1, 0.001, '音量 1');
});

describe('setVolume — 超出范围', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.setVolume(-0.5);
    assertApprox(engine.masterGain.gain.value, 0, 0.001, '负值钳制为 0');
    engine.setVolume(1.5);
    assertApprox(engine.masterGain.gain.value, 1, 0.001, '超1钳制为 1');
});

describe('setVolume — 未初始化', () => {
    const engine = new BellAudioEngine();
    assertEqual(engine.masterGain, null, '未 init 时 masterGain 为 null');
    engine.setVolume(0.5);
    assertEqual(engine.masterGain, null, '未 init 时 setVolume 不报错');
});

describe('playVirtualTuning — 正常数据', () => {
    const engine = new BellAudioEngine();
    engine.init();
    const data = {
        freqs: [440, 880],
        amplitudes: [0.5, 0.3],
        decay_rates: [2, 3]
    };
    engine.playVirtualTuning(data);
    assert(engine.currentOscillators.length > 0, 'playVirtualTuning 创建振荡器');
});

describe('playVirtualTuning — 异常: null', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playVirtualTuning(null);
    assertEqual(engine.currentOscillators.length, 0, 'null 不创建振荡器');
});

describe('playVirtualTuning — 异常: undefined', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playVirtualTuning(undefined);
    assertEqual(engine.currentOscillators.length, 0, 'undefined 不创建振荡器');
});

describe('playVirtualTuning — 异常: 空对象', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playVirtualTuning({});
    assertEqual(engine.currentOscillators.length, 0, '空对象不创建振荡器');
});

describe('playComparison — 频率参数传递', () => {
    const engine = new BellAudioEngine();
    engine.init();
    let called = [];
    engine.playBellTone = function(freq, dur) {
        called.push({ freq, dur });
    };
    engine.playComparison(440, 880);
    assertEqual(called.length, 1, '立即调用第一次 playBellTone');
    assertEqual(called[0].freq, 440, '第一个频率 440');
    assertEqual(called[0].dur, 2, 'duration 为 2');
});

describe('playBellSound — 多谐波合成', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playBellSound([440, 880, 1320], [0.5, 0.3, 0.1], [2, 3, 4]);
    assert(engine.currentOscillators.length >= 3, '3个谐波+1个strike 创建振荡器');
});

describe('playBellTone — 单音6谐波', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playBellTone(440, 3);
    assert(engine.currentOscillators.length >= 6, '6谐波+1个strike 创建振荡器');
});

describe('stopCurrentSound', () => {
    const engine = new BellAudioEngine();
    engine.init();
    engine.playBellTone(440, 3);
    assert(engine.currentOscillators.length > 0, '播放中有振荡器');
    engine.stopCurrentSound();
    assertEqual(engine.currentOscillators.length, 0, '停止后振荡器清空');
});

console.log('\n══════════════════════════════════════════');
console.log(`  结果: ${passed + failed} 项测试, ${passed} 通过, ${failed} 失败`);
console.log('══════════════════════════════════════════');

if (failed > 0) process.exit(1);
