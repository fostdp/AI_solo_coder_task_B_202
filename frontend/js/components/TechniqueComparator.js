class TechniqueComparator {
  constructor(container, options = {}) {
    this.container = typeof container === 'string' 
      ? document.querySelector(container) 
      : container;
    
    this.options = {
      bellId: options.bellId || null,
      currentFreq: options.currentFreq || 430,
      targetFreq: options.targetFreq || 440,
      onResult: options.onResult || null,
      apiBase: options.apiBase || '/api',
      ...options
    };

    this.results = [];
    this.bestProcess = null;
    this.isLoading = false;

    this._init();
  }

  _init() {
    if (!this.container) {
      console.error('TechniqueComparator: container not found');
      return;
    }
    this._render();
    this._bindEvents();
  }

  _render() {
    this.container.innerHTML = `
      <div class="technique-comparator">
        <div class="comparator-header">
          <h3>工艺对比分析</h3>
          <div class="freq-inputs">
            <label>
              当前频率(Hz):
              <input type="number" class="current-freq" value="${this.options.currentFreq}" step="0.1" min="0">
            </label>
            <label>
              目标频率(Hz):
              <input type="number" class="target-freq" value="${this.options.targetFreq}" step="0.1" min="0">
            </label>
            <button class="compare-btn">开始对比</button>
          </div>
        </div>
        <div class="comparator-results">
          <div class="loading" style="display:none;">计算中...</div>
          <div class="results-list"></div>
        </div>
      </div>
    `;
  }

  _bindEvents() {
    const btn = this.container.querySelector('.compare-btn');
    btn.addEventListener('click', () => this._handleCompare());

    const currentInput = this.container.querySelector('.current-freq');
    const targetInput = this.container.querySelector('.target-freq');
    
    currentInput.addEventListener('change', (e) => {
      this.options.currentFreq = parseFloat(e.target.value) || 0;
    });
    targetInput.addEventListener('change', (e) => {
      this.options.targetFreq = parseFloat(e.target.value) || 0;
    });
  }

  async _handleCompare() {
    if (this.isLoading) return;
    
    this._setLoading(true);
    
    try {
      const response = await fetch(`${this.options.apiBase}/compare-tuning-processes`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          bell_id: this.options.bellId,
          current_freq: this.options.currentFreq,
          target_freq: this.options.targetFreq
        })
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const data = await response.json();
      this.results = data.results || [];
      this.bestProcess = data.best_process;

      this._renderResults();
      
      if (this.options.onResult) {
        this.options.onResult(data);
      }
    } catch (e) {
      console.error('工艺对比失败:', e);
      this._showError(e.message);
    } finally {
      this._setLoading(false);
    }
  }

  _renderResults() {
    const list = this.container.querySelector('.results-list');
    
    if (this.results.length === 0) {
      list.innerHTML = '<p class="no-data">暂无对比结果</p>';
      return;
    }

    const processNames = {
      'grinding': '磨锉工艺',
      'casting_inlay': '铸镶工艺',
      'welding_repair': '焊补工艺'
    };

    let html = '';
    for (const r of this.results) {
      const isBest = r.process_type === this.bestProcess;
      const name = processNames[r.process_type] || r.process_type;
      
      html += `
        <div class="result-card ${isBest ? 'best' : ''}" data-type="${r.process_type}">
          <div class="result-header">
            <h4>${name}</h4>
            ${isBest ? '<span class="best-badge">推荐</span>' : ''}
          </div>
          <div class="result-stats">
            <div class="stat">
              <span class="label">预估频率</span>
              <span class="value">${r.estimated_freq?.toFixed(2) || '-'} Hz</span>
            </div>
            <div class="stat">
              <span class="label">音分偏差</span>
              <span class="value ${Math.abs(r.deviation_cents || 0) < 10 ? 'good' : 'warn'}">
                ${r.deviation_cents?.toFixed(2) || '-'} ¢
              </span>
            </div>
            <div class="stat">
              <span class="label">和谐度</span>
              <span class="value">${(r.harmonicity * 100).toFixed(1)}%</span>
            </div>
            <div class="stat">
              <span class="label">复杂度</span>
              <span class="value">${r.complexity || '-'}</span>
            </div>
            <div class="stat">
              <span class="label">可逆性</span>
              <span class="value">${r.reversibility ? '是' : '否'}</span>
            </div>
            <div class="stat">
              <span class="label">损伤风险</span>
              <span class="value">${(r.damage_risk * 100).toFixed(0)}%</span>
            </div>
            <div class="stat">
              <span class="label">预计时间</span>
              <span class="value">${r.required_time_min || '-'} 分钟</span>
            </div>
            <div class="stat">
              <span class="label">综合评分</span>
              <span class="value score">${(r.overall_score * 100).toFixed(1)}</span>
            </div>
          </div>
          ${r.measurement_source ? `
          <div class="measurement-info">
            <div><strong>数据来源:</strong> ${r.measurement_source}</div>
            <div><strong>测量方法:</strong> ${r.measurement_method || '-'}</div>
            <div><strong>不确定度:</strong> ${r.measurement_uncertainty_cents || '-'} 音分</div>
          </div>
          ` : ''}
        </div>
      `;
    }

    list.innerHTML = html;
  }

  _setLoading(loading) {
    this.isLoading = loading;
    const loadingEl = this.container.querySelector('.loading');
    const btn = this.container.querySelector('.compare-btn');
    
    if (loadingEl) {
      loadingEl.style.display = loading ? 'block' : 'none';
    }
    if (btn) {
      btn.disabled = loading;
      btn.textContent = loading ? '计算中...' : '开始对比';
    }
  }

  _showError(message) {
    const list = this.container.querySelector('.results-list');
    list.innerHTML = `<p class="error">对比失败: ${message}</p>`;
  }

  setBellId(bellId) {
    this.options.bellId = bellId;
  }

  getResults() {
    return [...this.results];
  }

  getBestProcess() {
    return this.bestProcess;
  }

  findResult(processType) {
    return this.results.find(r => r.process_type === processType) || null;
  }

  rankByAccuracy() {
    return [...this.results].sort((a, b) => 
      Math.abs(a.deviation_cents) - Math.abs(b.deviation_cents)
    );
  }

  destroy() {
    this.container.innerHTML = '';
    this.results = [];
  }
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = TechniqueComparator;
}
