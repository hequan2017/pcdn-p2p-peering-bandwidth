package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	hequanid   string `json:"hequanid"`
	ServerIP    string `json:"serverip"`
	ServerPort  int    `json:"serverport"`
	ClientPort  int    `json:"clientport"`
	Network     string `json:"network"`
	Tohequanid string `json:"tohequanid"`
}

const HAND_SHAKE_MSG = "我是打洞消息"

var uploadRateFixed = 0.00
var uploadRateDynamics = 0.00

func parseAddr(addr string) net.UDPAddr {
	t := strings.Split(addr, ":")
	port, _ := strconv.Atoi(t[1])
	return net.UDPAddr{
		IP:   net.ParseIP(t[0]),
		Port: port,
	}
}

type SpeedInfo struct {
	MaxDownloadRate float64 `json:"maxDownloadRate"`
}

func bidirectionalHole(log *log.Logger, srcAddr *net.UDPAddr, anotherAddr *net.UDPAddr, hequanid, Tohequanid string, uploadRate float64, downloadRate float64) {

	conn, err := net.DialUDP("udp", srcAddr, anotherAddr)
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()
	// 向另一个peer发送一条udp消息(对方peer的nat设备会丢弃该消息,非法来源),用意是在自身的nat设备打开一条可进入的通道,这样对方peer就可以发过来udp消息
	if _, err = conn.Write([]byte(HAND_SHAKE_MSG)); err != nil {
		log.Println("send handshake:", err)
	}
	go func() {
		for {
			pattern := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
			data := bytes.Repeat(pattern, 1*1024*1024/len(pattern))
			packetSize := 1400 // 根据实际情况设置包的大小
			//startTime := time.Now()
			upload := uploadRate
			if uploadRateFixed != 0.00 && uploadRateDynamics != 0.00 {
				if uploadRateFixed > uploadRateDynamics {
					upload = uploadRateDynamics * 1.2
				}
			}
			_ = sendLargeData(log, conn, data, packetSize, upload) // 调用函数
			//totalBytesSent := sendLargeData(log, conn, data, packetSize, upload) // 调用函数
			//elapsedTime := time.Since(startTime) // 计算发送数据所花费的总时间
			//uploadSpeedMbps := (float64(totalBytesSent) * 8) / (1024 * 1024) / elapsedTime.Seconds()
			//fmt.Printf("%s --> %s -------------上传速率: %.2f Mbps\n", hequanid, Tohequanid, uploadSpeedMbps)
			//log.Printf("%s --> %s -------------上传速率: %.2f Mbps\n", hequanid, Tohequanid, uploadSpeedMbps)

		}
	}()
	for {
		bytesPerSecond := downloadRate * 1024 * 1024 / 8 // 将速率从兆字节转换为字节
		bufferSize := 1400                               // 定义缓冲区大小
		data := make([]byte, bufferSize)
		readInterval := time.Duration(bufferSize) * time.Second / time.Duration(bytesPerSecond)
		var totalBytes int64         // 累计接收到的字节数
		totalStartTime := time.Now() // 获取当前时间
		for {
			startTime := time.Now() // 获取当前时间
			n, _, err1 := conn.ReadFromUDP(data)
			if err1 != nil {
				//log.Printf("Error during read: %s\n", err1)
				continue // 如果发生错误则跳过当前循环
			}
			// 成功读取数据
			totalBytes += int64(n)
			// 计算读取操作所需的时间，然后暂停直到这个时间过去
			time.Sleep(readInterval - time.Since(startTime))

			elapsed := time.Since(totalStartTime)
			if elapsed >= 5*time.Second { // 如果超过或等于10秒，计算并打印速率
				speedMbps := (float64(totalBytes) * 8) / (1024 * 1024) / elapsed.Seconds() // 转换为Mbps
				fmt.Printf("%s <-- %s -------------下载速率: %.2f Mbps \n", hequanid, Tohequanid, speedMbps)
				// 重置计数器和计时器
				totalBytes = 0
				totalStartTime = time.Now()
				speedInfo := SpeedInfo{MaxDownloadRate: speedMbps}
				infoData, _ := json.Marshal(speedInfo)
				conn.Write(infoData) // 发送当前上传速率信息

				if elapsed >= 600*time.Second {
					log.Printf("%s <-- %s -------------下载速率: %.2f Mbps \n", hequanid, Tohequanid, speedMbps)
				}
			}

			// 尝试解析作为JSON的速度信息
			if int64(n) < 1000 {
				var speedInfo SpeedInfo
				if err := json.Unmarshal(data[:n], &speedInfo); err == nil {
					// 成功解析为速度信息
					fmt.Printf("%s -->>>> %s ----对方服务器-------下载速率: %.2f Mbps \n", hequanid, Tohequanid, speedInfo.MaxDownloadRate)
					uploadRateDynamics = speedInfo.MaxDownloadRate
					if elapsed >= 600*time.Second {
						log.Printf("%s -->>>> %s ----对方服务器-------下载速率: %.2f Mbps \n", hequanid, Tohequanid, speedInfo.MaxDownloadRate)
					}
				}
			}
		}
	}
}

func sendLargeData(log *log.Logger, conn *net.UDPConn, data []byte, packetSize int, uploadRate float64) int64 {
	totalLen := len(data)
	bytesPerSecond := uploadRate * 1024 * 1024 / 8 // 将 Mbps 转换为字节/秒
	packetInterval := time.Second * time.Duration(packetSize) / time.Duration(bytesPerSecond)
	var totalBytesSent int64 = 0
	for i := 0; i < totalLen; i += packetSize {
		packetStartTime := time.Now() // 每个包的发送开始时间
		end := i + packetSize
		if end > totalLen {
			end = totalLen
		}
		n, err := conn.Write(data[i:end])
		if err != nil {
			//log.Printf("Failed to send packet: %s\n", err)
			continue
		}
		totalBytesSent += int64(n) // 更新发送的总字节数

		// 确保控制发送速率
		time.Sleep(packetInterval - time.Since(packetStartTime))
	}
	return totalBytesSent
}
func main() {

	logFile, err := os.OpenFile("peer.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}
	defer logFile.Close()

	// 创建一个新的日志记录器
	logger := log.New(logFile, "", log.Ldate|log.Ltime)

	// 定义命令行参数
	hequanid := flag.String("hequanid", "", "Identifier of the machine")
	serverIP := flag.String("serverip", "", "IP address of the server")
	serverPort := flag.Int("serverport", 0, "Port number of the server")
	clientPort := flag.Int("clientport", 0, "Client port number to use")
	network := flag.String("network", "", "Network interfaces to use")
	tohequanid := flag.String("tohequanid", "", "to Target machine identifier")
	timeout := flag.Int("time", 0, "Timeout in minutes after which the program will exit")
	uploadRate := flag.Float64("uploadrate", 0, "uploadrate rate in Mbps")
	downloadRate := flag.Float64("downloadrate", 0, "downloadrate rate in Mbps")
	// 解析命令行参数
	flag.Parse()
	// 设置超时定时器，到时间后退出程序

	time.AfterFunc(time.Duration(*timeout)*time.Minute, func() {
		logger.Printf("----------------程序运行超过 %d 分钟，已经自动退出。-------------------\n", *timeout)
		os.Exit(0)
	})

	logger.Printf("------------------程序运行超过 %d 分钟，将会自动退出。-------------------------\n", *timeout)

	// 参数验证
	if *hequanid == "" || *serverIP == "" || *serverPort == 0 || *clientPort == 0 || *network == "" || *tohequanid == "" || *timeout == 0 || *uploadRate == 0 || *downloadRate == 0 {
		logger.Println("All parameters are required and must not be empty")
		flag.Usage()
		os.Exit(1) // 退出程序
	}
	uploadRateFixed = *uploadRate
	networkList := strings.Split(*network, ",")

	for _, v := range networkList {
		go func(v string) {
			iface, _ := net.InterfaceByName(v)
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						logger.Printf("IPv4 address: %s\n", ipnet.IP.String())
						// 使用解析的参数建立UDP连接
						srcAddr := &net.UDPAddr{IP: ipnet.IP.To4(), Port: *clientPort}
						dstAddr := &net.UDPAddr{IP: net.ParseIP(*serverIP), Port: *serverPort}
						conn, err := net.DialUDP("udp", srcAddr, dstAddr)
						if err != nil {
							//logger.Println("Error dialing UDP:", err)
							continue
						}
						defer conn.Close()
						// 连接成功，打印配置信息
						logger.Printf("Connected to %s from %s\n", dstAddr, srcAddr)
						//logger.Printf("hequanid: %s, Network: %s, Tohequanid: %s  \n", *hequanid, *network, *tohequanid)

						tag := Config{
							hequanid:   *hequanid + "_" + v,
							ServerIP:    *serverIP,
							ServerPort:  *serverPort,
							ClientPort:  *clientPort,
							Network:     *network,
							Tohequanid: *tohequanid + "_" + v,
						}

						tagStr, _ := json.Marshal(tag)
						if _, err = conn.Write(tagStr); err != nil {
							//logger.Printf("error during read: %s", err)
							continue
						}
						data := make([]byte, 1024)
						n, remoteAddr, err := conn.ReadFromUDP(data)
						if err != nil {
							//logger.Printf("error during read: %s", err)
						}
						conn.Close()
						anotherPeer := parseAddr(string(data[:n]))
						logger.Printf("打洞开始: local:%s server:%s another:%s\n\n", srcAddr, remoteAddr, anotherPeer.String())

						// 开始打洞

						bidirectionalHole(logger, srcAddr, &anotherPeer, *hequanid+"_"+v, *tohequanid+"_"+v, *uploadRate, *downloadRate)
					}
				}
			}
		}(v)
	}
	select {}
}
