class VRTuning {
  constructor(container, options = {}) {
    this.container = typeof container === 'string' 
      ? document.querySelector(container) 
      : container;
    
    this.options = {
      bellId: options.bellId || null,
      audioEngine: options.audioEngine || null,
      onSessionStart: options.onSessionStart || null,
      onGrind: options.onGrind || null,
      onPlay: options.onPlay || null,
      apiBase: options.apiBase || '/api',
      prefetchEnabled: options.prefetchEnabled !== false,
      ...options
    };

    this.session = null;
    this.sessionId = null;
    this.isLoading = false;
    this.isPlaying = false;
    
    this._audioCache = new Map();
    this._prefetchTimer = null;

    this._init();
  }

  _init() {
    if (!this.container) {
      console.error('VRTuning: container not found');
      return;
    }
    this._render();
    this._bindEvents();
  }

  _render() {
    this.container.innerHTML = `
      <div class="vr-tuning">
        <div class="tuning-header">
          <h3>虚拟调音体验</h3>
          <div class="session-info" style="display:none;">
            <span class="session-id">会话: <span class="id-value">-</span></span>
            <span class="current-freq">当前: <span class="freq-value">-</span> Hz</span>
            <span class="target-freq">目标: <span class="freq-value">-</span> Hz</span>
          </div>
        </div>

        <div class="tuning-content">
          <div class="start-panel">
            <button class="start-btn">开始虚拟调音</button>
          </div>

          <div class="tuning-panel" style="display:none;">
            <div class="tuning-controls">
              <div class="grind-controls">
                <h4>磨锉参数</h4>
                <div class="control-group">
                  <label>深度 (mm)</label>
                  <input type="range" class="grind-depth" min="0.01" max="1" step="0.01" value="0.1">
                  <span class="depth-value">0.1</span>
                </div>
                <div class="control-group">
                  <label>工艺类型</label>
                  <select class="process-type">
                    <option value="grinding">磨锉</option>
                    <option value="casting_inlay">铸镶</option>
                    <option value="welding_repair">焊补</option>
                  </select>
                </div>
                <div class="control-group">
                  <label>位置 X</label>
                  <input type="number" class="pos-x" value="0" step="0.1">
                </div>
                <div class="control-group">
                  <label>位置 Y</label>
                  <input type="number" class="pos-y" value="0" step="0.1">
                </div>
                <div class="control-group">
                  <label>位置 Z</label>
                  <input type="number" class="pos-z" value="0" step="0.1">
                </div>
                <button class="apply-btn">应用工艺</button>
              </div>

              <div class="audio-controls">
                <h4>听觉反馈</h4>
                <button class="play-btn">播放当前音</button>
                <button class="compare-btn">对比目标音</button>
                <button class="reset-btn">重置</button>
              </div>
            </div>

            <div class="tuning-visual">
              <div class="freq-display">
                <div class="freq-meter">
                  <div class="freq-marker"></div>
                  <div class="target-marker"></div>
                </div>
                <div class="tolerance-indicator">
                  <span class="tol-label">容差范围</span>
                </div>
              </div>
              
              <div class="harmonicity-bar">
                <span class="label">和谐度</span>
                <div class="bar">
                  <div class="fill"></div>
                </div>
                <span class="value">0%</span>
              </div>

              <div class="status-info">
                <div class="status-item">
                  <span class="label">音分偏差</span>
                  <span class="value cents">0 ¢</span>
                </div>
                <div class="status-item">
                  <span class="label">总磨锉深度</span>
                  <span class="value">0 mm</span>
                </div>
                <div class="status-item">
                  <span class="label">操作次数</span>
                  <span class="value">0</span>
                </div>
              </div>
            </div>

            <div class="history-panel">
              <h4>操作历史</h4>
              <div class="history-list">
                <p class="empty">暂无操作记录</p>
              </div>
            </div>
          </div>

          <div class="loading" style="display:none;">处理中...</div>
        </div>
      </div>
    `;
  }

  _bindEvents() {
    const startBtn = this.container.querySelector('.start-btn');
    startBtn.addEventListener('click', () => this._startSession());

    const applyBtn = this.container.querySelector('.apply-btn');
    applyBtn.addEventListener('click', () => this._applyProcess());

    const playBtn = this.container.querySelector('.play-btn');
    playBtn.addEventListener('click', () => this._playCurrent());

    const compareBtn = this.container.querySelector('.compare-btn');
    compareBtn.addEventListener('click', () => this._playComparison());

    const resetBtn = this.container.querySelector('.reset-btn');
    resetBtn.addEventListener('click', () => this._resetSession());

    const depthSlider = this.container.querySelector('.grind-depth');
    const depthValue = this.container.querySelector('.depth-value');
    depthSlider.addEventListener('input', (e) => {
      depthValue.textContent = parseFloat(e.target.value).toFixed(2);
    });
  }

  async _startSession() {
    if (this.isLoading || !this.options.bellId) return;

    this._setLoading(true);

    try {
      const response = await fetch(`${this.options.apiBase}/virtual-tuning/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ bell_id: this.options.bellId })
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      this.session = await response.json();
      this.sessionId = this.session.session_id;

      this._showTuningPanel();
      this._updateDisplay();
      this._prefetchAudio();

      if (this.options.onSessionStart) {
        this.options.onSessionStart(this.session);
      }
    } catch (e) {
      console.error('启动虚拟调音失败:', e);
      alert('启动失败: ' + e.message);
    } finally {
      this._setLoading(false);
    }
  }

  async _applyProcess() {
    if (this.isLoading || !this.sessionId) return;

    this._setLoading(true);

    try {
      const processType = this.container.querySelector('.process-type').value;
      const depth = parseFloat(this.container.querySelector('.grind-depth').value);
      const pos = {
        x: parseFloat(this.container.querySelector('.pos-x').value) || 0,
        y: parseFloat(this.container.querySelector('.pos-y').value) || 0,
        z: parseFloat(this.container.querySelector('.pos-z').value) || 0
      };

      const response = await fetch(`${this.options.apiBase}/virtual-tuning/${this.sessionId}/grind`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          position: pos,
          depth_mm: depth,
          process_type: processType
        })
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const data = await response.json();
      this.session = data.session;

      this._updateDisplay();
      this._addToHistory(data.grind_result);
      this._prefetchAudio();

      if (this.options.onGrind) {
        this.options.onGrind(data);
      }
    } catch (e) {
      console.error('应用工艺失败:', e);
      alert('操作失败: ' + e.message);
    } finally {
      this._setLoading(false);
    }
  }

  async _playCurrent() {
    if (!this.sessionId || !this.options.audioEngine) return;

    const freq = this.session.current_freq;
    const cacheKey = `current_${freq}_${this.sessionId}`;

    if (this._audioCache.has(cacheKey)) {
      this._playFromCache(cacheKey);
      return;
    }

    try {
      const response = await fetch(`${this.options.apiBase}/virtual-tuning/${this.sessionId}/play`);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      
      const data = await response.json();
      
      if (this.options.audioEngine && this.options.audioEngine.playBellToneFast) {
        this.options.audioEngine.playBellToneFast(data.current_freq, 2.0);
      }

      if (this.options.onPlay) {
        this.options.onPlay(data);
      }
    } catch (e) {
      console.error('播放失败:', e);
    }
  }

  async _playComparison() {
    if (!this.sessionId || !this.options.audioEngine) return;

    try {
      const response = await fetch(`${this.options.apiBase}/virtual-tuning/${this.sessionId}/play`);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      
      const data = await response.json();
      
      if (this.options.audioEngine && this.options.audioEngine.playComparison) {
        this.options.audioEngine.playComparison(data.current_freq, data.target_freq, 1.5);
      }
    } catch (e) {
      console.error('对比播放失败:', e);
    }
  }

  async _resetSession() {
    if (!this.sessionId) return;

    try {
      const response = await fetch(`${this.options.apiBase}/virtual-tuning/${this.sessionId}/reset`, {
        method: 'POST'
      });
      
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      
      this.session = await response.json();
      this._updateDisplay();
      this._clearHistory();
      this._audioCache.clear();
    } catch (e) {
      console.error('重置失败:', e);
    }
  }

  _playFromCache(cacheKey) {
    const cached = this._audioCache.get(cacheKey);
    if (cached && this.options.audioEngine) {
      if (this.options.audioEngine._playFromCache) {
        this.options.audioEngine._playFromCache(cached);
      } else if (this.options.audioEngine.playBellToneFast) {
        this.options.audioEngine.playBellToneFast(cached.freq, cached.duration);
      }
    }
  }

  async _prefetchAudio() {
    if (!this.options.prefetchEnabled || !this.sessionId) return;

    if (this._prefetchTimer) {
      clearTimeout(this._prefetchTimer);
    }

    this._prefetchTimer = setTimeout(async () => {
      try {
        if (!this._audioCache.has(`play_${this.sessionId}`)) {
          const response = await fetch(`${this.options.apiBase}/virtual-tuning/${this.sessionId}/play`);
          if (response.ok) {
            const data = await response.json();
            this._audioCache.set(`play_${this.sessionId}`, {
              freq: data.current_freq,
              targetFreq: data.target_freq,
              freqs: data.freqs,
              amplitudes: data.amplitudes,
              timestamp: Date.now()
            });
          }
        }
      } catch (e) {
      }
    }, 300);
  }

  _showTuningPanel() {
    const startPanel = this.container.querySelector('.start-panel');
    const tuningPanel = this.container.querySelector('.tuning-panel');
    const sessionInfo = this.container.querySelector('.session-info');

    startPanel.style.display = 'none';
    tuningPanel.style.display = 'block';
    sessionInfo.style.display = 'block';
  }

  _updateDisplay() {
    if (!this.session) return;

    const idValue = this.container.querySelector('.id-value');
    if (idValue) {
      idValue.textContent = this.session.session_id?.substring(0, 8) + '...';
    }

    const freqValues = this.container.querySelectorAll('.current-freq .freq-value');
    freqValues.forEach(el => {
      el.textContent = this.session.current_freq?.toFixed(2) || '-';
    });

    const targetFreq = this.container.querySelector('.target-freq .freq-value');
    if (targetFreq) {
      targetFreq.textContent = this.session.target_freq?.toFixed(2) || '-';
    }

    const centsEl = this.container.querySelector('.status-item .cents');
    if (centsEl && this.session.history && this.session.history.length > 0) {
      const lastGrind = this.session.history[this.session.history.length - 1];
      centsEl.textContent = `${lastGrind.deviation?.toFixed(2) || 0} ¢`;
    }

    const totalDepthEl = this.container.querySelectorAll('.status-item .value')[1];
    if (totalDepthEl) {
      totalDepthEl.textContent = `${(this.session.total_depth_mm || 0).toFixed(2)} mm`;
    }

    const countEl = this.container.querySelectorAll('.status-item .value')[2];
    if (countEl) {
      countEl.textContent = this.session.history?.length || 0;
    }
  }

  _addToHistory(grindResult) {
    const list = this.container.querySelector('.history-list');
    
    const emptyEl = list.querySelector('.empty');
    if (emptyEl) {
      emptyEl.remove();
    }

    const processNames = {
      'grinding': '磨锉',
      'casting_inlay': '铸镶',
      'welding_repair': '焊补'
    };

    const item = document.createElement('div');
    item.className = 'history-item';
    item.innerHTML = `
      <span class="process-type">${processNames[grindResult.process_type] || grindResult.process_type}</span>
      <span class="freq-change">
        ${grindResult.before_freq?.toFixed(1)} → ${grindResult.after_freq?.toFixed(1)} Hz
      </span>
      <span class="deviation ${Math.abs(grindResult.deviation || 0) < 10 ? 'good' : 'warn'}">
        ${grindResult.deviation?.toFixed(2)} ¢
      </span>
    `;

    list.insertBefore(item, list.firstChild);
  }

  _clearHistory() {
    const list = this.container.querySelector('.history-list');
    list.innerHTML = '<p class="empty">暂无操作记录</p>';
  }

  _setLoading(loading) {
    this.isLoading = loading;
    const loadingEl = this.container.querySelector('.loading');
    const buttons = this.container.querySelectorAll('button');
    
    if (loadingEl) {
      loadingEl.style.display = loading ? 'block' : 'none';
    }
    buttons.forEach(btn => {
      btn.disabled = loading;
    });
  }

  setBellId(bellId) {
    this.options.bellId = bellId;
  }

  setAudioEngine(audioEngine) {
    this.options.audioEngine = audioEngine;
  }

  getSession() {
    return this.session ? { ...this.session } : null;
  }

  getSessionId() {
    return this.sessionId;
  }

  preloadAudio(sessionData) {
    if (this.options.audioEngine && this.options.audioEngine.preloadVirtualTuning) {
      this.options.audioEngine.preloadVirtualTuning(sessionData);
    }
  }

  clearCache() {
    this._audioCache.clear();
    if (this._prefetchTimer) {
      clearTimeout(this._prefetchTimer);
    }
  }

  destroy() {
    this.clearCache();
    this.container.innerHTML = '';
    this.session = null;
    this.sessionId = null;
  }
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = VRTuning;
}
