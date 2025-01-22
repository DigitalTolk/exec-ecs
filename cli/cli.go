package cli

import (
	"flag"
)

type Cli struct {
	Interactive bool
	Profile     string
	Region      string
	ClusterArn  string
	Service     string
	TaskArn     string
	Container   string
	Command     string
	Debug       bool
	Version     bool
	Upgrade     bool
}

func ParseArgs() Cli {
	var (
		profile   string
		region    string
		cluster   string
		service   string
		task      string
		container string
		command   string
		debug     bool
		version   bool
		upgrade   bool
	)

	flag.BoolVar(&debug, "debug", false, "Enable debug mode for logging AWS commands")
	flag.BoolVar(&version, "version", false, "Show the current version")
	flag.BoolVar(&upgrade, "upgrade", false, "Upgrade to the latest version")
	flag.StringVar(&profile, "pr", "", "AWS profile to use")
	flag.StringVar(&region, "rg", "", "AWS region to use")
	flag.StringVar(&cluster, "cl", "", "ECS cluster name")
	flag.StringVar(&service, "se", "", "AWS service name")
	flag.StringVar(&task, "tk", "", "Task ARN")
	flag.StringVar(&container, "cn", "app", "Container name")
	flag.StringVar(&command, "command", "bash", "Command to run in the container")
	flag.Parse()

	return Cli{
		Debug:       debug,
		Interactive: true,
		Profile:     profile,
		Region:      region,
		ClusterArn:  cluster,
		Service:     service,
		TaskArn:     task,
		Container:   container,
		Command:     command,
		Version:     version,
		Upgrade:     upgrade,
	}
}
