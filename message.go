package main

import (
	"encoding/json"
	"github.com/streadway/amqp"
	"time"
)

type HandleFunc func(delivery *amqp.Delivery)

type Message struct {
	Id   string
	Type string
	Body string
}

func NewMessage(id, tp string, body string) *Message {
	return &Message{
		Id:   id,
		Type: tp,
		Body: body,
	}
}

// Publishing 构建publishing结构体
func (m *Message) Publishing() amqp.Publishing {
	body := map[string]string{"type": m.Type, "body": m.Body}
	b, _ := json.Marshal(body)
	return amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		MessageId:    m.Id,
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Body:         b,
	}
}
