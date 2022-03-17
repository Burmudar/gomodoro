package pomodoro

import (
	"fmt"
	"github.com/burmudar/gomodoro/models"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type TimerFunc func(ts *TimerState)
type TimerEventType int

var TimeIntervalEvent TimerEventType = 0
var TimeCompleteEvent TimerEventType = 1

var ErrTimerNotFound error = fmt.Errorf("Timer was not found")

type TimerEvent struct {
	Key  int32
	Type TimerEventType
}
type Config struct {
	*models.TimerConfig
	IntervalCB TimerFunc
	CompleteCB TimerFunc
}

type Timer struct {
	Start         time.Time
	End           time.Time
	Interval      time.Duration
	Elapsed       time.Duration
	FocusDuration time.Duration
	BreakDuration time.Duration
	Stop          atomic.Value
	OnInterval    TimerFunc
	OnComplete    TimerFunc
}

type TimerManager struct {
	timers     map[int32]*Timer
	shutdownCh chan bool
	mapLock    sync.RWMutex
}

type TimerState struct {
	Interval time.Duration
	Elapsed  time.Duration
	StartAt  time.Time
	EndsAt   time.Time
}

func NewTimerManager() *TimerManager {
	manager := &TimerManager{
		make(map[int32]*Timer),
		make(chan bool),
		sync.RWMutex{},
	}
	return manager
}

func newTimer(c *Config) *Timer {
	now := time.Now()
	start := now
	end := start.Add(c.FocusTime)

	var stop atomic.Value
	stop.Store(false)

	return &Timer{
		start,
		end,
		c.Interval,
		0,
		c.FocusTime,
		c.BreakTime,
		stop,
		c.IntervalCB,
		c.CompleteCB,
	}
}

func (t *Timer) String() string {
	res := fmt.Sprintf("==== Timer Configuration ====\n")
	res += fmt.Sprintf("Start: %v\n", t.Start)
	res += fmt.Sprintf("Elapsed: %v\n", t.Elapsed)
	res += fmt.Sprintf("End: %v\n", t.End)
	res += fmt.Sprintf("Interval: %v\n", t.Interval)
	res += fmt.Sprintf("Focus: %v\n", t.FocusDuration)
	res += fmt.Sprintf("Interval Callback: %v\n", t.OnInterval)
	res += fmt.Sprintf("Complete Callback: %v\n", t.OnComplete)

	return res
}

func (t *Timer) start() {
	//Should Reset Timer state
	log.Println("Timer started")
	log.Printf("\n%v", t)

	go func() {
		complete := time.NewTimer(t.FocusDuration)
		interval := time.NewTicker(t.Interval)
		for {
			select {
			case <-interval.C:
				{
					if t.OnInterval != nil {
						t.Elapsed += t.Interval
						t.OnInterval(&TimerState{
							t.Interval,
							t.Elapsed,
							t.Start,
							t.End,
						})
					}
				}
			case <-complete.C:
				{
					interval.Stop()
					if t.OnComplete != nil {
						t.OnComplete(&TimerState{
							t.Interval,
							t.Elapsed,
							t.Start,
							t.End,
						})
					}
					return
				}
			default:
				{
					stop := t.Stop.Load().(bool)
					if stop {
						complete.Stop()
						interval.Stop()
						return
					}
				}
			}
		}
	}()
}

func (t *Timer) stop() {
	t.Stop.Store(true)
}

func (tm *TimerManager) NewTimer(config *Config) int32 {
	//Create the timer and register it a key

	key := rand.Int31()
	//TODO: We first need to persist the config into the DB
	//Then we create the timer and persist it too
	//We probably need to give the websocket a UUID
	t := newTimer(config)

	tm.mapLock.Lock()
	defer tm.mapLock.Unlock()
	tm.timers[key] = t
	log.Printf("Created timer - key: %d", key)

	return key
}

func (tm *TimerManager) StartTimer(key int32) error {
	tm.mapLock.Lock()
	defer tm.mapLock.Unlock()

	timer, ok := tm.timers[key]
	if !ok {
		log.Fatalf("'%d' Timer not found", key)
		return ErrTimerNotFound
	}

	log.Printf("Before timer start")
	timer.start()
	log.Printf("After timer start")
	return nil
}

func (tm *TimerManager) StopTimer(key int32) error {
	tm.mapLock.Lock()
	defer tm.mapLock.Unlock()

	timer, ok := tm.timers[key]

	if !ok {
		log.Fatalf("'%d' Timer not found", key)
		return ErrTimerNotFound
	}

	timer.stop()
	return nil
}

func (tm *TimerManager) StopAll() error {
	for k, _ := range tm.timers {
		tm.StopTimer(k)
	}
	return nil
}
