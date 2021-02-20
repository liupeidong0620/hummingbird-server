package app

import (
	"os"
	"strings"

	"github.com/liupeidong0620/hummingbird-server/server"
	"github.com/liupeidong0620/hummingbird-server/server/wss"

	"github.com/liupeidong0620/hummingbird/log"
)

type App struct {
	cmd CmdParam

	wssServer *wss.WssServer

	fileFp *os.File
}

func (a *App) initLog() error {
	var err error
	log.Info("log init.")
	if a.cmd.LogFile != "" {
		a.fileFp, err = os.OpenFile(a.cmd.LogFile, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			return err
		}

		log.SetOutput(a.fileFp)
	}

	switch strings.ToLower(a.cmd.LogLevel) {
	case "debug":
		log.SetLevel(log.LogLevelDebug)
	case "error":
		log.SetLevel(log.LogLevelError)
	case "warn":
		log.SetLevel(log.LogLevelWarn)
	default:
		log.SetLevel(log.LogLevelInfo)
	}
	log.Info("log init ok.")

	return nil
}

func (a *App) Init(cmd CmdParam) error {
	// init log
	a.cmd = cmd

	a.initLog()

	log.Info("init websocket server.")
	base, err := server.NewBase("tcp", cmd.Listen)
	if err != nil {
		return err
	}
	a.wssServer, err = wss.NewWssServer(base)
	log.Info("init websocket server ok.")

	return err
}

func (a *App) Stop() {
	if a.fileFp != nil {
		a.fileFp.Close()
	}

	if a.wssServer != nil {
		a.wssServer.Stop()
	}
}

func (a *App) Run() error {
	var err error

	log.Info("app run ...")
	if a.wssServer != nil {
		err = a.wssServer.Start()
	}
	return err
}
