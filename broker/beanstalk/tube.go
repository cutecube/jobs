package beanstalk

import (
	"fmt"
	"github.com/beanstalkd/go-beanstalk"
	"github.com/spiral/jobs"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type tube struct {
	active  int32
	pipe    *jobs.Pipeline
	mut     sync.Mutex
	tube    *beanstalk.Tube
	tubeSet *beanstalk.TubeSet
	reserve time.Duration

	// tube events
	lsn func(event int, ctx interface{})

	// stop channel
	wait chan interface{}

	// active operations
	muw sync.RWMutex
	wg  sync.WaitGroup

	// exec handlers
	execPool   chan jobs.Handler
	errHandler jobs.ErrorHandler
}

type entry struct {
	id   uint64
	data []byte
}

func (e *entry) String() string {
	return fmt.Sprintf("%v", e.id)
}

// create new tube consumer and producer
func newTube(pipe *jobs.Pipeline, lsn func(event int, ctx interface{})) (*tube, error) {
	if pipe.String("tube", "") == "" {
		return nil, fmt.Errorf("missing `tube` parameter on beanstalk pipeline")
	}

	return &tube{
		pipe:    pipe,
		tube:    &beanstalk.Tube{Name: pipe.String("tube", "")},
		tubeSet: beanstalk.NewTubeSet(nil, pipe.String("tube", "")),
		reserve: pipe.Duration("reserve", time.Second),
		lsn:     lsn,
	}, nil
}

// run consumers
func (t *tube) serve(connector connFactory) {
	// tube specific consume connection
	conn, err := connector.newConn()
	if err != nil {
		t.report(err)
		return
	}
	defer conn.Close()

	t.wait = make(chan interface{})
	atomic.StoreInt32(&t.active, 1)

	for {
		e, err := t.consume(conn)
		if err != nil {
			if isConnError(err) {
				t.report(err)
			}
			continue
		}

		if e == nil {
			return
		}

		h := <-t.execPool
		go func(h jobs.Handler, e *entry) {
			err := t.do(conn, h, e)
			t.execPool <- h
			t.wg.Done()
			t.report(err)
		}(h, e)
	}
}

// fetch consume
func (t *tube) consume(cn *conn) (*entry, error) {
	t.muw.Lock()
	defer t.muw.Unlock()

	select {
	case <-t.wait:
		return nil, nil
	default:
		conn, err := cn.acquire()
		if err != nil {
			return nil, err
		}

		t.tubeSet.Conn = conn

		id, data, err := t.tubeSet.Reserve(t.reserve)
		if err != nil {
			cn.release(err)
			return nil, err
		}
		cn.release(nil)

		t.wg.Add(1)
		return &entry{id: id, data: data}, nil
	}
}

// do data
func (t *tube) do(cn *conn, h jobs.Handler, e *entry) error {
	j, err := unpack(e.data)
	if err != nil {
		return err
	}

	err = h(e.String(), j)

	conn, connErr := cn.acquire()
	if connErr != nil {
		return err
	}

	if err == nil {
		return cn.release(conn.Delete(e.id))
	}

	stat, statErr := conn.StatsJob(e.id)
	if statErr != nil {
		return cn.release(statErr)
	}

	t.errHandler(e.String(), j, err)

	reserves, ok := strconv.Atoi(stat["reserves"])
	if ok != nil || !j.Options.CanRetry(reserves-1) {
		return cn.release(conn.Bury(e.id, 0))
	}

	return cn.release(conn.Release(e.id, 0, j.Options.RetryDuration()))
}

// stop tube consuming
func (t *tube) stop() {
	if atomic.LoadInt32(&t.active) == 0 {
		return
	}

	atomic.StoreInt32(&t.active, 0)

	close(t.wait)

	t.muw.Lock()
	t.wg.Wait()
	t.muw.Unlock()
}

// put data into pool or return error (no wait)
func (t *tube) put(cn *conn, attempt int, data []byte, delay, rrt time.Duration) (id string, err error) {
	conn, err := cn.acquire()
	if err != nil {
		return "", err
	}

	var bid uint64
	t.mut.Lock()
	t.tube.Conn = conn
	bid, err = t.tube.Put(data, 0, delay, rrt)
	t.mut.Unlock()

	cn.release(err)

	return strconv.FormatUint(bid, 10), err
}

// return tube stats
func (t *tube) stat(cn *conn) (stat *jobs.Stat, err error) {
	conn, err := cn.acquire()
	if err != nil {
		return nil, err
	}

	t.mut.Lock()
	t.tube.Conn = conn
	values, err := t.tube.Stats()
	t.mut.Unlock()

	cn.release(err)

	stat = &jobs.Stat{InternalName: t.tube.Name}

	if v, err := strconv.Atoi(values["current-jobs-ready"]); err == nil {
		stat.Queue = int64(v)
	}

	if v, err := strconv.Atoi(values["current-jobs-reserved"]); err == nil {
		stat.Active = int64(v)
	}

	if v, err := strconv.Atoi(values["current-jobs-delayed"]); err == nil {
		stat.Delayed = int64(v)
	}

	return stat, err
}

// report tube specific error
func (t *tube) report(err error) {
	if err != nil {
		t.lsn(jobs.EventPipelineError, &jobs.PipelineError{Pipeline: t.pipe, Caused: err})
	}
}
