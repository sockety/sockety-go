package cast

import "golang.org/x/exp/constraints"

func Cast[T constraints.Integer, U constraints.Integer](i U, e error) (T, error) {
	return T(i), e
}

func ToUint32[T constraints.Integer](i T, e error) (uint32, error) {
	return Cast[uint32](i, e)
}
