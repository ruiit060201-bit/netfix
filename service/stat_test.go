package service

import (
	"testing"

	"netfix/model"
)

// TestStatService 测试故障类型分布与维修人员工作量统计。
func TestStatService(t *testing.T) {
	svc, _ := newTestService()
	stat := NewStatService(svc)

	// 关闭一张完整工单：报修→审核→派单→接单→完成→确认→评价
	closedNo := submitOK(t, svc)
	svc.ReviewOrder(closedNo, true, "")
	svc.AssignOrder(closedNo, 2001)
	svc.AcceptOrder(closedNo, 2001)
	svc.FinishOrder(closedNo, &model.RepairRecord{FaultCause: "网口损坏", Solution: "更换模块"})
	svc.ConfirmOrder(closedNo, true, "")
	svc.Evaluate(closedNo, 5, "好评")

	// 另一张不同故障类型的工单，仅到待审核
	if _, err := svc.SubmitOrder(&model.RepairOrder{
		ReporterID: 1002, FaultType: "无线AP故障", Location: "图书馆-3F", Description: "AP掉线",
	}); err != nil {
		t.Fatalf("提交第二张开单失败: %v", err)
	}

	types := stat.StatFaultTypes()
	if len(types) != 2 || types[0].Name != "宿舍网口故障" || types[0].Count != 1 {
		t.Errorf("故障类型统计结果异常: %+v", types)
	}

	load := stat.StatWorkload()
	if len(load) != 1 || load[0].Name != "维修员#2001" || load[0].Count != 1 {
		t.Errorf("工作量统计结果异常: %+v", load)
	}
}
