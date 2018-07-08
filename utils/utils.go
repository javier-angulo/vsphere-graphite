package utils

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/vim25/types"
)

// Min : get the minimum of values
func Min(n ...int64) int64 {
	var min int64 = -1
	for _, i := range n {
		if i >= 0 {
			if min == -1 {
				min = i
			} else {
				if i < min {
					min = i
				}
			}
		}
	}
	return min
}

// Max : get the maximum of the values
func Max(n ...int64) int64 {
	var max int64 = -1
	for _, i := range n {
		if i >= 0 {
			if max == -1 {
				max = i
			} else {
				if i > max {
					max = i
				}
			}
		}
	}
	return max
}

// Sum : Sum the values
func Sum(n ...int64) int64 {
	var total int64
	for _, i := range n {
		if i > 0 {
			total += i
		}
	}
	return total
}

// Average : average the values
func Average(n ...int64) int64 {
	var total int64
	var count int64
	for _, i := range n {
		if i >= 0 {
			count++
			total += i
		}
	}
	favg := float64(total) / float64(count)
	return int64(math.Floor(favg + .5))
}

// MapObjRefs fills in object references into a map to another object reference
func MapObjRefs(sourceVal types.AnyType, dest map[types.ManagedObjectReference][]types.ManagedObjectReference, index types.ManagedObjectReference) error {
	mors, ok := sourceVal.(types.ArrayOfManagedObjectReference)
	if ok {
		if len(mors.ManagedObjectReference) > 0 {
			dest[index] = mors.ManagedObjectReference
			return nil
		}
		return errors.New("Property of " + index.String() + " didn't contain any object references")
	}
	return errors.New("Property " + index.String() + " was not a ManagedObjectReferences, it was " + fmt.Sprintf("%T", sourceVal))
}

// MapObjRef fills in object reference into a map to another object reference
func MapObjRef(sourceVal types.AnyType, dest map[types.ManagedObjectReference]types.ManagedObjectReference, index types.ManagedObjectReference) error {
	mor, ok := sourceVal.(types.ManagedObjectReference)
	if ok {
		dest[index] = mor
		return nil
	}
	return errors.New("Property of " + index.String() + " was not a ManagedObjectReference, it was " + fmt.Sprintf("%T", sourceVal))
}

// MapObjInt32 fills in an int32 into a map to another object reference
func MapObjInt32(sourceVal types.AnyType, dest map[types.ManagedObjectReference]int32, index types.ManagedObjectReference) error {
	val, ok := sourceVal.(int32)
	if ok {
		dest[index] = val
		return nil
	}
	return errors.New("Property of " + index.String() + " was not an int32, it was " + fmt.Sprintf("%T", sourceVal))
}

// StringMaptoString converts a string map to csv or get the first value
func StringMaptoString(value []string, separator string, noarray bool) string {
	if len(value) == 0 {
		return ""
	}
	if noarray {
		return value[0]
	}
	return strings.Join(value, separator)
}

// IntMaptoString converts a int map to csv or get the first value
func IntMaptoString(value []int, separator string, noarray bool) string {
	if len(value) == 0 {
		return ""
	}
	if noarray {
		return strconv.Itoa(value[0])
	}
	var strval []string
	for _, i := range value {
		strval = append(strval, strconv.Itoa(i))
	}
	return strings.Join(strval, separator)
}

// Int32MaptoString converts a int32 map to csv or get the first value
func Int32MaptoString(value []int32, separator string, noarray bool) string {
	if len(value) == 0 {
		return ""
	}
	if noarray {
		return strconv.FormatInt(int64(value[0]), 10)
	}
	var strval []string
	for _, i := range value {
		strval = append(strval, strconv.FormatInt(int64(i), 10))
	}
	return strings.Join(strval, separator)
}

// Int64MaptoString converts a int64 map to csv or get the first value
func Int64MaptoString(value []int64, separator string, noarray bool) string {
	if len(value) == 0 {
		return ""
	}
	if noarray {
		return strconv.FormatInt(value[0], 10)
	}
	var strval []string
	for _, i := range value {
		strval = append(strval, strconv.FormatInt(i, 10))
	}
	return strings.Join(strval, separator)
}

// ValToString : try to convert interface to string. Separated by separator if slice
func ValToString(value interface{}, separator string, noarray bool) string {
	switch v := value.(type) {
	case string:
		return v
	case []string:
		return StringMaptoString(v, separator, noarray)
	case int:
		return strconv.Itoa(v)
	case []int:
		return IntMaptoString(v, separator, noarray)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case []int32:
		return Int32MaptoString(v, separator, noarray)
	case int64:
		return strconv.FormatInt(v, 10)
	case []int64:
		return Int64MaptoString(v, separator, noarray)
	default:
		return ""
	}
}

// Join map[int]string into a string
func Join(values map[int]string, separator string) string {
	var keys []int
	for k := range values {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	// create a map with the key parts in order
	var tmp []string
	for _, k := range keys {
		tmp = append(tmp, values[k])
	}
	return strings.Join(tmp, separator)
}

// MustAtoi converts a string to integer and return 0 i case of error
func MustAtoi(value string) int {
	i, err := strconv.Atoi(value)
	if err != nil {
		i = 0
	}
	return i
}
