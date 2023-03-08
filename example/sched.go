package example

import (
	"context"
	"fmt"
	"github.com/Chendemo12/functools/cronjob"
	llog "github.com/Chendemo12/functools/logger"
	"github.com/Chendemo12/functools/zaplog"
	"time"
)

var lg llog.Iface

func init() {
	lg = zaplog.NewLogger(&zaplog.Config{
		Filename:   "example",
		Level:      zaplog.DEBUG,
		Rotation:   2,
		Retention:  3,
		MaxBackups: 4,
		Compress:   true,
	}).Sugar()
}

type Clock struct {
	cronjob.Func
	lastTime time.Time
}

func (c *Clock) Interval() time.Duration { return 1 * time.Second }
func (c *Clock) String() string          { return "报时器" }

func (c *Clock) Do(ctx context.Context) error {
	diff := time.Now().Sub(c.lastTime)
	c.lastTime = time.Now()
	lg.Info("time interval:", diff.String())

	return nil
}

type Click struct {
	cronjob.Func
	num int
}

func (c *Click) String() string          { return "计数器" }
func (c *Click) Interval() time.Duration { return 2 * time.Second }

func (c *Click) Do(ctx context.Context) error {
	c.num += 1
	lg.Info(fmt.Sprintf("%s run %d times", c.String(), c.num))

	return nil
}

func Example_NewScheduler() {

	pCtx, _ := context.WithTimeout(context.Background(), 50*time.Second)

	scheduler := cronjob.NewScheduler(pCtx, lg)
	scheduler.Add(&Clock{})
	scheduler.AddCronjob(&Click{})
	scheduler.Run()

	<-scheduler.Done()
}
