class EmpiricalRuleValidator {
  constructor(container, options = {}) {
    this.container = typeof container === 'string' 
      ? document.querySelector(container) 
      : container;
    
    this.options = {
      rules: options.rules || [],
      onValidate: options.onValidate || null,
      apiBase: options.apiBase || '/api',
      ...options
    };

    this.results = {};
    this.currentRuleId = null;
    this.isLoading = false;

    this._init();
  }

  _init() {
    if (!this.container) {
      console.error('EmpiricalRuleValidator: container not found');
      return;
    }
    this._render();
    this._bindEvents();
    this._loadRules();
  }

  _render() {
    this.container.innerHTML = `
      <div class="empirical-rule-validator">
        <div class="validator-header">
          <h3>经验法则验证</h3>
          <select class="rule-select">
            <option value="">选择经验法则...</option>
          </select>
        </div>
        
        <div class="validator-content">
          <div class="params-panel">
            <h4>参数设置</h4>
            <div class="params-form"></div>
            <button class="validate-btn" disabled>验证</button>
          </div>
          
          <div class="results-panel">
            <h4>验证结果</h4>
            <div class="loading" style="display:none;">验证中...</div>
            <div class="result-content">
              <p class="placeholder">选择法则并设置参数后点击验证</p>
            </div>
          </div>
        </div>
      </div>
    `;
  }

  _bindEvents() {
    const select = this.container.querySelector('.rule-select');
    select.addEventListener('change', (e) => this._handleRuleChange(e));

    const btn = this.container.querySelector('.validate-btn');
    btn.addEventListener('click', () => this._handleValidate());
  }

  async _loadRules() {
    try {
      const response = await fetch(`${this.options.apiBase}/empirical-rules`);
      if (response.ok) {
        this.options.rules = await response.json();
        this._populateRuleSelect();
      }
    } catch (e) {
      console.warn('加载经验法则失败，使用默认规则');
    }
  }

  _populateRuleSelect() {
    const select = this.container.querySelector('.rule-select');
    select.innerHTML = '<option value="">选择经验法则...</option>';
    
    for (const rule of this.options.rules) {
      const option = document.createElement('option');
      option.value = rule.id;
      option.textContent = rule.name || `规则 ${rule.id}`;
      select.appendChild(option);
    }
  }

  _handleRuleChange(e) {
    const ruleId = parseInt(e.target.value);
    this.currentRuleId = ruleId || null;
    
    this._renderParamsForm();
    this._updateValidateButton();
  }

  _renderParamsForm() {
    const form = this.container.querySelector('.params-form');
    const rule = this._getCurrentRule();
    
    if (!rule) {
      form.innerHTML = '';
      return;
    }

    const paramConfigs = this._getParamConfigs(rule.id);
    
    let html = '';
    for (const [key, config] of Object.entries(paramConfigs)) {
      html += `
        <div class="form-group">
          <label for="param-${key}">${config.label}</label>
          <input 
            type="number" 
            id="param-${key}" 
            name="${key}" 
            value="${config.default}"
            step="${config.step || '0.1'}"
            min="${config.min || '0'}"
          >
          <span class="unit">${config.unit || ''}</span>
        </div>
      `;
    }
    
    html += `
      <div class="form-group">
        <label for="param-sample_size">样本量</label>
        <input type="number" id="param-sample_size" name="sample_size" value="100" step="10" min="1">
        <span class="unit">个</span>
      </div>
    `;

    form.innerHTML = html;
  }

  _getParamConfigs(ruleId) {
    const configs = {
      1: {
        thickness_mm: { label: '厚度', default: 8, unit: 'mm', step: '0.5' },
        diameter_cm: { label: '直径', default: 20, unit: 'cm', step: '1' }
      },
      2: {
        mass_kg: { label: '质量', default: 2.5, unit: 'kg', step: '0.1' },
        height_cm: { label: '高度', default: 30, unit: 'cm', step: '1' }
      },
      3: {
        current_freq: { label: '当前频率', default: 400, unit: 'Hz', step: '1' },
        grind_depth_mm: { label: '磨锉深度', default: 0.5, unit: 'mm', step: '0.1' },
        thickness_mm: { label: '厚度', default: 8, unit: 'mm', step: '0.5' }
      },
      4: {
        diameter_cm: { label: '直径', default: 20, unit: 'cm', step: '1' }
      },
      5: {
        lower_freq: { label: '较低频率', default: 440, unit: 'Hz', step: '1' }
      }
    };
    return configs[ruleId] || {};
  }

  _getCurrentRule() {
    return this.options.rules.find(r => r.id === this.currentRuleId) || null;
  }

  _getParams() {
    const inputs = this.container.querySelectorAll('.params-form input');
    const params = {};
    
    for (const input of inputs) {
      const value = parseFloat(input.value);
      if (!isNaN(value)) {
        params[input.name] = value;
      }
    }
    
    return params;
  }

  _updateValidateButton() {
    const btn = this.container.querySelector('.validate-btn');
    btn.disabled = !this.currentRuleId;
  }

  async _handleValidate() {
    if (!this.currentRuleId || this.isLoading) return;
    
    this._setLoading(true);
    
    try {
      const params = this._getParams();
      
      const response = await fetch(`${this.options.apiBase}/validate-empirical-rule`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          rule_id: this.currentRuleId,
          params: params
        })
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const result = await response.json();
      this.results[this.currentRuleId] = result;

      this._renderResult(result);
      
      if (this.options.onValidate) {
        this.options.onValidate(result);
      }
    } catch (e) {
      console.error('验证失败:', e);
      this._showError(e.message);
    } finally {
      this._setLoading(false);
    }
  }

  _renderResult(result) {
    const content = this.container.querySelector('.result-content');
    
    const validClass = result.validation_result ? 'valid' : 'invalid';
    const validText = result.validation_result ? '通过' : '未通过';

    let html = `
      <div class="validation-result ${validClass}">
        <div class="result-status">
          <span class="status-badge">${validText}</span>
        </div>
        
        <div class="result-row">
          <span class="label">计算值</span>
          <span class="value">${result.computed_value?.toFixed(4) || '-'}</span>
        </div>
        
        <div class="result-row">
          <span class="label">预期值</span>
          <span class="value">${result.expected_value?.toFixed(4) || '-'}</span>
        </div>
        
        <div class="result-row">
          <span class="label">偏差百分比</span>
          <span class="value">${result.deviation_percent?.toFixed(2) || '-'}%</span>
        </div>
        
        <div class="result-row">
          <span class="label">置信度</span>
          <span class="value">${(result.confidence * 100).toFixed(1)}%</span>
        </div>

        <div class="stat-section">
          <h5>统计分析</h5>
          
          <div class="result-row">
            <span class="label">P值</span>
            <span class="value">${result.p_value?.toFixed(6) || '-'}</span>
          </div>
          
          <div class="result-row">
            <span class="label">统计显著性</span>
            <span class="value ${result.statistical_significance ? 'significant' : 'not-significant'}">
              ${result.statistical_significance ? '显著' : '不显著'}
            </span>
          </div>
          
          <div class="result-row">
            <span class="label">95%置信区间</span>
            <span class="value">
              [${result.confidence_interval_low?.toFixed(4) || '-'}, 
               ${result.confidence_interval_high?.toFixed(4) || '-'}]
            </span>
          </div>
          
          <div class="result-row">
            <span class="label">效应量</span>
            <span class="value">${result.effect_size?.toFixed(4) || '-'}</span>
          </div>
          
          <div class="result-row">
            <span class="label">标准误</span>
            <span class="value">${result.standard_error?.toFixed(6) || '-'}</span>
          </div>
          
          <div class="result-row">
            <span class="label">样本量</span>
            <span class="value">${result.sample_size || '-'}</span>
          </div>
        </div>
      </div>
    `;

    content.innerHTML = html;
  }

  _setLoading(loading) {
    this.isLoading = loading;
    const loadingEl = this.container.querySelector('.loading');
    const btn = this.container.querySelector('.validate-btn');
    
    if (loadingEl) {
      loadingEl.style.display = loading ? 'block' : 'none';
    }
    if (btn) {
      btn.disabled = loading || !this.currentRuleId;
      btn.textContent = loading ? '验证中...' : '验证';
    }
  }

  _showError(message) {
    const content = this.container.querySelector('.result-content');
    content.innerHTML = `<p class="error">验证失败: ${message}</p>`;
  }

  setRules(rules) {
    this.options.rules = rules;
    this._populateRuleSelect();
  }

  getResult(ruleId) {
    return this.results[ruleId] || null;
  }

  getAllResults() {
    return { ...this.results };
  }

  destroy() {
    this.container.innerHTML = '';
    this.results = {};
  }
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = EmpiricalRuleValidator;
}
