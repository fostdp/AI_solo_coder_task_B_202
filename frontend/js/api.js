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
}
