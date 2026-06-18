#!/usr/bin/env python3
"""
Bianzhong (编钟) Acoustic Tuning Simulator
==========================================

Simulates physical bells with configurable:
  - target pitch (frequency in Hz or musical note name)
  - grinding positions (angular + axial zones)
  - grinding depth, rate, noise, measurement error

Publishes synthetic acoustic measurements to the backend API and
optionally performs grinding operations, simulating the full
measurement -> analysis -> grinding feedback loop.

Usage:
    python bell_simulator.py                          # Default run
    TARGET_FREQ=256 python bell_simulator.py          # Middle C
    GRIND_POSITIONS="0,45,90" python bell_simulator.py

Environment Variables:
    API_BASE_URL             Backend base  (default: http://bianzhong-server:8080)
    MQTT_BROKER              MQTT broker   (default: mqtt-broker:1883)
    BELL_ID                  Bell id       (default: bell-sim-01)
    BELL_NAME                Display name  (default: 仿真编钟-Sim01)
    BELL_DIAMETER_CM         Bell diameter (default: 40.0)
    BELL_HEIGHT_CM           Bell height   (default: 80.0)
    BELL_THICKNESS_MM        Thickness     (default: 15.0)

    TARGET_FREQ_HZ           Target frequency in Hz (default: 329.63, E4)
    TARGET_NOTE              Musical note (e.g. C4, E4, A4). Overrides TARGET_FREQ_HZ.
    TARGET_TOLERANCE_CENTS   Acceptance window in cents (default: 10)

    MEASUREMENT_INTERVAL_S   Seconds between measurements (default: 10)
    MEASUREMENT_NOISE_CENTS  Gaussian noise in cents on measurements (default: 3)
    TEMPERATURE_C            Ambient temperature (default: 22)
    HUMIDITY_PCT             Ambient humidity percentage (default: 45)

    GRIND_ENABLED            1 = enable auto grinding loop (default: 1)
    GRIND_INTERVAL_MEASUREMENTS  N measurements between each grind (default: 3)
    GRIND_POSITIONS          Comma-sep angular positions in degrees (default: 0,45,90,135,180,225,270,315)
    GRIND_HEIGHTS            Comma-sep normalized heights 0-1 (default: 0.3,0.5,0.7)
    GRIND_DEPTH_MM           Per-grind depth in mm (default: 0.15)
    GRIND_DEPTH_RANDOM       +/- random jitter fraction (default: 0.3)
    INITIAL_OFFSET_CENTS     Starting detune in cents (default: +45)

    AUTO_TUNE                1 = request pitch-correction plan from backend (default: 1)
    SESSION_SECONDS          Stop after N seconds, 0 = infinite (default: 0)
    VERBOSE                  1 = debug logs (default: 0)
"""

from __future__ import annotations

import os
import sys
import json
import time
import math
import random
import signal
import logging
import threading
from dataclasses import dataclass, field, asdict
from typing import Optional

import requests
import paho.mqtt.client as mqtt

# ============================================================
# Musical note frequency lookup (A4 = 440 Hz)
# ============================================================
NOTE_OFFSETS = {
    "C": -9, "C#": -8, "Db": -8, "D": -7, "D#": -6, "Eb": -6,
    "E": -5, "F": -4, "F#": -3, "Gb": -3, "G": -2, "G#": -1,
    "Ab": -1, "A": 0, "A#": 1, "Bb": 1, "B": 2,
}

def note_to_freq(note: str) -> float:
    """e.g. 'E4' -> 329.6275569128699 Hz"""
    n = note.strip().capitalize()
    name = n[:-1]
    try:
        octave = int(n[-1])
    except ValueError:
        octave = 4
        name = n
    if name not in NOTE_OFFSETS:
        raise ValueError(f"Unknown note name: {name}")
    semitones = NOTE_OFFSETS[name] + 12 * (octave - 4)
    return 440.0 * (2 ** (semitones / 12))

def env(name, default, cast=str):
    v = os.getenv(name)
    if v is None or v == "":
        return default
    try:
        return cast(v)
    except (TypeError, ValueError):
        return default

# ============================================================
# Logging
# ============================================================
logging.basicConfig(
    level=logging.DEBUG if env("VERBOSE", "0", int) else logging.INFO,
    format="%(asctime)s | %(levelname)-7s | %(name)s | %(message)s",
)
log = logging.getLogger("bell-sim")

# ============================================================
# Configuration
# ============================================================
API_BASE = env("API_BASE_URL", "http://bianzhong-server:8080")
MQTT_HOST, MQTT_PORT = (
    env("MQTT_BROKER", "mqtt-broker:1883").split(":")[0],
    int(env("MQTT_BROKER", "mqtt-broker:1883").split(":")[-1]),
)

@dataclass
class BellSpec:
    id:          str = env("BELL_ID", "bell-sim-01")
    name:        str = env("BELL_NAME", "仿真编钟-Sim01")
    diameter_cm: float = env("BELL_DIAMETER_CM", 40.0, float)
    height_cm:   float = env("BELL_HEIGHT_CM", 80.0, float)
    thickness_mm:float = env("BELL_THICKNESS_MM", 15.0, float)

    target_freq_hz:    float = field(init=False)
    tolerance_cents:   float = env("TARGET_TOLERANCE_CENTS", 10.0, float)

    measure_interval_s:float = env("MEASUREMENT_INTERVAL_S", 10.0, float)
    noise_cents:       float = env("MEASUREMENT_NOISE_CENTS", 3.0, float)
    temperature_c:     float = env("TEMPERATURE_C", 22.0, float)
    humidity_pct:      float = env("HUMIDITY_PCT", 45.0, float)

    grind_enabled:     bool  = env("GRIND_ENABLED", "1", lambda s: bool(int(s)))
    grind_interval_m:  int   = env("GRIND_INTERVAL_MEASUREMENTS", 3, int)
    grind_depth_mm:    float = env("GRIND_DEPTH_MM", 0.15, float)
    grind_depth_rand:  float = env("GRIND_DEPTH_RANDOM", 0.3, float)
    grind_positions:   list  = field(default_factory=list)
    grind_heights:     list  = field(default_factory=list)

    initial_offset_cents: float = env("INITIAL_OFFSET_CENTS", 45.0, float)
    auto_tune:         bool  = env("AUTO_TUNE", "1", lambda s: bool(int(s)))
    session_seconds:   float = env("SESSION_SECONDS", 0.0, float)

    def __post_init__(self):
        note = os.getenv("TARGET_NOTE")
        if note:
            try:
                self.target_freq_hz = note_to_freq(note)
                log.info("Target note %s -> %.2f Hz", note, self.target_freq_hz)
            except ValueError as e:
                log.warning("Bad TARGET_NOTE (%s): %s. Falling back to TARGET_FREQ_HZ", note, e)
                self.target_freq_hz = env("TARGET_FREQ_HZ", 329.63, float)
        else:
            self.target_freq_hz = env("TARGET_FREQ_HZ", 329.63, float)

        gp = env("GRIND_POSITIONS", "0,45,90,135,180,225,270,315")
        gh = env("GRIND_HEIGHTS", "0.3,0.5,0.7")
        self.grind_positions = [float(x) for x in gp.split(",") if x.strip()]
        self.grind_heights   = [float(x) for x in gh.split(",") if x.strip()]

# ============================================================
# Bell physics (high-level empirical model)
# ============================================================
class VirtualBell:
    """
    Empirical tuning model — NOT a substitute for the backend FEM.
    Models:
      - thickness-reduces reduces frequency by factor (1 - delta_t/t)^0.75
      - each grind zone has an efficiency coefficient based on position
      - hysteresis: real grinding effect ~ 95% of theoretical
    """
    HARMONIC_RATIOS = [1.0, 2.0, 3.0, 4.16, 5.42, 6.78, 8.15, 9.63]

    def __init__(self, spec: BellSpec):
        self.spec = spec
        self.current_thickness_mm = spec.thickness_mm
        self.base_frequency_hz = (
            spec.target_freq_hz * (2 ** (spec.initial_offset_cents / 1200))
        )
        self.zone_efficiency = {}
        self._calibrate_zones()
        self.total_grinded_mm = 0.0
        self.grind_count = 0

    def _calibrate_zones(self):
        for ang in self.spec.grind_positions:
            for h in self.spec.grind_heights:
                axial_eff = 1.0 - 0.4 * abs(h - 0.5)
                radial_eff = 0.8 + 0.4 * math.sin(math.radians(ang)) ** 2
                key = (round(ang, 2), round(h, 3))
                self.zone_efficiency[key] = axial_eff * radial_eff

    def current_freq(self) -> float:
        rel_thickness = self.current_thickness_mm / self.spec.thickness_mm
        return self.base_frequency_hz * (rel_thickness ** 0.75)

    def apply_grind(self, angle_deg: float, height_ratio: float, depth_mm: float) -> dict:
        key = (round(angle_deg % 360, 2), round(max(0.0, min(1.0, height_ratio)), 3))
        eff = self.zone_efficiency.get(key, 0.7)
        effective_depth = depth_mm * eff * 0.95

        old_freq = self.current_freq()
        new_thickness = max(
            self.spec.thickness_mm * 0.4,
            self.current_thickness_mm - effective_depth,
        )
        actual_depth = self.current_thickness_mm - new_thickness
        self.current_thickness_mm = new_thickness
        new_freq = self.current_freq()
        delta_hz = new_freq - old_freq
        self.total_grinded_mm += actual_depth
        self.grind_count += 1

        return {
            "angle_deg":      key[0],
            "height_ratio":   key[1],
            "requested_mm":   depth_mm,
            "actual_mm":      actual_depth,
            "efficiency":     eff,
            "removed_mass_g": actual_depth * 4.2,
            "predicted_shift_hz": delta_hz,
            "thickness_before_mm": self.current_thickness_mm + actual_depth,
            "thickness_after_mm":  self.current_thickness_mm,
        }

    def harmonic_freqs(self) -> list[float]:
        f0 = self.current_freq()
        return [f0 * r for r in self.HARMONIC_RATIOS]

# ============================================================
# Backend + MQTT Client
# ============================================================
class BackendClient:
    def __init__(self, api_base: str, mqtt_host: str, mqtt_port: int):
        self.api_base = api_base.rstrip("/")
        self.session = requests.Session()
        self.session.headers.update({"Content-Type": "application/json"})

        self.mqtt_client = mqtt.Client(
            client_id=f"bell-sim-{os.getpid()}",
            clean_session=True,
            protocol=mqtt.MQTTv311,
        )
        self.mqtt_client.on_connect = self._on_mqtt_connect
        self.mqtt_client.on_message = self._on_mqtt_message
        self._mqtt_host = mqtt_host
        self._mqtt_port = mqtt_port
        self._received_alerts = []

    def wait_for_api(self, timeout_s: float = 120.0):
        log.info("Waiting for backend API at %s ...", self.api_base)
        deadline = time.time() + timeout_s
        while time.time() < deadline:
            try:
                r = self.session.get(f"{self.api_base}/api/healthz", timeout=2)
                if r.ok:
                    log.info("Backend is up: %s", r.json())
                    return True
            except (requests.RequestException, ValueError):
                pass
            time.sleep(2)
        log.error("Backend did not become available within %ss", timeout_s)
        return False

    def connect_mqtt(self, timeout_s: float = 60.0):
        log.info("Connecting MQTT to %s:%d ...", self._mqtt_host, self._mqtt_port)
        try:
            self.mqtt_client.connect_async(self._mqtt_host, self._mqtt_port, keepalive=60)
            self.mqtt_client.loop_start()
            deadline = time.time() + timeout_s
            while time.time() < deadline:
                if self.mqtt_client.is_connected():
                    log.info("MQTT connected, subscribing to alerts")
                    self.mqtt_client.subscribe("bianzhong/alerts/#", qos=1)
                    return True
                time.sleep(1)
        except Exception as e:
            log.error("MQTT connect failed: %s", e)
        return False

    def _on_mqtt_connect(self, client, userdata, flags, rc):
        if rc == 0:
            client.subscribe("bianzhong/alerts/#", qos=1)

    def _on_mqtt_message(self, client, userdata, msg):
        try:
            payload = json.loads(msg.payload.decode())
            self._received_alerts.append((time.time(), msg.topic, payload))
            log.info("MQTT ALERT [%s] %s", msg.topic, payload.get("alert_type"))
        except Exception as e:
            log.debug("MQTT msg parse failed: %s", e)

    def register_bell(self, spec: BellSpec):
        bells = self.session.get(f"{self.api_base}/api/bells", timeout=5).json()
        for b in bells:
            if b.get("bell_id") == spec.id:
                log.info("Bell %s already registered", spec.id)
                return b
        body = {
            "bell_id":           spec.id,
            "name":              spec.name,
            "diameter_cm":       spec.diameter_cm,
            "height_cm":         spec.height_cm,
            "thickness_mm":      spec.thickness_mm,
            "target_frequency_hz": spec.target_freq_hz,
            "tolerance_cents":   spec.tolerance_cents,
            "material":          "bronze",
            "location":          "simulator-lab-01",
            "status":            "active",
        }
        r = self.session.post(f"{self.api_base}/api/bells", json=body, timeout=10)
        if r.ok:
            log.info("Registered bell %s -> %s", spec.name, spec.id)
            return body
        log.warning("Bell registration response %d: %s", r.status_code, r.text[:200])
        return body

    def send_measurement(self, spec: BellSpec, bell: VirtualBell, modes: list = None) -> dict:
        if modes is None:
            modes = bell.harmonic_freqs()
        payload = {
            "bell_id":        spec.id,
            "ts":             time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "measurements":   [],
            "temperature_c":  spec.temperature_c + random.gauss(0, 0.3),
            "humidity_pct":   spec.humidity_pct + random.gauss(0, 0.5),
            "operator_id":    "simulator",
            "sensor_id":      f"mic-{spec.id}",
        }
        for i, f in enumerate(modes):
            noise_cents = random.gauss(0, spec.noise_cents)
            measured = f * (2 ** (noise_cents / 1200))
            target = spec.target_freq_hz * (bell.HARMONIC_RATIOS[i] if i < len(bell.HARMONIC_RATIOS) else 1)
            dev_cents = 1200 * math.log2(measured / target) if target > 0 else 0.0
            payload["measurements"].append({
                "mode_order":       i,
                "frequency_hz":     round(measured, 4),
                "amplitude_db":     round(80 - 5 * i - random.gauss(0, 1.2), 2),
                "deviation_cents":  round(dev_cents, 2),
                "snr_db":           round(65 - i * 3 + random.gauss(0, 1), 2),
            })
        try:
            r = self.session.post(f"{self.api_base}/api/measurements", json=payload, timeout=10)
            if r.ok:
                return r.json()
            log.warning("measurement POST %d: %s", r.status_code, r.text[:200])
        except requests.RequestException as e:
            log.error("measurement POST error: %s", e)
        return {}

    def send_grinding(self, spec: BellSpec, info: dict) -> dict:
        payload = {
            "bell_id":       spec.id,
            "ts":            time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "position": {
                "zone":        f"zone-A{info['angle_deg']:.0f}-H{int(info['height_ratio']*100)}",
                "angle_deg":   info["angle_deg"],
                "height_cm":   info["height_ratio"] * spec.height_cm,
                "radius_cm":   spec.diameter_cm * 0.5 * 0.8,
            },
            "depth_mm":                round(info["requested_mm"], 4),
            "removed_mass_g":          round(info["removed_mass_g"], 4),
            "predicted_frequency_shift_hz": round(info["predicted_shift_hz"], 4),
            "tool_id":                 "grinder-head-sim",
            "operator_id":             "simulator",
            "thickness_before_mm":     round(info["thickness_before_mm"], 3),
            "thickness_after_mm":      round(info["thickness_after_mm"], 3),
        }
        try:
            r = self.session.post(f"{self.api_base}/api/grinding", json=payload, timeout=10)
            if r.ok:
                return r.json()
            log.warning("grinding POST %d: %s", r.status_code, r.text[:200])
        except requests.RequestException as e:
            log.error("grinding POST error: %s", e)
        return {}

    def request_correction_plan(self, spec: BellSpec) -> list[dict]:
        if not spec.auto_tune:
            return []
        try:
            r = self.session.get(
                f"{self.api_base}/api/bells/{spec.id}/correction",
                params={"target_frequency_hz": spec.target_freq_hz},
                timeout=30,
            )
            if r.ok:
                data = r.json()
                plan = data.get("recommended_positions", [])
                if plan:
                    log.info("Backend tuning plan: %d positions", len(plan))
                    for p in plan[:3]:
                        log.info("  -> %.0f° / H%.0f : %.3f mm",
                                 p.get("position", {}).get("angle_deg"),
                                 p.get("position", {}).get("height_cm"),
                                 p.get("recommended_depth_mm"))
                return plan
        except requests.RequestException as e:
            log.debug("correction request error: %s", e)
        return []

# ============================================================
# Simulator control loop
# ============================================================
class Simulator:
    def __init__(self):
        self.spec = BellSpec()
        self.bell = VirtualBell(self.spec)
        self.client = BackendClient(API_BASE, MQTT_HOST, MQTT_PORT)
        self._stop = threading.Event()
        self.stats = {
            "measurements_sent": 0,
            "grinds_performed":  0,
            "alerts_received":   0,
            "start_ts":          time.time(),
        }

        signal.signal(signal.SIGINT, self._handle_signal)
        signal.signal(signal.SIGTERM, self._handle_signal)

    def _handle_signal(self, signum, frame):
        log.info("Caught signal %s — shutting down", signum)
        self._stop.set()

    def startup(self) -> bool:
        self._print_banner()
        if not self.client.wait_for_api():
            return False
        self.client.register_bell(self.spec)
        self.client.connect_mqtt()
        return True

    def _print_banner(self):
        width = 60
        print("=" * width)
        print("  编钟调音模拟器  |  Bianzhong Tuning Simulator")
        print("=" * width)
        print(f"  Bell ID       : {self.spec.id}")
        print(f"  Name          : {self.spec.name}")
        print(f"  Size          : {self.spec.diameter_cm}cm x {self.spec.height_cm}cm")
        print(f"  Target freq   : {self.spec.target_freq_hz:.2f} Hz"
              f" (tolerance ±{self.spec.tolerance_cents} cents)")
        print(f"  Initial detune: +{self.spec.initial_offset_cents:.0f} cents "
              f"-> start {self.bell.current_freq():.2f} Hz")
        print(f"  Grind zones   : {len(self.spec.grind_positions)} angles x "
              f"{len(self.spec.grind_heights)} heights")
        print(f"  Auto tune     : {'ON' if self.spec.auto_tune else 'OFF'}")
        print("=" * width)

    def _choose_grind_location(self) -> tuple[float, float, float]:
        plan = self.client.request_correction_plan(self.spec)
        if plan:
            best = min(plan, key=lambda p: -abs(p.get("sensitivity_hz_mm", 0)))
            pos = best.get("position", {})
            depth = min(
                self.spec.grind_depth_mm * 1.5,
                best.get("recommended_depth_mm", self.spec.grind_depth_mm),
            )
            return (pos.get("angle_deg", 0),
                    max(0, min(1, pos.get("height_cm", 0) / max(1, self.spec.height_cm))),
                    depth)
        ang = random.choice(self.spec.grind_positions)
        h   = random.choice(self.spec.grind_heights)
        dep = self.spec.grind_depth_mm * (
            1 + random.uniform(-self.spec.grind_depth_rand, self.spec.grind_depth_rand)
        )
        return ang, h, max(0.02, dep)

    def run(self):
        if not self.startup():
            sys.exit(1)

        last_correction_req = 0.0
        measurements_since_grind = 0

        while not self._stop.is_set():
            current = self.bell.current_freq()
            target = self.spec.target_freq_hz
            dev_cents = 1200 * math.log2(current / target)
            within_tol = abs(dev_cents) <= self.spec.tolerance_cents

            meas_resp = self.client.send_measurement(self.spec, self.bell)
            self.stats["measurements_sent"] += 1
            self.client.alerts_count = len(self.client._received_alerts)

            log.info(
                "f0=%.2fHz | target=%.2fHz | Δ=%+.1fcents %s | thk=%.2fmm | meas#%d",
                current, target, dev_cents,
                "✅" if within_tol else ("🔺" if abs(dev_cents) <= 2*self.spec.tolerance_cents else "🔴"),
                self.bell.current_thickness_mm,
                self.stats["measurements_sent"],
            )

            if within_tol and self.stats["grinds_performed"] > 0:
                log.info("🏆 Target reached! Bell is in tune within tolerance.")
                if self.spec.session_seconds <= 0:
                    log.info("Session complete — exiting")
                    break

            measurements_since_grind += 1
            if (
                self.spec.grind_enabled
                and not within_tol
                and measurements_since_grind >= self.spec.grind_interval_m
            ):
                ang, h, dep = self._choose_grind_location()
                info = self.bell.apply_grind(ang, h, dep)
                resp = self.client.send_grinding(self.spec, info)
                self.stats["grinds_performed"] += 1
                measurements_since_grind = 0
                log.info(
                    "⛏️  Grind #%d | %.0f° H%.0f%% | %.3fmm "
                    "(actual %.3fmm, eff=%.2f, Δfreq=%+.2fHz)",
                    self.stats["grinds_performed"],
                    info["angle_deg"], info["height_ratio"]*100,
                    info["requested_mm"], info["actual_mm"],
                    info["efficiency"], info["predicted_shift_hz"],
                )

            if time.time() - last_correction_req > 60:
                self.client.request_correction_plan(self.spec)
                last_correction_req = time.time()

            # Sleep in small increments so stop signal is responsive
            slept = 0.0
            while slept < self.spec.measure_interval_s and not self._stop.is_set():
                chunk = min(0.5, self.spec.measure_interval_s - slept)
                time.sleep(chunk)
                slept += chunk

            if 0 < self.spec.session_seconds < (time.time() - self.stats["start_ts"]):
                log.info("Session time limit reached")
                break

        self.shutdown()

    def shutdown(self):
        self.client.mqtt_client.loop_stop()
        self.client.mqtt_client.disconnect()
        elapsed = time.time() - self.stats["start_ts"]
        print("\n" + "=" * 60)
        print("  Session Summary")
        print("=" * 60)
        print(f"  Duration            : {elapsed:.1f}s")
        print(f"  Final frequency     : {self.bell.current_freq():.2f} Hz")
        print(f"  Target              : {self.spec.target_freq_hz:.2f} Hz"
              f" ({1200 * math.log2(self.bell.current_freq() / self.spec.target_freq_hz):+.1f} cents)")
        print(f"  Measurements sent   : {self.stats['measurements_sent']}")
        print(f"  Grinds performed    : {self.stats['grinds_performed']}")
        print(f"  Total metal removed : {self.bell.total_grinded_mm:.2f} mm (cumulative)")
        print(f"  Alerts received     : {len(self.client._received_alerts)}")
        print("=" * 60)

# ============================================================
def main():
    try:
        Simulator().run()
    except KeyboardInterrupt:
        log.info("Interrupted by user")

if __name__ == "__main__":
    main()
