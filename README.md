# batt

Prints battery stats as JSON, with percentage hooks.

`batt` reads your laptop battery from `pmset` and `ioreg` on macOS or
`/sys/class/power_supply` on Linux and prints it as a one-line summary or as
JSON you can pipe into `jq`.

## Install

```sh
go install github.com/dan2756/batt@latest
```

## Usage

```console
$ batt
87% charging · 1:12 to full · 412 cycles · health 91% · 30.4°C · AC

$ batt --json
{
  "percent": 87,
  "state": "charging",
  "power_source": "ac",
  "time_remaining_min": null,
  "time_to_full_min": 72,
  "cycle_count": 412,
  "design_capacity_mah": 5279,
  "max_capacity_mah": 4811,
  "current_capacity_mah": 4187,
  "health_percent": 91.1,
  "temperature_c": 30.4,
  "voltage_v": 12.684,
  "amperage_ma": 1834,
  "watts": 23.26,
  "condition": "Normal"
}
```

### JSON fields

| Field | Meaning |
| --- | --- |
| `percent` | charge level, 0–100 |
| `state` | `charging`, `discharging`, `charged` or `not charging` |
| `power_source` | `ac` or `battery` |
| `time_remaining_min` | minutes until empty; `null` while charging or while the OS is still estimating |
| `time_to_full_min` | minutes until full; `null` unless charging (0 once charged) |
| `cycle_count` | charge cycles |
| `design_capacity_mah` | capacity when new |
| `max_capacity_mah` | capacity now |
| `current_capacity_mah` | charge left |
| `health_percent` | `max_capacity_mah / design_capacity_mah`, one decimal |
| `temperature_c` | battery temperature |
| `voltage_v` | pack voltage |
| `amperage_ma` | current; negative while discharging |
| `watts` | power; negative while discharging |
| `condition` | e.g. `Normal`, `Service Recommended` (macOS) or `Good` (Linux) |

Fields a platform doesn't report are `null`. The battery serial number is
never included.

`batt` exits with status 1 and prints `no battery found` on machines without
a battery.

## License

MIT
