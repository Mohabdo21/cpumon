package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

const version = "0.1.7"

func main() {
	interval := flag.Duration("i", time.Second, "")
	showHelp := flag.Bool("h", false, "")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "cpumon v%s — real-time CPU monitor\n\n", version)
		fmt.Fprintln(os.Stderr, "Usage: cpumon [-i interval] [-h]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -i duration   refresh interval (default 1s, min 100ms)")
		fmt.Fprintln(os.Stderr, "  -h            show this help")
	}

	flag.Parse()

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
