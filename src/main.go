package main

import (
	"ShadowPlayer/src/net"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	go net.Start()

	fmt.Println("服务启动中")
	fmt.Println("按 Ctrl+C 退出程序")

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Printf("收到退出信号: %v", sig)
		done <- true
	}()
	log.Println("服务器运行中，等待退出信号")
	<-done
	log.Println("正在退出")
}
