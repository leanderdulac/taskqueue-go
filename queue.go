package taskqueue

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Queue struct for queue object/class
type Queue struct {
	messageCh      chan<- *message
	deleteCh       <-chan Notification
	messageTimeout time.Duration
	messageCounter uint64
	queueID        string
}

func isFull(q *Queue) bool {
	return cap(q.messageCh) == len(q.messageCh)
}

// New queue constructor
func New(id string, cap int, timeout time.Duration) *Queue {

	messageChannel := make(chan *message, cap)
	deleteChannel := make(chan Notification)

	queue := func(messageCh chan<- *message, deleteCh <-chan Notification) *Queue {
		return &Queue{
			messageCh:      messageCh,
			deleteCh:       deleteCh,
			messageTimeout: timeout,
			messageCounter: 0,
			queueID:        id,
		}
	}(messageChannel, deleteChannel)

	go consumer(messageChannel, deleteChannel, id)

	return queue
}

// CloseAsync send notification to queue deleted and returns a read only channel to user receive a Notification when
// deletion be completed
func (q *Queue) CloseAsync() <-chan Notification {
	fmt.Println("+ > Delete queue:", q.queueID)
	close(q.messageCh)
	fmt.Println("+ < Delete queue:", q.queueID)
	return q.deleteCh
}

// Close wait for all tasks be completed, after that, kill the consumer
func (q *Queue) Close() {
	<-q.CloseAsync()
}

// EnqueueAsync send a TaskHandler to the queue and return notification channels
func (q *Queue) EnqueueAsync(taskHandler TaskHandler) (doneCh, timeoutCh <-chan Notification, id uint64, err error) {

	if isFull(q) {
		return nil, nil, 0, ErrTaskQueueFull
	}

	messageID := atomic.AddUint64(&q.messageCounter, 1)
	doneCh, timeoutCh, message := newMessage(messageID, q.messageTimeout, taskHandler)
	q.messageCh <- message

	return doneCh, timeoutCh, messageID, nil
}

// Enqueue send a TaskHandler to the queue and wait for the task execution or timeout
func (q *Queue) Enqueue(taskHandler TaskHandler) (err error) {

	var (
		doneCh, timeoutCh <-chan Notification
		id                uint64
	)

	if doneCh, timeoutCh, id, err = q.EnqueueAsync(taskHandler); err != nil {
		return err
	}

	select {
	case <-doneCh:
		fmt.Println("+ received done event:", id)
		return nil
	case <-timeoutCh:
		fmt.Println("+ reveived timeout event:", id)
		return nil
	}
}
