# batt

Prints battery stats as JSON, with percentage hooks.

`batt` reads your laptop battery from `pmset` on macOS or
`/sys/class/power_supply` on Linux and prints it as a one-line summary or as
JSON you can pipe into `jq`.

## Install

```sh
go install github.com/dan2756/batt@latest
```

## Usage

```console
$ batt
64% discharging · 4:37 remaining · battery

$ batt --json
{
  "percent": 64,
  "state": "discharging",
  "power_source": "battery",
  "time_remaining_min": 277,
  "time_to_full_min": null,
  ...
}
```

`batt` exits with status 1 and prints `no battery found` on machines without
a battery.

## License

MIT
