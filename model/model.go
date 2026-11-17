// Package model 定义 NetFix 系统的核心实体模型与业务枚举。
//
// 实体与《NetFix 系统详细设计报告》第 4 章数据库设计一一对应，
// 所有实体均嵌入 BaseModel 以复用主键与时间戳字段。
package model

import "time"

// Role 表示系统用户角色。
type Role string

// 系统支持的三种用户角色。
const (
	RoleReporter Role = "报修人"
	RoleFixer    Role = "维修人员"
	RoleAdmin    Role = "管理员"
)

// OrderStatus 表示报修单在维修闭环中的状态。
type OrderStatus string

// 报修单状态常量，状态流转见业务流程图（图 5-1）。
const (
	StatusPending    OrderStatus = "待审核"
	StatusReviewed   OrderStatus = "待派单"
	StatusAssigned   OrderStatus = "待接单"
	StatusRepairing  OrderStatus = "维修中"
	StatusConfirm    OrderStatus = "待确认"
	StatusToEvaluate OrderStatus = "待评价"
	StatusClosed     OrderStatus = "已关闭"
	StatusRejected   OrderStatus = "已驳回"
	StatusCanceled   OrderStatus = "已取消"
)

// BaseModel 为所有实体提供公共主键与时间戳字段（Go 结构体嵌入，对应类图中的继承关系）。
type BaseModel struct {
	ID        uint      // 主键
	CreatedAt time.Time // 创建时间
	UpdatedAt time.Time // 更新时间
}

// User 表示系统用户，同一实体通过 Role 区分报修人、维修人员与管理员。
type User struct {
	BaseModel
	StudentNo string // 学工号，登录账号，唯一
	Name      string // 姓名
	Password  string // 密码（加密存储）
	Role      Role   // 角色
	Phone     string // 联系电话
	Email     string // 邮箱，可空
	Status    int    // 1 启用，0 停用
}

// RepairOrder 表示报修单，是维修闭环流转的核心单据。
type RepairOrder struct {
	BaseModel
	OrderNo     string      // 报修单号，规则：NF+8 位日期+4 位流水号
	ReporterID  uint        // 报修人 ID
	FixerID     uint        // 维修员 ID，派单后填写，0 表示未派单
	FaultType   string      // 故障类型
	Location    string      // 故障地点
	Description string      // 故障描述
	Photos      []string    // 故障照片路径，0~3 张
	Status      OrderStatus // 当前状态
	ReworkCount int         // 返工次数，最多 2 次
	FinishedAt  *time.Time  // 关闭时间，未关闭为空
}

// RepairRecord 表示维修记录，与报修单 1:1 对应。
type RepairRecord struct {
	BaseModel
	OrderNo     string // 关联报修单号
	FixerID     uint   // 填写维修员 ID
	FaultCause  string // 故障原因
	Solution    string // 处理措施
	PartChanged bool   // 是否更换配件
	PartName    string // 配件名称，可空
}

// Evaluation 表示报修人对维修服务的评价，与报修单 1:1 对应。
type Evaluation struct {
	BaseModel
	OrderNo string // 关联报修单号
	Score   int    // 评分，1~5
	Comment string // 评语，可空
}

// Message 表示系统内状态变更通知。
type Message struct {
	BaseModel
	UserID  uint   // 接收人 ID
	OrderNo string // 关联报修单号
	Content string // 消息内容
	IsRead  bool   // 是否已读
}
