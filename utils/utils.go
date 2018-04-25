package utils

import (
	"fmt"
	"log"
	"math"

	"github.com/vmware/govmomi/vim25/types"
)

var stdlog, errlog *log.Logger

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

// MapObjRefs fills in object references into a map to another objec reference
func MapObjRefs(sourceVal types.AnyType, dest map[types.ManagedObjectReference][]types.ManagedObjectReference, index types.ManagedObjectReference) {
	mors, ok := sourceVal.(types.ArrayOfManagedObjectReference)
	if ok {
		if len(mors.ManagedObjectReference) > 0 {
			dest[index] = mors.ManagedObjectReference
		} else {
			errlog.Println("Property didn't contain any object references")
		}
	} else {
		errlog.Println("Property " + index.String() + " was not a ManagedObjectReferences, it was " + fmt.Sprintf("%T", sourceVal))
	}
}
