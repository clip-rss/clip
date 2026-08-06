package api

import (
	"reflect"
	"testing"

	"github.com/clip-rss/clip/internal/cloudbackup"
	"github.com/clip-rss/clip/internal/syncer"
)

func TestCloudBackupConfigRequiresSavedCredentials(t *testing.T) {
	syncSvc, _, st := newSyncService(t)
	svc := NewCloudBackupService(st, syncSvc)

	err := svc.SaveCloudBackupConfig(cloudbackup.Config{
		Enabled: true, Interval: cloudbackup.IntervalDaily, Retention: 5,
	})
	if err == nil {
		t.Fatal("enabling cloud backup without credentials should fail")
	}

	// 配置同步可以关闭，数据库云备份仍可独立使用同一份已保存凭据。
	if err := syncSvc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled:  false,
		URL:      "https://dav.example.com/dav/",
		Username: "alice",
		Password: "pw",
	}); err != nil {
		t.Fatalf("SaveWebDAVConfig: %v", err)
	}
	if err := svc.SaveCloudBackupConfig(cloudbackup.Config{
		Enabled: true, Interval: cloudbackup.IntervalWeekly, Retention: 3,
	}); err != nil {
		t.Fatalf("SaveCloudBackupConfig: %v", err)
	}
	cfg, err := svc.GetCloudBackupConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.Interval != cloudbackup.IntervalWeekly || cfg.Retention != 3 {
		t.Errorf("config = %+v", cfg)
	}
}

func TestCloudBackupLifecycleArmsAndStopsTimer(t *testing.T) {
	syncSvc, _, st := newSyncService(t)
	if err := syncSvc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled: false, URL: "https://dav.example.com/dav/", Username: "alice", Password: "pw",
	}); err != nil {
		t.Fatal(err)
	}
	svc := NewCloudBackupService(st, syncSvc)
	if err := svc.SaveCloudBackupConfig(cloudbackup.Config{
		Enabled: true, Interval: cloudbackup.IntervalDaily, Retention: 5,
	}); err != nil {
		t.Fatal(err)
	}

	StartCloudBackup(svc)
	svc.mu.Lock()
	armed := svc.timer != nil && svc.started
	svc.mu.Unlock()
	if !armed {
		t.Fatal("enabled cloud backup did not arm timer")
	}

	StopCloudBackup(svc)
	svc.mu.Lock()
	stopped := svc.timer == nil && !svc.started
	svc.mu.Unlock()
	if !stopped {
		t.Fatal("StopCloudBackup did not disarm timer")
	}
}

func TestCloudBackupCanRearmAfterDisablingMidSession(t *testing.T) {
	syncSvc, _, st := newSyncService(t)
	if err := syncSvc.SaveWebDAVConfig(syncer.WebDAVInput{
		Enabled: false, URL: "https://dav.example.com/dav/", Username: "alice", Password: "pw",
	}); err != nil {
		t.Fatal(err)
	}
	svc := NewCloudBackupService(st, syncSvc)
	StartCloudBackup(svc)
	defer StopCloudBackup(svc)

	if err := svc.SaveCloudBackupConfig(cloudbackup.Config{
		Enabled: true, Interval: cloudbackup.IntervalDaily, Retention: 5,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveCloudBackupConfig(cloudbackup.Config{
		Enabled: false, Interval: cloudbackup.IntervalDaily, Retention: 5,
	}); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	disarmedButStarted := svc.timer == nil && svc.started
	svc.mu.Unlock()
	if !disarmedButStarted {
		t.Fatal("disabling automatic backup stopped the service lifecycle")
	}

	if err := svc.SaveCloudBackupConfig(cloudbackup.Config{
		Enabled: true, Interval: cloudbackup.IntervalWeekly, Retention: 3,
	}); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	rearmed := svc.timer != nil && svc.started
	svc.mu.Unlock()
	if !rearmed {
		t.Fatal("automatic backup did not rearm after re-enabling")
	}
}

func TestCloudBackupLifecycleMethodsAreNotBindable(t *testing.T) {
	typ := reflect.TypeOf(&CloudBackupService{})
	for _, name := range []string{"Start", "Stop", "Arm", "AutoBackup"} {
		if _, ok := typ.MethodByName(name); ok {
			t.Errorf("CloudBackupService.%s must not be exported to Wails", name)
		}
	}
}

func TestCloudBackupRemotePath(t *testing.T) {
	syncSvc, _, st := newSyncService(t)
	svc := NewCloudBackupService(st, syncSvc)
	if got := svc.CloudBackupRemotePath(); got != cloudbackup.RemoteDir() {
		t.Errorf("path = %q, want %q", got, cloudbackup.RemoteDir())
	}
}
