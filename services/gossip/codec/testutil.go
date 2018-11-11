package codec

import "reflect"

func emptyPayloads(num int) [][]byte {
	res := [][]byte{}
	for i := 0; i < num; i++ {
		res = append(res, []byte{})
	}
	return res
}

func containsNil(obj interface{}) bool {
	if obj == nil {
		return true
	}
	return valueContainsNil(reflect.ValueOf(obj))
}

func valueContainsNil(v reflect.Value) bool {
	k := v.Kind()
	switch k {
	case reflect.Ptr:
		return v.IsNil() || valueContainsNil(reflect.Indirect(v))
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Interface, reflect.Slice:
		return v.IsNil()
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if valueContainsNil(v.Field(i)) {
				return true
			}
		}
	}
	return false
}
