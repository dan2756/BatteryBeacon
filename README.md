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
changes. On Linux:

```sh
batt watch \
  --below 20 'notify-send "Battery low" "$BATT_PERCENT% left"' \
  --below 5  'systemctl suspend' \
  --above 80 'notify-send "Battery at $BATT_PERCENT%" "You can unplug now"' \
  --on-full  'notify-send "Fully charged"' \
  -i 60s
```

On macOS:

```sh
batt watch \
  --below 15 'osascript -e "display notification \"$BATT_PERCENT% left\" with title \"Battery low\""' \
  --below 5  'pmset sleepnow' \
  --on-unplug 'say "running on battery"' \
  -i 30s
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
`--below 20` over and over; it has to climb to 22% first. A threshold
within 2 points of 0% or 100% re-arms at that end of the scale instead
(`--below 99` re-arms at 100%, `--above 1` at 0%), since the charge can't
move any further. Thresholds and `--on-full` also fire on the first reading
if they already hold (starting `batt watch` at 15% runs the `--below 20`
hook straight away); `--on-plug`/`--on-unplug` only fire on a change.

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
Ctrl-C, SIGTERM or SIGHUP, and 1 straight away if there's no battery. A hook
still running then is stopped along with everything it started: its process
group gets SIGTERM, and whatever is left 5 seconds later is killed.

To keep it running on Linux, a systemd user unit works well (`$$` and `%%`
are systemd's escapes for `$` and `%`):

```ini
# ~/.config/systemd/user/batt-watch.service
[Unit]
Description=Battery hooks

[Service]
ExecStart=%h/go/bin/batt watch --below 15 'notify-send "Battery low" "$${BATT_PERCENT}%%"'
Restart=on-failure

[Install]
WantedBy=default.target
```

```sh
systemctl --user enable --now batt-watch
```

## Exit status

| Code | Meaning |
| --- | --- |
| 0 | success |
| 1 | no battery found (desktop, VM, CI), the battery couldn't be read, or output couldn't be written |
| 2 | bad usage, e.g. an unknown template field |

## Environment

| Variable | Effect |
| --- | --- |
| `BATTERY_DIR` | Linux only: read batteries from this directory instead of `/sys/class/power_supply`. It can also point straight at one battery, e.g. `/sys/class/power_supply/BAT1`. Handy for testing against a copy of sysfs. |

## Platform notes

**macOS.** The charge level, state and time estimate come from
`pmset -g batt`, so they match the menu bar. Cycle count, capacities,
temperature, voltage and amperage come from
`ioreg -rn AppleSmartBattery -a`; both Apple silicon and Intel layouts are
understood. When Optimized Battery Charging or a charge limit holds the
battery (`pmset` says `AC attached; not charging`), the state is
`not charging` and there's no time estimate. `condition` is `Normal` or
`Service Recommended`, from the battery's permanent-failure flag. Desktops
print `no battery found`.

**Linux.** batt uses the first battery in `/sys/class/power_supply`, skipping
empty bays and the batteries of wireless mice and keyboards. Both kinds of
battery driver are supported: `energy_*`/`power_now` (most ThinkPads) and
`charge_*`/`current_now` (many Dell and HP laptops). For energy-reporting
batteries, mAh figures are computed at `voltage_min_design`. Time estimates
are worked out from the current draw. Whether you're on AC comes from a
charger's `online` flag. Mains, UPS, USB (including USB-C, USB-PD, and
BrickID), and wireless supplies all count; a supply with no `online` file
does not. If nothing reports one, the source is inferred from the battery
status. `temperature_c` and `condition` are only filled in if the driver
exposes `temp` and `health`, which many laptops don't.

**Elsewhere.** Other systems get a clear "not supported" error.

## License

MIT. See [LICENSE](LICENSE).
