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

Install `lm-sensors` for better thermal data:

```sh
sudo pacman -S lm_sensors
sudo sensors-detect
```

## Supported Hardware

- Intel (coretemp)
- AMD (k10temp, zenpower)
- ARM (cpu_thermal)
- ThinkPad fan interface
- Generic hwmon fans

## License

[MIT](LICENSE)
