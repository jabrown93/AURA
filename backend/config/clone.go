package config

import "reflect"

// Clone returns a deep copy of the config
func (config Config) Clone() Config {
	var clone Config
	deepCopyValue(reflect.ValueOf(&clone).Elem(), reflect.ValueOf(config))
	return clone
}

func deepCopyValue(dst, src reflect.Value) {
	switch src.Kind() {
	case reflect.Struct:
		// Copy the whole struct first
		dst.Set(src)
		for i := 0; i < src.NumField(); i++ {
			if dst.Field(i).CanSet() {
				deepCopyValue(dst.Field(i), src.Field(i))
			}
		}
	case reflect.Slice:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		for i := 0; i < src.Len(); i++ {
			deepCopyValue(dst.Index(i), src.Index(i))
		}
	case reflect.Map:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeMapWithSize(src.Type(), src.Len()))
		for _, key := range src.MapKeys() {
			value := reflect.New(src.Type().Elem()).Elem()
			deepCopyValue(value, src.MapIndex(key))
			dst.SetMapIndex(key, value)
		}
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.New(src.Type().Elem()))
		deepCopyValue(dst.Elem(), src.Elem())
	default:
		dst.Set(src)
	}
}
