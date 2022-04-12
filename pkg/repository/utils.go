package repository

func SplitSliceByBlockSize(slice []interface{}, blockSize int) [][]interface{} {
	sliceLength := len(slice)
	if sliceLength == 0 {
		return nil
	}

	subSlicesCount := (sliceLength-1)/blockSize + 1
	resultSlice := make([][]interface{}, 0, subSlicesCount)

	var high int
	for low := 0; low < sliceLength; low += blockSize {
		high += blockSize
		if high > sliceLength {
			high = sliceLength
		}

		resultSlice = append(resultSlice, slice[low:high])
	}
	return resultSlice
}
