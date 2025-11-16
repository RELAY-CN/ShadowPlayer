package io

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
)

var (
	ErrEOF               = errors.New("EOF")
	ErrUTFDataFormat     = errors.New("malformed UTF-8 input")
	ErrNegativeUTFLength = errors.New("UTF length is negative")
)

type GameInputStream struct {
	buffer       io.Reader
	parseVersion int
}

func NewGameInputStreamFromBytes(data []byte, parseVersion int) *GameInputStream {
	return NewGameInputStream(bytes.NewReader(data), parseVersion)
}
func NewGameInputStream(reader io.Reader, parseVersion int) *GameInputStream {
	return &GameInputStream{
		buffer:       reader,
		parseVersion: parseVersion,
	}
}

func (gis *GameInputStream) ReadByte() (byte, error) {
	buf := make([]byte, 1)
	_, err := io.ReadFull(gis.buffer, buf)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

func (gis *GameInputStream) ReadBoolean() (bool, error) {
	b, err := gis.ReadByte()
	return b != 0, err
}

func (gis *GameInputStream) ReadInt() (int32, error) {
	var value int32
	err := binary.Read(gis.buffer, binary.BigEndian, &value)
	return value, err
}

func (gis *GameInputStream) ReadIntLE() (int32, error) {
	var value int32
	err := binary.Read(gis.buffer, binary.LittleEndian, &value)
	return value, err
}

func (gis *GameInputStream) ReadShort() (int16, error) {
	var value int16
	err := binary.Read(gis.buffer, binary.BigEndian, &value)
	return value, err
}

func (gis *GameInputStream) ReadUnsignedShort() (uint16, error) {
	var value uint16
	err := binary.Read(gis.buffer, binary.BigEndian, &value)
	return value, err
}

func (gis *GameInputStream) ReadShortLE() (int16, error) {
	var value int16
	err := binary.Read(gis.buffer, binary.LittleEndian, &value)
	return value, err
}

func (gis *GameInputStream) ReadFloat() (float32, error) {
	var value float32
	err := binary.Read(gis.buffer, binary.BigEndian, &value)
	return value, err
}

func (gis *GameInputStream) ReadDouble() (float64, error) {
	var value float64
	err := binary.Read(gis.buffer, binary.BigEndian, &value)
	return value, err
}

func (gis *GameInputStream) ReadLong() (int64, error) {
	var value int64
	err := binary.Read(gis.buffer, binary.BigEndian, &value)
	return value, err
}

func (gis *GameInputStream) ReadChar() (rune, error) {
	var value uint16
	err := binary.Read(gis.buffer, binary.BigEndian, &value)
	return rune(value), err
}

func (gis *GameInputStream) ReadString() (string, error) {
	return gis.readUTF(false)
}

func (gis *GameInputStream) ReadLongString() (string, error) {
	return gis.readUTF(true)
}

func (gis *GameInputStream) ReadIsString() (string, error) {
	exists, err := gis.ReadBoolean()
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}
	return gis.ReadString()
}

func (gis *GameInputStream) ReadIsInt() (int32, error) {
	exists, err := gis.ReadBoolean()
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}
	return gis.ReadInt()
}

func (gis *GameInputStream) Skip(n int) error {
	_, err := io.CopyN(io.Discard, gis.buffer, int64(n))
	return err
}

func (gis *GameInputStream) ReadNBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(gis.buffer, buf)
	return buf, err
}

func (gis *GameInputStream) ReadAllBytes() ([]byte, error) {
	return io.ReadAll(gis.buffer)
}

func (gis *GameInputStream) ReadStreamBytes() ([]byte, error) {
	length, err := gis.ReadInt()
	if err != nil {
		return nil, err
	}
	return gis.ReadNBytes(int(length))
}

func (gis *GameInputStream) ReadStreamBytesNew() ([]byte, error) {
	_, err := gis.ReadInt() // Relay type
	if err != nil {
		return nil, err
	}
	return gis.ReadStreamBytes()
}

func (gis *GameInputStream) ReadEnum(enumType reflect.Type) (interface{}, error) {
	index, err := gis.ReadInt()
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return nil, nil
	}

	enumValues := reflect.Zero(reflect.SliceOf(enumType)).Interface()
	values := reflect.ValueOf(enumValues)
	if index >= int32(values.Len()) {
		return nil, fmt.Errorf("enum index out of range: %d", index)
	}
	return values.Index(int(index)).Interface(), nil
}

func (gis *GameInputStream) TransferTo(w io.Writer) error {
	_, err := io.Copy(w, gis.buffer)
	return err
}

func (gis *GameInputStream) TransferToFixedLength(w io.Writer, length int) error {
	_, err := io.CopyN(w, gis.buffer, int64(length))
	return err
}

func (gis *GameInputStream) GetDecodeStream(bl bool) (*GameInputStream, error) {
	_, err := gis.ReadString()
	if err != nil {
		return nil, err
	}

	readStreamBytes, err := gis.ReadStreamBytes()
	if err != nil {
		return nil, err
	}

	return GetGzipInputStream(bl, readStreamBytes)
}

func (gis *GameInputStream) GetStream() (*GameInputStream, error) {
	readStreamBytes, err := gis.ReadStreamBytesNew()
	if err != nil {
		return nil, err
	}
	return GetGzipInputStream(false, readStreamBytes)
}

func (gis *GameInputStream) GetDecodeBytes() ([]byte, error) {
	_, err := gis.ReadString()
	if err != nil {
		return nil, err
	}
	return gis.ReadStreamBytes()
}

func (gis *GameInputStream) Size() int64 {
	if seeker, ok := gis.buffer.(io.Seeker); ok {
		pos, _ := seeker.Seek(0, io.SeekCurrent)
		end, _ := seeker.Seek(0, io.SeekEnd)
		_, _ = seeker.Seek(pos, io.SeekStart)
		return end - pos
	}
	return -1
}

func (gis *GameInputStream) Close() error {
	if closer, ok := gis.buffer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (gis *GameInputStream) readUTF(isLong bool) (string, error) {
	var utfLen int32
	var err error

	if isLong {
		utfLen, err = gis.ReadInt()
	} else {
		val, _ := gis.ReadUnsignedShort()
		utfLen = int32(val)
	}

	if err != nil {
		return "", err
	}

	if utfLen < 0 {
		return "", ErrNegativeUTFLength
	}

	byteArr, err := gis.ReadNBytes(int(utfLen))
	if err != nil {
		return "", err
	}

	return string(byteArr), nil
}

func GetGzipInputStream(bl bool, data []byte) (*GameInputStream, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return NewGameInputStream(reader, 0), nil
}
