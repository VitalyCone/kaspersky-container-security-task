package app

import (
	"github.com/VitalyCone/kaspersky-container-security-task/internal/app/apiserver"
	"github.com/VitalyCone/kaspersky-container-security-task/internal/config"
	"github.com/VitalyCone/kaspersky-container-security-task/internal/workerpool"
)

const(
	percentOfFailedTasks = 20 // Процент неуспешных задач
)

type App struct{
	APIServer *apiserver.APIServer
}

func New(config *config.Config) (*App, error) {
	wp, err := workerpool.New(config.WorkersConfig.QueueSize, percentOfFailedTasks)
	if err != nil {
	    return nil, err
	}
	server := apiserver.New(config.RESTPort, wp, config.WorkersConfig)

	return &App{
		APIServer: server,
	}, nil
}