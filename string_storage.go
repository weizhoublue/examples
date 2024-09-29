package main

/*
本文件实现了一个名为 StringStorage 的数据结构，用于存储和管理键值对。

主要功能和原理：

1. 数据结构：
   - 使用 Key 结构体封装 A 和 B 字符串作为键。
   - 使用 Value 结构体封装 C 和 D 字符串作为值。
   - 维护两个映射：keyToValue 和 valueToKey，实现双向查找。
   - 使用 keyOrder 切片维护键的插入顺序。
   - 通过 capacity 限制存储的最大容量。

2. 并发安全：
   - 使用 sync.RWMutex 确保并发操作的安全性。

3. 容量管理：
   - 当达到容量上限时，自动删除最旧的键值对。

4. 主要方法：
   - NewStringStorage：创建新的 StringStorage 实例。
   - Set：设置键值对，处理容量限制。
   - Get：根据键获取值。
   - Delete：删除指定的键值对。
   - GetByValue：根据值查找对应的键。
   - Len：返回当前存储的键值对数量。

5. 使用场景：
   - 适用于需要双向查找、有序存储和容量限制的键值对管理。
   - 可用于缓存系统、会话管理等场景。

注意事项：
- 所有公共方法都是并发安全的。
- 达到容量上限时会自动删除最旧的数据。
- 支持通过值查找键，但要注意值的唯一性。
*/

import (
	"fmt"
	"sync"
)

// PodName 封装 Podname 和 Namespace
type PodName struct {
	Podname   string
	Namespace string
}

// PodID 封装 PodUuid 和 ContainerId
type PodID struct {
	PodUuid     string
	ContainerId string
}

// PodRegistry 是一个存储结构，用于存储和检索 Pod 相关信息
type PodRegistry struct {
	mutex      sync.RWMutex
	keyToValue map[PodName]PodID
	valueToKey map[PodID]PodName
	keyOrder   []PodName // 用于维护键的插入顺序
	capacity   int       // 存储的最大容量
}

// NewPodRegistry 创建并返回一个新的 PodRegistry 实例
func NewPodRegistry(capacity int) *PodRegistry {
	return &PodRegistry{
		keyToValue: make(map[PodName]PodID),
		valueToKey: make(map[PodID]PodName),
		keyOrder:   make([]PodName, 0, capacity),
		capacity:   capacity,
	}
}

// Set 设置 PodName 对应的 PodID 值
func (pr *PodRegistry) Set(key PodName, value PodID) {
	pr.mutex.Lock()
	defer pr.mutex.Unlock()

	_, exists := pr.keyToValue[key]
	if exists {
		// 如果键已存在，直接更新值
		oldValue := pr.keyToValue[key]
		delete(pr.valueToKey, oldValue) // 删除旧的 value-key 映射
		pr.keyToValue[key] = value
		pr.valueToKey[value] = key
		// 更新键在 keyOrder 中的位置
		pr.removeFromKeyOrder(key)
		pr.keyOrder = append(pr.keyOrder, key)
	} else {
		// 如果是新键，检查是否达到容量上限
		if len(pr.keyToValue) >= pr.capacity {
			// 删除最旧的键值对
			oldestKey := pr.keyOrder[0]
			pr.deleteInternal(oldestKey)
		}
		// 添加新的键值对
		pr.keyToValue[key] = value
		pr.valueToKey[value] = key
		pr.keyOrder = append(pr.keyOrder, key)
	}
}

// Delete 删除与 PodName 对应的条目
func (pr *PodRegistry) Delete(key PodName) {
	pr.mutex.Lock()
	defer pr.mutex.Unlock()

	pr.deleteInternal(key)
}

// deleteInternal 内部使用的删除方法，不加锁
func (pr *PodRegistry) deleteInternal(key PodName) {
	value, exists := pr.keyToValue[key]
	if !exists {
		// 如果键不存在，直接返回，不做任何操作
		return
	}

	// 删除 keyToValue 中的条目
	delete(pr.keyToValue, key)

	// 删除 valueToKey 中的条目
	delete(pr.valueToKey, value)

	// 从 keyOrder 中移除键
	pr.removeFromKeyOrder(key)
}

// removeFromKeyOrder 从 keyOrder 切片中移除指定的键
func (pr *PodRegistry) removeFromKeyOrder(key PodName) {
	for i, k := range pr.keyOrder {
		if k == key {
			// 使用 copy 来移除元素，避免内存泄漏
			copy(pr.keyOrder[i:], pr.keyOrder[i+1:])
			pr.keyOrder = pr.keyOrder[:len(pr.keyOrder)-1]
			break
		}
	}
}

// GetValueByKey 根据 PodName 查询 PodID
func (pr *PodRegistry) GetValueByKey(key PodName) (PodID, bool) {
	pr.mutex.RLock()
	defer pr.mutex.RUnlock()

	value, exists := pr.keyToValue[key]
	return value, exists
}

// GetKeyByValue 根据 PodID 查询 PodName
func (pr *PodRegistry) GetKeyByValue(value PodID) (PodName, bool) {
	pr.mutex.RLock()
	defer pr.mutex.RUnlock()

	key, exists := pr.valueToKey[value]
	return key, exists
}

// Count 返回存储的键值对数量
func (pr *PodRegistry) Count() int {
	pr.mutex.RLock()
	defer pr.mutex.RUnlock()

	return len(pr.keyToValue)
}

// GetAll 返回所有存储的键值对
func (pr *PodRegistry) GetAll() map[PodName]PodID {
	pr.mutex.RLock()
	defer pr.mutex.RUnlock()

	result := make(map[PodName]PodID, len(pr.keyToValue))
	for k, v := range pr.keyToValue {
		result[k] = v
	}
	return result
}

// main 函数用于测试 PodRegistry
func main() {
	registry := NewPodRegistry(3) // 创建容量为 3 的注册表

	// 测试添加超过容量的键值对
	key1 := PodName{Podname: "pod1", Namespace: "ns1"}
	value1 := PodID{PodUuid: "uuid1", ContainerId: "container1"}
	registry.Set(key1, value1)

	key2 := PodName{Podname: "pod2", Namespace: "ns2"}
	value2 := PodID{PodUuid: "uuid2", ContainerId: "container2"}
	registry.Set(key2, value2)

	key3 := PodName{Podname: "pod3", Namespace: "ns3"}
	value3 := PodID{PodUuid: "uuid3", ContainerId: "container3"}
	registry.Set(key3, value3)

	key4 := PodName{Podname: "pod4", Namespace: "ns4"}
	value4 := PodID{PodUuid: "uuid4", ContainerId: "container4"}
	registry.Set(key4, value4)

	fmt.Printf("当前存储的键值对数量: %d\n", registry.Count())

	// 测试 GetAll
	allData := registry.GetAll()
	fmt.Println("所有存储的键值对:")
	for k, v := range allData {
		fmt.Printf("键: %v, 值: %v\n", k, v)
	}

	// 验证最旧的键值对（key1）是否被删
	if _, found := registry.GetValueByKey(key1); !found {
		fmt.Printf("键 %v 已被自动删除\n", key1)
	}
}
