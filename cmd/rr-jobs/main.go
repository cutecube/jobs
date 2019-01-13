package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spiral/jobs"
	"github.com/spiral/jobs/broker/amqp"
	"github.com/spiral/jobs/broker/beanstalk"
	"github.com/spiral/jobs/broker/local"
	"github.com/spiral/jobs/broker/sqs"
	rr "github.com/spiral/roadrunner/cmd/rr/cmd"
	"github.com/spiral/roadrunner/service/rpc"

	_ "github.com/spiral/jobs/cmd/rr-jobs/cmd"
)

func main() {
	rr.Container.Register(rpc.ID, &rpc.Service{})

	rr.Container.Register(jobs.ID, &jobs.Service{
		Brokers: map[string]jobs.Broker{
			"amqp":      &amqp.Broker{},
			"local":     &local.Broker{},
			"beanstalk": &beanstalk.Broker{},
			"sqs":       &sqs.Broker{},
		},
	})

	rr.Logger.Formatter = &logrus.TextFormatter{ForceColors: true}
	rr.Execute()
}
