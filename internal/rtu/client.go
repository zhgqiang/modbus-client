package rtu

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
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
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
	pflag.String("addr", "COM1", "串口号")
	pflag.String("parity", "N", "N、O、E 分别表示无校验、奇校验、偶校验")
	pflag.Int("baudrate", 115200, "波特率 [300 600 1200 2400 4800 9600 14400 19200 38400 57600 115200]")
	pflag.Int("databits", 8, "数据位 [5 6 7 8]")
	pflag.Int("stopbits", 1, "停止位 [1 2]")
	pflag.Bool("rs485", false, "RS485模式")
	pflag.Int("timeout", 30, "request timeout")
	pflag.Int("idle", 30, "connect idle time")
	pflag.Int("deviceId", 1, "device id")
	pflag.Int("rate", 1, "read rate")
	pflag.Int("area", 3, "read area [0 1 2 3]")
	pflag.Uint16("address", 0, "read address")
	pflag.Uint16("length", 10, "read length")
	pflag.Uint8("display", 9, "display data format [Binary:0 HEX:1 UnsignedDecimal:2 Integer:3 LongInteger:4 LongSwapped:5 Float:6 FloatSwapped:7 Double:8 DoubleSwapped:9]")
	pflag.IntSlice("data", []int{}, "write data")
	pflag.Int("delay", 1, "delay ms")
	pflag.Parse()
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic(err)
	}
	viper.SetConfigType("env")
	viper.AutomaticEnv()
}

func Run() {
	var (
		rate    = viper.GetInt("rate")
		area    = viper.GetInt("area")
		delay   = viper.GetInt("delay")
		address = uint16(viper.GetUint32("address"))
		length  = uint16(viper.GetUint32("length"))
		display = uint8(viper.GetUint32("display"))
	)
	handler := getHandler()
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
			rs, err = client.ReadCoils(address, length)
		case 2:
			rs, err = client.ReadDiscreteInputs(address, length)
		case 3:
			rs, err = client.ReadHoldingRegisters(address, length)
		case 4:
			rs, err = client.ReadInputRegisters(address, length)
		default:
			log.Fatalln("no found area")
		}

		if err != nil {
			if strings.Contains(err.Error(), "timeout") {
				log.Printf("timeout: %s \n", err.Error())
				err := handler.Close()
				if err != nil {
					log.Printf("连接关闭失败,%s \n", err.Error())
				}
				time.Sleep(time.Millisecond * time.Duration(delay))
				handler = getHandler()
				err = handler.Connect()
				if err != nil {
					log.Printf("设备timeout重连失败,%s \n", err.Error())
				} else {
					client = modbus.NewClient(handler)
				}
			} else if strings.Contains(err.Error(), "An established connection was aborted by the software in your host machine") || strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host") || strings.Contains(err.Error(), "broken pipe") {
				log.Printf("An established connection was aborted by the software in your host machine: %s \n", err.Error())
				time.Sleep(time.Millisecond * time.Duration(delay))
				handler = getHandler()
				err = handler.Connect()
				if err != nil {
					log.Printf("设备重连失败,%s \n", err.Error())
				} else {
					client = modbus.NewClient(handler)
				}
			} else if err == io.EOF {
				log.Printf("eof: %s \n", err.Error())
				time.Sleep(time.Millisecond * time.Duration(delay))
				handler = getHandler()
				err = handler.Connect()
				if err != nil {
					log.Printf("设备EOF重连失败,%s \n", err.Error())
				} else {
					client = modbus.NewClient(handler)
				}
			} else {
				log.Fatalln("read err, ", err.Error())
			}
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

func getHandler() *modbus.RTUClientHandler {
	var (
		host     = viper.GetString("addr")
		timeout  = viper.GetInt("timeout")
		idle     = viper.GetInt("idle")
		deviceId = viper.GetInt("deviceId")
		parity   = viper.GetString("parity")
		baudrate = viper.GetInt("baudrate")
		databits = viper.GetInt("databits")
		stopbits = viper.GetInt("stopbits")
		rs485    = viper.GetBool("rs485")
	)
	handler := modbus.NewRTUClientHandler(host)
	handler.Timeout = time.Duration(timeout) * time.Second
	handler.SlaveId = byte(deviceId)
	//log.Printf("连接options: %+v \n", p.config.Options)
	handler.BaudRate = baudrate
	handler.DataBits = databits
	handler.StopBits = stopbits
	handler.Parity = parity
	handler.RS485.Enabled = rs485
	handler.IdleTimeout = time.Duration(idle) * time.Second
	handler.Logger = log.New(os.Stdout, fmt.Sprintf("host=%s ", host), log.LstdFlags)
	return handler
}

func Write() {
	var (
		host     = viper.GetString("host")
		timeout  = viper.GetInt("timeout")
		idle     = viper.GetInt("idle")
		deviceId = viper.GetInt("deviceId")
		area     = viper.GetInt("area")
		address  = uint16(viper.GetUint32("address"))
		length   = uint16(viper.GetUint32("length"))
		data     = viper.GetIntSlice("data")
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
	buf := new(bytes.Buffer)
	uint16s := make([]uint16, 0)
	for _, d := range data {
		uint16s = append(uint16s, uint16(d))
	}
	err = binary.Write(buf, binary.BigEndian, uint16s)
	if err != nil {
		log.Fatalln("binary Write failed, ", err.Error())
	}
	var rs []byte
	switch area {
	case 1:
		rs, err = client.WriteMultipleCoils(address, length, buf.Bytes())
	case 3:
		rs, err = client.WriteMultipleRegisters(address, length, buf.Bytes())
	default:
		log.Fatalln("no found area")
	}

	if err != nil {
		log.Fatalln("write err, ", err.Error())
	}
	log.Printf("write result, %s \n", string(rs))

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
