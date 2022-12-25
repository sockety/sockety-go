package sockety

// Define uint sizes, that are not standard in Golang

type Uint12 uint16
type Uint24 uint32
type Uint48 uint64

func (u Uint12) Valid() bool {
	return u <= MaxUint12
}

func (u Uint24) Valid() bool {
	return u <= MaxUint24
}

func (u Uint48) Valid() bool {
	return u <= MaxUint48
}

// Configure sizes

// TODO: Consider uint32 for "offset" and limiting maximum packet size
type offset = uint64
