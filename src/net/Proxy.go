package net

import (
	"ShadowPlayer/src/type"
	"bufio"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	proxyReadTimeout  = 30 * time.Second
	proxyWriteTimeout = 30 * time.Second
)

type ProxyConnection struct {
	clientConn  net.Conn
	targetConn  net.Conn
	connData    *ConnectionData
	playerName  string
	isConnected bool
	mu          sync.RWMutex
	closeOnce   sync.Once
	closeChan   chan struct{}
	packetChan  chan _type.Packet
}

func NewProxyConnection(connData *ConnectionData, playerName string) *ProxyConnection {
	return &ProxyConnection{
		clientConn: connData.Conn,
		connData:   connData,
		playerName: playerName,
		closeChan:  make(chan struct{}),
		packetChan: make(chan _type.Packet, 100),
	}
}

func (pc *ProxyConnection) Start() error {
	targetIP := pc.connData.GetIP()
	targetPort := pc.connData.GetPort()

	if targetIP == "" || targetPort == 0 {
		return io.ErrUnexpectedEOF
	}

	targetAddr := net.JoinHostPort(targetIP, strconv.Itoa(int(targetPort)))
	targetConn, err := net.DialTimeout("tcp", targetAddr, 10*time.Second)
	if err != nil {
		log.Printf("玩家 %s 连接目标服务器失败 %s: %v", pc.playerName, targetAddr, err)
		return err
	}

	pc.mu.Lock()
	pc.targetConn = targetConn
	pc.isConnected = true
	pc.mu.Unlock()

	log.Printf("玩家 %s 已连接到目标服务器 %s", pc.playerName, targetAddr)

	go pc.forwardClientToTarget()
	go pc.forwardTargetToClient()

	return nil
}

func (pc *ProxyConnection) forwardClientToTarget() {
	defer pc.Close()

	for {
		select {
		case <-pc.closeChan:
			return
		case packet := <-pc.packetChan:
			pc.mu.RLock()
			targetConn := pc.targetConn
			isConnected := pc.isConnected
			pc.mu.RUnlock()

			if !isConnected || targetConn == nil {
				putBuffer(packet.Bytes)
				return
			}

			if err := pc.sendPacketToTarget(targetConn, packet); err != nil {
				log.Printf("玩家 %s 转发数据到目标服务器失败: %v", pc.playerName, err)
				putBuffer(packet.Bytes)
				return
			}

			putBuffer(packet.Bytes)
		}
	}
}

func (pc *ProxyConnection) ForwardPacket(packet _type.Packet) {
	packetCopy := _type.Packet{
		Type:  packet.Type,
		Bytes: make([]byte, len(packet.Bytes)),
	}
	copy(packetCopy.Bytes, packet.Bytes)

	select {
	case <-pc.closeChan:
		return
	case pc.packetChan <- packetCopy:
	default:
		log.Printf("玩家 %s 数据包通道已满，丢弃数据包", pc.playerName)
	}
}

func (pc *ProxyConnection) forwardTargetToClient() {
	defer pc.Close()

	pc.mu.RLock()
	targetConn := pc.targetConn
	pc.mu.RUnlock()

	if targetConn == nil {
		return
	}

	reader := bufio.NewReader(targetConn)

	for {
		select {
		case <-pc.closeChan:
			return
		default:
		}

		targetConn.SetReadDeadline(time.Now().Add(proxyReadTimeout))

		var msgLen int32
		if err := binary.Read(reader, binary.BigEndian, &msgLen); err != nil {
			if err != io.EOF {
				log.Printf("玩家 %s 从目标服务器读取消息长度错误: %v", pc.playerName, err)
			}
			return
		}

		if msgLen <= 0 || msgLen > maxMessageSize {
			log.Printf("玩家 %s 从目标服务器收到非法消息长度: %d", pc.playerName, msgLen)
			return
		}

		var msgType int32
		if err := binary.Read(reader, binary.BigEndian, &msgType); err != nil {
			log.Printf("玩家 %s 从目标服务器读取消息类型错误: %v", pc.playerName, err)
			return
		}

		msgData := getBuffer(msgLen)
		if _, err := io.ReadFull(reader, msgData); err != nil {
			log.Printf("玩家 %s 从目标服务器读取消息体错误: %v", pc.playerName, err)
			putBuffer(msgData)
			return
		}

		packet := _type.Packet{
			Type:  msgType,
			Bytes: msgData,
		}

		if err := sendBinaryResponse(pc.clientConn, packet); err != nil {
			log.Printf("玩家 %s 转发数据到客户端失败: %v", pc.playerName, err)
			putBuffer(msgData)
			return
		}

		putBuffer(msgData)
	}
}

func (pc *ProxyConnection) sendPacketToTarget(targetConn net.Conn, packet _type.Packet) error {
	targetConn.SetWriteDeadline(time.Now().Add(proxyWriteTimeout))

	total := 8 + len(packet.Bytes)
	out := make([]byte, total)
	binary.BigEndian.PutUint32(out[0:4], uint32(len(packet.Bytes)))
	binary.BigEndian.PutUint32(out[4:8], uint32(packet.Type))
	copy(out[8:], packet.Bytes)

	_, err := targetConn.Write(out)
	return err
}

func (pc *ProxyConnection) Close() {
	pc.closeOnce.Do(func() {
		close(pc.closeChan)

		pc.mu.Lock()
		if pc.targetConn != nil {
			pc.targetConn.Close()
			pc.targetConn = nil
		}
		pc.isConnected = false
		pc.mu.Unlock()

		log.Printf("玩家 %s 代理连接已关闭", pc.playerName)
	})
}

func (pc *ProxyConnection) IsConnected() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.isConnected
}

func StartProxyForPlayer(playerName string) error {
	connData, ok := GetConnectionData(playerName)
	if !ok {
		return io.ErrUnexpectedEOF
	}

	targetIP := connData.GetIP()
	targetPort := connData.GetPort()

	if targetIP == "" || targetPort == 0 {
		return io.ErrUnexpectedEOF
	}

	connData.mu.Lock()
	if connData.proxy != nil {
		connData.mu.Unlock()
		return nil
	}
	connData.mu.Unlock()

	proxy := NewProxyConnection(connData, playerName)
	if err := proxy.Start(); err != nil {
		return err
	}

	connData.mu.Lock()
	connData.proxy = proxy
	connData.mu.Unlock()

	return nil
}
