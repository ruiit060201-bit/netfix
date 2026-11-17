// Package service 实现 NetFix 系统业务逻辑层的核心服务。
//
// 本包聚焦工单管理模块（对应需求编号 NFRPT01~05、NFWOD01~04、NFFIX01~03、
// NFEVA01），模块之间仅通过 Notifier 接口协作，不直接相互依赖。
package service

import (
	"fmt"
	"sync"

	"netfix/model"
)

// Notifier 是消息通知接口。
//
// 工单状态变更时 OrderService 通过该接口发送系统内消息，
// 对应需求 NFRPT05。使用接口而非具体实现，便于二期替换为邮件/短信通知。
type Notifier interface {
	// Notify 向指定用户发送一条与报修单关联的系统内消息。
	Notify(userID uint, orderNo, content string)
}

// MsgService 是 Notifier 的系统内消息实现，使用内存切片存储消息。
type MsgService struct {
	mu       sync.Mutex
	messages []model.Message
	nextID   uint
}

// NewMsgService 创建一个空的消息服务。
func NewMsgService() *MsgService {
	return &MsgService{nextID: 1}
}

// Notify 实现 Notifier 接口，生成一条未读消息。
func (s *MsgService) Notify(userID uint, orderNo, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, model.Message{
		BaseModel: model.BaseModel{ID: s.nextID},
		UserID:    userID,
		OrderNo:   orderNo,
		Content:   content,
	})
	s.nextID++
}

// ListByUser 返回指定用户收到的全部消息，按生成顺序排列。
func (s *MsgService) ListByUser(userID uint) []model.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.Message
	for _, m := range s.messages {
		if m.UserID == userID {
			out = append(out, m)
		}
	}
	return out
}

// String 格式化输出一条消息，便于演示与调试。
func notifyContent(orderNo string, status model.OrderStatus) string {
	return fmt.Sprintf("您的报修单 %s 状态已更新为【%s】", orderNo, status)
}
