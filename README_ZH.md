# golang实现的rabbitmq封装

这是一个关于rabbitmq的封装包，实现了确认机制、事务机制和幂等性

## 安装

```shell
go get https://github.com/pyzxs/rmq@latest
```

## 示例

```go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/pyzxs/rmq"
)

// User 用户模型
type User struct {
	Id     string
	Name   string
	Email  string
	Mobile string
}

// RegisterJob 消费者处理消息Job
type RegisterJob struct {
}

// JobHandle 实现 IJob 接口
func (r *RegisterJob) JobHandle(s string) {
	u := User{}
	_ = json.Unmarshal([]byte(s), &u)
	fmt.Printf("%s register send email to %s\n", u.Name, u.Email)
}

// JobSendSms 消费者处理消息函数
func JobSendSms() func(string) {
	return func(s string) {
		u := User{}
		_ = json.Unmarshal([]byte(s), &u)
		fmt.Printf("%s register send sms to %s\n", u.Name, u.Mobile)
	}
}

func main() {
	forever := make(chan bool)
	u := User{
		Id:     "10001",
		Name:   "zxs",
		Email:  "zxs@163.com",
		Mobile: "12567823234",
	}

	ubyte, _ := json.Marshal(u)

	// 连接初始化，channel初始化等
	session := rmq.New(rmq.DEFAULT_QUEUE_NAME, "amqp://admin:admin@192.168.31.239:5672/")

	// 添加消费者处理方法
	session.AddFunc("user.register", JobSendSms())
	session.AddJob("user.register", &RegisterJob{})

	// 启动一个消费者侦听服务
	go session.Start()

	// 生产者发送消息
	session.Push(rmq.NewMessage("122", "user.register", string(ubyte)))

	<-forever
}

```

## 幂等

幂等性意味着操作在多次执行时与仅执行一次时具有相同的效果。在项目中创建一个内存缓存，存放MessageId。已消费的消息拒绝

```go
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
```