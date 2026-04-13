package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dccoding1118/more-line-points/cmd/scheduler/cli"
	"github.com/joho/godotenv"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
	time.Local = time.FixedZone("Asia/Taipei", 8*3600)
}

func main() {
	// 嘗試強制載入 .env（覆蓋目前 session 殘留的變數，例如舊的空字串）
	_ = godotenv.Overload()

	// Set up log rotation
	err := os.MkdirAll("logs", 0o750)
	if err == nil {
		log.SetOutput(&lumberjack.Logger{
			Filename:   filepath.Join("logs", "more-line-points.log"),
			MaxSize:    10, // megabytes
			MaxBackups: 30,
			MaxAge:     30, // days
			Compress:   true,
		})
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cli.Execute(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
