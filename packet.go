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

func Marshal(aStruct interface{}) ([]byte, error) {
	tmpMap, err := structToMap(aStruct)
	if err != nil {
		return nil, err
	}
	return msgpack.Marshal(tmpMap)
}

func structToMap(aStruct interface{}) (map[string]interface{}, error) {
	tmpMap := make(map[string]interface{})
	val := reflect.ValueOf(aStruct)
	if reflect.Indirect(val) == val {
		return nil, fmt.Errorf("attempt to call marshall without a pointer")
	}
	for {
		next := reflect.Indirect(val)
		if val == next {
			break
		}
		val = next
	}
	t := val.Type()
	for i := 0; i < val.NumField(); i++ {
		ft := t.Field(i)
		f := val.Field(i)
		f, err := privateStructValue(t, ft, f)
		if err != nil {
			return nil, err
		}
		tmpMap[ft.Name] = f.Interface()
	}
	return tmpMap, nil
}

func Unmarshal(bytes []byte, aStruct interface{}) error {
	var tmpMap map[string]interface{}

	err := msgpack.Unmarshal(bytes, &tmpMap)
	if err != nil {
		return err
	}
	return mapToStruct(tmpMap, aStruct)
}

func mapToStruct(aMap map[string]interface{}, aStruct interface{}) error {
	inputValue := reflect.Indirect(reflect.ValueOf(aStruct))
	if reflect.Indirect(inputValue) == inputValue {
		return fmt.Errorf("Attempt to call unmarshall without a pointer")
	}
	t := reflect.Indirect(inputValue).Type()
	fmt.Printf("TYPE: %v\n", t)
	result := reflect.New(t)
	newDataValue := reflect.Indirect(result)
	for k, v := range aMap {
		if sf, present := t.FieldByName(k); present {
			f, err := privateStructValue(t, sf, newDataValue.FieldByName(k))
			if err != nil {
				return err
			}
			vValue := reflect.ValueOf(v)
			if !vValue.Type().AssignableTo(sf.Type) {
				if !isNum(sf.Type) || !isNum(vValue.Type()) {
					return fmt.Errorf("bad value for %s.%s: %v", t.Name(), sf.Name, v)
				}
				vValue = vValue.Convert(sf.Type)
			}
			f.Set(vValue)
		} else {
			fmt.Printf("No field %s.%s\n", t.Name(), k)
		}
	}
	inputValue.Set(result)
	return nil
}

// thanks to cpcallen at Stackoverflow: https://stackoverflow.com/a/43918797/1026782
func privateStructValue(t reflect.Type, sf reflect.StructField, field reflect.Value) (reflect.Value, error) {
	if field.CanSet() {
		return field, nil
	}
	if sf.PkgPath == "" {
		return field, fmt.Errorf("cannot set %s.%s", t.Name(), sf.Name)
	}
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem(), nil
}

func isNum(t reflect.Type) bool {
	k := t.Kind()
	return reflect.Int <= k && k <= reflect.Complex128
}
