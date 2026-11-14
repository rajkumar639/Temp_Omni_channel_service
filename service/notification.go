package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"omniChannelservice/integration"
	"omniChannelservice/util"
)

const (
	notificationQueueSize = 1000
	notificationWorkerCount = 5
	notificationMaxAttempts = 3
	notificationAttemptTimeout = 20 * time.Second
)

type NotificationJob struct {
	Kind    string
	Payload any
	Attempt int
}

type NotificationService struct {
	comm *integration.Communicator
	log  *util.Logger
	jobs chan NotificationJob
	wg   sync.WaitGroup
}

func NewNotificationService(comm *integration.Communicator, log *util.Logger) *NotificationService {
	s := &NotificationService{
		comm: comm,
		log:  log,
		jobs: make(chan NotificationJob, notificationQueueSize),
	}

	for i := 0; i < notificationWorkerCount; i++ {
		s.wg.Add(1)
		go s.worker(i + 1)
	}

	return s
}

func (s *NotificationService) SendAsync(kind string, payload any) error {
	job := NotificationJob{Kind: kind, Payload: payload, Attempt: 1}

	select {
	case s.jobs <- job:
		return nil
	default:
		return errors.New("notification queue is full")
	}
}

func (s *NotificationService) worker(id int) {
	defer s.wg.Done()
	for job := range s.jobs {
		s.process(job)
	}
}
func (s *NotificationService) process(job NotificationJob) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), notificationAttemptTimeout)
		var err error
		switch job.Kind {
		case "sms":
			err = s.comm.SendSMS(ctx, job.Payload)
		case "whatsapp":
			err = s.comm.SendWhatsApp(ctx, job.Payload)
		case "email":
			err = s.comm.SendEmail(ctx, job.Payload)
		default:
			err = fmt.Errorf("unknown notification kind %q", job.Kind)
		}
		cancel()

		if err == nil {
			s.log.Info("async %s send succeeded", job.Kind)
			return
		}

		if job.Attempt >= notificationMaxAttempts {
			s.log.Error("async %s send failed after %d attempts: %v", job.Kind, job.Attempt, err)
			return
		}

		backoff := time.Duration(job.Attempt) * time.Second
		s.log.Warn("async %s send attempt %d failed: %v; retrying after %s", job.Kind, job.Attempt, err, backoff)
		time.Sleep(backoff)
		job.Attempt++
	}
}

func (s *NotificationService) Close() {
	close(s.jobs)
	s.wg.Wait()
}
