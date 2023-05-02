package utils

func ArrayContains[T comparable](array []T, value T) bool {
	set := make(map[T]struct{}, len(array))
	for _, v := range array {
		set[v] = struct{}{}
	}
	_, ok := set[value]
	return ok
}

func MapHasKey(m map[string]string, key string) bool {
	for k := range m {
		if k == key {
			return true
		}
	}
	return false
}
