package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

type TimerFunc func()
type TimerEventType int

var TimeIntervalEvent TimerEventType = 0
var TimeCompleteEvent TimerEventType = 1

var ErrTimerNotFound error = fmt.Errorf("Timer was not found")

type TimerEvent struct {
	Key int32,
	Type TimerEventType
}
type Config struct {
	FocusTime time.Duration
	BreakTime time.Duration
	Interval  time.Duration
}

type Timer struct {
	Start         time.Time
	End           time.Time
	Interval      time.Duration
	Elapsed       time.Duration
	FocusDuration time.Duration
	BreakDuration time.Duration
	Stop		  atomic.Value
	OnInterval	  TimerFunc
	OnComplete	  TimerFunc
}

type TimerManager struct {
	timers     map[int32]*Timer
	shutdownCh chan bool
	mapLock    sync.RWMutex
}

func newTimerManager() *TimerManager {
	manager := &TimerManager{
		make(map[int32]*Timer),
		make(chan bool),
		sync.RWMutex{},
	}
	go manager.streamActiveTimerEvents()
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
		5 * time.Minute,
		0,
		c.FocusTime,
		c.BreakTime,
		stop,
		nil,
		nil,
	}
}

func (t *Timer) String() string {
	res := fmt.Sprintf("==== Timer Configuration ====\n")
	res += fmt.Sprintf("Start: %v\n", t.Start)
	res += fmt.Sprintf("Elapsed: %v\n", t.Elapsed)
	res += fmt.Sprintf("End: %v\n", t.End)
	res += fmt.Sprintf("Interval: %v\n", t.Interval)

	return res
}

func (t *Timer) start() {
	//Should Reset Timer state

	go func() {
		//done := time.After(t.FocusDuration)
		complete := time.NewTimer(10 * time.Second)
		interval := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-interval.C:
				{
					if t.OnInterval {
						t.OnInterval()
					}
				}
			case <-complete.C:
				{
					interval.Stop()
					if t.OnComplete {
						t.OnComplete()
					}
					return
				}
			case t.Stop.Load():
				{
					complete.Stop()
					interval.Stop()
					return
				}
			}
		}
	}()
}

func (tm *TimerManager) NewTimer(conf *Config) int32 {
	//Create the timer and register it a key

	key := rand.Int31()
	t := newManagedTimer(key, newTimer(config))

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
		return nil, ErrTimerNotFound
	}

	timer.Start()
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

	timer.Stop()
	return nil
}