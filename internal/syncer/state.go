package syncer

import "time"

// stateKey 同步状态在 settings 表中的键名。
const stateKey = "sync_state"

// StateStore 同步状态的持久化能力，由 *store.Store 实现。
type StateStore interface {
	GetJSONSetting(key string, out any) (bool, error)
	SetJSONSetting(key string, value any) error
}

// State 持久化的同步状态。
//
// 两个基线哈希是同步判定的核心，含义必须分清：
//   - RemoteHash：最近一次同步时**远端载荷里**的配置哈希
//   - LocalHash：最近一次同步后**本机**的配置哈希
//
// 二者不能合并成一个字段。拉取时若远端含本端不认识的取值，Apply 会把该字段
// 回落成本机原值 —— 应用后的本机配置与远端载荷并不逐字节相等，两个哈希天然不同。
// 只存一个的话，每次同步都会把这种差异误判成「有一侧改过」，在两台版本不同的
// 机器间来回打转（表现为反复推送 / 拉取同一份配置）。
type State struct {
	RemoteETag     string    `json:"remoteETag"`     // 已归一化，供 If-Match 用
	RemoteHash     string    `json:"remoteHash"`     // 见上方说明
	RemoteRevision int       `json:"remoteRevision"` // 最近见到的远端版本号
	LocalHash      string    `json:"localHash"`      // 见上方说明
	LastSyncAt     time.Time `json:"lastSyncAt"`     // 零值表示从未同步成功
	LastError      string    `json:"lastError"`      // 上次失败原因，成功时清空

	// Conflict 非空表示有待用户裁决的冲突。
	//
	// 持久化它不是为了正确性（重启后再同步一次仍会检出同样的冲突），而是为了
	// 别让冲突在重启后变得无声无息：设置页据此仍能提示「有未解决的冲突」，
	// 而不是看起来像同步从没发生过。
	Conflict *ConflictInfo `json:"conflict"`
}

// ConflictInfo 冲突时远端一侧的来源信息，供弹窗向用户交代「远端是谁写的」。
type ConflictInfo struct {
	RemoteDeviceName string    `json:"remoteDeviceName"`
	RemoteUpdatedAt  time.Time `json:"remoteUpdatedAt"`
	RemoteRevision   int       `json:"remoteRevision"`
	DetectedAt       time.Time `json:"detectedAt"`
}

// Status 对外暴露的同步状态（供设置页展示）。
type Status struct {
	// LastSyncAt 上次同步成功时间；从未成功时为 null。
	// 用指针而非零值时间：让前端不必判别 "0001-01-01" 这种魔法值。
	LastSyncAt *time.Time    `json:"lastSyncAt"`
	LastError  string        `json:"lastError"`
	HasPending bool          `json:"hasPending"` // 本地有改动尚未推送
	Conflict   *ConflictInfo `json:"conflict"`
	DeviceName string        `json:"deviceName"` // 本机名，与远端一侧对照展示
}

// loadState 读取同步状态；从未写入过时返回零值。
func loadState(s StateStore) (State, error) {
	var st State
	if _, err := s.GetJSONSetting(stateKey, &st); err != nil {
		return State{}, err
	}
	return st, nil
}

// saveState 持久化同步状态。
func saveState(s StateStore, st State) error {
	return s.SetJSONSetting(stateKey, st)
}
