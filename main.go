// Command netfix 演示 NetFix 工单管理模块的维修闭环流程。
//
// 运行方式：go run .
package main

import (
	"fmt"
	"log"

	"netfix/model"
	"netfix/service"
)

// reporterID 与 fixerID 为演示用的报修人、维修人员 ID。
const (
	reporterID uint = 1001
	fixerID    uint = 2001
)

func main() {
	msg := service.NewMsgService()
	svc := service.NewOrderService(msg) // 通过 Notifier 接口注入消息服务

	// 1. 报修人提交报修单
	orderNo, err := svc.SubmitOrder(&model.RepairOrder{
		ReporterID:  reporterID,
		FaultType:   "宿舍网口故障",
		Location:    "番禺校区 T3-502",
		Description: "墙面网口无信号，指示灯不亮",
		Photos:      []string{"uploads/t3502-1.jpg"},
	})
	check(err)
	printStatus(svc, orderNo, "提交报修单")

	// 2. 管理员审核受理并派单
	check(svc.ReviewOrder(orderNo, true, ""))
	printStatus(svc, orderNo, "审核受理")
	check(svc.AssignOrder(orderNo, fixerID))
	printStatus(svc, orderNo, "分配工单")

	// 3. 维修人员接单、维修并提交完成申请
	check(svc.AcceptOrder(orderNo, fixerID))
	printStatus(svc, orderNo, "维修员接单")
	check(svc.FinishOrder(orderNo, &model.RepairRecord{
		FaultCause:  "网口模块损坏",
		Solution:    "更换网口模块并重新压接水晶头",
		PartChanged: true,
		PartName:    "RJ45 网口模块",
	}))
	printStatus(svc, orderNo, "提交完成申请")

	// 4. 报修人确认通过并评价，工单关闭
	check(svc.ConfirmOrder(orderNo, true, ""))
	check(svc.Evaluate(orderNo, 5, "师傅很专业，当天就修好了"))
	printStatus(svc, orderNo, "评价完成，工单关闭")

	// 查看维修员收到的全部通知
	fmt.Println("\n维修员收到的消息通知：")
	for _, m := range msg.ListByUser(fixerID) {
		fmt.Printf("  [%s] %s\n", m.OrderNo, m.Content)
	}
}

// check 在操作失败时输出错误并终止演示。
func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// printStatus 打印一次操作后的工单状态。
func printStatus(svc *service.OrderService, orderNo, step string) {
	o, err := svc.GetOrder(orderNo)
	check(err)
	fmt.Printf("步骤【%s】报修单 %s 状态 -> %s\n", step, o.OrderNo, o.Status)
}
