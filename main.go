package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goburrow/modbus"
	"github.com/robfig/cron/v3"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func init() {
	pflag.String("host", "127.0.0.1:502", "请求地址")
	pflag.Int("timeout", 30, "超时时间")
	pflag.Int("idle", 30, "空闲时间")
	pflag.Int("deviceid", 1, "站号")
	pflag.Int("rate", 1, "采集周期")
	pflag.Int("area", 3, "采集区域")
	pflag.Uint16("address", 0, "读取地址")
	pflag.Uint16("quantity", 10, "寄存器个数")
	pflag.String("display", "", "显示类型")

	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	viper.SetConfigType("env")
	viper.AutomaticEnv()
}

func main() {
	var (
		host     = viper.GetString("host")
		timeout  = viper.GetInt("timeout")
		idle     = viper.GetInt("idle")
		deviceId = viper.GetInt("deviceid")
		rate     = viper.GetInt("rate")
		area     = viper.GetInt("area")
		address  = uint16(viper.GetUint32("address"))
		quantity = uint16(viper.GetUint32("quantity"))
	)
	handler := modbus.NewTCPClientHandler(host)
	handler.Timeout = time.Duration(timeout) * time.Second
	handler.SlaveId = byte(deviceId)
	handler.IdleTimeout = time.Duration(idle) * time.Second
	handler.Logger = log.New(os.Stdout, fmt.Sprintf("host=%s ", host), log.LstdFlags)
	err := handler.Connect()
	if err != nil {
		log.Fatalln("create modbus conn err, ", err.Error())
	}
	defer func() {
		if err := handler.Close(); err != nil {
			log.Println("close modbus conn err, ", err.Error())
		}
	}()
	client := modbus.NewClient(handler)
	c := cron.New(cron.WithSeconds(), cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	c.Start()
	defer c.Stop()
	if _, err = c.AddFunc(fmt.Sprintf("@every %ds", rate), func() {
		var rs []byte
		var err error
		switch area {
		case 1:
			rs, err = client.ReadCoils(address, quantity)
		case 2:
			rs, err = client.ReadDiscreteInputs(address, quantity)
		case 3:
			rs, err = client.ReadHoldingRegisters(address, quantity)
		case 4:
			rs, err = client.ReadInputRegisters(address, quantity)
		default:
			log.Fatalln("no found area")
		}

		if err != nil {
			log.Fatalln("read err, ", err.Error())
		}

		log.Println("read result, ", rs)
	}); err != nil {
		log.Fatalln("cron add err, ", err.Error())
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	sig := <-ch
	log.Println("close service,", sig)
	os.Exit(0)
}
