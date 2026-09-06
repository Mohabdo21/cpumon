# cpumon

Real-time CPU monitoring for Linux. Shows temperatures, frequencies, throttling, and fan status.

## Install

**AUR (Arch Linux):**

```sh
yay -S cpumon
```

**Pre-built binary:**

```sh
curl -Lo cpumon https://github.com/Mohabdo21/cpumon/releases/latest/download/cpumon-linux-amd64
chmod +x cpumon
sudo mv cpumon /usr/local/bin/
```

**With Go:**

```sh
go install github.com/Mohabdo21/cpumon@latest
```

**From source:**

```sh
make build-optimized
sudo make install
```

## Run

```sh
cpumon           # 1 second refresh (default)
cpumon -i 500ms  # 500ms refresh
cpumon -i 2s     # 2 second refresh
```

Requires root for some metrics like power consumption, set read/search file capabilities with:

```sh
sudo setcap cap_dac_read_search=ep /bin/cpumon
```

## Configuration

Power threshold colors are configurable via env vars (defaults shown):

| Env var             | Default | Description                           |
| ------------------- | ------- | ------------------------------------- |
| `CPUMON_POWER_WARN` | `15`    | Power warning threshold in W (yellow) |
| `CPUMON_POWER_CRIT` | `28`    | Power critical threshold in W (red)   |

Example:

```sh
CPUMON_POWER_WARN=50 CPUMON_POWER_CRIT=80 cpumon
```

## Supported Hardware

- Intel (coretemp)
- AMD (k10temp, zenpower)
- ARM (cpu_thermal)
- ThinkPad fan interface
- Generic hwmon fans

## License

[MIT](LICENSE)
