class App {
    constructor() {
        this.viewer = null;
        this.virtualViewer = null;
        this.bells = [];
        this.currentBell = null;
        this.measurements = [];
        this.grindingOps = [];
        this.alerts = [];
        this.frequencyChart = null;
        this.spectrumChart = null;
        this.ws = null;
        this.audioEngine = null;
        this.rules = [];
        this.comparisonArticles = [];
        this.tuningProcesses = [];
        this.virtualSession = null;
        this.currentTab = 'dashboard';
        this.lastComparisonData = null;
        this.processChart = null;

        this.init();
    }

    init() {
        this.viewer = new Bell3DViewer('bellViewer');
        this.audioEngine = new BellAudioEngine();
        this.initCharts();
        this.bindEvents();
        this.loadData();
        this.initWebSocket();
        this.startStatsRefresh();
        this.initNewFeatures();
    }

    initCharts() {
        const freqCtx = document.getElementById('frequencyChart').getContext('2d');
        this.frequencyChart = new Chart(freqCtx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [
                    {
                        label: '实测基频 (Hz)',
                        data: [],
                        borderColor: '#ffd700',
                        backgroundColor: 'rgba(255, 215, 0, 0.1)',
                        fill: true,
                        tension: 0.3,
                        pointRadius: 3
                    },
                    {
                        label: '目标频率 (Hz)',
                        data: [],
                        borderColor: '#2ecc71',
                        backgroundColor: 'transparent',
                        borderDash: [5, 5],
                        fill: false,
                        tension: 0,
                        pointRadius: 0
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: '#8892a6', font: { size: 11 } } } },
                scales: {
                    x: { ticks: { color: '#8892a6', font: { size: 10 } }, grid: { color: '#2a3548' } },
                    y: { ticks: { color: '#8892a6', font: { size: 10 } }, grid: { color: '#2a3548' } }
                }
            }
        });

        const specCtx = document.getElementById('spectrumChart').getContext('2d');
        this.spectrumChart = new Chart(specCtx, {
            type: 'bar',
            data: {
                labels: ['基频', '2次', '3次', '4次', '5次', '6次', '7次', '8次'],
                datasets: [{
                    label: '幅度',
                    data: [0, 0, 0, 0, 0, 0, 0, 0],
                    backgroundColor: [
                        '#ffd700', '#ff8c00', '#ff6b6b', '#e74c3c',
                        '#9b59b6', '#3498db', '#2ecc71', '#1abc9c'
                    ],
                    borderRadius: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    x: { ticks: { color: '#8892a6', font: { size: 10 } }, grid: { display: false } },
                    y: { ticks: { color: '#8892a6', font: { size: 10 } }, grid: { color: '#2a3548' } }
                }
            }
        });
    }

    bindEvents() {
        document.getElementById('modeSelect').addEventListener('change', (e) => {
            this.viewer.setModeOrder(parseInt(e.target.value));
        });

        document.getElementById('showContours').addEventListener('change', (e) => {
            this.viewer.toggleContours(e.target.checked);
        });

        document.getElementById('showGrinding').addEventListener('change', (e) => {
            this.viewer.toggleGrinding(e.target.checked);
        });

        document.getElementById('showWireframe').addEventListener('change', (e) => {
            this.viewer.toggleWireframe(e.target.checked);
        });

        document.getElementById('autoRotate').addEventListener('change', (e) => {
            this.viewer.autoRotate = e.target.checked;
        });

        document.getElementById('simulateBtn').addEventListener('click', () => this.runSimulation());
        document.getElementById('correctBtn').addEventListener('click', () => this.runPitchCorrection());
    }

    async loadData() {
        try {
            this.bells = await API.getBells();
            this.renderBellList();

            if (this.bells.length > 0) {
                this.selectBell(this.bells[0]);
            }

            this.alerts = await API.getAlerts();
            this.renderAlerts();

            this.updateStats();
        } catch (e) {
            console.error('Failed to load data:', e);
        }
    }

    renderBellList() {
        const container = document.getElementById('bellList');
        container.innerHTML = '';

        this.bells.forEach(bell => {
            const item = document.createElement('div');
            item.className = 'bell-item';
            if (this.currentBell && this.currentBell.id === bell.id) {
                item.classList.add('active');
            }

            item.innerHTML = `
                <div class="bell-name">${bell.name}</div>
                <div class="bell-serial">${bell.serial_number}</div>
                <div class="bell-freq">目标: ${bell.target_frequency.toFixed(2)} Hz</div>
            `;

            item.addEventListener('click', () => this.selectBell(bell));
            container.appendChild(item);
        });
    }

    async selectBell(bell) {
        this.currentBell = bell;
        this.renderBellList();
        this.renderBellInfo();

        document.getElementById('viewerBellName').textContent = bell.name;

        this.viewer.clearBell();
        this.viewer.createBellGeometry(bell);

        try {
            this.measurements = await API.getMeasurements(bell.id, 50);
            this.updateFrequencyChart();
            this.updateSpectrumChart();

            this.grindingOps = await API.getGrindingOperations(bell.id);
            this.renderGrindingMarkers();
        } catch (e) {
            console.error('Failed to load bell data:', e);
        }
    }

    renderBellInfo() {
        const container = document.getElementById('bellInfo');
        if (!this.currentBell) {
            container.innerHTML = '<p class="placeholder">请选择编钟</p>';
            return;
        }

        const b = this.currentBell;
        container.innerHTML = `
            <div class="info-row"><span class="label">编号</span><span class="value">${b.serial_number}</span></div>
            <div class="info-row"><span class="label">材质</span><span class="value">${b.material || '青铜'}</span></div>
            <div class="info-row"><span class="label">质量</span><span class="value">${b.mass_kg.toFixed(1)} kg</span></div>
            <div class="info-row"><span class="label">高度</span><span class="value">${b.height_cm.toFixed(1)} cm</span></div>
            <div class="info-row"><span class="label">口径</span><span class="value">${b.diameter_cm.toFixed(1)} cm</span></div>
            <div class="info-row"><span class="label">壁厚</span><span class="value">${b.thickness_mm.toFixed(1)} mm</span></div>
            <div class="info-row"><span class="label">目标频率</span><span class="value" style="color:#ffd700">${b.target_frequency.toFixed(2)} Hz</span></div>
            <div class="info-row"><span class="label">容差</span><span class="value">±${b.tolerance_cents} 音分</span></div>
            <div class="info-row"><span class="label">最大磨削</span><span class="value">${b.max_grinding_depth_mm} mm</span></div>
        `;
    }

    updateFrequencyChart() {
        const data = this.measurements.slice().reverse();
        const labels = data.map(m => {
            const d = new Date(m.time);
            return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
        });
        const freqs = data.map(m => m.fundamental_freq);
        const targets = data.map(() => this.currentBell.target_frequency);

        this.frequencyChart.data.labels = labels;
        this.frequencyChart.data.datasets[0].data = freqs;
        this.frequencyChart.data.datasets[1].data = targets;
        this.frequencyChart.update('none');
    }

    updateSpectrumChart() {
        if (this.measurements.length === 0) return;

        const latest = this.measurements[0];
        const overtones = latest.overtone_freqs || [];
        const amplitudes = latest.overtone_amplitudes || [];

        const labels = [`基频\n${latest.fundamental_freq.toFixed(1)}Hz`];
        const data = [1.0];

        for (let i = 0; i < Math.min(7, overtones.length); i++) {
            labels.push(`${i + 2}次\n${overtones[i].toFixed(1)}Hz`);
            data.push(amplitudes[i] || 0);
        }

        this.spectrumChart.data.labels = labels;
        this.spectrumChart.data.datasets[0].data = data;
        this.spectrumChart.update('none');
    }

    renderGrindingMarkers() {
        this.viewer.clearGrindingMarkers();
        this.grindingOps.forEach(op => {
            this.viewer.addGrindingMarker(op.position, op.grinding_depth_mm);
        });
    }

    renderAlerts() {
        const container = document.getElementById('alertList');
        if (!this.alerts || this.alerts.length === 0) {
            container.innerHTML = '<p class="placeholder">暂无告警</p>';
            return;
        }

        container.innerHTML = '';
        this.alerts.slice(0, 20).forEach(alert => {
            const item = document.createElement('div');
            item.className = `alert-item ${alert.severity}`;

            const time = new Date(alert.time).toLocaleString('zh-CN');

            item.innerHTML = `
                <div class="alert-content">
                    <div class="alert-type">${this.getAlertTypeLabel(alert.alert_type)}</div>
                    <div class="alert-message">${alert.message}</div>
                </div>
                <div class="alert-time">${time}</div>
            `;

            container.appendChild(item);
        });
    }

    getAlertTypeLabel(type) {
        const labels = {
            'pitch_deviation': '⚠️ 音准偏差',
            'severe_pitch_deviation': '🚨 严重音准偏差',
            'grinding_excess': '🛑 磨削过量'
        };
        return labels[type] || type;
    }

    async runSimulation() {
        if (!this.currentBell) {
            alert('请先选择编钟');
            return;
        }

        const btn = document.getElementById('simulateBtn');
        btn.disabled = true;
        btn.textContent = '仿真计算中...';

        try {
            const grindingPositions = this.grindingOps.map(op => op.position);
            const result = await API.runSimulation(this.currentBell.id, grindingPositions);

            alert(`仿真完成！\n计算耗时: ${result.computation_time_ms}ms\n固有频率:\n${result.eigenfrequencies.slice(0, 4).map((f, i) => `  第${i + 1}阶: ${f.toFixed(2)} Hz`).join('\n')}`);
        } catch (e) {
            console.error('Simulation failed:', e);
            alert('仿真失败: ' + e.message);
        } finally {
            btn.disabled = false;
            btn.textContent = '运行声学仿真';
        }
    }

    async runPitchCorrection() {
        if (!this.currentBell) {
            alert('请先选择编钟');
            return;
        }

        const btn = document.getElementById('correctBtn');
        btn.disabled = true;
        btn.textContent = '分析中...';

        try {
            let currentFreq;
            if (this.measurements.length > 0) {
                currentFreq = this.measurements[0].fundamental_freq;
            } else {
                currentFreq = this.currentBell.target_frequency * 1.02;
            }

            const correction = await API.getPitchCorrection(this.currentBell.id, currentFreq);
            this.renderCorrectionResult(correction);
        } catch (e) {
            console.error('Correction failed:', e);
            alert('音高修正分析失败: ' + e.message);
        } finally {
            btn.disabled = false;
            btn.textContent = '音高修正分析';
        }
    }

    renderCorrectionResult(correction) {
        const container = document.getElementById('correctionResult');

        let deviationClass = 'low';
        if (Math.abs(correction.deviation_cents) > this.currentBell.tolerance_cents * 2) {
            deviationClass = 'high';
        } else if (Math.abs(correction.deviation_cents) > this.currentBell.tolerance_cents) {
            deviationClass = 'medium';
        }

        let statusLabel = '';
        switch (correction.status) {
            case 'within_tolerance': statusLabel = '✅ 已在容差范围内'; break;
            case 'achievable': statusLabel = '✅ 可达到目标音高'; break;
            case 'recommended': statusLabel = '⚙️ 推荐方案'; break;
            case 'depth_limit_reached': statusLabel = '⚠️ 达到磨削深度限制'; break;
            default: statusLabel = correction.status;
        }

        let html = `
            <div class="correction-summary">
                <div class="deviation ${deviationClass}">
                    ${correction.deviation_cents > 0 ? '+' : ''}${correction.deviation_cents.toFixed(2)} 音分
                </div>
                <div class="meta">
                    当前: ${correction.current_frequency.toFixed(2)} Hz → 目标: ${correction.target_frequency.toFixed(2)} Hz<br>
                    预估结果: ${correction.estimated_result_freq.toFixed(2)} Hz<br>
                    ${statusLabel} | 迭代 ${correction.iterations} 次
                </div>
            </div>
        `;

        if (correction.recommended_positions && correction.recommended_positions.length > 0) {
            correction.recommended_positions.forEach((rec, idx) => {
                this.viewer.addGrindingMarker(rec.position, rec.depth_mm);

                html += `
                    <div class="correction-rec">
                        <div class="rec-title">推荐位置 #${idx + 1}</div>
                        <div class="rec-detail">
                            坐标: (${rec.position.x.toFixed(1)}, ${rec.position.y.toFixed(1)}, ${rec.position.z.toFixed(1)}) cm<br>
                            磨削深度: <span style="color:#ffd700">${rec.depth_mm.toFixed(3)} mm</span><br>
                            敏感度: ${rec.sensitivity.toFixed(2)} Hz/mm<br>
                            频率变化: ${rec.frequency_change_hz > 0 ? '+' : ''}${rec.frequency_change_hz.toFixed(2)} Hz
                        </div>
                    </div>
                `;
            });
        } else if (correction.status === 'within_tolerance') {
            html += `<p style="color:#2ecc71;text-align:center;padding:20px">当前音高已在容差范围内，无需磨锉</p>`;
        }

        container.innerHTML = html;
    }

    initWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/api/ws`;

        try {
            this.ws = new WebSocket(wsUrl);

            this.ws.onopen = () => {
                document.getElementById('wsStatus').classList.add('connected');
            };

            this.ws.onclose = () => {
                document.getElementById('wsStatus').classList.remove('connected');
                setTimeout(() => this.initWebSocket(), 5000);
            };

            this.ws.onerror = () => {
                document.getElementById('wsStatus').classList.remove('connected');
            };

            this.ws.onmessage = (event) => {
                const data = JSON.parse(event.data);
                this.handleWSMessage(data);
            };
        } catch (e) {
            console.error('WebSocket init failed:', e);
        }
    }

    handleWSMessage(data) {
        if (data.type === 'measurement') {
            if (this.currentBell && data.data.bell_id === this.currentBell.id) {
                this.measurements.unshift(data.data);
                if (this.measurements.length > 100) this.measurements.pop();
                this.updateFrequencyChart();
                this.updateSpectrumChart();
            }
        } else if (data.type === 'grinding') {
            if (this.currentBell && data.data.bell_id === this.currentBell.id) {
                this.grindingOps.unshift(data.data);
                this.viewer.addGrindingMarker(data.data.position, data.data.grinding_depth_mm);
            }
        }
        this.updateStats();
    }

    async updateStats() {
        try {
            const stats = await API.getDashboardStats();
            document.getElementById('statBells').textContent = stats.total_bells || 0;
            document.getElementById('statMeasurements').textContent = stats.measurements_24h || 0;
            document.getElementById('statAlerts').textContent = stats.active_alerts || 0;
            document.getElementById('statGrinding').textContent = stats.grinding_ops_24h || 0;
        } catch (e) {
            console.error('Stats update failed:', e);
        }
    }

    startStatsRefresh() {
        setInterval(() => this.updateStats(), 30000);
        setInterval(async () => {
            this.alerts = await API.getAlerts();
            this.renderAlerts();
        }, 15000);
    }

    async initNewFeatures() {
        try {
            this.tuningProcesses = await API.getTuningProcesses();
            this.rules = await API.getEmpiricalRules();
            this.comparisonArticles = await API.getComparisonArticles();
            
            this.populateBellSelects();
            this.renderRules();
            this.renderComparisonArticles();
            this.updateProcessStats();
        } catch (e) {
            console.error('Failed to init new features:', e);
        }
    }

    populateBellSelects() {
        const processSelect = document.getElementById('processBellSelect');
        const virtualSelect = document.getElementById('virtualBellSelect');
        
        [processSelect, virtualSelect].forEach(select => {
            select.innerHTML = this.bells.map(b => 
                `<option value="${b.id}">${b.name} (${b.target_frequency.toFixed(2)} Hz)</option>`
            ).join('');
        });

        if (this.bells.length > 0) {
            const bell = this.bells[0];
            document.getElementById('targetFreqInput').value = bell.target_frequency;
            document.getElementById('currentFreqInput').value = (bell.target_frequency * 0.98).toFixed(2);
        }
    }

    bindEvents() {
        document.getElementById('modeSelect').addEventListener('change', (e) => {
            this.viewer.setModeOrder(parseInt(e.target.value));
        });

        document.getElementById('showContours').addEventListener('change', (e) => {
            this.viewer.toggleContours(e.target.checked);
        });

        document.getElementById('showGrinding').addEventListener('change', (e) => {
            this.viewer.toggleGrindingMarkers(e.target.checked);
        });

        document.getElementById('showWireframe').addEventListener('change', (e) => {
            this.viewer.toggleWireframe(e.target.checked);
        });

        document.getElementById('autoRotate').addEventListener('change', (e) => {
            this.viewer.toggleAutoRotate(e.target.checked);
        });

        document.getElementById('simulateBtn').addEventListener('click', () => this.runSimulation());
        document.getElementById('correctBtn').addEventListener('click', () => this.runPitchCorrection());

        document.getElementById('volumeSlider').addEventListener('input', (e) => {
            this.audioEngine.setVolume(parseFloat(e.target.value));
        });

        document.querySelectorAll('.feature-tabs .tab[data-tab]').forEach(tab => {
            tab.addEventListener('click', () => this.switchTab(tab.dataset.tab));
        });

        document.getElementById('compareProcessBtn').addEventListener('click', () => this.compareTuningProcesses());
        document.getElementById('listenComparisonBtn').addEventListener('click', () => this.listenComparison());
        document.getElementById('comparisonCategorySelect').addEventListener('change', (e) => this.filterComparisonArticles(e.target.value));
        document.getElementById('startVirtualBtn').addEventListener('click', () => this.startVirtualTuning());
        document.getElementById('virtualDepthSlider').addEventListener('input', (e) => {
            document.getElementById('virtualDepthValue').textContent = parseFloat(e.target.value).toFixed(2) + ' mm';
        });
        document.getElementById('virtualGrindBtn').addEventListener('click', () => this.doVirtualGrind());
        document.getElementById('virtualPlayBtn').addEventListener('click', () => this.playVirtualBell());
        document.getElementById('virtualResetBtn').addEventListener('click', () => this.resetVirtualTuning());

        document.getElementById('processBellSelect').addEventListener('change', (e) => {
            const bell = this.bells.find(b => b.id == e.target.value);
            if (bell) {
                document.getElementById('targetFreqInput').value = bell.target_frequency;
                document.getElementById('currentFreqInput').value = (bell.target_frequency * 0.98).toFixed(2);
            }
        });
    }

    switchTab(tabName) {
        document.querySelectorAll('.feature-tabs .tab').forEach(t => t.classList.remove('active'));
        document.querySelector(`.feature-tabs .tab[data-tab="${tabName}"]`).classList.add('active');
        
        document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));
        document.getElementById(`tab-${tabName}`).classList.add('active');
        
        this.currentTab = tabName;
        
        if (tabName === 'virtual' && !this.virtualViewer) {
            setTimeout(() => {
                this.virtualViewer = new Bell3DViewer('virtualViewer');
                if (this.bells.length > 0) {
                    this.virtualViewer.updateBell(this.bells[0]);
                }
                this.virtualViewer.toggleAutoRotate(true);
            }, 100);
        }
        
        if (tabName === 'virtual' && this.virtualViewer) {
            setTimeout(() => this.virtualViewer.onResize(), 100);
        }
    }

    async compareTuningProcesses() {
        const bellId = parseInt(document.getElementById('processBellSelect').value);
        const currentFreq = parseFloat(document.getElementById('currentFreqInput').value);
        const targetFreq = parseFloat(document.getElementById('targetFreqInput').value);

        const btn = document.getElementById('compareProcessBtn');
        btn.disabled = true;
        btn.textContent = '分析中...';

        try {
            const result = await API.compareTuningProcesses(bellId, currentFreq, targetFreq);
            this.lastComparisonData = result;
            this.renderProcessComparison(result);
            this.updateProcessStats();
        } catch (e) {
            console.error('Comparison failed:', e);
            alert('工艺对比分析失败: ' + e.message);
        } finally {
            btn.disabled = false;
            btn.textContent = '运行对比分析';
        }
    }

    renderProcessComparison(data) {
        const container = document.getElementById('processResults');
        const bell = this.bells.find(b => b.id === data.bell_id);

        const processLabels = {
            'grinding': '🪨 磨锉',
            'casting_inlay': '🔩 铸镶',
            'welding_repair': '🔥 焊补'
        };

        let html = `
            <div class="comparison-summary">
                <div class="best-process">
                    <strong>🏆 最佳工艺: ${processLabels[data.best_process] || data.best_process}</strong>
                    <span class="confidence">置信度: ${(data.confidence * 100).toFixed(1)}%</span>
                </div>
                <div class="comparison-target">
                    编钟: ${bell.name} | 当前: ${data.current_freq.toFixed(2)} Hz → 目标: ${data.target_freq.toFixed(2)} Hz
                </div>
            </div>
            <div class="process-cards">
        `;

        data.results.forEach(r => {
            const isBest = r.process_type === data.best_process;
            const deviationCents = Math.abs(r.deviation_cents);
            const deviationClass = deviationCents < 5 ? 'good' : deviationCents < 15 ? 'warn' : 'bad';
            
            html += `
                <div class="process-card ${isBest ? 'best' : ''}">
                    <div class="process-card-header">
                        <h4>${processLabels[r.process_type] || r.process_type}</h4>
                        ${isBest ? '<span class="best-badge">🏆 推荐</span>' : ''}
                    </div>
                    <div class="process-metrics">
                        <div class="metric">
                            <span class="metric-label">预估频率</span>
                            <span class="metric-value">${r.estimated_freq.toFixed(2)} Hz</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">频率变化</span>
                            <span class="metric-value ${r.freq_delta_hz > 0 ? 'positive' : 'negative'}">
                                ${r.freq_delta_hz > 0 ? '+' : ''}${r.freq_delta_hz.toFixed(2)} Hz
                            </span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">偏差</span>
                            <span class="metric-value ${deviationClass}">${r.deviation_cents.toFixed(1)} ¢</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">和谐度</span>
                            <div class="harmonicity-bar">
                                <div class="harmonicity-fill" style="width: ${r.harmonicity * 100}%"></div>
                            </div>
                            <span class="metric-value">${(r.harmonicity * 100).toFixed(0)}%</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">复杂度</span>
                            <span class="metric-value">${'●'.repeat(r.complexity)}${'○'.repeat(5 - r.complexity)}</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">可逆性</span>
                            <span class="metric-value">${r.reversibility ? '✅ 可逆' : '❌ 不可逆'}</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">损伤风险</span>
                            <span class="metric-value">${(r.damage_risk * 100).toFixed(0)}%</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">所需时间</span>
                            <span class="metric-value">${r.required_time_min} min</span>
                        </div>
                        <div class="metric overall">
                            <span class="metric-label">综合评分</span>
                            <span class="metric-value score">${(r.overall_score * 100).toFixed(0)}</span>
                        </div>
                    </div>
                </div>
            `;
        });

        html += '</div>';
        container.innerHTML = html;
    }

    async updateProcessStats() {
        try {
            const stats = await API.getProcessStats();
            document.getElementById('totalComparisons').textContent = stats.total_comparisons || 0;
            document.getElementById('grindingWins').textContent = stats.best_process_counts?.grinding || 0;
            document.getElementById('castingWins').textContent = stats.best_process_counts?.casting_inlay || 0;
            document.getElementById('weldingWins').textContent = stats.best_process_counts?.welding_repair || 0;
        } catch (e) {
            console.error('Failed to get process stats:', e);
        }
    }

    listenComparison() {
        if (!this.lastComparisonData) {
            alert('请先运行对比分析');
            return;
        }

        const bestProcess = this.lastComparisonData.results.find(
            r => r.process_type === this.lastComparisonData.best_process
        );

        if (bestProcess) {
            this.audioEngine.playComparison(
                this.lastComparisonData.current_freq,
                bestProcess.estimated_freq
            );
        }
    }

    renderRules() {
        const container = document.getElementById('rulesList');
        
        let html = '';
        this.rules.forEach(rule => {
            html += `
                <div class="rule-card">
                    <div class="rule-header">
                        <h3>${rule.name}</h3>
                        <span class="era-badge">${this.getEraLabel(rule.historical_era)}</span>
                    </div>
                    <blockquote class="rule-quote">"${rule.rule_text}"</blockquote>
                    <div class="rule-source">— ${rule.source}</div>
                    <div class="rule-formula">
                        <strong>数学表达:</strong> <code>${rule.formula}</code>
                    </div>
                    <div class="rule-description">${rule.description}</div>
                    <div class="rule-variables">
                        <strong>变量:</strong> ${rule.variables.join(', ')}
                    </div>
                    <div class="rule-validation">
                        <h4>🔬 现代验证</h4>
                        <div class="validation-form" id="validation-form-${rule.id}">
                            ${this.renderValidationForm(rule)}
                        </div>
                        <div class="validation-result" id="validation-result-${rule.id}"></div>
                    </div>
                </div>
            `;
        });

        container.innerHTML = html;

        this.rules.forEach(rule => {
            const form = document.getElementById(`validation-form-${rule.id}`);
            if (form) {
                form.addEventListener('submit', (e) => {
                    e.preventDefault();
                    this.validateRule(rule.id);
                });
            }
        });
    }

    renderValidationForm(rule) {
        const bell = this.bells[0];
        const defaultVals = {
            'thickness_mm': bell?.thickness_mm || 20,
            'diameter_cm': bell?.diameter_cm || 40,
            'mass_kg': bell?.mass_kg || 100,
            'height_cm': bell?.height_cm || 80,
            'current_freq': bell?.target_frequency || 130,
            'grind_depth_mm': 0.5,
            'lower_freq': 130.81
        };

        let html = '<form class="validation-inputs">';
        rule.variables.forEach(v => {
            const val = defaultVals[v] || 0;
            html += `
                <div class="input-group">
                    <label>${v}</label>
                    <input type="number" id="rule-${rule.id}-${v}" step="0.01" value="${val}" class="rule-input">
                </div>
            `;
        });
        html += `<button type="submit" class="btn btn-secondary">运行验证</button></form>`;
        return html;
    }

    async validateRule(ruleId) {
        const rule = this.rules.find(r => r.id === ruleId);
        const params = {};
        
        rule.variables.forEach(v => {
            const input = document.getElementById(`rule-${ruleId}-${v}`);
            params[v] = parseFloat(input.value);
        });

        try {
            const result = await API.validateEmpiricalRule(ruleId, params);
            this.renderValidationResult(ruleId, result);
        } catch (e) {
            console.error('Rule validation failed:', e);
        }
    }

    renderValidationResult(ruleId, result) {
        const container = document.getElementById(`validation-result-${ruleId}`);
        const statusClass = result.validation_result ? 'pass' : 'fail';
        const statusIcon = result.validation_result ? '✅' : '❌';
        
        container.innerHTML = `
            <div class="validation-status ${statusClass}">
                ${statusIcon} ${result.validation_result ? '验证通过' : '验证未通过'}
            </div>
            <div class="validation-details">
                <div>计算值: ${result.computed_value.toFixed(4)}</div>
                <div>期望值: ${result.expected_value.toFixed(4)}</div>
                <div>偏差: ${result.deviation_percent.toFixed(2)}%</div>
                <div>置信度: ${(result.confidence * 100).toFixed(0)}%</div>
                <div>样本量: ${result.sample_size}</div>
            </div>
        `;
    }

    getEraLabel(era) {
        const labels = {
            'Spring and Autumn': '春秋时期',
            'Warring States': '战国时期',
            'Song Dynasty': '宋代',
            'Qing Dynasty': '清代',
            'Modern': '现代'
        };
        return labels[era] || era;
    }

    renderComparisonArticles(category = null) {
        const container = document.getElementById('comparisonArticles');
        let articles = this.comparisonArticles;
        
        if (category) {
            articles = articles.filter(a => a.category === category);
        }

        let html = '';
        articles.forEach(article => {
            html += `
                <div class="comparison-card">
                    <div class="comparison-header">
                        <h3>${article.title}</h3>
                        <span class="category-badge">${this.getCategoryLabel(article.category)}</span>
                    </div>
                    <div class="comparison-grid">
                        <div class="comparison-col bianzhong">
                            <h4>🔔 编钟调音</h4>
                            ${this.renderComparisonDict(article.bianzhong)}
                        </div>
                        <div class="comparison-col piano">
                            <h4>🎹 钢琴调律</h4>
                            ${this.renderComparisonDict(article.piano)}
                        </div>
                    </div>
                    <div class="comparison-conclusion">
                        <strong>💡 结论:</strong> ${article.conclusion}
                    </div>
                    <div class="comparison-refs">
                        <strong>📚 参考文献:</strong>
                        <ul>${article.references.map(r => `<li>${r}</li>`).join('')}</ul>
                    </div>
                </div>
            `;
        });

        container.innerHTML = html || '<p class="placeholder">暂无数据</p>';
    }

    renderComparisonDict(dict) {
        if (!dict) return '';
        return `
            <table class="comparison-table">
                ${Object.entries(dict).map(([k, v]) => `
                    <tr>
                        <th>${this.formatKey(k)}</th>
                        <td>${Array.isArray(v) ? v.join(', ') : v}</td>
                    </tr>
                `).join('')}
            </table>
        `;
    }

    formatKey(key) {
        const map = {
            'name': '名称',
            'year': '年代',
            'method': '原理',
            'material': '材料',
            'adjustment': '调整方式',
            'tolerance': '精度容差',
            'harmonics': '泛音结构',
            'complexity': '复杂度',
            'target_accuracy': '目标精度',
            'measurement_method': '测量方式',
            'stability': '稳定性',
            'environmental_factor': '环境影响',
            'maintenance_interval': '维护周期',
            'harmonic_count': '泛音数',
            'ratios': '频率比',
            'inharmonicity': '非谐性',
            'tone_color': '音色',
            'decay_time': '衰减时间',
            'tools': '工具',
            'power_source': '动力',
            'skill_level': '技能要求',
            'adjustment_range': '调整范围',
            'reversibility': '可逆性',
            'cultural_level': '文化层次',
            'ceremony': '使用场合',
            'social_status': '社会地位',
            'symbolism': '象征意义',
            'craftsmanship': '工艺传承'
        };
        return map[key] || key;
    }

    getCategoryLabel(cat) {
        const labels = {
            'principle': '调音原理',
            'accuracy': '调律精度',
            'harmonics': '泛音结构',
            'tools': '调音工具',
            'culture': '文化意义'
        };
        return labels[cat] || cat;
    }

    filterComparisonArticles(category) {
        this.renderComparisonArticles(category || null);
    }

    async startVirtualTuning() {
        const bellId = parseInt(document.getElementById('virtualBellSelect').value);
        
        const btn = document.getElementById('startVirtualBtn');
        btn.disabled = true;
        btn.textContent = '创建中...';

        try {
            this.virtualSession = await API.startVirtualTuning(bellId);
            this.updateVirtualSessionUI();
            
            document.getElementById('virtualSessionInfo').style.display = 'block';
            document.getElementById('virtualControls').style.display = 'block';
            document.getElementById('virtualHistory').style.display = 'block';
            
            const bell = this.bells.find(b => b.id === bellId);
            if (bell && this.virtualViewer) {
                this.virtualViewer.updateBell(bell);
            }
            
            this.playVirtualBell();
        } catch (e) {
            console.error('Virtual tuning start failed:', e);
            alert('启动虚拟调音失败: ' + e.message);
        } finally {
            btn.disabled = false;
            btn.textContent = '开始调音体验';
        }
    }

    async doVirtualGrind() {
        if (!this.virtualSession) return;

        const processType = document.getElementById('virtualProcessSelect').value;
        const depthMm = parseFloat(document.getElementById('virtualDepthSlider').value);
        const posStr = document.getElementById('virtualPositionSelect').value;

        const bell = this.bells.find(b => b.id === this.virtualSession.bell_id);
        const radius = bell ? bell.diameter_cm / 2 : 20;
        const height = bell ? bell.height_cm : 80;

        const positions = {
            'center': { x: 0, y: height / 2, z: radius * 0.7 },
            'top': { x: 0, y: height * 0.8, z: radius * 0.5 },
            'bottom': { x: 0, y: height * 0.2, z: radius * 0.8 },
            'left': { x: -radius * 0.6, y: height / 2, z: radius * 0.5 },
            'right': { x: radius * 0.6, y: height / 2, z: radius * 0.5 }
        };

        const position = positions[posStr] || positions.center;
        const btn = document.getElementById('virtualGrindBtn');
        btn.disabled = true;
        btn.textContent = '磨锉中...';

        try {
            const result = await API.virtualTuningGrind(
                this.virtualSession.session_id,
                position,
                depthMm,
                processType
            );
            
            this.virtualSession = result.session;
            this.updateVirtualSessionUI();
            this.renderVirtualHistory();
            
            if (this.virtualViewer) {
                this.virtualViewer.addGrindingMarker(position, depthMm);
            }
            
            this.playVirtualBell();

            if (result.within_tolerance) {
                setTimeout(() => {
                    alert('🎉 恭喜！音高已调整至目标容差范围内！');
                }, 500);
            }
        } catch (e) {
            console.error('Virtual grind failed:', e);
            alert('虚拟磨锉失败: ' + e.message);
        } finally {
            btn.disabled = false;
            btn.textContent = '⚒️ 执行磨锉';
        }
    }

    async playVirtualBell() {
        if (!this.virtualSession) return;
        
        try {
            const data = await API.virtualTuningPlay(this.virtualSession.session_id);
            this.audioEngine.playVirtualTuning(data);
        } catch (e) {
            console.error('Play failed:', e);
        }
    }

    async resetVirtualTuning() {
        if (!this.virtualSession) return;

        if (!confirm('确定要重置编钟吗？所有磨锉历史将被清除。')) return;

        try {
            this.virtualSession = await API.virtualTuningReset(this.virtualSession.session_id);
            this.updateVirtualSessionUI();
            this.renderVirtualHistory();
            
            if (this.virtualViewer) {
                this.virtualViewer.clearGrindingMarkers();
            }
            
            this.playVirtualBell();
        } catch (e) {
            console.error('Reset failed:', e);
        }
    }

    updateVirtualSessionUI() {
        if (!this.virtualSession) return;

        const s = this.virtualSession;
        const deviationCents = 1200 * Math.log2(s.current_freq / s.target_freq);
        const bell = this.bells.find(b => b.id === s.bell_id);
        const tolerance = bell?.tolerance_cents || 5;

        document.getElementById('virtualOriginalFreq').textContent = s.original_freq.toFixed(2) + ' Hz';
        document.getElementById('virtualCurrentFreq').textContent = s.current_freq.toFixed(2) + ' Hz';
        document.getElementById('virtualTargetFreq').textContent = s.target_freq.toFixed(2) + ' Hz';
        document.getElementById('virtualDeviation').textContent = deviationCents.toFixed(1) + ' ¢';
        document.getElementById('virtualTotalDepth').textContent = s.total_depth_mm.toFixed(2) + ' mm';

        const maxDeviation = 50;
        const normalizedDeviation = Math.min(Math.abs(deviationCents) / maxDeviation, 1);
        const fillWidth = (1 - normalizedDeviation) * 100;
        const markerLeft = 50 + (deviationCents / maxDeviation) * 50;

        document.getElementById('toleranceFill').style.width = fillWidth + '%';
        document.getElementById('toleranceMarker').style.left = markerLeft + '%';

        const statusEl = document.getElementById('toleranceStatus');
        if (Math.abs(deviationCents) <= tolerance) {
            statusEl.textContent = '✅ 已达目标音高';
            statusEl.className = 'tolerance-status pass';
        } else if (Math.abs(deviationCents) <= tolerance * 2) {
            statusEl.textContent = '⚠️ 接近目标';
            statusEl.className = 'tolerance-status warn';
        } else {
            statusEl.textContent = '🔧 需要继续调整';
            statusEl.className = 'tolerance-status active';
        }
    }

    renderVirtualHistory() {
        const container = document.getElementById('virtualHistoryList');
        if (!this.virtualSession || this.virtualSession.history.length === 0) {
            container.innerHTML = '<p class="placeholder">暂无操作历史</p>';
            return;
        }

        const processLabels = {
            'grinding': '🪨 磨锉',
            'casting_inlay': '🔩 铸镶',
            'welding_repair': '🔥 焊补'
        };

        let html = '<table class="history-table"><thead><tr><th>时间</th><th>工艺</th><th>深度</th><th>位置</th><th>频率变化</th><th>偏差</th></tr></thead><tbody>';
        
        this.virtualSession.history.slice().reverse().forEach(h => {
            const time = new Date(h.time).toLocaleTimeString('zh-CN');
            const deltaHz = h.after_freq - h.before_freq;
            html += `
                <tr>
                    <td>${time}</td>
                    <td>${processLabels[h.process_type] || h.process_type}</td>
                    <td>${h.depth_mm.toFixed(2)} mm</td>
                    <td>(${h.position.x.toFixed(1)}, ${h.position.y.toFixed(1)}, ${h.position.z.toFixed(1)})</td>
                    <td class="${deltaHz > 0 ? 'positive' : 'negative'}">
                        ${deltaHz > 0 ? '+' : ''}${deltaHz.toFixed(2)} Hz
                    </td>
                    <td>${h.deviation.toFixed(1)} ¢</td>
                </tr>
            `;
        });

        html += '</tbody></table>';
        container.innerHTML = html;
    }
}

window.addEventListener('DOMContentLoaded', () => {
    window.app = new App();
});
