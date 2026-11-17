package service

import (
	"errors"
	"testing"

	"netfix/model"
)

// newTestService 创建测试用的工单服务与消息服务。
func newTestService() (*OrderService, *MsgService) {
	msg := NewMsgService()
	return NewOrderService(msg), msg
}

// submitOK 提交一张合法报修单，失败则终止测试。
func submitOK(t *testing.T, svc *OrderService) string {
	t.Helper()
	orderNo, err := svc.SubmitOrder(&model.RepairOrder{
		ReporterID:  1001,
		FaultType:   "宿舍网口故障",
		Location:    "T3-502",
		Description: "网口无信号",
	})
	if err != nil {
		t.Fatalf("提交报修单失败: %v", err)
	}
	return orderNo
}

// TestClosedLoop 测试报修→审核→派单→接单→完成→确认→评价的完整闭环。
func TestClosedLoop(t *testing.T) {
	svc, msg := newTestService()
	orderNo := submitOK(t, svc)

	steps := []struct {
		name string
		fn   func() error
		want model.OrderStatus
	}{
		{"审核受理", func() error { return svc.ReviewOrder(orderNo, true, "") }, model.StatusReviewed},
		{"分配工单", func() error { return svc.AssignOrder(orderNo, 2001) }, model.StatusAssigned},
		{"维修员接单", func() error { return svc.AcceptOrder(orderNo, 2001) }, model.StatusRepairing},
		{"提交完成申请", func() error {
			return svc.FinishOrder(orderNo, &model.RepairRecord{FaultCause: "网口损坏", Solution: "更换模块"})
		}, model.StatusConfirm},
		{"确认通过", func() error { return svc.ConfirmOrder(orderNo, true, "") }, model.StatusToEvaluate},
		{"评价并关闭", func() error { return svc.Evaluate(orderNo, 5, "好评") }, model.StatusClosed},
	}
	for _, st := range steps {
		if err := st.fn(); err != nil {
			t.Fatalf("%s 失败: %v", st.name, err)
		}
		o, _ := svc.GetOrder(orderNo)
		if o.Status != st.want {
			t.Errorf("%s 后状态 = %s, 期望 %s", st.name, o.Status, st.want)
		}
	}
	if got := len(msg.ListByUser(2001)); got != 2 {
		t.Errorf("维修员应收到 2 条通知, 实际 %d 条", got)
	}
}

// TestDuplicateSubmit 测试同一位置未关闭工单不可重复提交。
func TestDuplicateSubmit(t *testing.T) {
	svc, _ := newTestService()
	submitOK(t, svc)
	_, err := svc.SubmitOrder(&model.RepairOrder{
		ReporterID: 1001, FaultType: "无线AP故障", Location: "T3-502", Description: "再次报修",
	})
	if !errors.Is(err, ErrDuplicateOrder) {
		t.Errorf("重复提交应返回 ErrDuplicateOrder, 实际 %v", err)
	}
}

// TestReviewReject 测试审核驳回必须填写原因。
func TestReviewReject(t *testing.T) {
	svc, _ := newTestService()
	orderNo := submitOK(t, svc)
	if err := svc.ReviewOrder(orderNo, false, ""); !errors.Is(err, ErrMissingField) {
		t.Errorf("驳回未填原因应返回 ErrMissingField, 实际 %v", err)
	}
	if err := svc.ReviewOrder(orderNo, false, "非校园网设备"); err != nil {
		t.Fatalf("合法驳回失败: %v", err)
	}
	o, _ := svc.GetOrder(orderNo)
	if o.Status != model.StatusRejected {
		t.Errorf("驳回后状态 = %s, 期望 %s", o.Status, model.StatusRejected)
	}
}

// TestConfirmRework 测试确认不通过退回返工及返工次数上限。
func TestConfirmRework(t *testing.T) {
	svc, _ := newTestService()
	orderNo := submitOK(t, svc)
	svc.ReviewOrder(orderNo, true, "")
	svc.AssignOrder(orderNo, 2001)
	svc.AcceptOrder(orderNo, 2001)

	finish := func() {
		svc.FinishOrder(orderNo, &model.RepairRecord{FaultCause: "原因", Solution: "措施"})
	}
	finish()
	for i := 0; i < maxRework; i++ {
		if err := svc.ConfirmOrder(orderNo, false, "仍无法上网"); err != nil {
			t.Fatalf("第 %d 次返工退回失败: %v", i+1, err)
		}
		o, _ := svc.GetOrder(orderNo)
		if o.Status != model.StatusRepairing {
			t.Errorf("返工后状态 = %s, 期望 %s", o.Status, model.StatusRepairing)
		}
		finish()
	}
	if err := svc.ConfirmOrder(orderNo, false, "第三次不通过"); !errors.Is(err, ErrReworkLimit) {
		t.Errorf("超过返工上限应返回 ErrReworkLimit, 实际 %v", err)
	}
}

// TestInvalidScore 测试评分超出 1~5 范围被拒绝。
func TestInvalidScore(t *testing.T) {
	svc, _ := newTestService()
	orderNo := submitOK(t, svc)
	svc.ReviewOrder(orderNo, true, "")
	svc.AssignOrder(orderNo, 2001)
	svc.AcceptOrder(orderNo, 2001)
	svc.FinishOrder(orderNo, &model.RepairRecord{FaultCause: "原因", Solution: "措施"})
	svc.ConfirmOrder(orderNo, true, "")
	if err := svc.Evaluate(orderNo, 6, ""); !errors.Is(err, ErrInvalidScore) {
		t.Errorf("评分 6 应返回 ErrInvalidScore, 实际 %v", err)
	}
}

// TestInvalidStatusFlow 测试状态机非法跳转被拒绝。
func TestInvalidStatusFlow(t *testing.T) {
	svc, _ := newTestService()
	orderNo := submitOK(t, svc)
	if err := svc.AssignOrder(orderNo, 2001); !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("待审核状态直接派单应返回 ErrInvalidStatus, 实际 %v", err)
	}
}
