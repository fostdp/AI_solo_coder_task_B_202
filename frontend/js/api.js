const API_BASE = '/api';

class API {
    static async getBells() {
        const res = await fetch(`${API_BASE}/bells`);
        return res.json();
    }

    static async getBell(id) {
        const res = await fetch(`${API_BASE}/bells/${id}`);
        return res.json();
    }

    static async getMeasurements(bellId, limit = 100) {
        const res = await fetch(`${API_BASE}/bells/${bellId}/measurements?limit=${limit}`);
        return res.json();
    }

    static async postMeasurement(data) {
        const res = await fetch(`${API_BASE}/measurements`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        return res.json();
    }

    static async getGrindingOperations(bellId) {
        const res = await fetch(`${API_BASE}/bells/${bellId}/grinding`);
        return res.json();
    }

    static async runSimulation(bellId, grindingOps = []) {
        const res = await fetch(`${API_BASE}/bells/${bellId}/simulation`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ simulation_type: 'modal_analysis', grinding_operations: grindingOps })
        });
        return res.json();
    }

    static async getPitchCorrection(bellId, currentFreq) {
        const res = await fetch(`${API_BASE}/bells/${bellId}/correction?current_freq=${currentFreq}`);
        return res.json();
    }

    static async getAlerts(bellId = null, limit = 50) {
        let url = `${API_BASE}/alerts?limit=${limit}`;
        if (bellId) url += `&bell_id=${bellId}`;
        const res = await fetch(url);
        return res.json();
    }

    static async getDashboardStats() {
        const res = await fetch(`${API_BASE}/dashboard/stats`);
        return res.json();
    }

    static async getTuningProcesses() {
        const res = await fetch(`${API_BASE}/processes`);
        return res.json();
    }

    static async compareTuningProcesses(bellId, currentFreq, targetFreq) {
        const res = await fetch(`${API_BASE}/processes/compare`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ bell_id: bellId, current_freq: currentFreq, target_freq: targetFreq })
        });
        return res.json();
    }

    static async getProcessStats() {
        const res = await fetch(`${API_BASE}/processes/stats`);
        return res.json();
    }

    static async getEmpiricalRules() {
        const res = await fetch(`${API_BASE}/rules`);
        return res.json();
    }

    static async validateEmpiricalRule(ruleId, params) {
        const res = await fetch(`${API_BASE}/rules/validate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ rule_id: ruleId, params: params })
        });
        return res.json();
    }

    static async getComparisonArticles(category = null) {
        let url = `${API_BASE}/comparisons`;
        if (category) url += `?category=${category}`;
        const res = await fetch(url);
        return res.json();
    }

    static async startVirtualTuning(bellId) {
        const res = await fetch(`${API_BASE}/virtual/start`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ bell_id: bellId })
        });
        return res.json();
    }

    static async getVirtualSession(sessionId) {
        const res = await fetch(`${API_BASE}/virtual/${sessionId}`);
        return res.json();
    }

    static async virtualTuningGrind(sessionId, position, depthMm, processType = 'grinding') {
        const res = await fetch(`${API_BASE}/virtual/${sessionId}/grind`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ position: position, depth_mm: depthMm, process_type: processType })
        });
        return res.json();
    }

    static async virtualTuningPlay(sessionId) {
        const res = await fetch(`${API_BASE}/virtual/${sessionId}/play`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });
        return res.json();
    }

    static async virtualTuningReset(sessionId) {
        const res = await fetch(`${API_BASE}/virtual/${sessionId}/reset`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });
        return res.json();
    }
}
