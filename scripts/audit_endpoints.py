#!/usr/bin/env python3
"""核对本 SDK 是否跟上了 workbuddy-manager 的**最新端点**。

用法：
    python scripts/audit_endpoints.py /path/to/workbuddy-manager

退出码：0 = 全部覆盖（或只剩已声明的别名路径）；1 = 有未覆盖端点（可用于 CI / 发版检查）。

## 为什么按「路径段」比而不是字符串前缀

这个脚本被写坏过两次，教训留在注释里，免得下次再犯：

  * 只做 `lit.startswith('/api')` —— 会把 SDK 里**路由前缀分类**用的常量
    `"/api"`（`transport.go` 的 `isAdminPath`）当成端点，于是 `/api/*` 全部假阳性；
  * 放宽正则后又收进裸 `"/"`，`target.startswith("/")` **恒真**，变成全部「已覆盖」。

所以：按段比较，并且把已知的「前缀匹配器」字面量排除。
"""
from __future__ import annotations

import argparse
import pathlib
import re
import sys

# 只用于「路径前缀分类」的字面量，不是请求目标——必须排除，否则全是假阳性
NOISE = {'/', '/api', '/v1', '/v2', '/responses'}

# 已确认只做别名、且明确用逃生舱可达的端点（见 README）。列在这里是为了让
# 「未覆盖」列表只剩真正需要处理的东西。
KNOWN_ALIASES = {'/v2/chat/completions', '/responses'}


def segs(path: str) -> list[str]:
    return [s for s in path.strip('/').split('/') if s]


def ep_segs(path: str) -> list[str]:
    """端点路径的**静态**段（遇到路由参数就停）。"""
    out: list[str] = []
    for s in path.strip('/').split('/'):
        if s.startswith('{') or s == '%s':
            break
        out.append(s)
    return out


def collect_endpoints(upstream_root: pathlib.Path) -> list[tuple[str, str, str]]:
    routers = upstream_root / 'server' / 'routers'
    eps: list[tuple[str, str, str]] = []
    for f in sorted(routers.glob('*.py')):
        src = f.read_text(encoding='utf-8')
        m = re.search(r"APIRouter\(\s*prefix\s*=\s*['\"]([^'\"]*)['\"]", src)
        prefix = m.group(1) if m else ''
        for mm in re.finditer(
            r"@router\.(get|post|put|patch|delete)\(\s*['\"]([^'\"]*)['\"]", src
        ):
            eps.append((mm.group(1).upper(), prefix + mm.group(2), f.name))
    return eps


def collect_sdk_literals(sdk_root: pathlib.Path) -> list[list[str]]:
    lits: list[list[str]] = []
    for f in sorted(sdk_root.glob('*.go')):
        if f.name == 'doc.go' or f.name.endswith('_test.go'):
            continue
        for mm in re.finditer(r'"(/[^"]*)"', f.read_text(encoding='utf-8')):
            if mm.group(1) not in NOISE:
                lits.append(segs(mm.group(1)))
    return lits


def main() -> int:
    ap = argparse.ArgumentParser(description='核对 SDK 与上游端点的覆盖差异')
    ap.add_argument('upstream', help='workbuddy-manager 仓库根目录')
    args = ap.parse_args()

    upstream_root = pathlib.Path(args.upstream).resolve()
    if not (upstream_root / 'server' / 'routers').is_dir():
        print(f'找不到上游路由目录：{upstream_root / "server" / "routers"}', file=sys.stderr)
        return 2

    sdk_root = pathlib.Path(__file__).resolve().parent.parent
    eps = collect_endpoints(upstream_root)
    lits = collect_sdk_literals(sdk_root)

    missed: list[tuple[str, str, str]] = []
    for method, path, fname in eps:
        want = ep_segs(path)
        if not any(ls and ls == want[:len(ls)] for ls in lits):
            missed.append((method, path, fname))

    print(f'上游端点 {len(eps)}，SDK 字面量（按段，去噪）{len(lits)}，未覆盖 {len(missed)}')

    unexpected = []
    for method, path, fname in missed:
        tag = '（已声明的别名，逃生舱可达）' if path in KNOWN_ALIASES else '  ← 需要处理'
        if path not in KNOWN_ALIASES:
            unexpected.append(path)
        print(f'  {method:6} {path:44} ({fname}){tag}')

    return 1 if unexpected else 0


if __name__ == '__main__':
    raise SystemExit(main())
