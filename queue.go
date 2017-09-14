package tq

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
	queueID        uint64
}

func isFull(q *Queue) bool {
	return cap(q.messageCh) == len(q.messageCh)
}

// NewQueue queue constructor
func NewQueue(id uint64, cap int, timeout time.Duration) *Queue {

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

// DeleteAsync send notification to queue deleted and returns a read only channel to user receive a Notification when
// deletion be completed
func (q *Queue) DeleteAsync() <-chan Notification {
	fmt.Println("+ > Delete queue:", q.queueID)
	close(q.messageCh)
	fmt.Println("+ < Delete queue:", q.queueID)
	return q.deleteCh
}

// Delete wait for all tasks be completed, after that, kill the consumer
func (q *Queue) Delete() {
	<-q.DeleteAsync()
}

// AppendAsync send a TaskHandler to the queue and return notification channels
func (q *Queue) AppendAsync(taskHandler TaskHandler) (doneCh, timeoutCh <-chan Notification, id uint64, err error) {

	if isFull(q) {
		return nil, nil, 0, ErrTaskQueueFull
	}

	messageID := atomic.AddUint64(&q.messageCounter, 1)
	doneCh, timeoutCh, message := newMessage(messageID, q.messageTimeout, taskHandler)
	q.messageCh <- message

	return doneCh, timeoutCh, messageID, nil
}

// Append send a TaskHandler to the queue and wait for the task execution or timeout
func (q *Queue) Append(taskHandler TaskHandler) (err error) {

	var (
		doneCh, timeoutCh <-chan Notification
		id                uint64
	)

	if doneCh, timeoutCh, id, err = q.AppendAsync(taskHandler); err != nil {
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