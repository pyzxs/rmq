# go rabbitmq wrapper
[![Go Report Card](https://goreportcard.com/badge/github.com/pyzxs/rmq)](https://goreportcard.com/report/github.com/pyzxs/rmq)
[![Go Reference](https://pkg.go.dev/badge/github.com/pyzxs/rmq.svg)](https://pkg.go.dev/github.com/pyzxs/rmq)

This is a golang package for rabbitmq 

[English](README.md) | [中文](README_ZH.md)
## Install

```shell
go get https://github.com/pyzxs/rmq@latest
```
## License
This client uses the same 2-clause BSD license as the original project.


## Example

```go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/pyzxs/rmq"
)

// User  mode
type User struct {
	Id     string
	Name   string
	Email  string
	Mobile string
}

// RegisterJob a consumer handle Job
type RegisterJob struct {
}

// JobHandle implement IJob interface{}
func (r *RegisterJob) JobHandle(s string) {
	u := User{}
	_ = json.Unmarshal([]byte(s), &u)
	fmt.Printf("%s register send email to %s: %s\n", u.Name, u.Email)
}

// JobSendSms handle func 
func JobSendSms() func(string) {
	return func(s string) {
		u := User{}
		_ = json.Unmarshal([]byte(s), &u)
		fmt.Printf("%s register send sms to %s: %s\n", u.Name, u.Mobile)
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
    
	// init connection and channel
	session := rmq.New(rmq.DEFAULT_QUEUE_NAME, "amqp://admin:admin@192.168.31.239:5672/")

	// add consumer handle method
	session.AddFunc("user.register", JobSendSms())
	session.AddJob("user.register", &RegisterJob{})
    
	// start a consumer 
	go session.Start()

	// publisher send message
	session.Push(rmq.NewMessage("122", "user.register", string(ubyte)))

	<-forever
}

```

## idempotent

Idempotency means that an operation has the same effect when performed multiple times as it does only once

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