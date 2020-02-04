package jet

import (
	"fmt"
	"reflect"
	"sync"
)

// Ranger a value implementing a ranger interface is able to iterate on his value
// and can be used directly in a range statement
type Ranger interface {
	Range() (reflect.Value, reflect.Value, bool)
}

func getRanger(v reflect.Value) Ranger {
	tuP := v.Type()
	if tuP.Implements(rangerType) {
		return v.Interface().(Ranger)
	}
	k := tuP.Kind()
	switch k {
	case reflect.Ptr, reflect.Interface:
		v = v.Elem()
		k = v.Kind()
		fallthrough
	case reflect.Slice, reflect.Array:
		sliceranger := sliceRangers.Get().(*sliceRanger)
		sliceranger.i = -1
		sliceranger.len = v.Len()
		sliceranger.v = v
		return sliceranger
	case reflect.Map:
		mapranger := mapRangers.Get().(*mapRanger)
		*mapranger = mapRanger{v: v, keys: v.MapKeys(), len: v.Len()}
		return mapranger
	case reflect.Chan:
		chanranger := chanRangers.Get().(*chanRanger)
		*chanranger = chanRanger{v: v}
		return chanranger
	}
	panic(fmt.Errorf("type %s is not rangeable", tuP))
}

var (
	sliceRangers = sync.Pool{
		New: func() interface{} {
			return new(sliceRanger)
		},
	}
	mapRangers = sync.Pool{
		New: func() interface{} {
			return new(mapRanger)
		},
	}
	chanRangers = sync.Pool{
		New: func() interface{} {
			return new(chanRanger)
		},
	}
)

type sliceRanger struct {
	v   reflect.Value
	len int
	i   int
}

func (s *sliceRanger) Range() (index, value reflect.Value, end bool) {
	s.i++
	index = reflect.ValueOf(s.i)
	if s.i < s.len {
		value = s.v.Index(s.i)
		return
	}
	sliceRangers.Put(s)
	end = true
	return
}

type chanRanger struct {
	v reflect.Value
}

func (s *chanRanger) Range() (_, value reflect.Value, end bool) {
	value, end = s.v.Recv()
	if end {
		chanRangers.Put(s)
	}
	return
}

type mapRanger struct {
	v    reflect.Value
	keys []reflect.Value
	len  int
	i    int
}

func (s *mapRanger) Range() (index, value reflect.Value, end bool) {
	if s.i < s.len {
		index = s.keys[s.i]
		value = s.v.MapIndex(index)
		s.i++
		return
	}
	end = true
	mapRangers.Put(s)
	return
}
