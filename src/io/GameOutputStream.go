package io

import (
	"ShadowPlayer/src/type"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
)

type GameOutputStream struct {
	buffer io.Writer
}

func NewGameOutputStream(writer io.Writer) *GameOutputStream {
	return &GameOutputStream{
		buffer: writer,
	}
}

func NewGameOutputStreamFromBytes() *GameOutputStream {
	buf := bytes.NewBuffer(nil)
	return NewGameOutputStream(buf)
}

func (gos *GameOutputStream) CreatePacket(packetType int32) (_type.Packet, error) {
	if buf, ok := gos.buffer.(*bytes.Buffer); ok {
		return _type.Packet{
			Type:  packetType,
			Bytes: buf.Bytes(),
		}, nil
	}
	return _type.Packet{}, errors.New("buffer is not a byte buffer")
}

func (gos *GameOutputStream) GetByteArray() ([]byte, error) {
	if buf, ok := gos.buffer.(*bytes.Buffer); ok {
		return buf.Bytes(), nil
	}
	return nil, errors.New("buffer is not a byte buffer")
}

func (gos *GameOutputStream) Size() int {
	if buf, ok := gos.buffer.(*bytes.Buffer); ok {
		return buf.Len()
	}
	return -1
}

func (gos *GameOutputStream) WriteByte(value byte) error {
	_, err := gos.buffer.Write([]byte{value})
	return err
}

func (gos *GameOutputStream) WriteBytes(value []byte) error {
	_, err := gos.buffer.Write(value)
	return err
}

func (gos *GameOutputStream) WriteBytesAndLength(value []byte) error {
	err := gos.WriteInt(int32(len(value)))
	if err != nil {
		return err
	}
	return gos.WriteBytes(value)
}

func (gos *GameOutputStream) WriteBoolean(value bool) error {
	var b byte = 0
	if value {
		b = 1
	}
	return gos.WriteByte(b)
}

func (gos *GameOutputStream) WriteInt(value int32) error {
	return binary.Write(gos.buffer, binary.BigEndian, value)
}

func (gos *GameOutputStream) WriteIntLE(value int32) error {
	return binary.Write(gos.buffer, binary.LittleEndian, value)
}

func (gos *GameOutputStream) WriteIsInt(value *int32) error {
	if value == nil {
		err := gos.WriteBoolean(false)
		if err != nil {
			return err
		}
	} else {
		err := gos.WriteBoolean(true)
		if err != nil {
			return err
		}
		return gos.WriteInt(*value)
	}
	return nil
}

func (gos *GameOutputStream) WriteShort(value int16) error {
	return binary.Write(gos.buffer, binary.BigEndian, value)
}

func (gos *GameOutputStream) WriteBackwardsShort(value int16) error {
	return binary.Write(gos.buffer, binary.LittleEndian, value)
}

func (gos *GameOutputStream) WriteFloat(value float32) error {
	return binary.Write(gos.buffer, binary.BigEndian, value)
}

func (gos *GameOutputStream) WriteDouble(value float64) error {
	return binary.Write(gos.buffer, binary.BigEndian, value)
}

func (gos *GameOutputStream) WriteLong(value int64) error {
	return binary.Write(gos.buffer, binary.BigEndian, value)
}

func (gos *GameOutputStream) WriteChar(value rune) error {
	return binary.Write(gos.buffer, binary.BigEndian, uint16(value))
}

func (gos *GameOutputStream) WriteString(value string) error {
	return gos.writeUTF(value, false)
}

func (gos *GameOutputStream) WriteLongString(value string) error {
	return gos.writeUTF(value, true)
}

func (gos *GameOutputStream) WriteIsString(value *string) error {
	if value == nil {
		return gos.WriteBoolean(false)
	} else {
		err := gos.WriteBoolean(true)
		if err != nil {
			return err
		}
		return gos.WriteString(*value)
	}
}

func (gos *GameOutputStream) WriteEnum(value interface{}) error {
	if value == nil {
		return gos.WriteInt(-1)
	}

	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Int {
		val = val.Convert(reflect.TypeOf(0))
	}
	return gos.WriteInt(int32(val.Int()))
}

func (gos *GameOutputStream) TransferTo(input *GameInputStream) error {
	_, err := io.Copy(gos.buffer, input.buffer)
	return err
}

func (gos *GameOutputStream) TransferToFixedLength(input *GameInputStream, length int) error {
	_, err := io.CopyN(gos.buffer, input.buffer, int64(length))
	return err
}

func (gos *GameOutputStream) FlushEncodeData(enc *CompressOutputStream) error {
	byteArray, err := enc.GetByteArray()
	if err != nil {
		return err
	}

	err = gos.WriteString(enc.head)
	if err != nil {
		return err
	}

	err = gos.WriteInt(int32(len(byteArray)))
	if err != nil {
		return err
	}

	return gos.WriteBytes(byteArray)
}

func (gos *GameOutputStream) FlushMapData(mapSize int, bytes []byte) error {
	err := gos.WriteInt(int32(mapSize))
	if err != nil {
		return err
	}
	return gos.WriteBytes(bytes)
}

func (gos *GameOutputStream) Reset() {
	if buf, ok := gos.buffer.(*bytes.Buffer); ok {
		buf.Reset()
	}
}

func (gos *GameOutputStream) Close() error {
	if closer, ok := gos.buffer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (gos *GameOutputStream) writeUTF(str string, isLong bool) error {
	utfLen := len([]byte(str))

	if (isLong && utfLen > (1<<31-1)) || (!isLong && utfLen > (1<<16-1)) {
		return fmt.Errorf("string too long: %d bytes", utfLen)
	}

	if isLong {
		err := gos.WriteInt(int32(utfLen))
		if err != nil {
			return err
		}
	} else {
		err := gos.WriteShort(int16(utfLen))
		if err != nil {
			return err
		}
	}

	_, err := gos.buffer.Write([]byte(str))
	return err
}

type CompressOutputStream struct {
	buffer *bytes.Buffer
	writer *gzip.Writer
	head   string
}

func NewCompressOutputStream(head string) *CompressOutputStream {
	buf := bytes.NewBuffer(nil)
	writer := gzip.NewWriter(buf)
	return &CompressOutputStream{
		buffer: buf,
		writer: writer,
		head:   head,
	}
}

func (cos *CompressOutputStream) GetByteArray() ([]byte, error) {
	err := cos.writer.Close()
	if err != nil {
		return nil, err
	}
	return cos.buffer.Bytes(), nil
}

func (cos *CompressOutputStream) Write(p []byte) (int, error) {
	return cos.writer.Write(p)
}

func IntToBytes(value int32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(value))
	return buf
}
