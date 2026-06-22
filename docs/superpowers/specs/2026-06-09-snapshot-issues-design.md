# 设计:把 annotation(问题)数据嵌入 snapshot

日期:2026-06-09
分支:fix/pcie-tree-device-comma-split(实现时另开分支)
状态:已批准设计,待写实现计划

## 背景与目标

sichek 的 checker 发现问题后,会把「问题摘要」写进 K8s 节点的 `scitix.ai/sichek`
annotation。该 annotation 是一个 `nodeAnnotation` 对象:按组件(item)→ level
(Fatal/Critical/Warning)分组,每条问题是 `{error_name, device}`。

同时,daemon 还会把每个组件的原始采集数据(`LastInfo()`)写进本地
`snapshot.json`(`service/snapshot.go`),并由 Reporter 周期性 POST 给
`sichek-collector`。但 snapshot **目前只有原始状态,没有检测到的问题**。

**目标**:把 annotation 的问题数据也放进 snapshot,核心用途是**上报给 collector**,
让集中分析无需再读 K8s annotation。要求:

1. snapshot 顶层单独一个 `issues` 字段,内容就是完整的 `nodeAnnotation` 对象,
   格式与写给 K8s 的那份一致,例如:
   ```json
   "issues": {"podlog":{},"syslog":{},"hang":{},"nvidia":{},"infiniband":{},
              "ethernet":{},"gpfs":{},"cpu":{},"memory":null,"dmesg":{}}
   ```
2. 语义与 K8s annotation 一致:跨多次检查累加、去重,`HealthCheckTimeout` 走 append
   不清旧问题。
3. 非 K8s 环境(notifier 为 nil)也要能算出同样的累加结果并写进 snapshot。
4. **安全第一**:尽量不改动现有 K8s annotation 写入主路径,避免回归。

## 现状梳理

`service/daemon.go` 的 `monitorComponent`(每组件一个 goroutine)对每个
`*common.Result`:

1. 写 K8s annotation:`HealthCheckTimeout && abnormal` 走
   `notifier.AppendNodeAnnotation`,否则 `notifier.SetNodeAnnotation`。
2. 导出 Prometheus 指标 `d.metrics.ExportMetrics(result)`。
3. `d.snapshotMgr.Update(componentName, info)`(原始 info)。

annotation 的**累加状态保存在 K8s**:`notifier` 每周期重读
`node.Annotations[key]` → `GetAnnotationFromJson` → `ParseFromResult`/
`AppendFromResult` 累加 → marshal → 写回。所以无 K8s 时 notifier 为 nil,问题数据
不落盘到任何地方。

`componentName`(daemon map key)恒为 `consts.ComponentName*`,组件也用同一常量设
`result.Item`,二者一致(已核实)。`SetNodeAnnotation`/`AppendNodeAnnotation` 仅
被 `monitorComponent` 调用(已核实),改签名影响面可控。

## 方案 A(已选定):互斥双路径,主路径零改动

核心思路:**on-K8s 复用 notifier 已算好的 annotation;off-K8s 用进程内兜底累加器。
两条路径互斥,永不并行。**

```
result ─┬─ 有 notifier(on-K8s)─► notifier.Set/AppendNodeAnnotation
        │                          (原行为:读 K8s+累加+写 K8s,不变)
        │                          └─ 返回它刚算好的 *nodeAnnotation
        │
        └─ 无 notifier(off-K8s)─► AnnotationStore.Apply(result)
                                    (进程内累加,返回深拷贝)
                                            │
                两条路径都把 *nodeAnnotation ─┴─► snapshotMgr.SetIssues() ─► snapshot.json 顶层 issues
```

为什么安全/一致:
- on-K8s:notifier 行为完全不变(仅多一个返回值);snapshot 嵌入的就是写给 K8s 的
  **同一个对象**,连运行期外部手动编辑也会一并反映 —— 零分歧。
- off-K8s:兜底累加器**仅在 notifier 为 nil 时启用**,与 notifier 互斥 → 不会出现两套
  累加器并行、重复计算或重复 metric 导出。
- snapshot 是旁路:即便出错也不影响 K8s annotation 主路径。

## 改动点

### 1. `service/notifier.go` —— 仅加返回值(加性,不改行为)

接口与实现签名改为:
```go
SetNodeAnnotation(ctx context.Context, data *common.Result) (*nodeAnnotation, error)
AppendNodeAnnotation(ctx context.Context, data *common.Result) (*nodeAnnotation, error)
```
实现:在原逻辑末尾把已构建的 `anno`(`GetAnnotationFromJson` + `ParseFromResult`/
`AppendFromResult` 后的对象)随 `err` 一起返回。K8s 读/累加/写流程一字不改。
出错路径(如不支持 item 的 append)同样返回 `anno`(此时为未变更的当前状态)+ err。

注:`anno` 每周期由 `GetAnnotationFromJson` 新建,返回后不再被改写,daemon 可直接持有
该指针(无并发改写),无需深拷贝。

### 2. `service/annotation_store.go`(新增)—— off-K8s 兜底累加器

```go
type AnnotationStore struct {
    mu   sync.Mutex
    anno *nodeAnnotation
}

func NewAnnotationStore() *AnnotationStore   // 空起步(off-K8s 无种子来源)

// Apply 按与 monitorComponent 相同的规则选 Set/Append,复用 nodeAnnotation 现有方法,
// 返回当前累加状态的深拷贝(marshal→unmarshal),避免与 persist 的并发读产生 data race。
func (s *AnnotationStore) Apply(result *common.Result) (*nodeAnnotation, error)
```

Set/Append 判定(与现 `monitorComponent` 完全一致):
```go
len(result.Checkers) > 0 &&
  strings.Contains(result.Checkers[0].Name, "HealthCheckTimeout") &&
  result.Status == consts.StatusAbnormal   // → AppendFromResult,否则 ParseFromResult
```
多个组件 goroutine 共享该 store(off-K8s 时),由 `mu` 串行化,累加到同一个
`nodeAnnotation`,与 K8s annotation 的跨组件累加语义一致。

### 3. `service/snapshot.go` —— 加顶层 `issues`

```go
type Snapshot struct {
    Node       string                 `json:"node"`
    MgmtIP     string                 `json:"mgmt_ip,omitempty"`
    Timestamp  time.Time              `json:"timestamp"`
    Components map[string]interface{} `json:"components"`
    Issues     *nodeAnnotation        `json:"issues"`   // 新增
}

// SetIssues 加锁设置 data.Issues 并持久化。Update(componentName, info) 签名不变。
func (s *SnapshotManager) SetIssues(issues *nodeAnnotation)
```
`Components` 完全不动 → collector 的 `components` schema 无变化。
`Issues` 序列化即所需格式(空 map→`{}`,从未设置过的字段如 memory→`null`)。

### 4. `service/daemon.go` —— 接线

- `DaemonService` 增加 `annoStore *AnnotationStore`。
- `NewService`:`annoStore = NewAnnotationStore()`(仅 off-K8s 实际使用,创建开销可忽略)。
- `monitorComponent` 改为:
  ```go
  if result != nil {
      result.Node = d.node
      var anno *nodeAnnotation
      if d.notifier != nil {
          if isHealthCheckTimeout(result) {
              anno, err = d.notifier.AppendNodeAnnotation(d.ctx, result)
          } else {
              anno, err = d.notifier.SetNodeAnnotation(d.ctx, result)
          }
      } else if d.annoStore != nil {
          anno, err = d.annoStore.Apply(result)
      }
      if d.snapshotMgr != nil && anno != nil {
          d.snapshotMgr.SetIssues(anno)
      }
      d.metrics.ExportMetrics(result)
  }
  // 既有:组件原始 info 仍单独写入(签名不变)
  if d.snapshotMgr != nil {
      info, err := d.components[componentName].LastInfo()
      ... // 原有逻辑不变
      d.snapshotMgr.Update(componentName, info)
  }
  ```
  把现有内联的 timeout 判定抽成 `isHealthCheckTimeout(result) bool` 小函数,供两处复用。

## 错误处理

- off-K8s 不支持的 item 走 append(如 transceiver/lldp/pcie 的 HealthCheckTimeout):
  `AppendFromResult` 返回 err,`anno` 为未变更状态;daemon 照常记录 err,`anno != nil`
  时仍 `SetIssues`(反映上次良好状态)。与现状无回归。
- `metrics.ExportAnnotationMetrics` 仍由 `updateAnnotations` 内部触发:on-K8s 经
  notifier 路径(同今天);off-K8s 经 store 路径 —— 这是**新增**行为(以前 off-K8s 不导
  出该指标),属增益,无害。两路径互斥,不会重复导出。

## 测试

- 新增 `service/annotation_store_test.go`:
  - Set 覆盖单个 item;Append 累加且去重;跨组件累加;
  - `HealthCheckTimeout` 走 append 不清旧;
  - 不支持 item 的处理(返回 err,anno 不变);
  - 返回值是深拷贝(改返回值不影响内部状态)。
- 更新 `service/snapshot_test.go`:
  - 现有 `Components` 断言保持;
  - 新增:`SetIssues` 后 `snapshot.json` 顶层 `issues` 被正确填充且结构符合 `nodeAnnotation`。
- 可选:一个表驱动测试,对同一序列 result 断言 store 累加结果与逐次 `nodeAnnotation`
  累加(模拟 K8s 路径)一致。

## 注意点 / 取舍

1. **K8s 写路径**:本方案**不改其行为**(仅给两个方法加返回值),外部对 annotation 的
   合并行为保持原样 —— 这是相对早期 Approach 1(重写 notifier)更安全的关键。
2. **collector schema**:`components` 不变;只是 snapshot 顶层多了 `issues`。collector 端
   按需消费即可,旧字段不受影响。
3. off-K8s 兜底累加器无种子,daemon 重启后从空开始累加;但检查会很快重新填充。on-K8s
   不受影响(notifier 始终从 K8s 重读)。

## 涉及文件

- `service/notifier.go`(改:接口+两个方法返回值)
- `service/annotation_store.go`(新增)
- `service/snapshot.go`(改:`Snapshot.Issues` + `SetIssues`)
- `service/daemon.go`(改:`annoStore` 字段 + `monitorComponent` 接线 + `isHealthCheckTimeout` 助手)
- `service/annotation_store_test.go`(新增)
- `service/snapshot_test.go`(改)
