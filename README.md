# wattch

Tiny floating power monitor — free-floating window showing total watts + cost since reset, with coloured per-source lines + white total.

## Sources

- **intel-rapl** `/sys/class/powercap/intel-rapl:*/energy_uj` (CPU package, core, dram if present)
- **hwmon** `/sys/class/hwmon/*/power*_input` (e.g. amdgpu PPT)
- **NVIDIA** `libnvidia-ml.so.1` via `dlopen` (`nvmlDeviceGetPowerUsage`) — or `nvidia-smi` equivalent, no root needed

RAPL requires world-readable `energy_uj`. Fix via Settings → *Fix Permissions* (pkexec) or manually:
```
sudo cp etc/99-rapl.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules && sudo udevadm trigger --subsystem-match=powercap
```

## Build

```sh
sudo apt install libgtk-3-dev pkg-config
go build -o wattch ./cmd/wattch
./wattch
```

## Window

- Heading: `127 W · $0.42 since 2026-08-23 09:00` + ◨ History + ⚙
- Graph: line chart, per-source colour + white total (live, `history_s` window)
- Resizable from corners, frameless toggle, always-on-top toggle, position remembered
- 1 Hz default, configurable sample interval / history length

## Daily History (30-min blocks)

- Every sample aggregated into **48× 30-min blocks per calendar day** (`~/.config/wattch/history/YYYY-MM-DD.json`)
- Each block stores `avg_watts`, per-source avg, `kwh`, `cost` (tariff evaluated at sample time)
- Larger viewer: click **◨** in header → 800×500 window with stacked per-source bars, date navigation (Prev/Next/Today), daily total `kWh` + cost + peak, X labels every 2h, Y auto-scale
- Data persists across restarts; live block updates every ~30s

## Settings

- Currency symbol (any text, default `$`)
- Default price `$/kWh`
- Tariff rules: ordered first-match list. Each rule: `label, days[Mon..Sun], start HH:MM, end HH:MM, price`. Empty rules = default only. Supports two peaks + weekend cheap etc.
- Sampling interval & history length
- Reset accumulated cost
- RAPL fix button (pkexec) — copies `etc/99-rapl.rules` via `pkexec` and reloads udev

Config stored in `~/.config/wattch/{config.json,tariff.json,state.json,history/*.json}`

## License

MIT
