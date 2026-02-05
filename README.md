# cpumon

Real-time CPU monitoring for Linux. Shows temperatures, frequencies, throttling, and fan status.

## Install

```
go install github.com/Mohabdo21/cpumon@latest
```

Or build from source:

```
make build
```

For smaller binary:

```
make build-optimized
```

## Run

```
./build/cpumon
```

Requires root for some metrics. Install `lm-sensors` for better thermal data:

```
sudo apt install lm-sensors
sudo sensors-detect
```

## Supported Hardware

- Intel (coretemp)
- AMD (k10temp, zenpower)
- ARM (cpu_thermal)
- ThinkPad fan interface
- Generic hwmon fans

## License

Do whatever you want with it.
