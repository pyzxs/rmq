package main

import (
	"errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"os"
	"sync"
	"time"
)

type Session struct {
	name            string
	logger          *log.Logger
	connection      *amqp.Connection
	channel         *amqp.Channel
	done            chan bool
	handleFn        map[string]func(string)
	handleJobs      map[string]IJob
	cache           *MemoryCache
	mu              *sync.Mutex
	wg              *sync.WaitGroup
	notifyConnClose chan *amqp.Error
	notifyChanClose chan *amqp.Error
	notifyConfirm   chan amqp.Confirmation
	isReady         bool
}

const (
	// 重新连接延迟时间
	reconnectDelay = 5 * time.Second

	// 重新初始化channel延迟时间
	reInitDelay = 2 * time.Second

	// 重新发送消息时，服务器未确认
	resendDelay = 5 * time.Second
)

var (
	errNotConnected  = errors.New("not connected to a server")
	errAlreadyClosed = errors.New("already closed: not connected to the server")
	errShutdown      = errors.New("session is shutting down")
)

// New 创建一个新的会话实例，并自动连接
// 尝试连接到服务器。
func New(name string, addr string) *Session {
	session := &Session{
		logger:     log.New(os.Stdout, "", log.LstdFlags),
		name:       name,
		wg:         &sync.WaitGroup{},
		cache:      NewMemoryCache(),
		handleFn:   map[string]func(string){},
		handleJobs: map[string]IJob{},
		mu:         &sync.Mutex{},
		done:       make(chan bool, 1),
	}
	session.wg.Add(1)
	go session.handleReconnect(addr)
	session.wg.Wait()
	return session
}

// handleReconnect 将等待连接错误
// notifyConnClose, ，然后不断尝试重新连接。
func (session *Session) handleReconnect(addr string) {
	defer session.wg.Done()
	for {
		session.isReady = false
		session.logger.Println("Attempting to connect")

		conn, err := session.connect(addr)

		if session.errorRetry(err, reconnectDelay) {
			continue
		}

		if done := session.handleReInit(conn); done {
			session.logger.Println("done is ", done)
			break
		}
	}
}

// errorRetry 连接失败，延迟重试
func (session *Session) errorRetry(err error, deley time.Duration) bool {
	if err == nil {
		return false
	}
	session.logger.Println("Failed to connect. Retrying...")

	select {
	case <-session.done:
		session.logger.Println("session.done is true")
		return true
	case <-time.After(deley):
	}
	return false

}

// connect 将创建新的 AMQP 连接
func (session *Session) connect(addr string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(addr)

	if err != nil {
		return nil, err
	}

	session.changeConnection(conn)
	session.logger.Println("Connected!")
	return conn, nil
}

// handleReconnect 将等待 Channel 错误
// 然后不断尝试重新初始化两个通道
func (session *Session) handleReInit(conn *amqp.Connection) bool {
	for {
		session.isReady = false

		err := session.init(conn)

		if session.errorRetry(err, reInitDelay) {
			continue
		}
		select {
		case <-session.done:
			return true
		case <-session.notifyConnClose:
			session.logger.Println("Connection closed. Reconnecting...")
			return false
		case <-session.notifyChanClose:
			session.logger.Println("Channel closed. Re-running init...")
		}
	}
}

// init 将初始化channel并声明queue
func (session *Session) init(conn *amqp.Connection) error {
	ch, err := conn.Channel()

	if err != nil {
		return err
	}

	err = ch.Confirm(false)

	if err != nil {
		return err
	}
	_, err = ch.QueueDeclare(
		session.name,
		false, // Durable
		false, // Delete when unused
		false, // Exclusive
		false, // No-wait
		nil,   // Arguments
	)

	if err != nil {
		return err
	}

	session.changeChannel(ch)
	session.isReady = true
	session.done <- true
	session.logger.Println("init channel success")

	return nil
}

// changeConnection 将新连接连接到队列中，
// 并更新 Close 侦听器以反映这一点。
func (session *Session) changeConnection(connection *amqp.Connection) {
	session.connection = connection
	session.notifyConnClose = make(chan *amqp.Error)
	session.connection.NotifyClose(session.notifyConnClose)
}

// changeChannel 将新频道引入队列，
// 并更新通道侦听器以反映这一点。
func (session *Session) changeChannel(channel *amqp.Channel) {
	session.channel = channel
	session.notifyChanClose = make(chan *amqp.Error)
	session.notifyConfirm = make(chan amqp.Confirmation, 1)
	session.channel.NotifyClose(session.notifyChanClose)
	session.channel.NotifyPublish(session.notifyConfirm)
}

// Push 将数据推送到队列中，并等待确认。
// 如果在 resendTimeout 之前未收到确认，则
// 它会持续重新发送消息，直到收到确认。
// 这将阻塞，直到服务器发送确认。错误包括
// 仅在 push 操作本身失败时返回，请参阅 UnsafePush。
func (session *Session) Push(msg *Message) error {
	if !session.isReady {
		return errors.New("failed to push: not connected")
	}
	for {
		err := session.UnsafePush(msg)
		if err != nil {
			session.logger.Println("Push failed. Retrying...")
			select {
			case <-session.done:
				return errShutdown
			case <-time.After(resendDelay):
			}
			continue
		}
		select {
		case confirm := <-session.notifyConfirm:
			if confirm.Ack {
				//session.logger.Println("Push confirmed!")
				return nil
			}
		case <-time.After(resendDelay):
		}
		session.logger.Println("Push didn't confirm. Retrying...")
	}
}

// UnsafePush 将推送到队列，而不检查 确认。如果连接失败，它将返回错误。
// 不保证服务器是否会接收到消息。
func (session *Session) UnsafePush(msg *Message) error {
	if !session.isReady {
		return errNotConnected
	}
	return session.channel.Publish(
		"",           // Exchange
		session.name, // Routing key
		true,         // Mandatory
		false,        // Immediate
		msg.Publishing(),
	)
}

// Stream 将持续将队列项放入频道中。
// 需要使用delivery.Ack消费消息，并从队列中删除
// 已成功处理或交付。失败时delivery.nack。
// 忽略这一点将导致数据在服务器上堆积。
func (session *Session) Stream() (<-chan amqp.Delivery, error) {
	if !session.isReady {
		return nil, errNotConnected
	}
	return session.channel.Consume(
		session.name,
		"",    // Consumer
		false, // Auto-Ack
		false, // Exclusive
		false, // No-local
		false, // No-Wait
		nil,   // Args
	)
}

// Start 启动侦听服务，消费消息
func (session *Session) Start() error {
	stream, err := session.Stream()
	session.cache.StartGC(10 * time.Second)
	if err != nil {
		return err
	}

	for msg := range stream {
		if _, ok := session.cache.Get(msg.MessageId); ok {
			err := msg.Reject(false)
			if err != nil {
				session.logger.Println("Reject", err)
			}
			continue
		}

		tp, body := parseMessage(msg.Body)

		if v, ok := session.handleFn[tp]; ok {
			v(body)
		}
		if job, ok := session.handleJobs[tp]; ok {
			job.JobHandle(body)
		}

		session.cache.Set(msg.MessageId, msg.MessageId, time.Minute*120)

		err = msg.Ack(false)
		if err != nil {
			session.logger.Panic("consume listen err:", err.Error())
		}
	}

	return nil
}

// AddJob 添加处理对象
func (session *Session) AddJob(tp string, job IJob) {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.handleJobs[tp] = job
}

// AddFunc 添加处理函数
func (session *Session) AddFunc(tp string, fn func(string)) {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.handleFn[tp] = fn
}

// Close 将完全关闭 Channel 和 Connection。
func (session *Session) Close() error {
	if !session.isReady {
		return errAlreadyClosed
	}
	err := session.channel.Close()
	if err != nil {
		return err
	}
	err = session.connection.Close()
	if err != nil {
		return err
	}
	close(session.done)
	session.isReady = false
	return nil
}
