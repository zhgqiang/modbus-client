package app

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/goburrow/modbus"
	"github.com/imroc/biu"
	"github.com/robfig/cron/v3"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	Binary = iota
	HEX
	UnsignedDecimal
	Integer
	LongInteger
	LongSwapped
	Float
	FloatSwapped
	Double
	DoubleSwapped
)

func init() {
	pflag.String("host", "127.0.0.1:502", "modbus server")
	pflag.Int("timeout", 30, "request timeout")
	pflag.Int("idle", 30, "connect idle time")
	pflag.Int("deviceId", 1, "device id")
	pflag.Int("rate", 1, "read rate")
	pflag.Int("area", 3, "read area [0 1 2 3]")
	pflag.Uint16("address", 0, "read address")
	pflag.Uint16("quantity", 10, "read quantity")
	pflag.Uint8("display", 9, "display data format [Binary:0 HEX:1 UnsignedDecimal:2 Integer:3 LongInteger:4 LongSwapped:5 Float:6 FloatSwapped:7 Double:8 DoubleSwapped:9]")

	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	viper.SetConfigType("env")
	viper.AutomaticEnv()
}

func Run() {
	var (
		host     = viper.GetString("host")
		timeout  = viper.GetInt("timeout")
		idle     = viper.GetInt("idle")
		deviceId = viper.GetInt("deviceId")
		rate     = viper.GetInt("rate")
		area     = viper.GetInt("area")
		address  = uint16(viper.GetUint32("address"))
		quantity = uint16(viper.GetUint32("quantity"))
		display  = uint8(viper.GetUint32("display"))
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

		result, err := convert(display, rs)
		if err != nil {
			log.Fatalln("result convert err, ", err.Error())
		}
		log.Printf("read result, %+v \n", result)
	}); err != nil {
		log.Fatalln("cron add err, ", err.Error())
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	sig := <-ch
	log.Println("close service,", sig)
	os.Exit(0)
}

func convert(display uint8, data []byte) (interface{}, error) {
	l := len(data)
	switch display {
	case Binary:
		c := make([]string, l)
		for i := 0; i < l; i++ {
			c[i] = biu.ToBinaryString(data[i])
		}
		return c, nil
	case HEX:
		c := make([]string, l/2)
		b := make([]byte, 2)
		for i := 0; i < l/2; i++ {
			b[0] = data[i*2]
			b[1] = data[i*2+1]
			c[i] = hex.EncodeToString(b)
		}
		return c, nil
	case UnsignedDecimal:
		c := make([]uint16, l/2)
		b := make([]byte, 2)
		for i := 0; i < l/2; i++ {
			b[0] = data[i*2]
			b[1] = data[i*2+1]
			rs := new(uint16)
			err := binary.Read(bytes.NewReader(b), binary.BigEndian, rs)
			if err != nil {
				return nil, err
			}
			c[i] = *rs
		}
		return c, nil
	case Integer:
		c := make([]int16, l/2)
		b := make([]byte, 2)
		for i := 0; i < l/2; i++ {
			b[0] = data[i*2]
			b[1] = data[i*2+1]
			rs := new(int16)
			err := binary.Read(bytes.NewReader(b), binary.BigEndian, rs)
			if err != nil {
				return nil, err
			}
			c[i] = *rs
		}
		return c, nil
	case LongInteger:
		c := make([]int32, l/4)
		b := make([]byte, 4)
		for i := 0; i < l/4; i++ {
			b[2] = data[i*4]
			b[3] = data[i*4+1]
			b[0] = data[i*4+2]
			b[1] = data[i*4+3]
			rs := new(int32)
			err := binary.Read(bytes.NewReader(b), binary.BigEndian, rs)
			if err != nil {
				return nil, err
			}
			c[i] = *rs
		}
		return c, nil
	case LongSwapped:
		c := make([]int32, l/4)
		b := make([]byte, 4)
		for i := 0; i < l/4; i++ {
			b[0] = data[i*4]
			b[1] = data[i*4+1]
			b[2] = data[i*4+2]
			b[3] = data[i*4+3]
			rs := new(int32)
			err := binary.Read(bytes.NewReader(b), binary.BigEndian, rs)
			if err != nil {
				return nil, err
			}
			c[i] = *rs
		}
		return c, nil
	case Float:
		c := make([]float32, l/4)
		b := make([]byte, 4)
		for i := 0; i < l/4; i++ {
			b[2] = data[i*4]
			b[3] = data[i*4+1]
			b[0] = data[i*4+2]
			b[1] = data[i*4+3]
			rs := new(float32)
			err := binary.Read(bytes.NewReader(b), binary.BigEndian, rs)
			if err != nil {
				return nil, err
			}
			c[i] = *rs
		}
		return c, nil
	case FloatSwapped:
		c := make([]float32, l/4)
		b := make([]byte, 4)
		for i := 0; i < l/4; i++ {
			b[0] = data[i*4]
			b[1] = data[i*4+1]
			b[2] = data[i*4+2]
			b[3] = data[i*4+3]
			rs := new(float32)
			err := binary.Read(bytes.NewReader(b), binary.BigEndian, rs)
			if err != nil {
				return nil, err
			}
			c[i] = *rs
		}
		return c, nil
	case Double:
		c := make([]float64, l/8)
		b := make([]byte, 8)
		for i := 0; i < l/8; i++ {
			b[6] = data[i*8]
			b[7] = data[i*8+1]
			b[4] = data[i*8+2]
			b[5] = data[i*8+3]
			b[2] = data[i*8+4]
			b[3] = data[i*8+5]
			b[0] = data[i*8+6]
			b[1] = data[i*8+7]
			rs := new(float64)
			err := binary.Read(bytes.NewReader(b), binary.BigEndian, rs)
			if err != nil {
				return nil, err
			}
			c[i] = *rs
		}
		return c, nil
	case DoubleSwapped:
		c := make([]float64, l/8)
		b := make([]byte, 8)
		for i := 0; i < l/8; i++ {
			b[0] = data[i*8]
			b[1] = data[i*8+1]
			b[2] = data[i*8+2]
			b[3] = data[i*8+3]
			b[4] = data[i*8+4]
			b[5] = data[i*8+5]
			b[6] = data[i*8+6]
			b[7] = data[i*8+7]
			rs := new(float64)
			err := binary.Read(bytes.NewReader(b), binary.BigEndian, rs)
			if err != nil {
				return nil, err
			}
			c[i] = *rs
		}
		return c, nil
	default:
		return nil, errors.New("not found data type")
	}
}
