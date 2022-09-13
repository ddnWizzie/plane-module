package transport

import "fmt"

type Transport interface {
	Publish([]byte)
	GetTopic() string
}

type Screen string

func (s *Screen) Publish(msg []byte) {
	fmt.Println(string(msg))
}

func (s *Screen) GetTopic() string {
	return string(*s)
}
