package net

import (
	"ShadowPlayer/src/io"
	"ShadowPlayer/src/type"
	"errors"
	"net"
	"strings"
	"sync"
)

type Packet_160 struct {
	clientVersion int32
	queryString   string
	playerName    string
}

type Packet_110 struct {
	CheckPacketName     string
	ClientPacketVersion int32
	VersionA            int32
	VersionB            int32
	Name                string
	PasswdHex           string
	ClientPacketName    string
	PlayerHex           string
	UnitCheckSun        int32
	KA                  string
	KB                  string
}

type Packet_106 struct {
	FirstString string
	FirstInt    int32
	MapType     int32
	MapName     string
	Credits     int32
	Fog         int32
	RevealedMap bool
	AIDifficuly int32
	InitUnit    int32
	Income      float32
	Nukes       bool
}

type PacketParseError struct {
	Op  string
	Err error
}

func (e *PacketParseError) Error() string {
	return e.Op + ": " + e.Err.Error()
}

func tryParse(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = &PacketParseError{Op: "packet parse", Err: e}
			} else {
				err = &PacketParseError{Op: "packet parse", Err: errors.New("unknown error")}
			}
		}
	}()
	fn()
	return nil
}

func Analysis_160(packet _type.Packet) (result Packet_160, err error) {
	err = tryParse(func() {
		read := io.NewGameInputStreamFromBytes(packet.Bytes, 0)
		read.ReadString()
		packetVersion, _ := read.ReadInt()
		clientVersion, _ := read.ReadInt()

		var queryString = ""
		var playerName = ""

		if packetVersion >= 1 {
			read.Skip(4)
		}
		if packetVersion >= 2 {
			queryString, _ = read.ReadIsString()
		}
		if packetVersion >= 3 {
			playerName, _ = read.ReadString()
		}

		result = Packet_160{
			clientVersion: clientVersion,
			queryString:   queryString,
			playerName:    playerName,
		}
	})

	return
}

func Analysis_140(packet _type.Packet) (result string, err error) {
	err = tryParse(func() {
		read := io.NewGameInputStreamFromBytes(packet.Bytes, 0)
		result, _ = read.ReadString()
	})

	return
}

func Analysis_118(packet _type.Packet) (result string, err error) {
	err = tryParse(func() {
		read := io.NewGameInputStreamFromBytes(packet.Bytes, 0)
		read.Skip(5)
		result, _ = read.ReadString()
		result = strings.TrimSpace(result)
	})
	return
}

func Creat_161() _type.Packet {
	outputStreamFromBytes := io.NewGameOutputStreamFromBytes()
	outputStreamFromBytes.WriteString("kim.der.ironcore.server.shadow")
	outputStreamFromBytes.WriteInt(1)
	outputStreamFromBytes.WriteInt(55)
	outputStreamFromBytes.WriteInt(0)
	outputStreamFromBytes.WriteString("com.corrodinggames.rts.server")
	outputStreamFromBytes.WriteString("IronCore-Shadow-SERVER")
	outputStreamFromBytes.WriteInt(0)
	result, _ := outputStreamFromBytes.CreatePacket(161)
	return result
}

func Creat_113() _type.Packet {
	outputStreamFromBytes := io.NewGameOutputStreamFromBytes()
	outputStreamFromBytes.WriteInt(0)
	result, _ := outputStreamFromBytes.CreatePacket(113)
	return result
}

func Creat_117(msg string) _type.Packet {
	outputStreamFromBytes := io.NewGameOutputStreamFromBytes()
	outputStreamFromBytes.WriteByte(1)
	outputStreamFromBytes.WriteInt(5)
	outputStreamFromBytes.WriteString(msg)
	result, _ := outputStreamFromBytes.CreatePacket(117)
	return result
}

func Creat_178(ip string) _type.Packet {
	outputStreamFromBytes := io.NewGameOutputStreamFromBytes()
	outputStreamFromBytes.WriteByte(0)
	outputStreamFromBytes.WriteInt(3)
	outputStreamFromBytes.WriteBoolean(false)
	outputStreamFromBytes.WriteInt(1)
	outputStreamFromBytes.WriteString(ip)
	result, _ := outputStreamFromBytes.CreatePacket(178)
	return result
}

func Creat_115(activeConnections sync.Map, conn net.Conn) _type.Packet {
	outputStreamFromBytesGzipBlock := io.NewGameOutputStreamFromBytes()

	var playerSize = 0
	var playerCount = 0
	activeConnections.Range(func(key, value interface{}) bool {
		playerCount++
		if value != conn {
			playerSize++
		}

		if playerCount <= 8 {
			storedKey, _ := key.(string)
			outputStreamFromBytesGzipBlock.WriteBoolean(true)
			outputStreamFromBytesGzipBlock.WriteInt(0)

			outputStreamFromBytesGzipBlock.WriteByte(byte(playerCount - 1))
			outputStreamFromBytesGzipBlock.WriteInt(0)
			outputStreamFromBytesGzipBlock.WriteInt(0)
			outputStreamFromBytesGzipBlock.WriteIsString(&storedKey)
			outputStreamFromBytesGzipBlock.WriteBoolean(false)

			outputStreamFromBytesGzipBlock.WriteInt(1)
			outputStreamFromBytesGzipBlock.WriteLong(0)

			outputStreamFromBytesGzipBlock.WriteBoolean(false)
			outputStreamFromBytesGzipBlock.WriteInt(0)
		}
		return true
	})

	if playerCount <= 8 {
		diff := 8 - playerCount
		if diff > 0 {
			for i := 0; i < diff; i++ {
				outputStreamFromBytesGzipBlock.WriteBoolean(false)
			}
		}
	}

	outputStreamFromBytes := io.NewGameOutputStreamFromBytes()
	outputStreamFromBytes.WriteInt(int32(playerSize))

	bytes, _ := outputStreamFromBytesGzipBlock.GetByteArray()
	outputStreamFromBytes.WriteBytes(bytes)
	outputStreamFromBytes.WriteInt(2)
	outputStreamFromBytes.WriteInt(0)
	outputStreamFromBytes.WriteBoolean(true)
	outputStreamFromBytes.WriteInt(1)
	outputStreamFromBytes.WriteByte(0)
	outputStreamFromBytes.WriteInt(0)
	outputStreamFromBytes.WriteInt(0)
	result, _ := outputStreamFromBytes.CreatePacket(115)
	return result
}

func Creat_141(msg string, sendBy string, team int32) _type.Packet {
	outputStreamFromBytes := io.NewGameOutputStreamFromBytes()
	outputStreamFromBytes.WriteString(msg)
	outputStreamFromBytes.WriteByte(3)
	outputStreamFromBytes.WriteIsString(&sendBy)
	outputStreamFromBytes.WriteInt(team)
	outputStreamFromBytes.WriteInt(team)
	result, _ := outputStreamFromBytes.CreatePacket(141)
	return result
}

func Creat_141_System(msg string) _type.Packet {
	return Creat_141(msg, "SERVER", 5)
}

func Analysis_108(packet _type.Packet) (sendTime int64, err error) {
	err = tryParse(func() {
		read := io.NewGameInputStreamFromBytes(packet.Bytes, 0)
		sendTime, _ = read.ReadLong()
	})
	return
}

func Creat_108() _type.Packet {
	outputStreamFromBytes := io.NewGameOutputStreamFromBytes()
	outputStreamFromBytes.WriteLong(0)
	outputStreamFromBytes.WriteByte(0)
	result, _ := outputStreamFromBytes.CreatePacket(108)
	return result
}

func Creat_109(sendTime int64) _type.Packet {
	outputStreamFromBytes := io.NewGameOutputStreamFromBytes()
	outputStreamFromBytes.WriteLong(sendTime)
	outputStreamFromBytes.WriteByte(0)
	result, _ := outputStreamFromBytes.CreatePacket(109)
	return result
}

func Analysis_110(packet _type.Packet) (result Packet_110, err error) {
	err = tryParse(func() {
		read := io.NewGameInputStreamFromBytes(packet.Bytes, 0)
		checkPacketName, _ := read.ReadString()
		clientPacketVersion, _ := read.ReadInt()
		versionA, _ := read.ReadInt()
		versionB, _ := read.ReadInt()
		name, _ := read.ReadString()
		passwdHex, _ := read.ReadIsString()
		clientPacketName, _ := read.ReadString()
		playerHex, _ := read.ReadString()
		unitCheckSun, _ := read.ReadInt()
		ka, _ := read.ReadString()

		var kb string = ""
		if clientPacketVersion >= 5 {
			kb, _ = read.ReadString()
		}

		result = Packet_110{
			CheckPacketName:     checkPacketName,
			ClientPacketVersion: clientPacketVersion,
			VersionA:            versionA,
			VersionB:            versionB,
			Name:                name,
			PasswdHex:           passwdHex,
			ClientPacketName:    clientPacketName,
			PlayerHex:           playerHex,
			UnitCheckSun:        unitCheckSun,
			KA:                  ka,
			KB:                  kb,
		}
	})
	return
}

func Creat_110(data Packet_110) _type.Packet {
	outputStreamFromBytes := io.NewGameOutputStreamFromBytes()
	outputStreamFromBytes.WriteString(data.CheckPacketName)
	outputStreamFromBytes.WriteInt(data.ClientPacketVersion)
	outputStreamFromBytes.WriteInt(data.VersionA)
	outputStreamFromBytes.WriteInt(data.VersionB)
	outputStreamFromBytes.WriteString(data.Name)
	outputStreamFromBytes.WriteIsString(&data.PasswdHex)
	outputStreamFromBytes.WriteString(data.ClientPacketName)
	outputStreamFromBytes.WriteString(data.PlayerHex)
	outputStreamFromBytes.WriteInt(data.UnitCheckSun)
	outputStreamFromBytes.WriteString(data.KA)
	if data.ClientPacketVersion >= 5 {
		outputStreamFromBytes.WriteString(data.KB)
	}
	result, _ := outputStreamFromBytes.CreatePacket(110)
	return result
}

func Analysis_106(packet _type.Packet) (result Packet_106, err error) {
	err = tryParse(func() {
		read := io.NewGameInputStreamFromBytes(packet.Bytes, 0)
		firstString, _ := read.ReadString()
		firstInt, _ := read.ReadInt()
		mapType, _ := read.ReadInt()
		mapName, _ := read.ReadString()
		credits, _ := read.ReadInt()
		fog, _ := read.ReadInt()
		revealedMap, _ := read.ReadBoolean()
		aiDifficuly, _ := read.ReadInt()
		readByte, _ := read.ReadByte()
		read.Skip(2)
		if readByte >= 1 {
			read.Skip(8)
		}
		if readByte < 2 {
			panic(errors.New("readByte must be >= 2"))
		}
		initUnit, _ := read.ReadInt()
		income, _ := read.ReadFloat()
		nukes, _ := read.ReadBoolean()

		result = Packet_106{
			FirstString: firstString,
			FirstInt:    firstInt,
			MapType:     mapType,
			MapName:     mapName,
			Credits:     credits,
			Fog:         fog,
			RevealedMap: revealedMap,
			AIDifficuly: aiDifficuly,
			InitUnit:    initUnit,
			Income:      income,
			Nukes:       nukes,
		}
	})
	return
}

func Creat_106_ModifyFog(packet _type.Packet, isFog bool) (_type.Packet, error) {
	if !isFog {
		return packet, nil
	}

	read := io.NewGameInputStreamFromBytes(packet.Bytes, 0)
	output := io.NewGameOutputStreamFromBytes()

	firstString, _ := read.ReadString()
	output.WriteString(firstString)

	firstInt, _ := read.ReadInt()
	output.WriteInt(firstInt)

	mapType, _ := read.ReadInt()
	output.WriteInt(mapType)

	mapName, _ := read.ReadString()
	output.WriteString(mapName)

	credits, _ := read.ReadInt()
	output.WriteInt(credits)

	_, _ = read.ReadInt()
	output.WriteInt(0)

	revealedMap, _ := read.ReadBoolean()
	output.WriteBoolean(revealedMap)

	aiDifficuly, _ := read.ReadInt()
	output.WriteInt(aiDifficuly)

	readByte, _ := read.ReadByte()
	output.WriteByte(readByte)

	output.TransferToFixedLength(read, 2)

	if readByte >= 1 {
		output.TransferToFixedLength(read, 8)
	}

	initUnit, _ := read.ReadInt()
	output.WriteInt(initUnit)

	income, _ := read.ReadFloat()
	output.WriteFloat(income)

	nukes, _ := read.ReadBoolean()
	output.WriteBoolean(nukes)

	output.TransferTo(read)

	result, err := output.CreatePacket(106)
	return result, err
}

func Creat_115_Modify(packet _type.Packet, isFog bool) (_type.Packet, error) {
	if !isFog {
		return packet, nil
	}

	read := io.NewGameInputStreamFromBytes(packet.Bytes, 0)
	output := io.NewGameOutputStreamFromBytes()

	playerSize, _ := read.ReadInt()
	output.WriteInt(playerSize)

	relayCustomMaxPlayer, _ := read.ReadBoolean()
	output.WriteBoolean(relayCustomMaxPlayer)

	maxPlayerSize, _ := read.ReadInt()
	output.WriteInt(maxPlayerSize)

	head, _ := read.ReadString()
	output.WriteString(head)

	gzipBlockLength, _ := read.ReadInt()
	output.WriteInt(gzipBlockLength)

	output.TransferToFixedLength(read, int(gzipBlockLength))

	_, _ = read.ReadInt()
	output.WriteInt(0)

	output.TransferTo(read)

	result, err := output.CreatePacket(115)
	return result, err
}
