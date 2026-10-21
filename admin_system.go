package wbsdk

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SystemService 封装一键更新与版本信息（/api/system/*）。
type SystemService struct{ c *Client }

// System 返回系统服务。
func (c *Client) System() *SystemService { return &SystemService{c: c} }

// 更新目标枚举。
const (
	UpdateTargetManager  = "manager"
	UpdateTargetUpstream = "upstream"
	UpdateTargetBoth     = "both"
)

// Sysinfo 返回服务信息（版本、上游地址等）。需要登录。
func (s *SystemService) Sysinfo(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/sysinfo", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateStatus 读取更新进度与当前版本（含运行中的日志）。
func (s *SystemService) UpdateStatus(ctx context.Context) (*UpdateStatus, error) {
	var out UpdateStatus
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/system/update-status", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckUpdate 检测是否有新版本；force=true 绕过 6 小时缓存。
func (s *SystemService) CheckUpdate(ctx context.Context, force bool) (*UpdateCheck, error) {
	q := url.Values{}
	q.Set("force", strconv.FormatBool(force))
	var out UpdateCheck
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/system/check-update", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Versions 读取当前部署版本。
func (s *SystemService) Versions(ctx context.Context) (*Versions, error) {
	var out Versions
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/system/versions", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Changelog 读取更新日志（解析仓库根目录的 CHANGELOG.md，离线可用）。
func (s *SystemService) Changelog(ctx context.Context) (*Changelog, error) {
	var out Changelog
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/system/changelog", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update 启动一键更新。target 取 UpdateTargetManager / UpdateTargetUpstream / UpdateTargetBoth。
func (s *SystemService) Update(ctx context.Context, target string) (*ActionResult, error) {
	body := map[string]string{"target": target}
	var out ActionResult
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/system/update", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetUpstreamRef 固定上游版本（ref 为空串表示取消固定、恢复跟随分支）。
func (s *SystemService) SetUpstreamRef(ctx context.Context, ref string) (map[string]any, error) {
	body := map[string]string{"ref": ref}
	var out map[string]any
	if _, err := s.c.doJSON(ctx, http.MethodPost, "/api/system/upstream-ref", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Package 读取已上传的离线升级包元信息（Present=false 表示记录还在但文件被清掉了）。
func (s *SystemService) Package(ctx context.Context) (*UpdatePackage, error) {
	var out UpdatePackage
	if _, err := s.c.doJSON(ctx, http.MethodGet, "/api/system/update-package", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadPackage 上传离线升级包（发布页下载的那个 .tar.gz）。
//
// r 会被**流式**读取，不必先读进内存；filename 只用于显示。expectedSHA256
// 为空表示只做结构校验（服务端会解开确认是 tar.gz 且带 .version）。
//
// 注意：上传接口不参与重试、也不会自动重登（请求体是一次性的），会话过期时
// 会直接返回 401。
func (s *SystemService) UploadPackage(ctx context.Context, filename string, r io.Reader, expectedSHA256 string) (*UpdatePackage, error) {
	if filename == "" {
		filename = "update-package.tar.gz"
	}
	fields := map[string]string{}
	if want := strings.TrimSpace(expectedSHA256); want != "" {
		fields["expected_sha256"] = want
	}
	var out UpdatePackage
	if _, err := s.c.doUpload(ctx, "/api/system/update-package", "file", filename, r, fields, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadPackageFile 从磁盘文件上传离线升级包（UploadPackage 的便捷包装）。
func (s *SystemService) UploadPackageFile(ctx context.Context, path, expectedSHA256 string) (*UpdatePackage, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("wbsdk: 打开升级包: %w", err)
	}
	defer fh.Close()
	return s.UploadPackage(ctx, filepath.Base(path), fh, expectedSHA256)
}

// DeletePackage 删掉已上传的升级包（更新完成后清理）。
func (s *SystemService) DeletePackage(ctx context.Context) error {
	_, err := s.c.doJSON(ctx, http.MethodDelete, "/api/system/update-package", nil, nil, nil)
	return err
}
