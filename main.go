package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

const releaseVersion = "0.2.6"

var (
	version = releaseVersion
	commit  = "unknown"
)

func canReadPower() bool {
	return len(discoverRAPL(sysFileReader{})) > 0
}

func main() {
	interval := flag.Duration("i", time.Second, "refresh interval (default 1s, min 100ms)")
	showVersion := flag.Bool("V", false, "print version and exit")
	showHelp := flag.Bool("h", false, "show help")

	flag.Usage = func() {
		powerNote := ""
		if !canReadPower() {
			powerNote = "\nNote: run as root (or with cap_dac_read_search) for power consumption data.\n"
		}
		fmt.Fprintf(
			os.Stderr,
			"cpumon v%s (%s) - real-time CPU monitor\n\n"+
				"Usage: cpumon [-i interval] [-V] [-h]\n\n"+
				"Options:\n"+
				"  -i duration   refresh interval (default 1s, min 100ms)\n"+
				"  -V            print version and exit\n"+
				"  -h            show this help\n\n"+
				"Environment:\n"+
				"  %-16s power warning threshold in W (default %.0f)\n"+
				"  %-16s power critical threshold in W (default %.0f)\n"+
				"%s",
			version, commit,
			envPowerWarn, defaultPowerWarn,
			envPowerCrit, defaultPowerCrit,
			powerNote,
		)
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("cpumon v%s (%s)\n", version, commit)
		os.Exit(0)
	}

	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}

	if *interval < 100*time.Millisecond {
		fmt.Fprintln(os.Stderr, "Error: interval must be at least 100ms")
		os.Exit(1)
	}

	m, err := NewMonitor()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := m.Run(context.Background(), *interval); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
