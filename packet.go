/* Copyright (c) 2020, William R. Burdick Jr., Roy Riggs, and TEAM CTHLUHU
 *
 * The MIT License (MIT)
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 */

package packet

import (
	"fmt"
	"reflect"
	"unsafe"

	msgpack "github.com/vmihailenco/msgpack/v5"
)

//Marshal use msgpack to marshal a struct which can have private fields
func Marshal(aStruct interface{}) ([]byte, error) {
	tmpMap, err := StructToMap(aStruct)
	if err != nil {return nil, err}
	return msgpack.Marshal(tmpMap)
}

//Encode use msgpack to marshal a struct which can have private fields
func Encode(encoder *msgpack.Encoder, aStruct interface{}) error {
	tmpMap, err := StructToMap(aStruct)
	if err != nil {return err}
	return encoder.Encode(tmpMap)
}

//StructToMap convert a simple structure to a map (not recursive)
func StructToMap(aStruct interface{}) (map[string]interface{}, error) {
	tmpMap := make(map[string]interface{})
	val := reflect.ValueOf(aStruct)
	if reflect.Indirect(val) == val {return nil, fmt.Errorf("attempt to call marshall without a pointer")}
	for {
		next := reflect.Indirect(val)
		if val == next {break}
		val = next
	}
	t := val.Type()
	for i := 0; i < val.NumField(); i++ {
		ft := t.Field(i)
		f := val.Field(i)
		f, err := privateStructValue(t, ft, f)
		if err != nil {return nil, err}
		tmpMap[ft.Name] = f.Interface()
	}
	return tmpMap, nil
}

//Unmarshal unmarshal msgpack bytes into a struct allowing private fields
func Unmarshal(bytes []byte, aStruct interface{}) (interface{}, error) {
	var tmpMap map[string]interface{}

	err := msgpack.Unmarshal(bytes, &tmpMap)
	if err != nil {return nil, err}
	err = MapToStruct(tmpMap, aStruct)
	return aStruct, err
}

//Decode decode a stream item into a struct allowing private fields
func Decode(decoder *msgpack.Decoder, aStruct interface{}) (interface{}, error) {
	var tmpMap map[string]interface{}

	err := decoder.Decode(&tmpMap)
	if err != nil {return nil, err}
	err = MapToStruct(tmpMap, aStruct)
	return aStruct, err
}

//MapToStruct convert a map to a simple struct (not recursive)
func MapToStruct(aMap map[string]interface{}, aStruct interface{}) error {
	inputValue := reflect.ValueOf(aStruct)
	//fmt.Printf("Decode kind %#v, %#v\n", inputValue.Kind(), aStruct)
	referent := reflect.Indirect(inputValue)
	if inputValue == referent {return fmt.Errorf("attempt to call Unmarshal without a pointer to a struct: %#v is not a pointer to a struct, it is a %#v", aStruct, referent.Kind())}
	fmt.Printf("Final kind %#v is %#v\n", aStruct, referent.Kind())
	t := referent.Type()
	if referent.Interface() == nil {
		referent := reflect.New(t)
		inputValue.Set(referent)
	}
	for k, v := range aMap {
		if sf, present := t.FieldByName(k); present {
			f, err := privateStructValue(t, sf, referent.FieldByName(k))
			if err != nil {return err}
			vValue := reflect.ValueOf(v)
			vValue, err = convert(vValue, sf.Type, f)
			if err != nil {return err}
			f.Set(vValue)
		}
	}
	return nil
}

func convert(val reflect.Value, cvtType reflect.Type, cvtValue reflect.Value) (reflect.Value, error) {
	if val.Type().Kind() == reflect.Interface {
		val = val.Elem()
	}
	t := val.Type()
	if t == cvtType {return val, nil}
	if t.Kind() == reflect.Map && t.Kind() == cvtType.Kind() {return convertMap(val, cvtType)}
	if t.Kind() == reflect.Map && cvtType.Kind() == reflect.Struct {
		err := MapToStruct(val.Interface().(map[string]interface{}), cvtValue.Addr())
		if err != nil {return cvtValue, err}
		return cvtValue, nil
	}
	if t.Kind() == reflect.Slice && t.Kind() == cvtType.Kind() {return convertSlice(val, cvtType)}
	if isNum(t) && isNum(cvtType) {return val.Convert(cvtType), nil}
	return val, fmt.Errorf("cannot convert %v to %v", val.Type(), cvtType)
}

func convertMap(aMap reflect.Value, cvtType reflect.Type) (reflect.Value, error) {
	etype := cvtType.Elem()
	output := reflect.MakeMap(cvtType)
	for _, k := range aMap.MapKeys() {
		v, err := convert(aMap.MapIndex(k), etype, aMap.MapIndex(k))
		if err != nil {return aMap, err}
		output.SetMapIndex(k, v)
	}
	return output, nil
}

func convertSlice(array reflect.Value, cvtType reflect.Type) (reflect.Value, error) {
	output := reflect.MakeSlice(cvtType, array.Len(), array.Cap())
	etype := cvtType.Elem()
	for i := 0; i < array.Len(); i++ {
		e, err := convert(array.Index(i), etype, array.Index(i))
		if err != nil {return array, err}
		output.Index(i).Set(e)
	}
	return output, nil
}

// thanks to cpcallen at Stackoverflow: https://stackoverflow.com/a/43918797/1026782
func privateStructValue(t reflect.Type, sf reflect.StructField, field reflect.Value) (reflect.Value, error) {
	if field.CanSet() {return field, nil}
	if sf.PkgPath == "" {return field, fmt.Errorf("cannot set %s.%s", t.Name(), sf.Name)}
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem(), nil
}

func isNum(t reflect.Type) bool {
	k := t.Kind()
	return reflect.Int <= k && k <= reflect.Complex128
}
