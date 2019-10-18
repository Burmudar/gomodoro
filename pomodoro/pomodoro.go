package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

type Payload struct {
	Reply chan interface{}
	Data  interface{}
}

type Create struct {
	Config
}

type TimerID struct {
	Key int32
}

type Start struct {
	TimerID
}

type Stop struct {
	TimerID
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
	ShutdownCh    chan struct{}
}

type TimerManager struct {
	timers     map[int32]*Timer
	payloadCh  chan *Payload
	shutdownCh chan bool
	mapLock    sync.RWMutex
}

type TimerEvent struct {
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
		make(chan struct{}),
	}
}

func (t *Timer) fire(event IntervalElapsedEvent) {
	fmt.Println("FIRING EVENT!")
}

func (t *Timer) String() string {
	res := fmt.Sprintf("==== Timer Configuration ====\n")
	res += fmt.Sprintf("Start: %v\n", t.Start)
	res += fmt.Sprintf("Elapsed: %v\n", t.Elapsed)
	res += fmt.Sprintf("End: %v\n", t.End)
	res += fmt.Sprintf("Interval: %v\n", t.Interval)

	return res
}

func (t *Timer) start() chan TimerEvent {
	eventCh := make(chan TimerEvent)
	go t.run(eventCh)
	return eventCh
}

func (t *Timer) run(eventCh chan TimerEvent) {
	timeDone := time.After(10 * time.Second)
	interval := time.After(1 * time.Second)

	for {
		select {
		case <-interval:
			{
				go func() { eventCh <- TimerEvent{} }()
				interval = time.After(1 * time.Second)
			}
		case <-timeDone:
			{
				go func() { eventCh <- TimerEvent{} }()
				return
			}
		case <-t.ShutdownCh:
			{
				log.Printf("Shutting down Timer ...")
				return
			}
		}
	}
}

func (tm *TimerManager) streamListen() {
	for {
		select {
		case p := <-tm.payloadCh:
			log.Printf("Handling payload")
			go tm.handlePayload(p)
		case <-tm.shutdownCh:
			log.Printf("Shutting down TM listen channel")
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

func (tm *TimerManager) handleStart(start *Start) chan TimerEvent {
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
	return v.start()
}

func (tm *TimerManager) handleStop(stop *Stop) error {
	tm.mapLock.Lock()
	defer tm.mapLock.Unlock()

	timer, ok := tm.timers[stop.Key]
	if !ok {
		log.Fatalf("'%d' Timer not found", stop.Key)
	}

	log.Printf("Closing timer['%v']", stop.Key)
	close(timer.ShutdownCh)
	return nil
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
	v = comms(wrap(Start{TimerID{key}}))

	eventCh := v.(chan TimerEvent)
	stop := false
	counter := 0
	for !stop {
		select {
		case i := <-eventCh:
			{
				counter += 1
				log.Printf("From timer: %v [%d]", i, counter)
				if counter > 10 {
					tm.handleStop(&Stop{TimerID{key}})
					//close(tm.shutdownCh)
					stop = true
					log.Printf("Stopping")
				}
				break
			}
		}
	}

}
