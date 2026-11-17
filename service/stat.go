package service

import (
	"fmt"
	"sort"

	"netfix/model"
)

// StatItem 为一条统计结果，按数量倒序输出。
type StatItem struct {
	Name  string // 统计维度名称，如故障类型、维修人员
	Count int    // 工单数量
}

// StatService 提供维修数据统计分析，对应需求 NFSTA01、NFSTA02。
// 数据源为 OrderService，通过 ListOrders 快照读取，不修改工单数据。
type StatService struct {
	orders *OrderService
}

// NewStatService 创建统计服务，orders 不可为 nil。
func NewStatService(orders *OrderService) *StatService {
	return &StatService{orders: orders}
}

// StatFaultTypes 统计各故障类型的工单数量，用于故障类型分布分析。对应 NFSTA01。
func (s *StatService) StatFaultTypes() []StatItem {
	m := make(map[string]int)
	for _, o := range s.orders.ListOrders() {
		m[o.FaultType]++
	}
	return sortedItems(m)
}

// StatWorkload 统计各维修人员已关闭的工单数量，用于工作量统计。对应 NFSTA02。
func (s *StatService) StatWorkload() []StatItem {
	m := make(map[string]int)
	for _, o := range s.orders.ListOrders() {
		if o.FixerID != 0 && o.Status == model.StatusClosed {
			m[fmt.Sprintf("维修员#%d", o.FixerID)]++
		}
	}
	return sortedItems(m)
}

// sortedItems 将统计 map 转为按数量倒序（数量相同按名称升序）的切片。
func sortedItems(m map[string]int) []StatItem {
	items := make([]StatItem, 0, len(m))
	for name, count := range m {
		items = append(items, StatItem{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	return items
}
