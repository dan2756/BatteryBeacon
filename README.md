# batt

Prints battery stats as JSON, with percentage hooks.

`batt` reads your laptop battery from `pmset` and `ioreg` on macOS or
`/sys/class/power_supply` on Linux and prints it as a one-line summary, as
JSON you can pipe into `jq`, or as a single value or template for tmux and
other status bars.

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

### Single values

For status bars, these print one bare value:

```console
$ batt percent
87
$ batt state
charging
$ batt time        # until empty, or until full while charging
1:12
$ batt health
91
```

### Templates

`--format` fills `{field}` placeholders with any JSON field name, plus
`{time}` (H:MM) and `{health}` (whole percent):

```console
$ batt --format '{percent}% {state}'
87% charging
$ batt --format '{watts}W at {temperature_c}°C'
23.26W at 30.4°C
```

Values the platform doesn't report print as an empty string. Write `{{` and
`}}` for literal braces. In tmux:

```tmux
set -g status-right '#(batt --format "{percent}%% {time}")'
```

## Hooks: `batt watch`

`batt watch` polls the battery and runs shell commands when something
changes:

```sh
batt watch \
  --below 20 'notify "battery low: $BATT_PERCENT%"' \
  --below 8  'pmset sleepnow' \
  --above 80 'notify "unplug"' \
  --on-plug  'notify "charging"' \
  --on-unplug 'brightness 0.6' \
  --on-full  'notify "fully charged"' \
  -i 60s
```

| Option | Runs when |
| --- | --- |
| `--below N CMD` | the charge drops to N% or lower (repeatable) |
| `--above N CMD` | the charge rises to N% or higher (repeatable) |
| `--on-plug CMD` | the charger is connected |
| `--on-unplug CMD` | the charger is disconnected |
| `--on-full CMD` | the battery is fully charged |
| `-i, --interval DUR` | time between readings: `30s`, `5m`, or bare seconds (default `60s`) |
| `-v, --verbose` | log every reading and hook to stderr |

Each threshold fires **once per crossing**. It re-arms only after the charge
moves 2 points back, so a battery hovering around 20% doesn't fire
`--below 20` over and over; it has to climb to 22% first. Thresholds and
`--on-full` also fire on the first reading if they already hold (starting
`batt watch` at 15% runs the `--below 20` hook straight away);
`--on-plug`/`--on-unplug` only fire on a change.

Hooks run one at a time with `/bin/sh -c`, inherit batt's stdout and
stderr, and get:

| Variable | Value |
| --- | --- |
| `BATT_EVENT` | `below`, `above`, `plug`, `unplug` or `full` |
| `BATT_PERCENT` | charge level, e.g. `19` |
| `BATT_STATE` | `charging`, `discharging`, `charged` or `not charging` |
| `BATT_POWER_SOURCE` | `ac` or `battery` |
| `BATT_THRESHOLD` | the level that fired, for `below`/`above` |

A hook that fails is logged and watching continues. `batt watch` exits 0 on
Ctrl-C or SIGTERM, and 1 straight away if there's no battery.

## Exit status

| Code | Meaning |
| --- | --- |
| 0 | success |
| 1 | no battery found (desktop, VM, CI) or the battery couldn't be read |
| 2 | bad usage, e.g. an unknown template field |

## License

MIT
