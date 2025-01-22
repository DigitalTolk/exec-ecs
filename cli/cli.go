package cli

import (
	"flag"
	"os"
)

type Cli struct {
	Interactive bool
	Profile     string
	Region      string
	Service     string
	Container   string
	Command     string
	Debug       bool
	Version     bool
	Upgrade     bool
}

func ParseArgs() Cli {
	var debug, version, upgrade bool

	flag.BoolVar(&debug, "debug", false, "Enable debug mode for logging AWS commands")
	flag.BoolVar(&version, "version", false, "Show the current version")
	flag.BoolVar(&upgrade, "upgrade", false, "Upgrade to the latest version")
	flag.Parse()

	return Cli{
		Debug:       debug,
		Interactive: true,
		Profile:     getArg("-p", "dt-infra"),
		Region:      getArg("-r", "eu-north-1"),
		Service:     getArg("--service", ""),
		Container:   getArg("--container", "app"),
		Command:     getArg("--command", "bash"),
	}
}

func getArg(flag, defaultValue string) string {
	for i, arg := range os.Args {
		if arg == flag && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return defaultValue
}
