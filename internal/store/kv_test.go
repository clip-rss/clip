package store

import (
	"strings"
	"testing"
)

type kvProbe struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestJSONSettingRoundTrip(t *testing.T) {
	st := setupTestDB(t)

	var got kvProbe
	found, err := st.GetJSONSetting("probe", &got)
	if err != nil {
		t.Fatalf("GetJSONSetting (missing): %v", err)
	}
	if found {
		t.Fatal("missing key reported as found")
	}
	if got != (kvProbe{}) {
		t.Errorf("out mutated on missing key: %+v", got)
	}

	want := kvProbe{Name: "alpha", Count: 3}
	if err := st.SetJSONSetting("probe", want); err != nil {
		t.Fatalf("SetJSONSetting: %v", err)
	}
	found, err = st.GetJSONSetting("probe", &got)
	if err != nil {
		t.Fatalf("GetJSONSetting: %v", err)
	}
	if !found {
		t.Fatal("stored key reported as missing")
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}

	// 覆盖写入：同键再写应替换而非追加。
	want.Count = 9
	if err := st.SetJSONSetting("probe", want); err != nil {
		t.Fatalf("SetJSONSetting (overwrite): %v", err)
	}
	got = kvProbe{}
	if _, err := st.GetJSONSetting("probe", &got); err != nil {
		t.Fatalf("GetJSONSetting after overwrite: %v", err)
	}
	if got != want {
		t.Errorf("after overwrite = %+v, want %+v", got, want)
	}
}

// TestJSONSettingDecodeFailureIsError 值损坏时必须报错，不能当成「键不存在」。
// 静默退化会让调用方拿着零值继续跑，覆盖掉本该保留的数据。
func TestJSONSettingDecodeFailureIsError(t *testing.T) {
	st := setupTestDB(t)

	if _, err := st.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)`, "probe", `{not json`,
	); err != nil {
		t.Fatal(err)
	}

	var got kvProbe
	found, err := st.GetJSONSetting("probe", &got)
	if err == nil {
		t.Fatal("corrupt value should return an error")
	}
	if found {
		t.Error("found should be false when decoding fails")
	}
	if !strings.Contains(err.Error(), "probe") {
		t.Errorf("error should name the key, got %v", err)
	}
}

// TestJSONSettingIsolatedFromAppSettings 通用键值读写不得干扰 key='app' 的全局设置。
func TestJSONSettingIsolatedFromAppSettings(t *testing.T) {
	st := setupTestDB(t)

	before, err := st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if err := st.SetJSONSetting("probe", kvProbe{Name: "x"}); err != nil {
		t.Fatalf("SetJSONSetting: %v", err)
	}
	after, err := st.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after kv write: %v", err)
	}
	if before != after {
		t.Errorf("app settings changed by kv write:\nbefore %+v\nafter  %+v", before, after)
	}
}

func TestDeleteSettingIsIdempotent(t *testing.T) {
	st := setupTestDB(t)

	// 键不存在时删除不报错。
	if err := st.DeleteSetting("probe"); err != nil {
		t.Fatalf("DeleteSetting (missing): %v", err)
	}
	if err := st.SetJSONSetting("probe", kvProbe{Name: "y"}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSetting("probe"); err != nil {
		t.Fatalf("DeleteSetting: %v", err)
	}
	var got kvProbe
	found, err := st.GetJSONSetting("probe", &got)
	if err != nil {
		t.Fatalf("GetJSONSetting after delete: %v", err)
	}
	if found {
		t.Error("key still present after delete")
	}
}
