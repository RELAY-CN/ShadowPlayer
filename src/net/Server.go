package net

import (
	"ShadowPlayer/src/data"
	"ShadowPlayer/src/http"
	_type "ShadowPlayer/src/type"
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxConnections = 1000       // 最大并发连接数
	maxMessageSize = 512 * 1024 // 单条消息最大0.5MB
	readTimeout    = 30 * time.Second
	writeTimeout   = 30 * time.Second
)

type ConnectionData struct {
	Conn         net.Conn
	IP           string
	Port         int32
	IsFog        bool
	proxy        *ProxyConnection
	packet160    *_type.Packet
	received106  bool
	ClientIP     string
	OldPlayerHex string
	NewPlayerHex string
	mu           sync.RWMutex
}

func NewConnectionData(conn net.Conn) *ConnectionData {
	return &ConnectionData{
		Conn:  conn,
		IsFog: false,
	}
}

func (cd *ConnectionData) GetIP() string {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	return cd.IP
}

func (cd *ConnectionData) SetIP(ip string) {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.IP = ip
}

func (cd *ConnectionData) GetPort() int32 {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	return cd.Port
}

func (cd *ConnectionData) SetPort(port int32) {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.Port = port
}

func (cd *ConnectionData) GetIsFog() bool {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	return cd.IsFog
}

func (cd *ConnectionData) SetIsFog(isFog bool) {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.IsFog = isFog
}

var (
	connSemaphore     = make(chan struct{}, maxConnections)
	activeConnections sync.Map
)

func Start() {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(int(data.GlobalConfig.Port)))
	if err != nil {
		log.Printf("端口 %d 已被占用: %v", data.GlobalConfig.Port, err)
		fmt.Println("\n端口被占用，请检查配置或关闭占用该端口的程序")
		fmt.Println("按回车键退出...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		os.Exit(1)
	}
	defer listener.Close()

	log.Println("服务器启动，监听端口:", data.GlobalConfig.Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("接受连接错误: %v", err)
			continue
		}

		select {
		case connSemaphore <- struct{}{}:
			go func(c net.Conn) {
				connData := NewConnectionData(c)
				defer func() {
					connData.mu.Lock()
					if connData.proxy != nil {
						connData.proxy.Close()
						connData.proxy = nil
					}
					connData.mu.Unlock()

					activeConnections.Range(func(key, value interface{}) bool {
						if value == connData {
							storedKey, ok := key.(string)
							if ok {
								activeConnections.Delete(storedKey)
							}
							return false
						}
						return true
					})
					c.Close()
					<-connSemaphore
				}()
				handleBinaryConnection(connData)
			}(conn)
		default:
			conn.Close()
			log.Println("连接数已达上限，拒绝新连接")
		}
	}
}

func handleBinaryConnection(connData *ConnectionData) {
	defer connData.Conn.Close()

	reader := bufio.NewReader(connData.Conn)

	for {
		connData.Conn.SetReadDeadline(time.Now().Add(readTimeout))

		var msgLen int32
		if err := binary.Read(reader, binary.BigEndian, &msgLen); err != nil {
			if err != io.EOF {
				log.Printf("读取消息长度错误: %v", err)
			}
			return
		}

		if msgLen <= 0 || msgLen > maxMessageSize {
			log.Printf("非法消息长度: %d", msgLen)
			return
		}

		var msgType int32
		if err := binary.Read(reader, binary.BigEndian, &msgType); err != nil {
			log.Printf("读取消息类型错误: %v", err)
			return
		}

		msgData := getBuffer(msgLen)
		if _, err := io.ReadFull(reader, msgData); err != nil {
			log.Printf("读取消息体错误: %v", err)
			putBuffer(msgData)
			return
		}

		processBinaryMessage(connData, _type.Packet{
			Type:  msgType,
			Bytes: msgData,
		})

		putBuffer(msgData)
	}
}

var (
	bucketSizes = []int{4 * 1024, 64 * 1024, 512 * 1024}
	bufferPools = func() []sync.Pool {
		pools := make([]sync.Pool, len(bucketSizes))
		for i := range bucketSizes {
			size := bucketSizes[i]
			pools[i] = sync.Pool{New: func() interface{} { return make([]byte, size) }}
		}
		return pools
	}()
)

func pickBucket(size int32) int {
	for i, b := range bucketSizes {
		if int(size) <= b {
			return i
		}
	}
	return -1
}

func getBuffer(size int32) []byte {
	idx := pickBucket(size)
	if idx < 0 {
		return make([]byte, size)
	}
	buf := bufferPools[idx].Get().([]byte)
	return buf[:size]
}

func putBuffer(buf []byte) {
	capBuf := cap(buf)
	for i, b := range bucketSizes {
		if capBuf == b {
			bufferPools[i].Put(buf[:b])
			return
		}
	}
}

func processBinaryMessage(connData *ConnectionData, packet _type.Packet) {
	defer func() {
		if r := recover(); r != nil {
			connData.Conn.Close()
		}
	}()
	connData.Conn.SetWriteDeadline(time.Now().Add(writeTimeout))

	connData.mu.RLock()
	proxy := connData.proxy
	connData.mu.RUnlock()

	if proxy != nil && proxy.IsConnected() {
		if packet.Type == 109 {
			return
		}
		if packet.Type == 110 {
			packet110, err := Analysis_110(packet)
			if err != nil {
				log.Printf("解析110包失败: %v", err)
				proxy.ForwardPacket(packet)
				return
			}
			oldHex := packet110.PlayerHex
			hash := sha256.Sum256([]byte(packet110.Name))
			newHex := strings.ToUpper(fmt.Sprintf("%x", hash))
			packet110.PlayerHex = newHex
			log.Printf("玩家 %s 110包 PlayerHex: 原值=%s, 新值=%s", packet110.Name, oldHex, newHex)

			connData.mu.Lock()
			connData.OldPlayerHex = oldHex
			connData.NewPlayerHex = newHex
			connData.mu.Unlock()

			modifiedPacket := Creat_110(packet110)
			proxy.ForwardPacket(modifiedPacket)
			return
		}
		proxy.ForwardPacket(packet)
		return
	}
	switch packet.Type {
	case 160:
		packetData, err := Analysis_160(packet)
		if err != nil {
			return
		}
		if len(packetData.playerName) == 0 {
			return
		}

		oldConnData, exists := activeConnections.Load(packetData.playerName)
		if exists {
			if oldConn, ok := oldConnData.(*ConnectionData); ok && oldConn != connData {
				oldConn.mu.Lock()
				if oldConn.proxy != nil {
					oldConn.proxy.Close()
					oldConn.proxy = nil
				}
				oldConn.mu.Unlock()
				oldConn.Conn.Close()
			}
		}

		connData.mu.Lock()
		connData.packet160 = &_type.Packet{
			Type:  packet.Type,
			Bytes: make([]byte, len(packet.Bytes)),
		}
		copy(connData.packet160.Bytes, packet.Bytes)
		connData.mu.Unlock()

		activeConnections.Store(packetData.playerName, connData)
		sendBinaryResponse0(connData.Conn, Creat_161())
	case 110:
		sendBinaryResponse0(connData.Conn, Creat_117(
			`欢迎使用 ShadowPlayer 代理服务器

使用说明：
1. 请输入需要代理的游戏服务器IP地址
   格式：IP:端口 或 IP（默认端口5123）
   例如：192.168.1.1:5123 或 192.168.1.1

2. 然后选择是否需要去雾功能
   输入 y/yes 启用去雾，输入其他内容禁用

© RELAY-CN Team`))
	case 118:
		userInput, err := Analysis_118(packet)
		if err != nil {
			log.Printf("解析用户输入失败: %v", err)
			return
		}

		playerName := findPlayerNameByConnData(connData)
		if playerName == "" {
			return
		}

		currentIP := connData.GetIP()
		currentPort := connData.GetPort()

		if currentIP == "" && currentPort == 0 {
			ip, port := parseIPAndPort(userInput)
			if ip != "" {
				connData.SetIP(ip)
				connData.SetPort(port)
				log.Printf("玩家 %s 设置 IP: %s, Port: %d", playerName, ip, port)
				sendBinaryResponse0(connData.Conn, Creat_117(fmt.Sprintf(
					`服务器地址设置成功

目标服务器：%s:%d

是否需要启用去雾功能？
输入 y 或 yes 启用去雾
输入其他内容（如 n、no）禁用去雾`, ip, port)))
			} else {
				sendBinaryResponse0(connData.Conn, Creat_117(
					`IP地址格式无效，请重新输入

正确格式：
IP:端口（例如：192.168.1.1:5123）
或仅输入IP（默认端口5123，例如：192.168.1.1）

请重新输入服务器地址：`))
			}
		} else {
			userInputLower := strings.ToLower(strings.TrimSpace(userInput))
			isFog := userInputLower == "y" || userInputLower == "yes"
			connData.SetIsFog(isFog)
			log.Printf("玩家 %s 设置 IsFog: %v", playerName, isFog)

			if err := StartProxyForPlayer(playerName); err != nil {
				log.Printf("玩家 %s 启动代理失败: %v", playerName, err)
				sendBinaryResponse0(connData.Conn, Creat_117(
					`代理连接失败

可能的原因：
目标服务器地址错误
目标服务器无法访问
网络连接问题

请检查服务器地址后重试`))
			} else {
				log.Printf("玩家 %s 代理已启动", playerName)
				connData.mu.RLock()
				proxy := connData.proxy
				savedPacket160 := connData.packet160
				connData.mu.RUnlock()

				if proxy != nil && savedPacket160 != nil {
					proxy.ForwardPacket(*savedPacket160)
				}
			}
		}
		return
	default:
		return
	}
}

func findConnectionDataByConn(conn net.Conn) *ConnectionData {
	var foundConnData *ConnectionData
	activeConnections.Range(func(key, value interface{}) bool {
		if connData, ok := value.(*ConnectionData); ok {
			if connData.Conn == conn {
				foundConnData = connData
				return false
			}
		}
		return true
	})
	return foundConnData
}

func sendBinaryResponse(conn net.Conn, packet _type.Packet) error {
	switch packet.Type {
	case 108:
		connData := findConnectionDataByConn(conn)
		if connData != nil {
			connData.mu.RLock()
			proxy := connData.proxy
			connData.mu.RUnlock()

			if proxy != nil && proxy.IsConnected() {
				sendTime, err := Analysis_108(packet)
				if err != nil {
					log.Printf("解析108包失败: %v", err)
					return sendBinaryResponse0(conn, packet)
				}

				delay := time.Now().UnixMilli() - sendTime
				if delay >= 0 && delay <= 500 {
					// 可以在这里保存延迟信息，如果需要的话
				}

				packet109 := Creat_109(sendTime)
				proxy.mu.RLock()
				targetConn := proxy.targetConn
				proxy.mu.RUnlock()

				if targetConn != nil {
					if err := proxy.sendPacketToTarget(targetConn, packet109); err != nil {
						log.Printf("发送109到目标服务器失败: %v", err)
					}
				}
			}
		}
	case 106:
		connData := findConnectionDataByConn(conn)
		if connData != nil {
			connData.mu.RLock()
			isFog := connData.IsFog
			connData.mu.RUnlock()

			if isFog {
				modifiedPacket, err := Creat_106_ModifyFog(packet, isFog)
				if err != nil {
					log.Printf("修改106包失败: %v", err)
				} else {
					packet = modifiedPacket
				}
			}

			connData.mu.Lock()
			if !connData.received106 {
				connData.received106 = true
				connData.mu.Unlock()

				clientIP := getClientIPFromConnection(connData.Conn)
				if clientIP != "" {
					connData.mu.Lock()
					connData.ClientIP = clientIP
					connData.mu.Unlock()
					log.Printf("玩家客户端IP: %s", clientIP)
				}

				connData.mu.RLock()
				oldHex := connData.OldPlayerHex
				newHex := connData.NewPlayerHex
				connData.mu.RUnlock()

				msg1 := "欢迎使用 ShadowPlayer 代理服务器"
				msg2 := ""
				if oldHex != "" && newHex != "" {
					msg2 = fmt.Sprintf("PlayerHex已更新\n原值: %s\n新值: %s", oldHex, newHex)
				}

				sendBinaryResponse0(connData.Conn, Creat_141_System(msg1))
				if msg2 != "" {
					sendBinaryResponse0(connData.Conn, Creat_141_System(msg2))
				}

				go func() {
					traceIP := getClientIPFromTrace()
					if traceIP != "" {
						log.Printf("Trace服务返回IP: %s", traceIP)
					}
					msg3 := fmt.Sprintf("网络信息\n客户端IP: %s\n外部IP: %s", clientIP, traceIP)
					sendBinaryResponse0(connData.Conn, Creat_141_System(msg3))
				}()
			} else {
				connData.mu.Unlock()
			}
		}
	case 115:
		connData := findConnectionDataByConn(conn)
		isFog := false
		if connData != nil {
			connData.mu.RLock()
			isFog = connData.IsFog
			connData.mu.RUnlock()
		}
		modifiedPacket, err := Creat_115_Modify(packet, isFog)
		if err != nil {
			log.Printf("修改115包失败: %v", err)
		} else {
			packet = modifiedPacket
		}
	}
	return sendBinaryResponse0(conn, packet)
}

func getClientIPFromConnection(conn net.Conn) string {
	addr := conn.RemoteAddr()
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func getClientIPFromTrace() string {
	resp, err := http.GetRequest("https://image.nebulapause.com/cdn-cgi/trace", nil, map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36 Edg/142.0.0.0",
	})
	if err != nil {
		log.Printf("获取Trace IP失败: %v", err)
		return ""
	}

	content := string(resp)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ip=") {
			ip := strings.TrimPrefix(line, "ip=")
			ip = strings.TrimSpace(ip)
			return ip
		}
	}
	return ""
}

func sendBinaryResponse0(conn net.Conn, packet _type.Packet) error {
	total := 8 + len(packet.Bytes)
	out := make([]byte, total)
	binary.BigEndian.PutUint32(out[0:4], uint32(len(packet.Bytes)))
	binary.BigEndian.PutUint32(out[4:8], uint32(packet.Type))
	copy(out[8:], packet.Bytes)
	_, err := conn.Write(out)
	return err
}

func refreshTheTeam() {
	activeConnections.Range(func(key, value interface{}) bool {
		if connData, ok := value.(*ConnectionData); ok {
			sendBinaryResponse0(connData.Conn, Creat_115(activeConnections, connData.Conn))
		}
		return true
	})
}

func RefreshPing() {
	var packet = Creat_108()
	activeConnections.Range(func(key, value interface{}) bool {
		if connData, ok := value.(*ConnectionData); ok {
			sendBinaryResponse0(connData.Conn, packet)
		}
		return true
	})
}

func GetConnectionData(playerName string) (*ConnectionData, bool) {
	value, ok := activeConnections.Load(playerName)
	if !ok {
		return nil, false
	}
	connData, ok := value.(*ConnectionData)
	return connData, ok
}

func findPlayerNameByConnData(targetConnData *ConnectionData) string {
	var playerName string
	activeConnections.Range(func(key, value interface{}) bool {
		if connData, ok := value.(*ConnectionData); ok {
			if connData == targetConnData {
				if name, ok := key.(string); ok {
					playerName = name
					return false
				}
			}
		}
		return true
	})
	return playerName
}

func parseIPAndPort(input string) (string, int32) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", 0
	}

	defaultPort := int32(5123)

	if strings.Contains(input, ":") {
		parts := strings.SplitN(input, ":", 2)
		if len(parts) == 2 {
			ip := strings.TrimSpace(parts[0])
			portStr := strings.TrimSpace(parts[1])
			port, err := strconv.ParseInt(portStr, 10, 32)
			if err != nil {
				log.Printf("端口解析失败: %s, 使用默认端口 %d", portStr, defaultPort)
				port = int64(defaultPort)
			}
			return ip, int32(port)
		}
	}

	return input, defaultPort
}
