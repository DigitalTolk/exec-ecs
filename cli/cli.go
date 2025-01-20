package cli

import "os"

type Cli struct {
	Interactive bool
	Profile     string
	Region      string
	Service     string
	Container   string
	Command     string
	ShowCommand bool
}

func ParseArgs() Cli {
	return Cli{
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
