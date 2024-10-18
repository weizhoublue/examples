/*
本文件实现了一个名为 PodStore 的数据结构，用于存储和管理 Kubernetes Pod 的信息。

主要功能和原理：

1. 数据结构：
   - 使用 PodInfo 结构体封装 Pod 的标签和 IP 地址（包括 IPv4 和 IPv6）。
   - 使用 PodStore 结构体以 name 和 namespace 作为键存储 Pod 信息。
   - 提供线程安全的操作，使用 sync.RWMutex 确保并发安全。

2. 主要方法：
   - NewPodStore：创建新的 PodStore 实例。
   - AddPod：添加 Pod 信息到存储中。
   - DeletePod：从存储中删除指定的 Pod 信息。
   - GetIPWithLabelSelector：根据 metav1.LabelSelector 查找匹配的 IP 地址（返回 IpInfo 结构体切片）。

3. 使用场景：
   - 适用于需要存储和查询 Kubernetes Pod 信息的场景。
   - 可用于网络管理、监控和调试等场景。

4. 示例用法：
   - 创建 PodStore 实例。
   - 添加 Pod 信息。
   - 使用标签选择器查询匹配的 IP 地址。
   - 删除 Pod 信息。

注意事项：
- 所有公共方法都是并发安全的。
- IP 地址字段（IPv4 和 IPv6）允许为空字符串。
*/

package main

import (
	"fmt"
	"net"
	"sort"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodInfo 结构体用于存储 Pod 的标签和 IP 地址（包括 IPv4 和 IPv6）
type PodInfo struct {
	Labels map[string]string
	IPv4   string
	IPv6   string
}

// IpInfo 结构体用于存储 IP 地址信息
type IpInfo struct {
	IPv4 string
	IPv6 string
}

// PodStore 结构体用于存储 Pod 信息，以 name 和 namespace 作为键
type PodStore struct {
	mutex sync.RWMutex
	data  map[string]map[string]PodInfo
}

// NewPodStore 创建一个新的 PodStore
func NewPodStore() *PodStore {
	return &PodStore{
		data: make(map[string]map[string]PodInfo),
	}
}

// AddPod 添加一个 Pod 信息到存储中
func (ps *PodStore) AddPod(namespace, name string, labels map[string]string, ipv4, ipv6 string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if _, exists := ps.data[namespace]; !exists {
		ps.data[namespace] = make(map[string]PodInfo)
	}
	ps.data[namespace][name] = PodInfo{Labels: labels, IPv4: ipv4, IPv6: ipv6}
}

// DeletePod 从存储中删除一个 Pod 信息
func (ps *PodStore) DeletePod(namespace, name string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if _, exists := ps.data[namespace]; exists {
		delete(ps.data[namespace], name)
		if len(ps.data[namespace]) == 0 {
			delete(ps.data, namespace)
		}
	}
}

// GetIPWithLabelSelector 根据 metav1.LabelSelector 查找匹配的 IP 地址（包括 IPv4 和 IPv6）
func (ps *PodStore) GetIPWithLabelSelector(selector *metav1.LabelSelector) []IpInfo {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	// 将 LabelSelector 转换为 map[string]string
	selectorMap := make(map[string]string)
	for key, value := range selector.MatchLabels {
		selectorMap[key] = value
	}

	var ipInfos []IpInfo
	for _, namespaceData := range ps.data {
		for _, podInfo := range namespaceData {
			if matchesSelector(podInfo.Labels, selectorMap) {
				ipInfo := IpInfo{IPv4: podInfo.IPv4, IPv6: podInfo.IPv6}
				ipInfos = append(ipInfos, ipInfo)
			}
		}
	}
	// 对 IP 地址进行排序
	sort.Slice(ipInfos, func(i, j int) bool {
		return net.ParseIP(ipInfos[i].IPv4).String() < net.ParseIP(ipInfos[j].IPv4).String()
	})
	return ipInfos
}

// matchesSelector 检查给定的标签是否匹配选择器
func matchesSelector(labels, selector map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func main() {
	store := NewPodStore()

	// 添加 Pod 信息
	store.AddPod("default", "pod1", map[string]string{"app": "nginx", "env": "prod"}, "192.168.1.1", "fe80::1")
	store.AddPod("default", "pod2", map[string]string{"app": "nginx", "env": "dev"}, "192.168.1.2", "")
	store.AddPod("kube-system", "pod3", map[string]string{"app": "kube-dns"}, "", "fe80::2")

	// 创建 LabelSelector
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "nginx"},
	}

	// 查找匹配的 IP 地址
	ipInfos := store.GetIPWithLabelSelector(selector)
	fmt.Println("匹配的 IP 地址:")
	for _, ipInfo := range ipInfos {
		fmt.Printf("IPv4: %s, IPv6: %s\n", ipInfo.IPv4, ipInfo.IPv6)
	}

	// 删除 Pod 信息
	store.DeletePod("default", "pod1")
}
