class BellAudioEngine {
    constructor() {
        this.audioContext = null;
        this.masterGain = null;
        this.isInitialized = false;
        this.currentOscillators = [];
        this.reverb = null;
    }

    init() {
        if (this.isInitialized) return;
        
        this.audioContext = new (window.AudioContext || window.webkitAudioContext)();
        this.masterGain = this.audioContext.createGain();
        this.masterGain.gain.value = 0.3;
        this.masterGain.connect(this.audioContext.destination);
        
        this.isInitialized = true;
    }

    resume() {
        if (this.audioContext && this.audioContext.state === 'suspended') {
            this.audioContext.resume();
        }
    }

    playBellSound(freqs, amplitudes, decayRates, duration = 8) {
        if (!this.isInitialized) this.init();
        this.resume();
        
        const ctx = this.audioContext;
        const now = ctx.currentTime;
        
        this.stopCurrentSound();
        
        freqs.forEach((freq, i) => {
            if (freq <= 0) return;
            
            const osc = ctx.createOscillator();
            const gain = ctx.createGain();
            const filter = ctx.createBiquadFilter();
            
            osc.type = i === 0 ? 'sine' : 'triangle';
            osc.frequency.value = freq;
            
            filter.type = 'lowpass';
            filter.frequency.value = Math.min(freq * 4, 10000);
            filter.Q.value = 1;
            
            const amp = amplitudes ? amplitudes[i] : Math.exp(-i * 0.4);
            const decay = decayRates ? decayRates[i] : (2 + i * 0.8);
            
            gain.gain.setValueAtTime(0, now);
            gain.gain.linearRampToValueAtTime(amp * 0.5, now + 0.01);
            gain.gain.exponentialRampToValueAtTime(0.001, now + duration * (decay / 5));
            
            if (i >= 2) {
                const detune = (Math.random() - 0.5) * 20;
                osc.detune.value = detune;
            }
            
            osc.connect(filter);
            filter.connect(gain);
            gain.connect(this.masterGain);
            
            osc.start(now);
            osc.stop(now + duration + 2);
            
            this.currentOscillators.push({ osc, gain });
        });
        
        const strike = ctx.createOscillator();
        const strikeGain = ctx.createGain();
        strike.type = 'square';
        strike.frequency.value = freqs[0] * 3;
        strikeGain.gain.setValueAtTime(0.1, now);
        strikeGain.gain.exponentialRampToValueAtTime(0.001, now + 0.15);
        strike.connect(strikeGain);
        strikeGain.connect(this.masterGain);
        strike.start(now);
        strike.stop(now + 0.2);
    }

    playBellTone(frequency, duration = 3) {
        if (!this.isInitialized) this.init();
        this.resume();
        
        const ctx = this.audioContext;
        const now = ctx.currentTime;
        
        this.stopCurrentSound();
        
        const harmonics = [
            { ratio: 1.0, amp: 0.5, decay: 1.5 },
            { ratio: 2.0, amp: 0.3, decay: 2.0 },
            { ratio: 3.0, amp: 0.2, decay: 2.8 },
            { ratio: 4.16, amp: 0.15, decay: 3.5 },
            { ratio: 5.42, amp: 0.1, decay: 4.2 },
            { ratio: 6.78, amp: 0.08, decay: 5.0 },
        ];
        
        harmonics.forEach((h, i) => {
            const freq = frequency * h.ratio;
            const osc = ctx.createOscillator();
            const gain = ctx.createGain();
            
            osc.type = i === 0 ? 'sine' : 'triangle';
            osc.frequency.value = freq;
            
            gain.gain.setValueAtTime(0, now);
            gain.gain.linearRampToValueAtTime(h.amp, now + 0.01);
            gain.gain.exponentialRampToValueAtTime(0.001, now + duration * (h.decay / 3));
            
            osc.connect(gain);
            gain.connect(this.masterGain);
            
            osc.start(now);
            osc.stop(now + duration + 2);
            
            this.currentOscillators.push({ osc, gain });
        });
        
        const strike = ctx.createOscillator();
        const strikeGain = ctx.createGain();
        strike.type = 'square';
        strike.frequency.value = frequency * 4;
        strikeGain.gain.setValueAtTime(0.08, now);
        strikeGain.gain.exponentialRampToValueAtTime(0.001, now + 0.1);
        strike.connect(strikeGain);
        strikeGain.connect(this.masterGain);
        strike.start(now);
        strike.stop(now + 0.15);
    }

    playComparison(freq1, freq2) {
        if (!this.isInitialized) this.init();
        this.resume();
        
        this.playBellTone(freq1, 2);
        
        setTimeout(() => {
            this.playBellTone(freq2, 2);
        }, 2500);
    }

    playVirtualTuning(sessionData) {
        if (!sessionData || !sessionData.freqs) return;
        this.playBellSound(
            sessionData.freqs,
            sessionData.amplitudes,
            sessionData.decay_rates,
            10
        );
    }

    stopCurrentSound() {
        const now = this.audioContext ? this.audioContext.currentTime : 0;
        this.currentOscillators.forEach(({ osc, gain }) => {
            try {
                gain.gain.cancelScheduledValues(now);
                gain.gain.setValueAtTime(gain.gain.value, now);
                gain.gain.exponentialRampToValueAtTime(0.001, now + 0.1);
                osc.stop(now + 0.15);
            } catch (e) {
                try { osc.stop(); } catch (e2) {}
            }
        });
        this.currentOscillators = [];
    }

    setVolume(volume) {
        if (this.masterGain) {
            this.masterGain.gain.value = Math.max(0, Math.min(1, volume));
        }
    }

    getFrequencyName(frequency) {
        const notes = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B'];
        if (frequency <= 0) return '—';
        const noteNum = 12 * (Math.log2(frequency / 440)) + 69;
        const octave = Math.floor(noteNum / 12) - 1;
        const noteIndex = Math.round(noteNum) % 12;
        const cents = 1200 * (Math.log2(frequency / 440) - (noteNum - 69) / 12);
        const centsStr = cents > 0 ? `+${cents.toFixed(0)}` : cents.toFixed(0);
        return `${notes[noteIndex]}${octave} (${centsStr}¢)`;
    }

    centsToDelta(cents) {
        return Math.pow(2, cents / 1200);
    }
}

window.BellAudioEngine = BellAudioEngine;
