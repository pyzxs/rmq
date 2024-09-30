package main

import (
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"
)

func TestSort(t *testing.T) {
	c := NewMemoryCache()
	l := []struct {
		key        string
		expiration time.Duration
		value      interface{}
	}{
		{"test1", 10 * time.Second, "test1"},
		{"test2", 20 * time.Second, "test2"},
		{"test3", 3 * time.Second, "test3"},
		{"test4", 5 * time.Second, "test4"},
	}

	for _, v := range l {
		c.Set(v.key, v.value, v.expiration)
	}

	its := sortItems{}
	for k, it := range c.items {
		im := &keyItem{expiration: it.expiration, key: k}
		its = append(its, im)
	}

	sort.Sort(its)

	le := len(its)

	if len(its)-2 > 0 {
		last := its[:le-2]
		for _, i2 := range last {
			delete(c.items, i2.key)
		}
	}

	t.Log(c.items)
}

func TestPushMessage(t *testing.T) {
	session := New(DEFAULT_QUEUE_NAME, "amqp://admin:admin@192.168.31.239:5672/")
	fmt.Println("开始发送消息")
	for i := 0; i < 96; i++ {
		err := session.Push(NewMessage(strconv.Itoa(i+1000), "order.stock", "hello world"))
		if err != nil {
			t.Error(err)
		}
	}
	session.Close()
}

func TestStartServe(t *testing.T) {
	session := New(DEFAULT_QUEUE_NAME, "amqp://admin:admin@192.168.31.239:5672/")
	defer session.Close()

	//session.AddFunc("order.stock", func(s string) {
	//	t.Log("addfunc", s)
	//})
	session.AddJob("order.stock", &OrderJob{})
	session.Start()
}

type OrderJob struct {
}

func (o *OrderJob) JobHandle(body string) {
	fmt.Println("orderJob", body)
}
