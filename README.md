# cpumon

Real-time CPU monitoring for Linux. Shows temperatures, frequencies, throttling, and fan status.

## Install

**Pre-built binary:**

```
curl -Lo cpumon https://github.com/Mohabdo21/cpumon/releases/latest/download/cpumon-linux-amd64
chmod +x cpumon
sudo mv cpumon /usr/local/bin/
```

**With Go:**

```
go install github.com/Mohabdo21/cpumon@latest
```

**From source:**

```
make build-optimized
sudo make install
```

## Run

```
./build/cpumon
```

Requires root for some metrics. Install `lm-sensors` for better thermal data:

```
sudo dnf install lm_sensors
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
