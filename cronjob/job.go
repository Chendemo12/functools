package cronjob

import (
	cj "github.com/Chendemo12/fastapi-tool/cronjob"
)

type CronJob = cj.CronJob
type Func = cj.Job
type Schedule = cj.Schedule
type Scheduler = cj.Scheduler

var NewScheduler = cj.NewScheduler
