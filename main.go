package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"

	"github.com/HiroseKakeru/mqtt-speedtest/pkg/mqtt"
	"github.com/HiroseKakeru/mqtt-speedtest/service"
	"github.com/joho/godotenv"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

var exitCode = 0

func exitWithErr(err any) {
	exitCode = 1
	log.Println(err)
}
func main() {
	defer os.Exit(exitCode)

	mqttClientManager := mqtt.Init()
	defer mqttClientManager.Close()

	ctx := context.Background()

	size := 8 * 1024 // 8kB
	data := make([]byte, size)

	// ランダムなデータを生成
	_, err := rand.Read(data)
	if err != nil {
		fmt.Println("Error generating random data:", err)
		return
	}

	if err := service.SendSpeedTest(ctx, data); err != nil {
		return
	}

}
