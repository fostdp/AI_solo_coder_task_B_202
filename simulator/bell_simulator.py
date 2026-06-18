#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
古代编钟调音磨锉声学仿真与音高修正系统
编钟声学传感器模拟器

模拟每件编钟每1分钟通过传感器上报:
- 基频 (fundamental frequency)
- 泛音列 (overtone frequencies and amplitudes)
- 磨锉位置 (grinding position)
- 磨削量 (grinding depth)

用法:
    python bell_simulator.py                    # 使用默认设置
    python bell_simulator.py --interval 10      # 每10秒上报一次 (演示用)
    python bell_simulator.py --bell-id 3        # 只模拟特定编钟
    python bell_simulator.py --deviation 15     # 设置初始偏差(音分)
    python bell_simulator.py --fast             # 快速演示模式 (5秒间隔)
"""

import argparse
import json
import math
import random
import sys
import time
from datetime import datetime, timezone
from typing import Dict, List, Tuple

try:
    import requests
except ImportError:
    print("Error: requests library not installed.")
    print("Install with: pip install requests")
    sys.exit(1)


class BellAcousticSimulator:
    def __init__(self, api_base: str = "http://localhost:8080/api"):
        self.api_base = api_base
        self.bells: List[Dict] = []
        self.bell_states: Dict[int, Dict] = {}
        self.grinding_history: Dict[int, List[Dict]] = {}

    def load_bells(self) -> None:
        try:
            resp = requests.get(f"{self.api_base}/bells", timeout=5)
            resp.raise_for_status()
            self.bells = resp.json()
            print(f"✓ 成功加载 {len(self.bells)} 件编钟")
        except Exception as e:
            print(f"✗ 加载编钟列表失败: {e}")
            print("请确保后端服务已启动")
            sys.exit(1)

    def init_bell_state(self, bell: Dict, initial_deviation: float = None) -> None:
        bell_id = bell["id"]
        target_freq = bell["target_frequency"]

        if initial_deviation is None:
            initial_deviation = random.uniform(-30, 30)

        initial_freq = target_freq * (2 ** (initial_deviation / 1200))

        self.bell_states[bell_id] = {
            "current_freq": initial_freq,
            "total_grinded_mm": 0.0,
            "initial_deviation_cents": initial_deviation,
            "deviation_cents": initial_deviation,
            "temperature": 20.0 + random.uniform(-2, 2),
            "humidity": 50.0 + random.uniform(-10, 10),
            "consecutive_measurements": 0,
            "grinding_schedule": self._generate_grinding_schedule(bell),
            "current_grind_step": 0
        }

        self.grinding_history[bell_id] = []
        print(f"  [{bell['serial_number']}] {bell['name']}: "
              f"目标 {target_freq:.2f} Hz, "
              f"初始 {initial_freq:.2f} Hz ({initial_deviation:+.1f} 音分)")

    def _generate_grinding_schedule(self, bell: Dict) -> List[Dict]:
        schedule = []
        num_steps = random.randint(2, 5)
        height_cm = bell.get("height_cm", 80)
        radius_cm = bell.get("diameter_cm", 40) / 2.0

        for i in range(num_steps):
            y_ratio = random.uniform(0.15, 0.85)
            theta = random.uniform(0, 2 * math.pi)
            r_ratio = random.uniform(0.7, 0.85)
            r = radius_cm * r_ratio

            schedule.append({
                "position": {
                    "x": r * math.cos(theta),
                    "y": y_ratio * height_cm,
                    "z": r * math.sin(theta)
                },
                "depth_mm": round(random.uniform(0.1, 0.5), 3),
                "trigger_measurement": random.randint(3, 8)
            })

        return schedule

    def generate_overtones(self, fundamental: float, material_factor: float = 1.0) -> Tuple[List[float], List[float]]:
        harmonic_ratios = [2.0, 3.01, 4.16, 5.42, 6.78, 8.15, 9.63]
        overtone_freqs = []
        overtone_amplitudes = []

        for i, ratio in enumerate(harmonic_ratios):
            inharmonicity = 1 + random.gauss(0, 0.005) * material_factor
            freq = fundamental * ratio * inharmonicity
            overtone_freqs.append(round(freq, 3))

            base_amplitude = math.exp(-i * 0.4)
            amplitude = base_amplitude * (1 + random.gauss(0, 0.05))
            overtone_amplitudes.append(round(max(0.01, amplitude), 4))

        return overtone_freqs, overtone_amplitudes

    def generate_measurement(self, bell_id: int, bell: Dict) -> Dict:
        state = self.bell_states[bell_id]

        drift = random.gauss(0, 0.002) * state["current_freq"]
        noise = random.gauss(0, 0.005) * state["current_freq"]
        state["current_freq"] += drift

        measured_freq = state["current_freq"] + noise
        target_freq = bell["target_frequency"]
        deviation_cents = 1200 * math.log2(measured_freq / target_freq)
        state["deviation_cents"] = deviation_cents

        overtone_freqs, overtone_amplitudes = self.generate_overtones(
            measured_freq,
            material_factor=1.0 + state["total_grinded_mm"] * 0.05
        )

        state["temperature"] += random.gauss(0, 0.1)
        state["humidity"] += random.gauss(0, 0.5)
        state["temperature"] = max(10, min(35, state["temperature"]))
        state["humidity"] = max(20, min(80, state["humidity"]))

        state["consecutive_measurements"] += 1

        measurement = {
            "time": datetime.now(timezone.utc).isoformat(),
            "bell_id": bell_id,
            "fundamental_freq": round(measured_freq, 4),
            "overtone_freqs": overtone_freqs,
            "overtone_amplitudes": overtone_amplitudes,
            "temperature": round(state["temperature"], 2),
            "humidity": round(state["humidity"], 2),
            "sensor_id": f"SENSOR-{bell['serial_number']}",
            "deviation_cents": round(deviation_cents, 3)
        }

        return measurement

    def maybe_apply_grinding(self, bell_id: int, bell: Dict) -> Dict:
        state = self.bell_states[bell_id]
        schedule = state["grinding_schedule"]

        if state["current_grind_step"] >= len(schedule):
            return None

        step = schedule[state["current_grind_step"]]

        if state["consecutive_measurements"] < step["trigger_measurement"]:
            return None

        target_freq = bell["target_frequency"]
        deviation = state["deviation_cents"]

        if abs(deviation) < bell.get("tolerance_cents", 5.0):
            return None

        depth_factor = min(1.0, abs(deviation) / 20.0)
        actual_depth = step["depth_mm"] * depth_factor

        if deviation > 0:
            freq_change = -actual_depth * (target_freq * 0.008)
        else:
            freq_change = actual_depth * (target_freq * 0.003)

        before_freq = state["current_freq"]
        state["current_freq"] += freq_change
        after_freq = state["current_freq"]

        state["total_grinded_mm"] += actual_depth
        state["consecutive_measurements"] = 0
        state["current_grind_step"] += 1

        grinding_op = {
            "time": datetime.now(timezone.utc).isoformat(),
            "bell_id": bell_id,
            "position": step["position"],
            "grinding_depth_mm": round(actual_depth, 4),
            "grinding_area": round(math.pi * 1.5 ** 2, 2),
            "operator_id": "SIM-OPERATOR-001",
            "before_frequency": round(before_freq, 4),
            "after_frequency": round(after_freq, 4),
            "predicted_frequency": round(before_freq + freq_change, 4),
            "notes": f"自动调音步骤 {state['current_grind_step']}/{len(schedule)}"
        }

        self.grinding_history[bell_id].append(grinding_op)
        return grinding_op

    def send_measurement(self, measurement: Dict) -> bool:
        try:
            resp = requests.post(
                f"{self.api_base}/measurements",
                json=measurement,
                timeout=5
            )
            resp.raise_for_status()
            return True
        except Exception as e:
            print(f"    ✗ 上报测量数据失败: {e}")
            return False

    def send_grinding(self, grinding_op: Dict) -> bool:
        try:
            resp = requests.post(
                f"{self.api_base}/grinding",
                json=grinding_op,
                timeout=5
            )
            resp.raise_for_status()
            return True
        except Exception as e:
            print(f"    ✗ 上报磨锉操作失败: {e}")
            return False

    def print_status(self, bell: Dict, measurement: Dict) -> None:
        deviation = measurement["deviation_cents"]
        tolerance = bell.get("tolerance_cents", 5.0)

        if abs(deviation) > 2 * tolerance:
            status_icon = "🚨"
        elif abs(deviation) > tolerance:
            status_icon = "⚠️"
        else:
            status_icon = "✓"

        state = self.bell_states[bell["id"]]

        print(f"  [{bell['serial_number']}] {status_icon} "
              f"{measurement['fundamental_freq']:.2f} Hz "
              f"({deviation:+.1f} 音分) | "
              f"已磨削 {state['total_grinded_mm']:.2f}mm | "
              f"温度 {measurement['temperature']:.1f}°C")

    def print_grinding(self, bell: Dict, grinding: Dict) -> None:
        print(f"  🔧 [{bell['serial_number']}] 磨锉操作: "
              f"位置({grinding['position']['x']:.1f}, "
              f"{grinding['position']['y']:.1f}, "
              f"{grinding['position']['z']:.1f})cm | "
              f"深度 {grinding['grinding_depth_mm']:.3f}mm | "
              f"{grinding['before_frequency']:.2f} → {grinding['after_frequency']:.2f} Hz")

    def run(self, interval: int = 60, bell_id: int = None,
            initial_deviation: float = None, max_cycles: int = None) -> None:
        print("=" * 70)
        print("  古代编钟调音磨锉声学仿真系统 - 传感器模拟器")
        print("=" * 70)

        self.load_bells()
        print()

        target_bells = [b for b in self.bells if bell_id is None or b["id"] == bell_id]
        if not target_bells:
            print(f"✗ 未找到ID为 {bell_id} 的编钟")
            sys.exit(1)

        print("初始化编钟状态:")
        for bell in target_bells:
            self.init_bell_state(bell, initial_deviation)
        print()

        print(f"开始模拟 - 上报间隔: {interval}秒")
        if max_cycles:
            print(f"运行周期: {max_cycles} 次")
        print("-" * 70)

        cycle = 0
        try:
            while True:
                cycle += 1
                timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                print(f"\n[{timestamp}] 第 {cycle} 轮数据上报")

                for bell in target_bells:
                    measurement = self.generate_measurement(bell["id"], bell)
                    success = self.send_measurement(measurement)

                    if success:
                        self.print_status(bell, measurement)

                        grinding = self.maybe_apply_grinding(bell["id"], bell)
                        if grinding:
                            success_g = self.send_grinding(grinding)
                            if success_g:
                                self.print_grinding(bell, grinding)
                    else:
                        print(f"  [{bell['serial_number']}] ✗ 上报失败")

                if max_cycles and cycle >= max_cycles:
                    print(f"\n已完成 {max_cycles} 轮模拟")
                    break

                print(f"  等待 {interval} 秒后进行下一轮...")
                time.sleep(interval)

        except KeyboardInterrupt:
            print("\n\n模拟器已停止")

        print("\n" + "=" * 70)
        print("模拟总结:")
        for bell in target_bells:
            state = self.bell_states[bell["id"]]
            target = bell["target_frequency"]
            final_deviation = 1200 * math.log2(state["current_freq"] / target)
            num_grinds = len(self.grinding_history[bell["id"]])

            print(f"  [{bell['serial_number']}] {bell['name']}")
            print(f"    初始偏差: {state['initial_deviation_cents']:+.1f} 音分 → "
                  f"最终偏差: {final_deviation:+.1f} 音分")
            print(f"    磨削次数: {num_grinds} 次, 总磨削量: {state['total_grinded_mm']:.3f} mm")


def main():
    parser = argparse.ArgumentParser(
        description="编钟声学传感器模拟器",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  %(prog)s                          # 默认模式，每分钟上报
  %(prog)s --interval 10            # 每10秒上报 (演示用)
  %(prog)s --bell-id 3              # 只模拟ID为3的编钟
  %(prog)s --deviation 20           # 初始偏差+20音分
  %(prog)s --fast --max-cycles 20   # 快速演示20轮
        """
    )

    parser.add_argument(
        "--api",
        default="http://localhost:8080/api",
        help="后端API地址 (默认: http://localhost:8080/api)"
    )
    parser.add_argument(
        "--interval",
        type=int,
        default=60,
        help="上报间隔秒数 (默认: 60, 即1分钟)"
    )
    parser.add_argument(
        "--bell-id",
        type=int,
        default=None,
        help="只模拟指定ID的编钟 (默认: 全部)"
    )
    parser.add_argument(
        "--deviation",
        type=float,
        default=None,
        help="所有编钟的初始音准偏差(音分), 不指定则随机"
    )
    parser.add_argument(
        "--max-cycles",
        type=int,
        default=None,
        help="最大运行轮数, 不指定则无限运行"
    )
    parser.add_argument(
        "--fast",
        action="store_true",
        help="快速演示模式 (5秒间隔)"
    )

    args = parser.parse_args()

    interval = 5 if args.fast else args.interval

    simulator = BellAcousticSimulator(args.api)
    simulator.run(
        interval=interval,
        bell_id=args.bell_id,
        initial_deviation=args.deviation,
        max_cycles=args.max_cycles
    )


if __name__ == "__main__":
    main()
