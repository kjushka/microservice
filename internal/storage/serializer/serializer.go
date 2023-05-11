package serializer

import (
	"fmt"
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

const (
	messagePackSerializerName    string = "message-pack"
	messagePackSerializationFlag uint32 = 1 << iota
)

type MessagePackSerializer struct{}

func NewMessagePackSerializer() *MessagePackSerializer {
	return &MessagePackSerializer{}
}

func (s *MessagePackSerializer) Name() string {
	return messagePackSerializerName
}

func (s *MessagePackSerializer) SerializationFlag() uint32 {
	return messagePackSerializationFlag
}

func (s *MessagePackSerializer) Encode(w io.Writer, value any) error {
	enc := msgpack.NewEncoder(w)
	enc.UseArrayEncodedStructs(true)
	enc.SetOmitEmpty(true)
	enc.SetSortMapKeys(true)
	enc.UseCompactFloats(true)
	enc.UseCompactInts(true)
	return enc.Encode(value)
}

func (s *MessagePackSerializer) Decode(r io.Reader, destination any) (err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("cannot decode to destination: %v", panicErr)
		}
	}()

	dec := msgpack.NewDecoder(r)
	dec.SetMapDecoder(func(dec *msgpack.Decoder) (interface{}, error) {
		return dec.DecodeUntypedMap()
	})
	dec.UseLooseInterfaceDecoding(true)
	err = dec.Decode(destination)
	if err != nil {
		return err
	}
	return nil
}
