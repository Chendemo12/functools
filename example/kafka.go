package example

import (
	"fmt"
	"github.com/Chendemo12/functools/kafkac"
	"time"
)

func Example_KafkaClient_NewAsyncProducer() {
	kc := kafkac.KafkaClient{
		Addrs:   []string{"10.64.5.70:30095", "10.64.5.70:30094"},
		GroupId: "FLYING",
	}

	go func() {
		err := kc.NewAsyncProducer()
		if err != nil {
			fmt.Println("kafka: NewAsyncProducer failed: " + err.Error())
		}
	}()

	for {
		kc.SendMessage("FLYING1", []byte("1"), []byte("hello")) // 生产一条数据
		time.Sleep(2000 * time.Microsecond)
	}
	// Output:
	// ...
}
