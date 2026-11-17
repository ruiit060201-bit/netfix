# NetFix 校园网络设备故障报修与维修闭环管理系统

《软件工程实验》实验 6 成果：基于实验 5 详细设计，使用 Go 语言实现的**工单管理核心模块**
（故障报修 + 维修闭环流转），模块独立、可单独编译运行。

## 功能范围

实现"报修 → 审核 → 派单 → 接单 → 维修 → 确认 → 评价 → 关闭"的完整维修闭环，
对应需求规格说明书中的功能点：NFRPT01~05（故障报修）、NFWOD01~04（工单管理）、
NFFIX01~03（维修处理）、NFEVA01（服务评价）。

- 支持报修单取消（NFRPT04）：待审核状态下报修人可取消自己的工单，取消后同位置可重新报修。
- 支持维修统计查询（NFSTA01/02）：按故障类型统计工单分布、按维修人员统计已关闭工单工作量。

## 项目结构

```
netfix/
├── go.mod                  # Go 模块定义，无第三方依赖
├── main.go                 # 演示入口：完整跑通一次维修闭环
├── model/
│   └── model.go            # 实体模型：User / RepairOrder / RepairRecord /
│                           #   Evaluation / Message，角色与状态枚举
└── service/
    ├── msg.go              # MsgService：消息通知服务（实现 Notifier 接口）
    ├── order.go            # OrderService：工单闭环核心业务
    └── order_test.go       # 单元测试：闭环、重复报修、驳回、返工、评分、状态机
```

## 运行方式

需要 Go 1.22 及以上版本。

```bash
go run .        # 运行闭环演示
go test ./...   # 运行单元测试
go vet ./...    # 静态检查
```

## 设计要点

- **模块间接口调用**：`OrderService` 不直接依赖消息模块，而是依赖 `Notifier` 接口
  （`service/msg.go`），`MsgService` 实现该接口并在 `main` 中注入。二期可将通知
  替换为邮件/短信实现而不改动工单模块代码。
- **业务规则内聚**：报修单号生成（NF+日期+流水号）、防重复报修、状态机校验、
  返工上限（2 次）、评分范围（1~5）等规则全部内聚在 `OrderService`。
- **状态机**：工单状态严格按 `待审核 → 待派单 → 待接单 → 维修中 → 待确认 →
  待评价 → 已关闭` 单向流转，非法跳转返回 `ErrInvalidStatus`。

## 编码规范

遵循 Go 官方编码规范（Effective Go / Go Code Review Comments）：

- 命名：导出标识符大写开头的驼峰命名（如 `SubmitOrder`），接口名采用 `Notifier`
  这类 "动作+er" 形式；
- 注释：所有导出的包、类型、方法均有以标识符开头的文档注释；
- 格式：全部代码经 `gofmt` 格式化，通过 `go vet` 静态检查；
- 错误处理：预定义哨兵错误（如 `ErrDuplicateOrder`），调用方用 `errors.Is` 判断。

## 与设计的对应关系

| 设计文档（实验 5） | 代码实现 |
| ------------------ | -------- |
| 类图：BaseModel 继承 | `model.BaseModel` 结构体嵌入 |
| 类图：OrderService 九个方法 | `service/order.go` 九个公开方法 |
| 类图：OrderService → MsgService 依赖 | `Notifier` 接口注入 |
| 时序图：维修闭环四阶段 | `main.go` 演示流程 |
| 数据表：user / repair_order / repair_record / evaluation / message | `model` 包五个实体 |

> 说明：为聚焦模块逻辑并保证独立可运行，本模块使用内存 map 存储数据；
> 接入 MySQL（GORM）在后续迭代中实现，接口保持不变。
