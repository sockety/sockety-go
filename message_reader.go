package sockety

type messageReaderStep uint8

const (
	messageReaderStepFlags messageReaderStep = iota
	messageReaderStepUuid
	messageReaderStepActionSize
	messageReaderStepAction
	messageReaderStepDataSize
	messageReaderStepFilesCount
	messageReaderStepFilesSize
)

type messageReader struct {
	step messageReaderStep

	flags       uint8
	flagsReader *streamReaderUint8
	uuidReader  *streamReaderUUID

	actionSize8Reader  *streamReaderUint8
	actionSize16Reader *streamReaderUint16
	actionReader       *streamReaderString

	dataSize8Reader  *streamReaderUint8
	dataSize16Reader *streamReaderUint16
	dataSize48Reader *streamReaderUint48

	filesCount8Reader  *streamReaderUint8
	filesCount16Reader *streamReaderUint16
	filesCount24Reader *streamReaderUint24

	filesSize16Reader *streamReaderUint16
	filesSize24Reader *streamReaderUint24
	filesSize32Reader *streamReaderUint32
	filesSize48Reader *streamReaderUint48
}

func newMessageReader() *messageReader {
	return &messageReader{
		step: messageReaderStepFlags,

		flagsReader: newStreamReaderUint8(),
		uuidReader:  newStreamReaderUUID(),

		actionSize8Reader:  newStreamReaderUint8(),
		actionSize16Reader: newStreamReaderUint16(),
		actionReader:       newStreamReaderString(16),

		dataSize8Reader:  newStreamReaderUint8(),
		dataSize16Reader: newStreamReaderUint16(),
		dataSize48Reader: newStreamReaderUint48(),

		filesCount8Reader:  newStreamReaderUint8(),
		filesCount16Reader: newStreamReaderUint16(),
		filesCount24Reader: newStreamReaderUint24(),

		filesSize16Reader: newStreamReaderUint16(),
		filesSize24Reader: newStreamReaderUint24(),
		filesSize32Reader: newStreamReaderUint32(),
		filesSize48Reader: newStreamReaderUint48(),
	}
}

// TODO: Consider emitting earlier
func (p *messageReader) Get(message *message, r BufferedReader) (bool, error) {
	if p.step == messageReaderStepFlags {
		flags, err := p.flagsReader.Get(r)
		if err != nil {
			return false, err
		}
		p.flags = flags
		p.step = messageReaderStepUuid
	}

	if p.step == messageReaderStepUuid {
		id, err := p.uuidReader.Get(r)
		if err != nil {
			return false, err
		}
		(*message).id = id
		p.step = messageReaderStepActionSize
	}

	if p.step == messageReaderStepActionSize {
		switch p.flags & nameSizeBitmask {
		case nameSizeUint8Bits:
			actionSize, err := p.actionSize8Reader.Get(r)
			if err != nil {
				return false, err
			}
			p.actionReader.Resize(uint16(actionSize))
		case nameSizeUint16Bits:
			actionSize, err := p.actionSize16Reader.Get(r)
			if err != nil {
				return false, err
			}
			p.actionReader.Resize(actionSize)
		}
		p.step = messageReaderStepAction
	}

	if p.step == messageReaderStepAction {
		action, err := p.actionReader.Get(r)
		if err != nil {
			return false, err
		}
		(*message).action = action
		p.step = messageReaderStepDataSize
	}

	if p.step == messageReaderStepDataSize {
		switch p.flags & dataSizeBitmask {
		case dataSizeNoneBits:
			(*message).data = emptyMessageData
		case dataSizeUint8Bits:
			dataSize, err := p.dataSize8Reader.Get(r)
			if err != nil {
				return false, err
			}
			(*message).data = newMessageData(offset(dataSize))
		case dataSizeUint16Bits:
			dataSize, err := p.dataSize16Reader.Get(r)
			if err != nil {
				return false, err
			}
			(*message).data = newMessageData(offset(dataSize))
		case dataSizeUint48Bits:
			dataSize, err := p.dataSize48Reader.Get(r)
			if err != nil {
				return false, err
			}
			(*message).data = newMessageData(offset(dataSize))
		}
		p.step = messageReaderStepFilesCount
	}

	if p.step == messageReaderStepFilesCount {
		switch p.flags & filesCountBitmask {
		case filesCountNoneBits:
			(*message).filesCount = 0
			(*message).totalFilesSize = 0
		case filesCountUint8Bits:
			filesCount, err := p.filesCount8Reader.Get(r)
			if err != nil {
				return false, err
			}
			(*message).filesCount = uint32(filesCount)
			p.step = messageReaderStepFilesSize
		case filesCountUint16Bits:
			filesCount, err := p.filesCount16Reader.Get(r)
			if err != nil {
				return false, err
			}
			(*message).filesCount = uint32(filesCount)
			p.step = messageReaderStepFilesSize
		case filesCountUint24Bits:
			filesCount, err := p.filesCount24Reader.Get(r)
			if err != nil {
				return false, err
			}
			(*message).filesCount = uint32(filesCount)
			p.step = messageReaderStepFilesSize
		}
	}

	if p.step == messageReaderStepFilesSize {
		switch p.flags & fileSizeBitmask {
		case fileSizeUint16Bits:
			filesSize, err := p.filesSize16Reader.Get(r)
			if err != nil {
				return false, err
			}
			(*message).totalFilesSize = uint64(filesSize)
		case fileSizeUint24Bits:
			filesSize, err := p.filesSize24Reader.Get(r)
			if err != nil {
				return false, err
			}
			(*message).totalFilesSize = uint64(filesSize)
		case fileSizeUint32Bits:
			filesSize, err := p.filesSize32Reader.Get(r)
			if err != nil {
				return false, err
			}
			(*message).totalFilesSize = uint64(filesSize)
		case fileSizeUint48Bits:
			filesSize, err := p.filesSize48Reader.Get(r)
			if err != nil {
				return false, err
			}
			(*message).totalFilesSize = uint64(filesSize)
		}
		// TODO: Move to files header
	}

	// TODO: Files header

	p.step = messageReaderStepFlags
	return true, nil
}
