package config

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
)

const defaultRESTPort = "8080"
const workersEnvName = "WORKERS"
const queueSizeEnvName = "QUEUE_SIZE"
const workersDefaultCount = 4
const queueSizeDefault = 64

type Config struct{
	RESTPort string
	WorkersConfig *WorkerConfig
}

type WorkerConfig struct{
	WorkersCount int
	QueueSize int
}


func MustLoad() *Config{
	loadEnvFile(".env")
	
	RESTPort := fetchRESTPort()

	var workersCountInt int
	workersCount := os.Getenv(workersEnvName)
	if workersCount == ""{
		log.Printf("Workers count is not set, using default %d", workersDefaultCount)
		workersCountInt = workersDefaultCount
	}else{
		var err error
		workersCountInt, err = strconv.Atoi(workersCount)
		if err != nil{
			log.Printf("Failed to parse, using default %d", workersCountInt)
			workersCountInt = workersDefaultCount
		}
	}
	
	var queueSizeInt int
	queueSize := os.Getenv(queueSizeEnvName)
	if queueSize == ""{
		log.Printf("Queue size is not set, using default %d", queueSizeDefault)
		queueSizeInt = queueSizeDefault
	}else{
		var err error
		queueSizeInt, err = strconv.Atoi(queueSize)
		if err != nil{
			log.Printf("Failed to parse, using default %d", queueSizeInt)
			queueSizeInt = queueSizeDefault
		}
	}

	return &Config{
		RESTPort: RESTPort,
		WorkersConfig: &WorkerConfig{
			WorkersCount: workersCountInt,
			QueueSize: queueSizeInt,
		},
	}
}

func fetchRESTPort() string{
	var RESTPort string

	flag.StringVar(&RESTPort, "port", "", "REST port for api")
	flag.Parse()

	if RESTPort == ""{
		log.Println("REST port is not set, using default :8080")
		RESTPort = defaultRESTPort
	}
	return RESTPort
}

func loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		
		// Пропускаем пустые строки и комментарии
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Разделяем строку на ключ и значение
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Устанавливаем переменную окружения
		os.Setenv(key, value)
	}

	return scanner.Err()
}