class App {
    constructor() {
        this.viewer = null;
        this.bells = [];
        this.currentBell = null;
        this.measurements = [];
        this.grindingOps = [];
        this.alerts = [];
        this.frequencyChart = null;
        this.spectrumChart = null;
        this.ws = null;

        this.init();
    }

    init() {
        this.viewer = new Bell3DViewer('bellViewer');
        this.initCharts();
        this.bindEvents();
        this.loadData();
        this.initWebSocket();
        this.startStatsRefresh();
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
}

window.addEventListener('DOMContentLoaded', () => {
    window.app = new App();
});
