package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type Config struct {
	ID         string `json:"ID"`
	ServerIP   string `json:"serverip"`
	ServerPort int    `json:"serverport"`
	ClientPort int    `json:"clientport"`
	Network    string `json:"network"`
	ToID       string `json:"toID"`
}

type Peer struct {
	Addr      net.UDPAddr
	Timestamp time.Time
}

var (
	configs = make(map[string]Config)
	peers   = make(map[string]Peer)
	mu      sync.Mutex
)

func main() {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 9981})
	if err != nil {
		fmt.Println(err)
		return
	}
	log.Printf("本地地址: <%s> \n", listener.LocalAddr().String())

	go cleanUpConfigs()

	for {
		data := make([]byte, 1024)
		n, remoteAddr, err := listener.ReadFromUDP(data)
		if err != nil {
			fmt.Printf("读取错误: %s", err)
			continue
		}
		log.Printf("接收到 <%s> 的数据: %s\n", remoteAddr.String(), data[:n])

		var config Config
		err = json.Unmarshal(data[:n], &config)
		if err != nil {
			log.Printf("JSON反序列化错误: %s", err)
			continue
		}

		mu.Lock()
		configs[config.ID] = config
		peers[config.ID] = Peer{Addr: *remoteAddr, Timestamp: time.Now()}
		mu.Unlock()

		mu.Lock()
		targetConfig, exists := configs[config.ToID]
		if exists {
			targetPeer, addrExists := peers[targetConfig.ID]
			if addrExists {
				message := fmt.Sprintf("给 %s 的 目标server: %s", config.ID, targetPeer.Addr.String())
				message1 := fmt.Sprintf("给 %s 的 目标server: %s", config.ToID, remoteAddr.String())
				sourcePeer := peers[config.ToID]
				listener.WriteToUDP([]byte(targetPeer.Addr.String()), remoteAddr)
				listener.WriteToUDP([]byte(remoteAddr.String()), &sourcePeer.Addr)
				log.Printf(" \n %s \n %s \n", message, message1)
				delete(configs, config.ID)
				delete(configs, config.ToID)
			}
		}
		log.Printf("当前configs数量: %d ", len(configs))
		mu.Unlock()
	}
}

func cleanUpConfigs() {
	for {
		time.Sleep(30 * time.Minute)
		mu.Lock()
		for id, peer := range peers {
			if time.Since(peer.Timestamp) > 30*time.Minute {
				log.Printf("删除超时配置: %s", id)
				delete(configs, id)
				delete(peers, id)
			}
		}
		mu.Unlock()
	}
}