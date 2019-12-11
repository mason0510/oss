package utils

import (
	"fmt"
	"log"
	"os"
	"sync"
)

type CommonMap struct {
	sync.RWMutex
	m map[string]interface{}
}

type Tuple struct {
	Key string
	Val interface{}
}

type Common struct {
}

func NewCommonMap(size int) *CommonMap {
	if size > 0 {
		return &CommonMap{m: make(map[string]interface{}, size)}
	} else {
		return &CommonMap{m: make(map[string]interface{})}
	}
}
func (cMap *CommonMap) GetValue(k string) (interface{}, bool) {
	cMap.RLock()
	defer cMap.RUnlock()
	v, ok := cMap.m[k]
	return v, ok
}
func (cMap *CommonMap) Put(k string, v interface{}) {
	cMap.Lock()
	defer cMap.Unlock()
	cMap.m[k] = v
}
func (cMap *CommonMap) Iter() <-chan Tuple { // reduce memory
	ch := make(chan Tuple)
	go func() {
		cMap.RLock()
		for k, v := range cMap.m {
			ch <- Tuple{Key: k, Val: v}
		}
		close(ch)
		cMap.RUnlock()
	}()
	return ch
}
func (cMap *CommonMap) LockKey(k string) {
	cMap.Lock()
	if v, ok := cMap.m[k]; ok {
		cMap.m[k+"_lock_"] = true
		cMap.Unlock()
		switch v.(type) {
		case *sync.Mutex:
			v.(*sync.Mutex).Lock()
		default:
			log.Print(fmt.Sprintf("LockKey %sMap", k))
		}
	} else {
		cMap.m[k] = &sync.Mutex{}
		v = cMap.m[k]
		cMap.m[k+"_lock_"] = true
		v.(*sync.Mutex).Lock()
		cMap.Unlock()
	}
}
func (cMap *CommonMap) UnLockKey(k string) {
	cMap.Lock()
	if v, ok := cMap.m[k]; ok {
		switch v.(type) {
		case *sync.Mutex:
			v.(*sync.Mutex).Unlock()
		default:
			log.Print(fmt.Sprintf("UnLockKey %sMap", k))
		}
		delete(cMap.m, k+"_lock_") // memory leak
		delete(cMap.m, k)          // memory leak
	}
	cMap.Unlock()
}
func (cMap *CommonMap) IsLock(k string) bool {
	cMap.Lock()
	if v, ok := cMap.m[k+"_lock_"]; ok {
		cMap.Unlock()
		return v.(bool)
	}
	cMap.Unlock()
	return false
}
func (cMap *CommonMap) Keys() []string {
	cMap.Lock()
	keys := make([]string, len(cMap.m))
	defer cMap.Unlock()
	for k := range cMap.m {
		keys = append(keys, k)
	}
	return keys
}
func (cMap *CommonMap) Clear() {
	cMap.Lock()
	defer cMap.Unlock()
	cMap.m = make(map[string]interface{})
}
func (cMap *CommonMap) Remove(key string) {
	cMap.Lock()
	defer cMap.Unlock()
	if _, ok := cMap.m[key]; ok {
		delete(cMap.m, key)
	}
}
func (cMap *CommonMap) AddUniq(key string) {
	cMap.Lock()
	defer cMap.Unlock()
	if _, ok := cMap.m[key]; !ok {
		cMap.m[key] = nil
	}
}
func (cMap *CommonMap) AddCount(key string, count int) {
	cMap.Lock()
	defer cMap.Unlock()
	if _v, ok := cMap.m[key]; ok {
		v := _v.(int)
		v = v + count
		cMap.m[key] = v
	} else {
		cMap.m[key] = 1
	}
}
func (cMap *CommonMap) AddCountInt64(key string, count int64) {
	cMap.Lock()
	defer cMap.Unlock()
	if _v, ok := cMap.m[key]; ok {
		v := _v.(int64)
		v = v + count
		cMap.m[key] = v
	} else {
		cMap.m[key] = count
	}
}
func (cMap *CommonMap) Add(key string) {
	cMap.Lock()
	defer cMap.Unlock()
	if _v, ok := cMap.m[key]; ok {
		v := _v.(int)
		v = v + 1
		cMap.m[key] = v
	} else {
		cMap.m[key] = 1
	}
}
func (cMap *CommonMap) Zero() {
	cMap.Lock()
	defer cMap.Unlock()
	for k := range cMap.m {
		cMap.m[k] = 0
	}
}
func (cMap *CommonMap) Contains(i ...interface{}) bool {
	cMap.Lock()
	defer cMap.Unlock()
	for _, val := range i {
		if _, ok := cMap.m[val.(string)]; !ok {
			return false
		}
	}
	return true
}
func (cMap *CommonMap) Get() map[string]interface{} {
	cMap.Lock()
	defer cMap.Unlock()
	m := make(map[string]interface{})
	for k, v := range cMap.m {
		m[k] = v
	}
	return m
}

func FileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil
}
