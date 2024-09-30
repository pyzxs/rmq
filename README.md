# rmq
消息中间件rabbitmq封装


```go
// InitQueue
//
//	@Description: 初始化RabbitMQ
//	@param conf
func InitRabbit(dsn string) {
	 session = rmq.New(DEFAULT_QUEUE_DEFAUT, dsn)
	// 注册消息处理句柄
	registerHandle(session)
	// 启动服务
	go session.Start()
}


```



消费消息

```go
func registerHandle(q *queue.Session) {
    // 通过函数进行消息处理
	q.AddFunc("user.register", func(s string) {
		fmt.Printf("执行队列接收函数%s\n", s)
	})
    // 通过接口消息处理
    q.AddJob("user.register",&UserRegisterJob)
}

# 获取实现IJob接口
type UserRegisterJob struct {
}

func (u *UserRegisterJob) JobHandle(s string) {
	fmt.Println("通过UserRegister处理")
}
```



生产者发布消息

```go
// 具体业务中
// rmq.NewMessage(id,"topic","daa") 消息ID， 业务主题，消息内容
go session.Push(rmq.NewMessage(unique.SnowFlaskID(1), "user.register", "测试数据"))
```

