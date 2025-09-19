package workerpool

import (
	"errors"
	"log"
	"sync"
	"time"

	"math/rand/v2"

	"github.com/VitalyCone/kaspersky-container-security-task/internal/domain/dto"
)

var (
	ErrQueueFull        = errors.New("queue is full")
	ErrQueueClosed      = errors.New("queue is closed")
	ErrQueueSizeInvalid = errors.New("queue size is invalid")
	ErrInvalidTask      = errors.New("invalid task")
)

const (
	StateQueued  = "queued"
	StateRunning = "running"
	StateDone    = "done"
	StateFailed  = "failed"
	minWaitMs = 100
	maxWaitMs = 500
	baseDelayMs = 1000
	jitterFactor = 0.1
)

// WorkerPool - структура пула воркеров для обработки задач.
// Управляет очередью задач, распределением их между воркерами
// и отслеживанием их состояний.
type WorkerPool struct {
	isClosed bool            
	mu       sync.Mutex      
	taskCh   chan *Task     
	doneCh   chan struct{}   
	percentOfFailedTasks int  // Процент задач, которые должны завершиться с ошибкой
	taskStates chan *Task    // Канал для передачи состояний задач
}

// Task - структура задачи для выполнения в пуле воркеров.
// Включает в себя данные задачи, её текущее состояние и количество повторных попыток.
type Task struct {
	dto.Task    
	State   string // Текущее состояние задачи (queued, running, done, failed)
	Retries int   
}

// New - создает новый экземпляр пула воркеров.
// Инициализирует все необходимые каналы и запускает мониторинг состояний задач.
//
// Параметры:
//   - queueSize: размер очереди задач
//   - percentOfFailedTasks: процент задач, которые должны завершиться с ошибкой
//
// Возвращаемые значения:
//   - *WorkerPool: указатель на созданный пул воркеров
//   - error: ошибка, если размер очереди некорректен
func New(queueSize, percentOfFailedTasks int) (*WorkerPool, error) {
	if queueSize <= 0 {
		return nil, ErrQueueSizeInvalid
	}
	wp := &WorkerPool{
		isClosed: false,
		mu:       sync.Mutex{},
		taskCh:   make(chan *Task, queueSize),
		doneCh:   make(chan struct{}),
		percentOfFailedTasks: percentOfFailedTasks,
		taskStates: make(chan *Task),
	}
	go wp.checkStates()
	return wp, nil
}

// Start - запускает пул воркеров с указанным количеством воркеров.
// Создает горутину, которая управляет воркерами для обработки задач из очереди.
//
// Параметры:
//   - workersCount: количество воркеров для запуска
func (w *WorkerPool) Start(workersCount int) {
	go w.worker(workersCount)
}

// AddTask - добавляет задачу в очередь пула воркеров.
// Устанавливает состояние задачи как "queued" и пытается отправить её в канал задач.
// Если очередь полна, возвращает ошибку ErrQueueFull.
//
// Параметры:
//   - task: указатель на задачу для добавления
//
// Возвращаемые значения:
//   - error: ошибка, если очередь закрыта, полна или задача некорректна
func (w *WorkerPool) AddTask(task *Task) error {
	if task == nil {
		return ErrInvalidTask
	}
	
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.isClosed {
		return ErrQueueClosed
	}

	w.changeState(task, StateQueued)

	select {
	case w.taskCh <- task:
		return nil
	default:
		w.changeState(task, StateFailed)
		return ErrQueueFull
	}
}

// Stop - останавливает пул воркеров, завершая все текущие задачи.
// Закрывает канал задач и ожидает завершения всех воркеров.
// После завершения закрывает канал состояний задач.
//
// Возвращаемые значения:
//   - error: ошибка, если пул уже закрыт
func (w *WorkerPool) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.isClosed {
		return ErrQueueClosed
	}
	w.isClosed = true
	close(w.taskCh)
	<-w.doneCh
	close(w.taskStates)
	return nil
}

// worker - основной метод воркера, который запускает указанное количество горутин для обработки задач.
// Создает WaitGroup для отслеживания завершения всех воркеров.
// Каждая горутина читает задачи из канала и обрабатывает их с возможностью повторных попыток.
//
// Параметры:
//   - workersCount: количество воркеров для запуска
func (w *WorkerPool) worker(workersCount int) {
	wg := &sync.WaitGroup{}
	wg.Add(workersCount)
	for range workersCount {
		go func() {
			for task := range w.taskCh {
				// Обработка с возможностью повторных попыток
				if err := w.taskProcessWithRetry(task, w.percentOfFailedTasks); err != nil {
					log.Printf("Task %s failed after %d retries: %v", task.ID, task.Retries, err)
				} else {
					w.changeState(task, StateDone)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	close(w.doneCh)
}

// taskProcessWithRetry - обрабатывает задачу с возможностью повторных попыток в случае ошибки.
// Устанавливает состояние задачи как "running" и вызывает основную обработку.
// При ошибке и наличии оставшихся попыток выполняет экспоненциальный бэкофф с джиттером
// и повторяет обработку задачи.
//
// Параметры:
//   - task: указатель на задачу для обработки
//   - percentToFailTask: процент задач, которые должны завершиться с ошибкой
//
// Возвращаемые значения:
//   - error: ошибка выполнения задачи после всех попыток
func (w *WorkerPool) taskProcessWithRetry(task *Task, percentToFailTask int) error {
	w.changeState(task, StateRunning)
	log.Println("Start task")
	// Базовая обработка задачи
	if err := w.taskProcess(task, percentToFailTask); err != nil {
		// Если есть повторные попытки и ошибка, пытаемся повторить
		if task.Retries < task.MaxRetries {
			// Экспоненциальный бэкофф с джиттером
			delayMs := baseDelayMs * (1 << task.Retries) // Экспоненциальное увеличение
			// Добавляем джиттер (случайная вариация)
			jitter := int(float64(delayMs) * jitterFactor)
			delayMs += rand.IntN(2*jitter) - jitter // От -jitter до +jitter
			
			log.Printf("Task %s failed, retrying in %d ms (attempt %d/%d)", 
				task.ID, delayMs, task.Retries+1, task.MaxRetries)
			
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
			task.Retries++
			return w.taskProcessWithRetry(task, percentToFailTask)
		}else{
			w.changeState(task, StateFailed)
		}
		return err
	}
	w.changeState(task, StateDone)
	return nil
}

// taskProcess - основная логика обработки задачи.
// Симулирует выполнение задачи со случайной задержкой между minWaitMs и maxWaitMs.
// С вероятностью percentToFailTask% возвращает ошибку для симуляции сбоя.
//
// Параметры:
//   - task: указатель на задачу для обработки
//   - percentToFailTask: процент задач, которые должны завершиться с ошибкой
//
// Возвращаемые значения:
//   - error: ошибка выполнения задачи (симулируется с заданным процентом вероятности)
func (w *WorkerPool) taskProcess(task *Task, percentToFailTask int) error{
    randomNumber := rand.IntN(maxWaitMs - minWaitMs) + minWaitMs
	time.Sleep(time.Duration(randomNumber) * time.Millisecond)

	// Симуляция падения 20% задач
    if rand.IntN(100) <= percentToFailTask {
        return errors.New("simulated task failure")
    }
    
	return nil
}

// changeState - изменяет состояние задачи и отправляет его в канал состояний.
// Обновляет поле State задачи и передает её в канал taskStates для мониторинга.
//
// Параметры:
//   - task: указатель на задачу, состояние которой нужно изменить
//   - state: новое состояние задачи
func (w *WorkerPool) changeState(task *Task, state string) {
	task.State = state

	w.taskStates <- task
}


// checkStates - мониторинг состояний задач.
// Запускается при создании пула воркеров.
// Отслеживает изменения состояний задач, логирует их и хранит информацию о задачах в карте.
// Для каждой задачи выводит информацию о смене состояния или о новой задаче.
func (w *WorkerPool) checkStates() {
	tasks := make(map[string]*Task, 128)
	for	task := range w.taskStates{
		if t, ok := tasks[task.ID]; ok{
			log.Printf("task [%s] change state [%s] is [%s]", task.ID, t.State, task.State)
		    tasks[task.ID] = task
			continue
		}
		log.Printf("task [%s] is new with state [%s]", task.ID, task.State)
	}
}