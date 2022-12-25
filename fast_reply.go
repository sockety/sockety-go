package sockety

import (
	"errors"
	"github.com/google/uuid"
)

type FastReply struct {
	Code Uint12
	Id   uuid.UUID
}

func (f FastReply) pass(w Writer) error {
	if !f.Code.Valid() {
		return errors.New("fast reply code may be maximum of uint12")
	}

	if f.Code < MaxUint4 {
		return w.WriteStandalone(newPacket(packetFastReplyLowBits|uint8(f.Code), 17).UUID(f.Id))
	} else {
		return w.WriteStandalone(newPacket(packetFastReplyHighBits|uint8(f.Code>>8), 18).Uint8(uint8(f.Code & 0x00ff)).UUID(f.Id))
	}
}
