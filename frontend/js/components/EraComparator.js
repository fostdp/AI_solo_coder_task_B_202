class EraComparator {
  constructor(container, options = {}) {
    this.container = typeof container === 'string' 
      ? document.querySelector(container) 
      : container;
    
    this.options = {
      bellId: options.bellId || null,
      currentFreq: options.currentFreq || 400,
      targetFreq: options.targetFreq || 440,
      onResult: options.onResult || null,
      apiBase: options.apiBase || '/api',
      ...options
    };

    this.metrics = [];
    this.isLoading = false;

    this._init();
  }

  _init() {
    if (!this.container) {
      console.error('EraComparator: container not found');
      return;
    }
    this._render();
    this._bindEvents();
  }

  _render() {
    this.container.innerHTML = `
      <div class="era-comparator">
        <div class="comparator-header">
          <h3>跨时代技术演进对比</h3>
          <button class="compare-btn">分析演进</button>
        </div>
        <div class="comparator-content">
          <div class="loading" style="display:none;">分析中...</div>
          <div class="era-list"></div>
          <div class="evolution-summary"></div>
        </div>
      </div>
    `;
  }

  _bindEvents() {
    const btn = this.container.querySelector('.compare-btn');
    btn.addEventListener('click', () => this._handleCompare());
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
      this.metrics = this._transformToEraMetrics(data.results || []);

      this._renderEras();
      this._renderSummary();
      
      if (this.options.onResult) {
        this.options.onResult(this.metrics);
      }
    } catch (e) {
      console.error('跨时代对比失败:', e);
      this._showError(e.message);
    } finally {
      this._setLoading(false);
    }
  }

  _transformToEraMetrics(results) {
    const eraMap = {
      'grinding': 'ancient',
      'casting_inlay': 'ancient',
      'welding_repair': 'modern'
    };

    const processNames = {
      'grinding': '磨锉 (古代)',
      'casting_inlay': '铸镶 (古代)',
      'welding_repair': '焊补 (现代)'
    };

    return results.map(r => ({
      eraName: r.process_type,
      displayName: processNames[r.process_type] || r.process_type,
      historicalEra: eraMap[r.process_type] || 'unknown',
      complexityScore: 1 - (r.complexity || 0) / 10,
      reversibilityScore: r.reversibility ? 1.0 : 0.0,
      damageRiskScore: 1 - (r.damage_risk || 0),
      estimatedHours: (r.required_time_min || 0) / 60,
      harmonicityImpact: r.harmonicity || 0,
      overallScore: r.overall_score || 0
    }));
  }

  _renderEras() {
    const list = this.container.querySelector('.era-list');
    
    if (this.metrics.length === 0) {
      list.innerHTML = '<p class="no-data">暂无数据</p>';
      return;
    }

    let html = '<div class="era-cards">';
    for (const m of this.metrics) {
      html += `
        <div class="era-card" data-era="${m.eraName}">
          <h4>${m.displayName}</h4>
          <div class="era-badge era-${m.historicalEra}">${this._eraLabel(m.historicalEra)}</div>
          
          <div class="metric-row">
            <span class="metric-label">工艺复杂度</span>
            <div class="metric-bar">
              <div class="metric-fill" style="width:${m.complexityScore * 100}%"></div>
            </div>
            <span class="metric-value">${(m.complexityScore * 100).toFixed(0)}%</span>
          </div>
          
          <div class="metric-row">
            <span class="metric-label">可逆性</span>
            <div class="metric-bar">
              <div class="metric-fill" style="width:${m.reversibilityScore * 100}%"></div>
            </div>
            <span class="metric-value">${(m.reversibilityScore * 100).toFixed(0)}%</span>
          </div>
          
          <div class="metric-row">
            <span class="metric-label">安全性</span>
            <div class="metric-bar">
              <div class="metric-fill" style="width:${m.damageRiskScore * 100}%"></div>
            </div>
            <span class="metric-value">${(m.damageRiskScore * 100).toFixed(0)}%</span>
          </div>
          
          <div class="metric-row">
            <span class="metric-label">和谐度保持</span>
            <div class="metric-bar">
              <div class="metric-fill" style="width:${m.harmonicityImpact * 100}%"></div>
            </div>
            <span class="metric-value">${(m.harmonicityImpact * 100).toFixed(0)}%</span>
          </div>
          
          <div class="era-footer">
            <span>预计: ${m.estimatedHours.toFixed(1)} 小时</span>
            <span class="overall">综合: ${(m.overallScore * 100).toFixed(1)}</span>
          </div>
        </div>
      `;
    }
    html += '</div>';

    list.innerHTML = html;
  }

  _renderSummary() {
    const summary = this.container.querySelector('.evolution-summary');
    
    if (this.metrics.length < 2) {
      summary.innerHTML = '';
      return;
    }

    const sorted = [...this.metrics].sort((a, b) => b.overallScore - a.overallScore);
    const best = sorted[0];
    const worst = sorted[sorted.length - 1];
    const improvement = worst.overallScore > 0 
      ? ((best.overallScore - worst.overallScore) / worst.overallScore * 100).toFixed(1)
      : 0;

    summary.innerHTML = `
      <div class="evolution-card">
        <h4>技术演进分析</h4>
        <div class="evolution-stats">
          <div class="stat-item">
            <span class="stat-label">最优工艺</span>
            <span class="stat-value">${best.displayName}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">相对提升</span>
            <span class="stat-value positive">+${improvement}%</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">对比工艺</span>
            <span class="stat-value">${this.metrics.length} 种</span>
          </div>
        </div>
      </div>
    `;
  }

  _eraLabel(era) {
    const labels = {
      'ancient': '古代工艺',
      'modern': '现代技术',
      'digital': '数字仿真'
    };
    return labels[era] || era;
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
      btn.textContent = loading ? '分析中...' : '分析演进';
    }
  }

  _showError(message) {
    const list = this.container.querySelector('.era-list');
    list.innerHTML = `<p class="error">分析失败: ${message}</p>`;
  }

  setBellId(bellId) {
    this.options.bellId = bellId;
  }

  getMetrics() {
    return [...this.metrics];
  }

  getEraByName(eraName) {
    return this.metrics.find(m => m.eraName === eraName) || null;
  }

  getEvolutionIndex() {
    if (this.metrics.length < 2) return 0;
    
    const scores = this.metrics.map(m => m.overallScore);
    const best = Math.max(...scores);
    const worst = Math.min(...scores);
    
    return worst > 0 ? ((best - worst) / worst * 100) : 0;
  }

  destroy() {
    this.container.innerHTML = '';
    this.metrics = [];
  }
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = EraComparator;
}
