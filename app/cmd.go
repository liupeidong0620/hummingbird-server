package app

import "flag"

type CmdParam struct {
	Listen   string
	LogLevel string
	LogFile  string
	Version  bool
	Help     bool
}

var (
	Cmd CmdParam
)

func init() {
	// wss or ws
	flag.StringVar(&Cmd.Listen, "listen", "ws://0.0.0.0:80", "Server listen addr.")
	flag.BoolVar(&Cmd.Version, "version", false, "Show version information and quit")
	flag.StringVar(&Cmd.LogFile, "logfile", "", "Log file.")
	flag.StringVar(&Cmd.LogLevel, "loglevel", "info", "Log level [debug|info|warn|error]")
	flag.BoolVar(&Cmd.Help, "help", false, "this help.")
	flag.Parse()
}
