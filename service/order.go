package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"netfix/model"
)

// 工单业务错误定义。
var (
	ErrOrderNotFound  = errors.New("报修单不存在")
	ErrInvalidStatus  = errors.New("当前状态不允许该操作")
	ErrDuplicateOrder = errors.New("同一位置存在未关闭的报修单，请勿重复提交")
	ErrMissingField   = errors.New("必填字段缺失")
	ErrInvalidScore   = errors.New("评分必须为 1~5")
	ErrReworkLimit    = errors.New("返工次数已达上限，请转管理员协调")
)

// maxRework 为确认不通过时允许的最大返工次数，对应需求 NFFIX03。
const maxRework = 2

// OrderService 实现报修—审核—派单—维修—确认—评价的维修闭环业务，
// 使用内存 map 存储，模块可脱离数据库独立编译运行。
type OrderService struct {
	mu       sync.Mutex
	orders   map[string]*model.RepairOrder
	records  map[string]*model.RepairRecord
	evals    map[string]*model.Evaluation
	seq      int
	notifier Notifier // 消息通知，通过接口解耦
}

// NewOrderService 创建工单服务，notifier 不可为 nil。
func NewOrderService(notifier Notifier) *OrderService {
	return &OrderService{
		orders:   make(map[string]*model.RepairOrder),
		records:  make(map[string]*model.RepairRecord),
		evals:    make(map[string]*model.Evaluation),
		notifier: notifier,
	}
}

// SubmitOrder 创建报修单，返回报修单号。对应需求 NFRPT01。
//
// 业务规则：必填字段校验；同一报修人对同一位置存在未关闭工单时拒绝重复提交。
func (s *OrderService) SubmitOrder(o *model.RepairOrder) (string, error) {
	if o.ReporterID == 0 || o.FaultType == "" || o.Location == "" || o.Description == "" {
		return "", ErrMissingField
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, exist := range s.orders {
		if exist.ReporterID == o.ReporterID && exist.Location == o.Location && !isClosed(exist.Status) {
			return "", ErrDuplicateOrder
		}
	}
	s.seq++
	o.OrderNo = fmt.Sprintf("NF%s%04d", time.Now().Format("20060102"), s.seq)
	o.Status = model.StatusPending
	s.orders[o.OrderNo] = o
	return o.OrderNo, nil
}

// ReviewOrder 审核报修单。pass 为 false 时 reason 必填。对应需求 NFWOD01。
func (s *OrderService) ReviewOrder(orderNo string, pass bool, reason string) error {
	o, err := s.get(orderNo)
	if err != nil {
		return err
	}
	if o.Status != model.StatusPending {
		return ErrInvalidStatus
	}
	if !pass {
		if reason == "" {
			return fmt.Errorf("驳回时必须填写原因: %w", ErrMissingField)
		}
		s.updateStatus(o, model.StatusRejected)
		s.notifier.Notify(o.ReporterID, o.OrderNo, "报修单被驳回："+reason)
		return nil
	}
	s.updateStatus(o, model.StatusReviewed)
	return nil
}

// AssignOrder 将已受理的工单分配给维修人员。对应需求 NFWOD02。
func (s *OrderService) AssignOrder(orderNo string, fixerID uint) error {
	o, err := s.get(orderNo)
	if err != nil {
		return err
	}
	if o.Status != model.StatusReviewed {
		return ErrInvalidStatus
	}
	if fixerID == 0 {
		return ErrMissingField
	}
	o.FixerID = fixerID
	s.updateStatus(o, model.StatusAssigned)
	s.notifier.Notify(fixerID, o.OrderNo, notifyContent(o.OrderNo, model.StatusAssigned))
	return nil
}

// AcceptOrder 维修人员接单。对应需求 NFWOD03。
func (s *OrderService) AcceptOrder(orderNo string, fixerID uint) error {
	o, err := s.get(orderNo)
	if err != nil {
		return err
	}
	if o.Status != model.StatusAssigned || o.FixerID != fixerID {
		return ErrInvalidStatus
	}
	s.updateStatus(o, model.StatusRepairing)
	return nil
}

// FinishOrder 填写维修记录并提交完成申请。对应需求 NFFIX01、NFFIX02。
func (s *OrderService) FinishOrder(orderNo string, rec *model.RepairRecord) error {
	o, err := s.get(orderNo)
	if err != nil {
		return err
	}
	if o.Status != model.StatusRepairing {
		return ErrInvalidStatus
	}
	if rec.FaultCause == "" || rec.Solution == "" {
		return fmt.Errorf("故障原因与处理措施必填: %w", ErrMissingField)
	}
	rec.OrderNo = orderNo
	rec.FixerID = o.FixerID
	s.mu.Lock()
	s.records[orderNo] = rec
	s.mu.Unlock()
	s.updateStatus(o, model.StatusConfirm)
	s.notifier.Notify(o.ReporterID, o.OrderNo, notifyContent(o.OrderNo, model.StatusConfirm))
	return nil
}

// ConfirmOrder 报修人确认维修结果。对应需求 NFFIX03。
//
// 不通过时工单退回“维修中”返工，返工次数超过 maxRework 返回 ErrReworkLimit。
func (s *OrderService) ConfirmOrder(orderNo string, pass bool, reason string) error {
	o, err := s.get(orderNo)
	if err != nil {
		return err
	}
	if o.Status != model.StatusConfirm {
		return ErrInvalidStatus
	}
	if pass {
		s.updateStatus(o, model.StatusToEvaluate)
		return nil
	}
	if o.ReworkCount >= maxRework {
		return ErrReworkLimit
	}
	o.ReworkCount++
	s.updateStatus(o, model.StatusRepairing)
	s.notifier.Notify(o.FixerID, o.OrderNo, "报修人确认不通过，需返工："+reason)
	return nil
}

// Evaluate 报修人评价维修服务，评价后工单关闭，闭环结束。对应需求 NFEVA01。
func (s *OrderService) Evaluate(orderNo string, score int, comment string) error {
	o, err := s.get(orderNo)
	if err != nil {
		return err
	}
	if o.Status != model.StatusToEvaluate {
		return ErrInvalidStatus
	}
	if score < 1 || score > 5 {
		return ErrInvalidScore
	}
	s.mu.Lock()
	s.evals[orderNo] = &model.Evaluation{OrderNo: orderNo, Score: score, Comment: comment}
	s.mu.Unlock()
	s.updateStatus(o, model.StatusClosed)
	now := time.Now()
	o.FinishedAt = &now
	s.notifier.Notify(o.FixerID, o.OrderNo, fmt.Sprintf("工单已关闭，收到 %d 星评价", score))
	return nil
}

// CancelOrder 报修人取消报修单，仅待审核状态可由本人取消。对应需求 NFRPT04。
func (s *OrderService) CancelOrder(orderNo string, reporterID uint) error {
	o, err := s.get(orderNo)
	if err != nil {
		return err
	}
	if o.Status != model.StatusPending || o.ReporterID != reporterID {
		return ErrInvalidStatus
	}
	s.updateStatus(o, model.StatusCanceled)
	return nil
}

// GetOrder 查询报修单当前状态，对应需求 NFRPT03。
func (s *OrderService) GetOrder(orderNo string) (*model.RepairOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[orderNo]
	if !ok {
		return nil, ErrOrderNotFound
	}
	cp := *o
	return &cp, nil
}

// get 加锁读取报修单，内部复用。
func (s *OrderService) get(orderNo string) (*model.RepairOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[orderNo]
	if !ok {
		return nil, ErrOrderNotFound
	}
	return o, nil
}

// updateStatus 更新工单状态及修改时间。
func (s *OrderService) updateStatus(o *model.RepairOrder, st model.OrderStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o.Status = st
	o.UpdatedAt = time.Now()
}

// isClosed 判断工单是否处于终态（已关闭/已驳回/已取消）。
func isClosed(st model.OrderStatus) bool {
	switch st {
	case model.StatusClosed, model.StatusRejected, model.StatusCanceled:
		return true
	}
	return false
}
