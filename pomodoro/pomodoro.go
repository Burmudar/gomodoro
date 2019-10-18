package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type Payload struct {
	Reply chan interface{}
	Data  interface{}
}

type Create struct {
	Config
}

type Start struct {
	Key int32
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
}

type TimerManager struct {
	timers     map[int32]*Timer
	payloadCh  chan *Payload
	shutdownCh chan bool
	mapLock    sync.RWMutex
}

type IntervalElapsedEvent struct {
	Interval time.Duration
	Elapsed  time.Duration
}

func newTimerManager() *TimerManager {
	manager := &TimerManager{
		make(map[int32]*Timer),
		make(chan *Payload),
		make(chan bool),
		sync.RWMutex{},
	}
	return manager
}

func newTimer(c *Config) *Timer {
	now := time.Now()
	start := now
	end := start.Add(c.FocusTime)

	return &Timer{
		start,
		end,
		5 * time.Minute,
		0,
		c.FocusTime,
		c.BreakTime,
	}
}

func (t *Timer) fire(event IntervalElapsedEvent) {
	fmt.Println("FIRING EVENT!")
}

func (t *Timer) startInterval(interval time.Duration) chan bool {
	done := make(chan bool)
	go func() {
		after := time.After(interval)
		select {
		case <-after:
			done <- true
		}
	}()
	return done
}

func (t *Timer) String() string {
	res := fmt.Sprintf("==== Timer Configuration ====\n")
	res += fmt.Sprintf("Start: %v\n", t.Start)
	res += fmt.Sprintf("Elapsed: %v\n", t.Elapsed)
	res += fmt.Sprintf("End: %v\n", t.End)
	res += fmt.Sprintf("Interval: %v\n", t.Interval)

	return res
}

func (t *Timer) start() chan int {
	cmd := make(chan int)
	go func() {
		//done := time.After(t.FocusDuration)
		done := time.After(10 * time.Second)
		interval := time.After(1 * time.Second)
		atomicStop := atomic.Value{}
		atomicStop.Store(false)

		defer close(cmd)
		for {
			select {
			case <-interval:
				{
					cmd <- 1
					stop := atomicStop.Load().(bool)

					if !stop {
						interval = time.After(1 * time.Second)
					} else {
						cmd <- -1
						break
					}
				}
			case <-done:
				{
					atomicStop.Store(true)
				}
			}
		}
	}()
	return cmd

}

func (tm *TimerManager) streamListen() {
	for {
		select {
		case p := <-tm.payloadCh:
			log.Printf("Handling payload")
			go tm.handlePayload(p)
		case <-tm.shutdownCh:
			log.Printf("Shutting down TM listen channel")
			close(tm.payloadCh)
			return
		}
	}
}

func payloadError(p *Payload, err error) {
	p.Reply <- err
}

func (tm *TimerManager) handlePayload(p *Payload) {
	switch p.Data.(type) {
	case Create:
		{
			c, ok := p.Data.(Create)
			if !ok {
				payloadError(p, fmt.Errorf("Failed to cast to create struct"))
			}

			result := tm.handleCreate(&c.Config)
			p.Reply <- result
		}
		break
	case Start:
		{
			s, ok := p.Data.(Start)
			if !ok {
				payloadError(p, fmt.Errorf("Failed to cast to Start struct"))
			}

			result := tm.handleStart(&s)
			p.Reply <- result
		}
		break
	default:
		{
			fmt.Printf("Don't know how to handle: %v", p.Data)
		}
		break
	}
}

func (tm *TimerManager) handleCreate(conf *Config) int32 {
	//Create the timer and register it a key
	t := newTimer(conf)

	key := rand.Int31()

	tm.mapLock.Lock()
	defer tm.mapLock.Unlock()
	tm.timers[key] = t
	log.Printf("Created timer - key: %d", key)

	return key
}

func (tm *TimerManager) handleStart(start *Start) chan int {
	tm.mapLock.Lock()
	defer tm.mapLock.Unlock()

	v, ok := tm.timers[start.Key]
	if !ok {
		log.Fatalf("'%d' Timer not found", start.Key)
	}

	// so we start the timer but ...
	// 1. How do we stop it ?
	// 2. How do we know its status ?
	// 3. How does the timer report back on things ?
	cmdCh := v.start()
	return cmdCh
}

func main() {
	tm := newTimerManager()
	go tm.streamListen()

	wrap := func(d interface{}) *Payload {
		return &Payload{
			make(chan interface{}),
			d,
		}
	}

	comms := func(p *Payload) interface{} {

		tm.payloadCh <- p

		reply := <-p.Reply

		log.Printf("Reply: %v", reply)

		return reply
	}

	v := comms(wrap(
		Create{
			Config{
				1 * time.Minute,
				5 * time.Minute,
				1 * time.Minute,
			},
		}))

	key := v.(int32)

	log.Printf("Starting timer['%d']", key)
	v = comms(wrap(Start{key}))

	cmdCh := v.(chan int)
	for {
		select {
		case i := <-cmdCh:
			{
				log.Printf("From timer: %v", i)
				if i < 0 {
					tm.shutdownCh <- true
					break
				}
			}
		}
		log.Println("!")
	}

}
